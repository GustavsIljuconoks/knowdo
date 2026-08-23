"""Tests for the chunker.

`chunk_text` is a thin seam over LangChain's RecursiveCharacterTextSplitter,
so these assert the properties that splitter guarantees — chunks bounded by
`size`, broken on the largest separator that fits, overlapping, and covering
the input — rather than exact character offsets. A splitter that respects
word and paragraph boundaries cannot also promise `chunks[1] == text[80:180]`:
that promise is exactly what the fixed-window version bought, and what we
traded away for boundaries that do not cut words in half.
"""

import pytest

from knowdo_worker.chunk import chunk_text

SIZE = 100
OVERLAP = 20


def words_text(count: int = 200) -> str:
    return " ".join(f"word{i}" for i in range(count))


def test_empty_text_yields_no_chunks():
    assert chunk_text("", size=SIZE, overlap=OVERLAP) == []


def test_whitespace_only_yields_no_chunks():
    assert chunk_text("   \n\t  ", size=SIZE, overlap=OVERLAP) == []


def test_short_text_is_one_chunk():
    assert chunk_text("hello world", size=SIZE, overlap=OVERLAP) == ["hello world"]


def test_no_chunk_exceeds_the_size():
    chunks = chunk_text(words_text(), size=SIZE, overlap=OVERLAP)

    assert len(chunks) > 1
    assert max(len(c) for c in chunks) <= SIZE


def test_chunks_break_on_word_boundaries():
    # The whole reason for the dependency: no chunk starts or ends
    # halfway through a word.
    text = words_text()
    vocabulary = set(text.split())

    chunks = chunk_text(text, size=SIZE, overlap=OVERLAP)

    assert all(word in vocabulary for chunk in chunks for word in chunk.split())
    assert all(chunk == chunk.strip() for chunk in chunks)


def test_chunks_prefer_paragraph_boundaries():
    # Paragraphs are packed together up to `size`, but a paragraph is
    # never cut in half while a coarser boundary still fits.
    paragraphs = ["Alpha " * 5, "Beta " * 5, "Gamma " * 5, "Delta " * 5]
    text = "\n\n".join(p.strip() for p in paragraphs)

    chunks = chunk_text(text, size=70, overlap=0)

    rejoined = [part for chunk in chunks for part in chunk.split("\n\n")]
    assert rejoined == [p.strip() for p in paragraphs]


def test_consecutive_chunks_overlap():
    chunks = chunk_text(words_text(), size=SIZE, overlap=OVERLAP)

    for earlier, later in zip(chunks, chunks[1:], strict=False):
        assert set(earlier.split()) & set(later.split())


def test_chunks_cover_the_whole_text_in_order():
    text = words_text()

    chunks = chunk_text(text, size=SIZE, overlap=OVERLAP)

    seen, ordered = set(), []
    for chunk in chunks:
        for word in chunk.split():
            if word not in seen:
                seen.add(word)
                ordered.append(word)

    assert ordered == text.split()


def test_text_without_separators_still_splits():
    # No spaces or newlines to break on — the splitter falls back to
    # character boundaries rather than emitting one oversized chunk.
    digits = "".join(str(i % 10) for i in range(250))

    chunks = chunk_text(digits, size=SIZE, overlap=OVERLAP)

    assert len(chunks) > 1
    assert max(len(c) for c in chunks) <= SIZE


def test_defaults_are_the_stage_three_constants():
    chunks = chunk_text(words_text(400))

    assert max(len(c) for c in chunks) <= 800


def test_overlap_larger_than_size_raises():
    # LangChain validates this itself; we do not re-implement the guard.
    with pytest.raises(ValueError):
        chunk_text("abc", size=10, overlap=50)
