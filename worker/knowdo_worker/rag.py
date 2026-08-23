"""The ask pipeline: embed -> search -> prompt -> chat."""

from . import vectors
from .embed import embed_texts
from .llm import chat

SYSTEM_PROMPT = """You answer questions about the user's documents.

Answer only from the context provided in the user's message. If the
context does not contain the answer, say so plainly and do not guess —
an answer that is not grounded in the context is worse than no answer.
"""

NO_CONTEXT = "I don't have anything indexed that answers that."


def ask(
    question: str,
    *,
    embed=embed_texts,
    search=vectors.search,
    chat=chat,
    top_k: int = 5,
) -> dict:
    """Answer `question` from the retrieved chunks.

    Returns {"answer": str, "sources": [document_id, ...]} where
    `sources` lists the distinct documents the retrieved chunks came
    from, in the order they were first retrieved.

    ponytail: no conversation history and no reranking. Both belong to
    Stage 4, or to whenever retrieval quality proves insufficient.
    """

    # use the same model for embedding question
    vector = embed([question])[0]
    chunks = search(vector, top_k)

    if not chunks:
        return {"answer": NO_CONTEXT, "sources": []}

    context = "\n\n".join(chunk["text"] for chunk in chunks)

    # dict.fromkeys is the one-liner for "distinct, order preserved"
    sources = list(dict.fromkeys(chunk["document_id"] for chunk in chunks))

    answer = chat(
        [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": f"Context:\n{context}\n\nQuestion: {question}"},
        ]
    )

    return {"answer": answer, "sources": sources}
