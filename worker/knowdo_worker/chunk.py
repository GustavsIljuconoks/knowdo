"""Split extracted text into overlapping windows.

Backed by LangChain's RecursiveCharacterTextSplitter, which tries
paragraph, then line, then word, then character boundaries and uses the
coarsest one that fits. Chunks therefore start and end at readable
boundaries instead of mid-word — a chunk beginning "...ngs the vector"
embeds worse than one beginning at a sentence.

The dependency is a deliberate down payment on Stage 4, which brings
LangGraph and `langchain-core` in anyway.
"""

from langchain_text_splitters import RecursiveCharacterTextSplitter

DEFAULT_SIZE = 800
DEFAULT_OVERLAP = 100

def chunk_text(text: str, size: int = DEFAULT_SIZE, overlap: int = DEFAULT_OVERLAP) -> list[str]:
    """Split `text` into chunks of at most `size` characters, overlapping
    by roughly `overlap`.

    "Roughly" is the trade against the old fixed-window splitter: the
    splitter merges boundary-aligned pieces up to `size`, so overlap is
    honoured approximately rather than to the character.

    Text that is empty or only whitespace produces no chunks. An
    `overlap` greater than `size` raises ValueError — LangChain checks
    this when the splitter is constructed, so do not re-implement it.

    ponytail: still character-counted, not token-counted. If the
    embedding model starts truncating chunks, switch to
    `.from_huggingface_tokenizer()` — same class, same seam, no caller
    changes.
    """

    splitter = RecursiveCharacterTextSplitter(chunk_size=size, chunk_overlap=overlap)

    return splitter.split_text(text)
