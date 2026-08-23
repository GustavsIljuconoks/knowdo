"""The internal AI service. Only the Go API calls it."""

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from .graph import run as act
from .plan import plan as generate_plan
from .rag import ask

app = FastAPI(title="knowdo-ai")


class AskRequest(BaseModel):
    question: str


class PlanRequest(BaseModel):
    request: str


class ActRequest(BaseModel):
    request: str


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}


@app.post("/ask")
def handle_ask(request: AskRequest) -> dict:
    question = request.question.strip()

    if not question:
        raise HTTPException(status_code=400, detail="question is required")

    return ask(question)


@app.post("/plan")
def handle_plan(request: PlanRequest) -> dict:
    text = request.request.strip()

    if not text:
        raise HTTPException(status_code=400, detail="request is required")

    return generate_plan(text)


@app.post("/act")
def handle_act(request: ActRequest) -> dict:
    text = request.request.strip()

    if not text:
        raise HTTPException(status_code=400, detail="request is required")

    return act(text)
