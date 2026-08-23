"""Qdrant collection holding chunk vectors."""

import os
from functools import lru_cache

from qdrant_client import QdrantClient, models

COLLECTION = "knowdo"

# BAAI/bge-small-en-v1.5. Changing EMBED_MODEL to a model with a
# different width means recreating the collection — Qdrant will reject
# mismatched vectors rather than silently misbehave.
DIM = 384

# ponytail: chunk ids are derived, not stored, so re-upserting a
# document overwrites its own points instead of duplicating them.
# Ceiling: 100k chunks per document. Switch to UUIDs if that is ever hit.
CHUNKS_PER_DOCUMENT = 100_000


@lru_cache(maxsize=1)
def client() -> QdrantClient:
    """The shared client, with the collection created on first use."""
    c = QdrantClient(url=os.environ["QDRANT_URL"])

    if not c.collection_exists(COLLECTION):
        c.create_collection(
            COLLECTION,
            vectors_config=models.VectorParams(size=DIM, distance=models.Distance.COSINE),
        )

    return c


def upsert_chunks(document_id: int, chunks: list[str], vectors: list[list[float]]) -> None:
    """Store one point per chunk, carrying the text in its payload.

    The text lives in the payload so answering a question needs one
    round trip to Qdrant and none to Postgres.
    """
    points = [
        models.PointStruct(
            id=document_id * CHUNKS_PER_DOCUMENT + index,
            vector=vector,
            payload={"document_id": document_id, "chunk_index": index, "text": text},
        )
        for index, (text, vector) in enumerate(zip(chunks, vectors, strict=True))
    ]

    client().upsert(COLLECTION, points=points)


def search(vector: list[float], limit: int = 5) -> list[dict]:
    """Return the payloads of the nearest chunks, closest first."""
    hits = client().query_points(COLLECTION, query=vector, limit=limit).points

    return [hit.payload for hit in hits]
