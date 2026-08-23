"""Route a request to the planner or the answerer via a LangGraph graph.

Design (locked, see knowdo_ai_task_knowledge_api.md Stage 4):

- GraphState carries `request` in, `route` set by the router node,
  `result` set by whichever node runs next.
- router classifies the request as "plan" or "answer" via `classify()`
  — an LLM call validated with llm.chat_json(), same pattern plan.py
  uses for Plan. Not a heuristic/keyword sniff: this is the "understand
  intent" step from section 12 of the doc.
- planner/answer are thin wrappers around plan.py's plan() and
  rag.py's ask() — no new logic in this file beyond routing.

TODO(you): implement classify() below (a RouteDecision model + prompt
+ llm.chat_json call, same shape as plan.py's Plan).
"""

from typing import Literal, TypedDict

from langgraph.graph import END, START, StateGraph
from pydantic import BaseModel

from .llm import chat as default_chat
from .llm import chat_json
from .plan import plan as default_plan
from .rag import ask as default_ask

SYSTEM_PROMPT = """Classify the user's request as one of two routes.

"plan": the user wants a goal turned into tasks/a schedule (e.g.
"I need to learn ArgoCD this weekend", "help me plan learning Kubernetes").

"answer": the user is asking a question expected to be answered from
their documents (e.g. "what do my notes say about Deployments?").

Emit only JSON matching this shape: {"route": "plan" | "answer"}.
No markdown fences, JSON only.
"""

class GraphState(TypedDict):
    request: str
    route: Literal["plan", "answer"] | None
    result: dict | None

class RouteDecision(BaseModel):
    route: Literal["plan", "answer"]

def classify(request: str, *, chat=default_chat) -> Literal["plan", "answer"]:
    """Classify `request` as a task-planning ask or a question."""

    messages = [
        {"role": "system", "content": SYSTEM_PROMPT},
        {"role": "user", "content": request},
    ]

    result = chat_json(messages, RouteDecision, chat=chat)
    return result.route


def build_graph(*, planner=default_plan, answerer=default_ask, classifier=classify):
    """Assemble the router -> {planner, answer} graph.

    `planner`/`answerer`/`classifier` are injectable so tests can run
    the wiring without hitting an LLM — same DI style as ask()'s
    embed=/search=/chat= params.
    """

    def router_node(state: GraphState) -> dict:
        return {"route": classifier(state["request"])}

    def planner_node(state: GraphState) -> dict:
        return {"result": planner(state["request"])}

    def answer_node(state: GraphState) -> dict:
        return {"result": answerer(state["request"])}

    graph = StateGraph(GraphState)
    graph.add_node("router", router_node)
    graph.add_node("planner", planner_node)
    graph.add_node("answer", answer_node)

    graph.add_edge(START, "router")
    graph.add_conditional_edges(
        "router",
        lambda state: state["route"],
        {"plan": "planner", "answer": "answer"},
    )
    graph.add_edge("planner", END)
    graph.add_edge("answer", END)

    return graph.compile()


_graph = build_graph()


def run(request: str) -> dict:
    """Route `request` through the compiled graph and return its result."""

    return _graph.invoke({"request": request, "route": None, "result": None})["result"]
