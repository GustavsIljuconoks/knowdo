"""The two Postgres statements the worker needs.

The worker never touches `tasks` and never writes `documents.content` —
it reads bytes in and writes status out. Keeping that surface to two
functions makes it obvious the Go API stays the only writer of rows.
"""

import os

import psycopg


def connect() -> psycopg.Connection:
    """Open a connection from DATABASE_URL.

    ponytail: a fresh connection per job instead of a pool. The consumer
    is a single sequential loop, so a pool would only add a way for a
    connection to go stale — and scaling out means more worker replicas,
    each with its own connection, not threads. Pool only if the consumer
    grows an in-process thread pool, or if jobs get cheap enough that the
    per-job handshake stops being noise against the embedding cost.
    """
    url = os.environ["DATABASE_URL"]

    return psycopg.connect(url)


def load_document(conn: psycopg.Connection, document_id: int) -> tuple[bytes, str] | None:
    """Return (content, content_type), or None if the row is gone."""
    row = conn.execute(
        "SELECT content, content_type FROM documents WHERE id = %s",
        (document_id,),
    ).fetchone()

    if row is None:
        return None

    return bytes(row[0]), row[1]


def set_status(
    conn: psycopg.Connection,
    document_id: int,
    status: str,
    *,
    error: str = "",
    chunk_count: int = 0,
) -> None:
    """Move a document to pending|processing|ready|failed.

    `error` and `chunk_count` are always written so a retry of a
    previously failed document does not keep its stale error text.
    """
    conn.execute(
        "UPDATE documents SET status = %s, error = %s, chunk_count = %s WHERE id = %s",
        (status, error, chunk_count, document_id),
    )
    conn.commit()
