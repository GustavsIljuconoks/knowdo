CREATE TABLE documents (
    id           BIGSERIAL PRIMARY KEY,
    filename     TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    content      BYTEA NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    error        TEXT NOT NULL DEFAULT '',
    chunk_count  INT  NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
