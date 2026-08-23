"""The ingest pipeline: extract -> chunk -> embed -> upsert."""

from . import db as db_module
from . import vectors
from .chunk import chunk_text
from .embed import embed_texts
from .extract import extract_text


def ingest(
    document_id: int,
    *,
    embed=embed_texts,
    store=vectors.upsert_chunks,
    db=db_module,
) -> int:
    """Process one uploaded document and return the number of chunks stored.

        load row -> status='processing' -> extract -> chunk -> embed
          -> upsert vectors -> status='ready', chunk_count=N

    Any failure sets status='failed' with the message in `error` and
    re-raises, so the consumer can log it. A missing row is not an error
    worth raising — there is nothing left to mark failed.

    `embed`, `store` and `db` are injected so tests can run the whole
    pipeline without a model download, a Qdrant, or a Postgres.

    no retries and no dead-letter queue — re-upload to retry.
    """

    with db.connect() as conn:
        document = db.load_document(conn, document_id)

        # The row is gone — deleted between enqueue and now.
        # There is nothing left to mark failed, so this is a no-op.
        if document is None:
            return 0

        content, content_type = document

        # ensure if user pools file has changed status
        db.set_status(conn, document_id, "processing")

        try:
            text = extract_text(content, content_type)
            chunks = chunk_text(text)

            # an empty document is a successful ingest of nothing. skip
            # the round trip rather than upserting an empty batch.
            if chunks:
                store(document_id, chunks, embed(chunks))
        except Exception as exc:
            db.set_status(conn, document_id, "failed", error=str(exc) or type(exc).__name__)
            raise

        db.set_status(conn, document_id, "ready", chunk_count=len(chunks))

        return len(chunks)
