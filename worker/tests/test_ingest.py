import pytest

from knowdo_worker.ingest import ingest


class FakeConn:
    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False


class FakeDB:
    """Stands in for knowdo_worker.db — no Postgres involved."""

    def __init__(self, document=(b"one two three", "text/plain")):
        self.document = document
        self.statuses = []

    def connect(self):
        return FakeConn()

    def load_document(self, conn, document_id):
        return self.document

    def set_status(self, conn, document_id, status, *, error="", chunk_count=0):
        self.statuses.append((status, error, chunk_count))


def fake_embed(texts):
    return [[float(len(t))] * 3 for t in texts]


class FakeStore:
    def __init__(self):
        self.calls = []

    def __call__(self, document_id, chunks, vectors):
        self.calls.append((document_id, chunks, vectors))


def test_happy_path_stores_chunks_and_marks_ready():
    db = FakeDB()
    store = FakeStore()

    count = ingest(7, embed=fake_embed, store=store, db=db)

    assert count == 1
    assert db.statuses == [("processing", "", 0), ("ready", "", 1)]

    document_id, chunks, vectors = store.calls[0]
    assert document_id == 7
    assert chunks == ["one two three"]
    assert len(vectors) == len(chunks)


def test_extraction_failure_marks_failed_with_a_message():
    db = FakeDB(document=(b"\x89PNG\r\n\x1a\n\xff", "image/png"))
    store = FakeStore()

    with pytest.raises(ValueError):
        ingest(7, embed=fake_embed, store=store, db=db)

    assert db.statuses[0] == ("processing", "", 0)

    status, error, chunk_count = db.statuses[-1]
    assert status == "failed"
    assert error != ""
    assert chunk_count == 0
    assert store.calls == []


def test_missing_document_is_a_no_op():
    db = FakeDB(document=None)
    store = FakeStore()

    assert ingest(7, embed=fake_embed, store=store, db=db) == 0
    assert db.statuses == []
    assert store.calls == []


def test_document_with_no_extractable_text_is_ready_with_zero_chunks():
    db = FakeDB(document=(b"   \n  ", "text/plain"))
    store = FakeStore()

    assert ingest(7, embed=fake_embed, store=store, db=db) == 0
    assert db.statuses[-1] == ("ready", "", 0)
