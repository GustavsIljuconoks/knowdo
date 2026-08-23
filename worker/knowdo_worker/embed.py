"""Local embeddings via fastembed.

fastembed rather than sentence-transformers: it is ONNX based, so it
does not pull PyTorch into the image. The model is baked into the
Docker image at build time, so the first job does not wait on a
download.
"""

import os
from functools import lru_cache

from fastembed import TextEmbedding

DEFAULT_MODEL = "BAAI/bge-small-en-v1.5"

@lru_cache(maxsize=1)
def _load_model() -> TextEmbedding:
    return TextEmbedding(model_name=os.getenv("EMBED_MODEL") or DEFAULT_MODEL)


def embed_texts(texts: list[str]) -> list[list[float]]:
    """Embed each text, preserving order.
    Load the model once and cache it; constructing it per call would re-read the weights every job.
    """

    return [vector.tolist() for vector in _load_model().embed(texts)]
