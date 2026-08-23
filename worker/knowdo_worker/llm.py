"""Chat completions over any OpenAI-compatible provider.

One function, configured by LLM_BASE_URL, LLM_MODEL and LLM_API_KEY.
Kimi, DeepSeek, Groq, Together, OpenRouter, Ollama and vLLM all speak
this shape, so swapping providers is configuration rather than code —
no registry, no factory, no interface with one implementation.
"""

import os
from functools import lru_cache

from openai import OpenAI


@lru_cache(maxsize=1)
def _client() -> OpenAI:
    """One client for the process — it holds a connection pool worth reusing."""

    return OpenAI(
        base_url=os.environ["LLM_BASE_URL"],
        api_key=os.environ["LLM_API_KEY"],
    )


def chat(messages: list[dict]) -> str:
    """Send `messages` (OpenAI role/content dicts) and return the reply text."""

    response = _client().chat.completions.create(
        model=os.environ["LLM_MODEL"],
        messages=messages,
    )

    # `content` is Optional in the SDK's types — a refusal or a
    # tool-call-only reply has none. We ask for neither, so "" is the
    # honest floor rather than letting None reach the API response.
    return response.choices[0].message.content or ""
