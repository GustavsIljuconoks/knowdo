# KnowDo Stage 3 — AI: Python Worker, Redis Queue, Qdrant, Basic RAG

## Purpose

Third increment of the KnowDo learning project (see
`knowdo_ai_task_knowledge_api.md` §7–10 and the Stage 3 roadmap
checklist). Adds the AI half of the system: documents can be uploaded to
the Go API, are processed asynchronously by a Python worker (extract →
chunk → embed → store vectors), and can then be questioned in natural
language through a basic RAG pipeline.

Stage 1 shipped the Go API + Postgres + tasks CRUD. Stage 2 shipped CI.
This stage introduces asynchronous processing and the first genuinely
distributed-system problem in the project: work that outlives the HTTP
request that started it.

Learning goals for this stage: queue-backed background jobs, a
polyglot service boundary (Go ↔ Python), embeddings and vector search,
and the retrieval-augmented generation loop.

## Scope

In scope:

- `documents` table, with file bytes stored in Postgres
- `POST /documents` (multipart upload), `GET /documents`,
  `GET /documents/{id}` on the Go API
- Redis list used as a job queue; Go enqueues, Python consumes
- Python worker image with two entry points: a queue consumer and a
  FastAPI service
- Text extraction (PDF + plain text/Markdown), chunking, embedding
- Qdrant collection holding chunk vectors
- `POST /ask` on the Go API, proxied to the Python `/ask`, answering
  from retrieved chunks via the Kimi (Moonshot) chat API
- Minimal frontend additions: an upload form and an ask box
- Tests both sides; a Python job added to CI

Out of scope (deferred per the roadmap):

- Authentication / users, and therefore `user_id` on `documents`
- Redis caching of `GET /tasks` (§7 of the source doc) — Stage 3 has no
  read-performance problem to solve
- Conversations / messages persistence (Stage 4)
- AI task planning, structured output validation, LangGraph, evaluation
  (Stage 4)
- Streaming responses, reranking, per-sentence citations, chunk dedup on
  re-upload
- Terraform, Kubernetes, observability (Stages 5+)

## Topology

```text
browser → Go API :8080 ──┬── Postgres   tasks, documents (+ bytes)
                         ├── Redis      LPUSH knowdo:jobs:ingest
                         └── ai :8000   POST /ask   (FastAPI, synchronous)

worker (same image, no port) ── BLPOP ──→ extract → chunk → embed → Qdrant
```

The Go API stays the only publicly-routed service. It is the edge:
validation, and later auth, rate limiting and tracing, live in exactly
one place. The Python service is internal.

`worker` and `ai` are the same image started with different commands.
They are separate Compose services rather than one process with a
background thread, because they scale on different signals: `/ask` is
latency-sensitive and scales on request rate, while ingestion is bursty
and scales on queue depth. Splitting them now costs nothing — no extra
code, just a second `command:` — and means a backlog of uploaded PDFs
cannot degrade answer latency.

### Why `/ask` is synchronous HTTP, not a queue job

Retrieval plus one LLM call is a seconds-long operation that fits
inside an HTTP request. Routing it through Redis would mean
hand-building request-reply over a queue: correlation ids, reply
channels, timeouts, orphaned-response cleanup, and a poll loop in Go.
That machinery is worth it when the work genuinely outlives a request —
which is what Stage 4's LangGraph workflows may become. The queue will
already exist when that question arrives.

## Data Layer

Migration `000002_create_documents_table`:

```sql
CREATE TABLE documents (
    id           BIGSERIAL PRIMARY KEY,
    filename     TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    content      BYTEA NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending', -- pending|processing|ready|failed
    error        TEXT NOT NULL DEFAULT '',
    chunk_count  INT  NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

File bytes live in a `BYTEA` column rather than on a shared volume. A
shared read-write-many volume is the first thing that breaks when this
moves to GKE in Stage 6, whereas Postgres works identically there. It is
also one less dependency than adding object storage now; Stage 5 can
move bytes to GCS when there is a real reason to.

No `user_id` — there is still no `users` table, and Stage 1 already
established that unused columns wait for the stage that needs them.

`status` is the contract between the two services: Go writes `pending`,
the worker moves it to `processing` and then `ready` or `failed`.
`GET /documents/{id}` is how a client learns the upload finished.

## Go API

New package `internal/document`, mirroring `internal/task`: a `Store`
interface with a `PGStore` implementation, and `Handlers` taking that
interface so tests can substitute a fake. Two small helpers alongside
it: an enqueue function wrapping Redis `LPUSH`, and an HTTP client for
the AI service.

| Endpoint | Behavior |
|---|---|
| `POST /documents` | multipart upload → insert row with `status='pending'` → enqueue `{"document_id": N}` → `202` with the row |
| `GET /documents` | list metadata only; never selects `content` |
| `GET /documents/{id}` | metadata, `status`, `chunk_count`, `error` |
| `POST /ask` | `{"question": "..."}` → forward to `ai:8000/ask` → `{"answer": "...", "sources": [...]}` |

Validation at the boundary, as in Stage 1: missing file → `400`, empty
question → `400`, unknown id → `404`. Upload size is capped
(`http.MaxBytesReader`) so a large file cannot exhaust memory — the
bytes are read into memory before insert.

`GET /documents` deliberately never selects the `content` column; a list
endpoint that loads every uploaded file into memory is a trivially
avoidable failure.

One new dependency: `github.com/redis/go-redis/v9`. The AI call is
plain `net/http`.

## Python Worker

New top-level `worker/` directory. Small modules, each with one job:

- `extract.py` — PDF text via `pypdf`; everything else decoded as UTF-8
  (Markdown, plain text). Unknown/binary content → a failed job with a
  readable error, not a crash.
- `chunk.py` — 800-character chunks with 100-character overlap, via
  LangChain's `RecursiveCharacterTextSplitter`. It tries paragraph, then
  line, then word, then character boundaries and takes the coarsest that
  fits, so chunks do not start mid-word. This costs a dependency
  (`langchain-text-splitters`, 14 transitive packages) that a fixed
  character window would not, taken deliberately: Stage 4 pulls
  `langchain-core` in via LangGraph anyway, so the marginal cost falls to
  one package then, and the seam is a single function either way. The
  trade is that overlap is honoured approximately rather than to the
  character — boundary-aligned chunks cannot also be exact windows.
  `ponytail:` still character-counted, not token-counted; switch to
  `.from_huggingface_tokenizer()` if the embedding model starts
  truncating chunks.
- `embed.py` — `fastembed`, model from `EMBED_MODEL` (default
  `BAAI/bge-small-en-v1.5`, 384 dimensions). Chosen over
  `sentence-transformers` because it is ONNX based and does not pull in
  PyTorch. The model is baked into the Docker image at build time so the
  first request is not a model download.
- `vectors.py` — Qdrant client. Collection `knowdo`, 384-dim, cosine
  distance, created on startup if absent. Chunk payload is
  `{document_id, chunk_index, text}`.
- `llm.py` — a single `chat(messages) -> str` over the `openai` SDK,
  configured by `LLM_BASE_URL`, `LLM_MODEL`, `LLM_API_KEY`. See
  "Swapping the LLM" below.
- `db.py` — the two Postgres statements the worker needs: read a
  document's bytes, write back its status.
- `ingest.py` — the ingest pipeline, composing extract → chunk → embed →
  upsert.
- `rag.py` — the ask pipeline: embed → search → prompt → `chat()`.
- `consumer.py` — `BLPOP` loop over `knowdo:jobs:ingest`.
- `api.py` — FastAPI: `POST /ask`, `GET /health`.

`ingest.py` and `rag.py` take their embedder and chat function as
default arguments, so tests inject stubs and neither the model download
nor the LLM API is reachable from the test suite.

### Embeddings local, generation hosted

Kimi is OpenAI-compatible for chat but exposes no embeddings endpoint,
so the two halves are split by necessity — and the split is the one
worth having anyway. Embedding runs locally, where it is cheap, offline,
and keeps tests free of network calls. Generation uses the hosted model,
where hosted quality actually matters.

### Swapping the LLM

The OpenAI chat-completions shape is the de-facto standard: Kimi,
DeepSeek, Groq, Together, Mistral, OpenRouter, Ollama, vLLM and
llama.cpp's server all speak it. So provider choice is configuration,
not code — `llm.py` is one function and every caller goes through it.

| Provider | `LLM_BASE_URL` | `LLM_MODEL` |
|---|---|---|
| Kimi (default) | `https://api.moonshot.ai/v1` | `kimi-k3` |
| Ollama, local | `http://ollama:11434/v1` | `qwen2.5:7b` |
| vLLM, self-hosted | `http://vllm:8000/v1` | `Qwen/Qwen2.5-7B-Instruct` |

No provider hierarchy, registry, or factory: an interface with one
implementation buys nothing that three environment variables do not.
Should a genuinely non-compatible provider ever be needed, it becomes a
branch inside `chat()` — still no caller changes.

This is also deliberately short-lived. Stage 4 introduces
LangChain/LangGraph, whose `init_chat_model()` is a maintained provider
abstraction covering the non-compatible providers too. A single function
is the cheapest possible thing to replace at that point; a hand-rolled
hierarchy would be written now only to be deleted then.

vLLM is worth a note because it is the one option with a real reason to
arrive later: its value is throughput (PagedAttention, continuous
batching), which is invisible at one concurrent user and decisive for
Stage 4's evaluation runs, a GPU node pool in Stage 6, and load testing
in Stage 10. It also needs an NVIDIA GPU in practice. Adding it then is
two environment variables and a Compose block.

### Ingest pipeline

```text
BLPOP job → load document row → status='processing'
  → extract text → chunk → embed chunks → upsert vectors to Qdrant
  → status='ready', chunk_count=N
```

Any exception sets `status='failed'` and writes the message to `error`;
the job is then dropped. `ponytail:` no retries and no dead-letter
queue — re-upload to retry. Retry logic gets added when a job actually
fails for a transient reason, not in anticipation of one.

### Ask pipeline

```text
question → embed → Qdrant top-5 → build prompt from chunks
  → Kimi chat → {answer, sources}
```

`sources` is the set of `document_id`s the retrieved chunks came from.
No conversation history, no reranking — those belong to Stage 4 and to
whenever retrieval quality proves insufficient.

## Docker Compose

Four new services alongside `postgres` and `api`:

- `redis` (official image, healthcheck)
- `qdrant` (official image, named volume)
- `worker` (built from `worker/`, command: the consumer, no ports)
- `ai` (same build, command: `uvicorn`, port 8000 exposed internally)

New environment: `REDIS_URL`, `QDRANT_URL`, `DATABASE_URL`,
`LLM_BASE_URL`, `LLM_MODEL`, `LLM_API_KEY` and `EMBED_MODEL` for the
Python services; `REDIS_URL` and `AI_URL` for the Go API. `.env` gains
an `LLM_API_KEY=` placeholder to be filled in locally; it is not
committed with a value.

Migrations continue to run via the `migrate` CLI, as in Stage 1, not
automatically on startup.

## Frontend

`api/static/index.html` gains an upload form (posting multipart to
`/documents`, showing each document's status) and an ask box (posting to
`/ask`, rendering the answer and its source ids). Still vanilla JS
against `fetch()`, still no build step — the frontend remains a manual
testing surface, not the focus.

## Testing

Go, following Stage 1's approach — stdlib `testing` and `httptest`,
table-driven, no assertion library:

- Document handlers against a fake store and a fake queue, covering
  upload success, missing file, unknown id, and that `202` is returned
  before any processing happens.
- `/ask` against an `httptest` stub standing in for the AI service,
  covering the happy path, an empty question, and the AI service being
  unavailable.

Python, with `pytest`:

- Chunking: boundaries and overlap, including text shorter than one
  chunk.
- Extraction: a small PDF fixture and a Markdown file.
- The ingest pipeline end to end with a stub embedder and an in-memory
  vector store, asserting the status transitions including the failure
  path.

No test touches the network — neither the embedding model download nor
the Kimi API.

## CI

A new `python` job in `.github/workflows/ci.yml` running `ruff` and
`pytest` against `worker/`, alongside the existing Go build, test, lint
and docker-build jobs. The docker-build job also builds the worker
image.

## Error Handling

Errors surface where the user can see them. A failed ingest is visible
as `status='failed'` plus a message on `GET /documents/{id}`, not only
in worker logs. An unreachable AI service returns `503` from `/ask`
rather than a generic `500`, since the distinction is the one that
matters when debugging the stack locally.

No speculative handling for conditions this stage cannot produce.

## Non-Goals (this stage)

Explicitly not doing yet, to avoid scope creep: retries and dead-letter
queues, Redis caching, streaming responses, gRPC between Go and Python,
reranking, multi-tenancy, rate limiting. Each gets introduced when a
concrete problem calls for it — several of them in Stage 4, when `/ask`
grows into a real workflow.
