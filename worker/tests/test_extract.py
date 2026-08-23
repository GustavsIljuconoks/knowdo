import io

import pytest
from reportlab.pdfgen import canvas

from knowdo_worker.extract import extract_text


def make_pdf(text: str) -> bytes:
    """A one-page PDF, built here so no binary fixture is committed."""
    buffer = io.BytesIO()
    pdf = canvas.Canvas(buffer)
    pdf.drawString(72, 720, text)
    pdf.save()

    return buffer.getvalue()


def test_markdown_is_decoded_as_utf8():
    content = "# Title\n\nSome nötes.".encode()

    assert extract_text(content, "text/markdown") == "# Title\n\nSome nötes."


def test_plain_text_without_a_content_type():
    assert extract_text(b"just text", "") == "just text"


def test_pdf_text_is_extracted():
    text = extract_text(make_pdf("hello from a pdf"), "application/pdf")

    assert "hello from a pdf" in text


def test_pdf_is_detected_by_magic_bytes_when_content_type_lies():
    text = extract_text(make_pdf("sneaky pdf"), "application/octet-stream")

    assert "sneaky pdf" in text


def test_undecodable_content_raises():
    # A PNG header — not text, not a PDF.
    with pytest.raises(ValueError):
        extract_text(b"\x89PNG\r\n\x1a\n\x00\x01\x02\xff", "image/png")


def test_control_characters_are_stripped():
    # pypdf renders PDF bullet glyphs as DEL (\x7f). They carry no meaning
    # and must not reach the embedder as content.
    content = b"Key Resources\n\x7f  Engine\n\x00\x08Fine print\ttabbed"

    text = extract_text(content, "text/plain")

    assert text == "Key Resources\n  Engine\nFine print\ttabbed"


def test_newlines_and_tabs_survive_stripping():
    # \n carries the paragraph boundaries the chunker splits on, so the
    # strip must not touch it.
    assert extract_text(b"a\n\nb\tc", "text/plain") == "a\n\nb\tc"
