# KnowDo Stage 1 — Go API, Postgres, Tasks CRUD, Docker Compose

## Purpose

First increment of the KnowDo learning project (see
`knowdo_ai_task_knowledge_api.md` for the full roadmap). Ships a working,
locally-runnable application: a Go API backed by Postgres, full CRUD on
tasks, a minimal static frontend, and a Docker Compose stack. No auth, no
Redis, no AI worker, no cloud — those are later stages.

Learning goals for this stage: idiomatic Go project layout, `net/http`
routing (Go 1.22+ pattern matching, no framework), direct SQL with `pgx`
(no ORM), SQL schema migrations, Docker Compose for local dev.

## Scope

In scope:
- `tasks` table + full CRUD API (`GET/POST /tasks`, `GET/PATCH/DELETE /tasks/{id}`)
- `GET /health`
- Single static HTML+JS page served by the API for manual testing
- Docker Compose stack (`api` + `postgres`)
- Table-driven handler tests via `httptest`

Out of scope (explicitly deferred to later stages per the roadmap):
- Authentication / users
- Redis, Qdrant, AI worker
- Documents, conversations, messages tables
- CI, Terraform, Kubernetes, ArgoCD, observability

## Repo Layout

```
knowdo/
├── api/
│   ├── cmd/api/main.go        # entrypoint: config, DB pool, server start
│   ├── internal/
│   │   ├── task/               # model, store (SQL), HTTP handlers
│   │   └── httpserver/         # router wiring, middleware, /health
│   ├── static/                 # single index.html + JS, served by the API
│   ├── migrations/             # golang-migrate .sql up/down files
│   ├── go.mod / go.sum
│   └── Dockerfile
├── docker-compose.yml
└── README.md
```

Only what Stage 1 needs exists. `terraform/`, `kubernetes/`, `argocd/`,
`worker/`, `frontend/` (as a separate service) are created when their
stage arrives, not before.

## Data Layer

- `database/sql` + `jackc/pgx` (stdlib-compatible driver), no ORM.
- Schema managed with `golang-migrate`: one migration for the `tasks`
  table (`id`, `title`, `description`, `status`, `due_date`,
  `created_at`). No `user_id` yet — there's no `users` table until the
  auth stage, and adding an unused column now is scaffolding for a
  feature that isn't built. The auth stage's migration adds it.
- Migrations run via the `migrate` CLI (documented in README), not
  auto-applied by the app on startup — keeps app startup simple and
  matches how migrations work in most real Go services.

## API

- stdlib `net/http`, Go 1.22+ `ServeMux` pattern routing
  (`"GET /tasks/{id}"` etc.) — no router dependency.
- JSON in/out. Standard library `encoding/json`.
- Handlers live in `internal/task`, take the store as a dependency
  (interface), so tests can use a fake/in-memory store without a real DB
  where that's simpler, and `httptest` + a real test DB where it isn't
  (decided per-test, not dogmatically).
- Validation: required fields (`title`) checked at the handler boundary;
  malformed JSON / missing fields → `400`. Not-found → `404`.

## Frontend

- No separate frontend service or container. The Go API serves a single
  static `index.html` (vanilla JS, `fetch()` against `/tasks`) from
  `api/static/`. Matches the source doc's "frontend is not the focus"
  and avoids a second build step / container for Stage 1.
- Upgradeable later: if a real frontend is wanted, it becomes its own
  service without changing the API.

## Docker Compose

Two services:
- `postgres` (official image, named volume for data, healthcheck)
- `api` (built from `api/Dockerfile`, depends on `postgres` healthcheck)

`docker compose up` yields a working stack on `localhost`.

## Testing

- Go `testing` + `net/http/httptest`, table-driven per endpoint.
- No test framework/assertion library — stdlib `testing` is enough at
  this size.

## Error Handling

- Standard boundary validation only (bad JSON, missing required fields,
  not-found id). No speculative error handling for scenarios Stage 1
  can't produce (e.g. no auth errors, since there's no auth yet).

## Non-Goals (this stage)

Explicitly not doing yet, to avoid scope creep: caching, background
jobs, rate limiting, structured logging framework, config management
beyond env vars. These get introduced in later stages when a concrete
problem calls for them (per the roadmap).
