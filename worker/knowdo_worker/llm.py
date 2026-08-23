"""Chat completions over any OpenAI-compatible provider.

Configured by LLM_BASE_URL, LLM_MODEL and LLM_API_KEY. Kimi, DeepSeek,
Groq, Together, OpenRouter, Ollama and vLLM all speak this shape, so
swapping providers is configuration rather than code — no registry, no
factory, no interface with one implementation.
"""

import os
from functools import lru_cache

from openai import OpenAI
from pydantic import BaseModel, ValidationError


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


def chat_json(messages: list[dict], schema: type[BaseModel], *, chat=chat) -> BaseModel:
    """Call `chat` and validate the reply against `schema`.

    On a validation error, retries once with the error appended to the
    conversation before giving up — a second failure raises.
    """

    answer = chat(messages)

    try:
        return schema.model_validate_json(answer)
    except ValidationError as e:
        messages.append({"role": "assistant", "content": answer})
        messages.append({"role": "user", "content": f"That was invalid: {e}. Retry, JSON only."})
        answer = chat(messages)
        return schema.model_validate_json(answer)  # let a second failure raise
