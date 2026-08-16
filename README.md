# KnowDo

AI-powered task and knowledge management system, built as a learning
project in stages. See `knowdo_ai_task_knowledge_api.md` for the full
roadmap and `docs/superpowers/specs/` for per-stage design docs.

## Stage 1: Go API + Postgres + tasks CRUD

### Prerequisites

- Go 1.25+
- Docker + Docker Compose
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI, for
  running schema migrations:
  ```
  go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
  ```

### Run

```
docker compose up --build
```

Applies no migrations automatically — run them once against the running
Postgres:

```
migrate -path api/migrations -database "postgres://knowdo:knowdo@localhost:5432/knowdo?sslmode=disable" up
```

Then open http://localhost:8080.

### Test

```
cd api && go test ./...
```
