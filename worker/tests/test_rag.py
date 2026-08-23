from knowdo_worker.rag import ask


def fake_embed(texts):
    return [[0.1, 0.2, 0.3] for _ in texts]


class FakeChat:
    def __init__(self, reply="the answer"):
        self.reply = reply
        self.calls = []

    def __call__(self, messages):
        self.calls.append(messages)

        return self.reply


def search_returning(*payloads):
    def search(vector, limit=5):
        return list(payloads[:limit])

    return search


def test_answer_and_sources_come_back():
    chat = FakeChat()

    result = ask(
        "what is knowdo?",
        embed=fake_embed,
        search=search_returning(
            {"document_id": 3, "chunk_index": 0, "text": "KnowDo is a learning project."},
            {"document_id": 3, "chunk_index": 1, "text": "It has a Go API."},
            {"document_id": 9, "chunk_index": 0, "text": "And a Python worker."},
        ),
        chat=chat,
    )

    assert result["answer"] == "the answer"
    # Distinct documents, in the order they were first retrieved.
    assert result["sources"] == [3, 9]


def test_question_and_chunks_both_reach_the_prompt():
    chat = FakeChat()

    ask(
        "who wrote it?",
        embed=fake_embed,
        search=search_returning(
            {"document_id": 1, "chunk_index": 0, "text": "Written by Gustavs."}
        ),
        chat=chat,
    )

    prompt = " ".join(m["content"] for m in chat.calls[0])
    assert "who wrote it?" in prompt
    assert "Written by Gustavs." in prompt


def test_no_chunks_means_no_llm_call():
    chat = FakeChat()

    result = ask("anything?", embed=fake_embed, search=search_returning(), chat=chat)

    assert chat.calls == []
    assert result["sources"] == []
    assert result["answer"] != ""
