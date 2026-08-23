from datetime import date

import pytest
from pydantic import ValidationError

from knowdo_worker.plan import plan


class FakeChat:
    def __init__(self, replies):
        # One reply per call, so a retry-after-invalid-JSON test can
        # queue a bad reply followed by a good one.
        self.replies = list(replies)
        self.calls = []

    def __call__(self, messages):
        self.calls.append(messages)

        return self.replies.pop(0)


GOOD_REPLY = (
    '{"goal": "learn kubernetes", '
    '"tasks": [{"title": "pods", "description": "learn pods", "day_offset": 2}]}'
)


def test_valid_reply_parses_with_due_date():
    result = plan("learn kubernetes", chat=FakeChat([GOOD_REPLY]), today=date(2026, 1, 1))

    assert result == {
        "goal": "learn kubernetes",
        "tasks": [
            {"title": "pods", "description": "learn pods", "due_date": "2026-01-03"},
        ],
    }


def test_today_reaches_the_prompt():
    chat = FakeChat([GOOD_REPLY])

    plan("learn kubernetes", chat=chat, today=date(2026, 1, 1))

    prompt = " ".join(m["content"] for m in chat.calls[0])
    assert "2026-01-01" in prompt


def test_invalid_json_retries_once_then_raises():
    chat = FakeChat(["not json", "still not json"])

    with pytest.raises(ValidationError):
        plan("learn kubernetes", chat=chat, today=date(2026, 1, 1))

    assert len(chat.calls) == 2


def test_invalid_json_then_valid_reply_recovers():
    chat = FakeChat(["not json", GOOD_REPLY])

    result = plan("learn kubernetes", chat=chat, today=date(2026, 1, 1))

    assert len(chat.calls) == 2
    assert result["goal"] == "learn kubernetes"


def test_empty_tasks_fails_validation():
    chat = FakeChat(['{"goal": "learn kubernetes", "tasks": []}'] * 2)

    with pytest.raises(ValidationError):
        plan("learn kubernetes", chat=chat, today=date(2026, 1, 1))
