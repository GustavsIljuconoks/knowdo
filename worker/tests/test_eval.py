"""Evaluation: does the real LLM route and plan sensibly?

Hits the configured provider directly — a judgment call on prompt
quality, not a unit test. Excluded from the default `pytest` run (see
addopts in pyproject.toml); run explicitly:

    pytest -m eval

Requires LLM_BASE_URL, LLM_MODEL and LLM_API_KEY to be set (same as
running the service for real).
"""

from datetime import date, timedelta

import pytest

from knowdo_worker.graph import classify
from knowdo_worker.plan import plan

pytestmark = pytest.mark.eval

PLAN_REQUESTS = [
    "I need to learn ArgoCD this weekend",
    "help me learn Kubernetes networking over the next two weeks",
    "plan out learning Terraform basics, I have 5 hours a week",
]

ANSWER_REQUESTS = [
    "what do my notes say about Deployments?",
    "summarize what I have on Prometheus",
    "how does the RAG pipeline work according to my docs?",
]


@pytest.mark.parametrize("request_text", PLAN_REQUESTS)
def test_classify_routes_plan_requests_to_plan(request_text):
    assert classify(request_text) == "plan"


@pytest.mark.parametrize("request_text", ANSWER_REQUESTS)
def test_classify_routes_questions_to_answer(request_text):
    assert classify(request_text) == "answer"


@pytest.mark.parametrize("request_text", PLAN_REQUESTS)
def test_plan_produces_sane_tasks(request_text):
    today = date.today()

    result = plan(request_text, today=today)

    assert result["tasks"], "plan produced no tasks"

    for t in result["tasks"]:
        assert t["title"].strip(), "task has an empty title"

        due = date.fromisoformat(t["due_date"])
        assert today <= due <= today + timedelta(days=60), (
            f"due_date {due} is outside a sane 60-day planning window"
        )
