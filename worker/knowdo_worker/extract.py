"""Turn uploaded bytes into plain text."""

import io

from pypdf import PdfReader

# C0 controls and DEL, minus the whitespace that carries meaning: \t, and
# \n, which is the paragraph boundary the chunker splits on. pypdf renders
# PDF bullet glyphs as DEL, so without this every bullet is embedded as
# content.
_CONTROL_CHARS = dict.fromkeys([c for c in range(32) if c not in (9, 10)] + [127])


def extract_text(content: bytes, content_type: str = "") -> str:
    """Extract text from a PDF, or decode anything else as UTF-8.

    PDFs are detected by `content_type` containing "pdf" or by the
    "%PDF-" magic bytes, since browsers do not always send a useful
    content type. Everything else (Markdown, plain text) is decoded as
    UTF-8.

    Content that is neither — an image, a zip — raises ValueError. That
    becomes a document with status 'failed' and a readable message, not
    a crashed worker.
    """

    if "pdf" in content_type.lower() or content.startswith(b"%PDF-"):
        reader = PdfReader(io.BytesIO(content))
        pages = "\n".join(page.extract_text() or "" for page in reader.pages)

        return pages.translate(_CONTROL_CHARS)

    try:
        return content.decode("utf-8").translate(_CONTROL_CHARS)
    except UnicodeDecodeError as exc:
        raise ValueError(
            f"not a PDF and not UTF-8 text (content type {content_type or 'unknown'})"
        ) from exc
