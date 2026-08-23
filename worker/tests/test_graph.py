from knowdo_worker.graph import build_graph


def fake_planner(request):
    return {"goal": request, "tasks": []}


def fake_answerer(request):
    return {"answer": f"answer to {request}", "sources": []}


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
