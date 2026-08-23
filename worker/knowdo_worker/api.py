"""The internal AI service. Only the Go API calls it."""

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from .rag import ask

app = FastAPI(title="knowdo-ai")


class AskRequest(BaseModel):
    question: str


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}


@app.post("/ask")
def handle_ask(request: AskRequest) -> dict:
    question = request.question.strip()

    if not question:
        raise HTTPException(status_code=400, detail="question is required")

    return ask(question)
