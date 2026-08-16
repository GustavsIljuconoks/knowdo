CREATE TABLE tasks (
    id          BIGSERIAL PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'open',
    due_date    DATE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
