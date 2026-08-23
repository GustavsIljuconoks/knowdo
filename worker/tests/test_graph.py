import pytest
from pydantic import ValidationError

from knowdo_worker.graph import build_graph, classify


def fake_planner(request):
    return {"goal": request, "tasks": []}


def fake_answerer(request):
    return {"answer": f"answer to {request}", "sources": []}


class FakeChat:
    def __init__(self, replies):
        # One reply per call, so a retry-after-invalid-JSON test can
        # queue a bad reply followed by a good one.
        self.replies = list(replies)
        self.calls = []

    def __call__(self, messages):
        self.calls.append(messages)

        return self.replies.pop(0)


def test_plan_route_calls_planner_not_answerer():
    graph = build_graph(
        planner=fake_planner,
        answerer=fake_answerer,
        classifier=lambda request: "plan",
    )

    result = graph.invoke({"request": "learn kubernetes", "route": None, "result": None})

    assert result["result"] == {"goal": "learn kubernetes", "tasks": []}


def test_answer_route_calls_answerer_not_planner():
    graph = build_graph(
        planner=fake_planner,
        answerer=fake_answerer,
        classifier=lambda request: "answer",
    )

    result = graph.invoke({"request": "what is knowdo?", "route": None, "result": None})

    assert result["result"] == {"answer": "answer to what is knowdo?", "sources": []}


def test_route_reflects_classifier_output():
    graph = build_graph(
        planner=fake_planner,
        answerer=fake_answerer,
        classifier=lambda request: "answer",
    )

    result = graph.invoke({"request": "anything", "route": None, "result": None})

    assert result["route"] == "answer"


def test_classify_plan_route():
    route = classify(
        "I need to learn ArgoCD this weekend",
        chat=FakeChat(['{"route": "plan"}']),
    )

    assert route == "plan"


def test_classify_answer_route():
    route = classify(
        "what do my notes say about Deployments?",
        chat=FakeChat(['{"route": "answer"}']),
    )

    assert route == "answer"


def test_classify_request_reaches_the_prompt():
    chat = FakeChat(['{"route": "plan"}'])

    classify("learn kubernetes before September", chat=chat)

    prompt = " ".join(m["content"] for m in chat.calls[0])
    assert "learn kubernetes before September" in prompt


def test_classify_invalid_json_retries_once_then_raises():
    chat = FakeChat(["not json", "still not json"])

    with pytest.raises(ValidationError):
        classify("anything", chat=chat)

    assert len(chat.calls) == 2


def test_classify_invalid_json_then_valid_reply_recovers():
    chat = FakeChat(["not json", '{"route": "answer"}'])

    route = classify("anything", chat=chat)

    assert route == "answer"
    assert len(chat.calls) == 2
