"""Turn a free-text goal into a structured task plan.

Design (locked, see knowdo_ai_task_knowledge_api.md Stage 4):

- Schema: {"goal": str, "tasks": [{"title": str, "description": str,
  "day_offset": int}]}, wrapped in an object (not a bare array) so
  provider JSON modes that require an object root still work.
- `day_offset` is relative to `today`, not an absolute date — the
  model reasons about pacing ("5 hours/week") more reliably in offsets
  than it does about calendar dates, and the actual date arithmetic
  belongs in this file, not in the prompt.
- `today` must be injected into the prompt explicitly; the model has
  no reliable notion of "now".
- Validate with Pydantic (`tasks` non-empty). On a validation error,
  retry once with the error appended to the prompt before giving up —
  same provider-neutral shape as llm.chat(), no provider-specific
  json_schema/tool-calling mode.

TODO(you): define the Pydantic schema, the system prompt, and the
parse-validate-retry loop below.
"""

from datetime import date, timedelta

from pydantic import BaseModel, Field, NonNegativeInt

from .llm import chat as default_chat
from .llm import chat_json

SYSTEM_PROMPT = """You plan user's tasks from a goal.

Emit only JSON matching this shape: {"goal": str, "tasks": [{"title": str,
"description": str, "day_offset": non-negative int}]}. day_offset is
relative to today, not an absolute date.

No markdown fences, JSON only.
"""

class GeneratedTask(BaseModel):
    title: str
    description: str
    day_offset: NonNegativeInt


class Plan(BaseModel):
    goal: str
    tasks: list[GeneratedTask] = Field(min_length=1)


def plan(request: str, *, chat=default_chat, today: date | None = None) -> dict:
    today = today or date.today()

    messages = [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "system", "content": f"Today is {today.isoformat()}."},
        {"role": "user", "content": request},
    ]

    result = chat_json(messages, Plan, chat=chat)

    return {
        "goal": result.goal,
        "tasks": [
            {
                "title": t.title,
                "description": t.description,
                "due_date": (today + timedelta(days=t.day_offset)).isoformat(),
            }
            for t in result.tasks
        ],
    }
