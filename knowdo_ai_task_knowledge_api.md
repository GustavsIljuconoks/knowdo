# KnowDo — AI Task & Knowledge API

## 1. Project Overview

**KnowDo** is a small AI-powered personal task and knowledge management system designed primarily as a hands-on learning project for modern backend, AI, cloud, and DevOps engineering.

The application itself should remain relatively simple. The complexity comes from progressively building and operating it using production-style infrastructure.

The project combines:

- Go backend engineering
- Python AI workers
- PostgreSQL
- Redis
- Qdrant / vector search
- RAG
- LLMs
- LangChain / LangGraph
- Docker
- GitHub
- GitHub Actions
- Terraform
- GCP / GKE
- Kubernetes
- ArgoCD
- Atlantis
- Prometheus
- Grafana
- Datadog

The goal is not to use every technology immediately. The system should be built in layers, with each technology introduced because it solves a real problem.

---

# 2. Product Idea

KnowDo combines two concepts:

1. **Tasks** — things the user wants to accomplish.
2. **Knowledge** — documents and information the user wants the AI to understand.

The AI connects the two.

For example:

> "I want to learn Kubernetes before September. I have about 5 hours per week."

The system could turn this into:

- Learn Kubernetes architecture
- Learn Pods
- Learn Deployments
- Learn Services
- Learn Ingress
- Learn ConfigMaps and Secrets
- Learn Kubernetes networking

The user can also upload documents such as:

- PDFs
- Markdown files
- notes
- technical documentation

and later ask:

> "What do my Kubernetes notes say about Deployments?"

The system uses RAG to retrieve relevant information and generate an answer.

---

# 3. Example User Interface

```text
┌──────────────────────────────────────────┐
│ KnowDo                                   │
├──────────────────────────────────────────┤
│                                          │
│ What do you want to do?                  │
│                                          │
│ > I need to learn ArgoCD this weekend    │
│                                          │
│               [ Ask AI ]                 │
│                                          │
├──────────────────────────────────────────┤
│ Tasks                                    │
│                                          │
│ ☐ Learn ArgoCD                            │
│   Saturday · Learning                    │
│                                          │
│ ☐ Deploy application to GKE              │
│   Sunday · DevOps                        │
│                                          │
├──────────────────────────────────────────┤
│ Knowledge                                │
│                                          │
│ Kubernetes notes                         │
│ Terraform notes                          │
│ ArgoCD documentation                     │
└──────────────────────────────────────────┘
```

The frontend is deliberately not the main focus. It can initially be very simple.

---

# 4. High-Level Architecture

```text
                         USER
                           │
                           ▼
                     ┌───────────┐
                     │ Frontend  │
                     └─────┬─────┘
                           │
                           ▼
                     ┌───────────┐
                     │  Go API   │
                     └─────┬─────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
              ▼            ▼            ▼
         PostgreSQL      Redis        Qdrant
                           │
                           │ jobs
                           ▼
                    ┌─────────────┐
                    │ AI Worker   │
                    │   Python    │
                    └──────┬──────┘
                           │
                    ┌──────┴──────┐
                    ▼             ▼
                   RAG           LLM
```

Eventually this runs on Kubernetes/GKE:

```text
                         GCP
                          │
                         GKE
                          │
                    Kubernetes
                          │
          ┌───────────────┼────────────────┐
          │               │                │
       KnowDo          ArgoCD         Monitoring
          │               │                │
          │               │          ┌─────┴─────┐
          │               │          ▼           ▼
          │               │     Prometheus    Datadog
          │               │          │
          │               │       Grafana
          │               │
          └───────────────┘
```

Infrastructure is managed with Terraform:

```text
GitHub
   │
   ├── Terraform
   │
   └── Atlantis
         │
         ▼
       GCP
         │
         ├── VPC
         ├── GKE
         ├── IAM
         ├── Storage
         └── Database
```

---

# 5. Core Services

## 5.1 Go API

The main backend service is written in Go.

Responsibilities:

- Authentication
- Users
- Tasks
- Documents
- Conversations
- API endpoints
- Queueing AI jobs
- Communicating with PostgreSQL, Redis and Qdrant

Example endpoints:

```text
GET    /health
GET    /tasks
POST   /tasks
GET    /tasks/:id
PATCH  /tasks/:id
DELETE /tasks/:id

POST   /documents
GET    /documents
GET    /documents/:id

POST   /ask
GET    /conversations
```

Example:

```http
POST /tasks

{
  "title": "Learn ArgoCD",
  "description": "Understand GitOps",
  "due_date": "2026-08-16"
}
```

---

# 6. PostgreSQL

PostgreSQL stores structured application state.

Potential tables:

```text
users
-----
id
email
created_at


tasks
-----
id
user_id
title
description
status
due_date
created_at


documents
---------
id
user_id
filename
status
created_at


conversations
-------------
id
user_id
created_at


messages
--------
id
conversation_id
role
content
created_at
```

PostgreSQL should be treated as the source of truth for application state.

It is not the primary vector store.

---

# 7. Redis

Redis serves two main purposes.

## Caching

For example:

```text
GET /tasks
     │
     ▼
   Redis
     │
     ├── cache hit → return
     │
     └── miss → PostgreSQL → cache result
```

## Background jobs

Uploading a document should not require the API request to wait for the entire AI pipeline.

Instead:

```text
Upload PDF
    │
    ▼
Go API
    │
    ▼
Redis queue
    │
    ▼
AI Worker
```

This introduces asynchronous processing and gives the project a realistic distributed-system problem.

---

# 8. Python AI Worker

The Python worker processes AI-related jobs.

Potential responsibilities:

- PDF/text extraction
- Chunking
- Embedding generation
- Vector storage
- RAG retrieval
- Summarization
- Task extraction
- Classification
- AI planning

Example document pipeline:

```text
PDF
 ↓
Extract text
 ↓
Split into chunks
 ↓
Generate embeddings
 ↓
Store vectors in Qdrant
```

---

# 9. Qdrant

Qdrant is the vector database.

It stores embeddings representing the meaning of document chunks.

Example:

```text
Document
   ↓
Chunk
   ↓
Embedding
   ↓
Qdrant
```

When the user asks:

> "What does my Kubernetes document say about Deployments?"

the system performs:

```text
Question
   ↓
Embedding
   ↓
Qdrant similarity search
   ↓
Relevant document chunks
   ↓
LLM
   ↓
Answer
```

This provides the RAG component of the application.

---

# 10. RAG

The basic RAG pipeline:

```text
                User Question
                     │
                     ▼
                Create Query
                 Embedding
                     │
                     ▼
                  Qdrant
                     │
             Retrieve relevant
                  chunks
                     │
                     ▼
               Build Prompt
                     │
                     ▼
                    LLM
                     │
                     ▼
                  Answer
```

The answer should ideally include references to the retrieved documents/chunks.

---

# 11. AI Task Planning

The AI can also interact with the task system.

Example input:

> "I want to learn Kubernetes before September. I have about 5 hours per week."

The AI could produce:

```text
Learn Kubernetes
│
├── Kubernetes architecture
├── Pods
├── Deployments
├── Services
├── Ingress
├── ConfigMaps
├── Secrets
└── Kubernetes networking
```

The AI worker sends structured task information back to the Go API.

The Go API stores the tasks in PostgreSQL.

---

# 12. LangGraph

Once the basic AI workflow works, introduce LangGraph.

A possible workflow:

```text
                  User request
                       │
                       ▼
                Understand intent
                       │
                ┌──────┴──────┐
                │             │
             Task?         Question?
                │             │
                ▼             ▼
          Task planner      RAG search
                │             │
                ▼             ▼
          Validate task     LLM answer
                │             │
                └──────┬──────┘
                       ▼
                    Response
```

You can then introduce loops:

```text
Generate plan
     ↓
Check plan
     ↓
Is it good?
  │       │
 No      Yes
  │       │
  └──→    ↓
       Save tasks
```

This provides a practical environment for experimenting with:

- state
- graphs
- loops
- agents
- tools
- evaluation
- guardrails

---

# 13. Docker

Each major service gets its own container.

For example:

```text
knowdo-api
knowdo-worker
knowdo-frontend
```

Supporting services:

```text
postgres
redis
qdrant
```

Local development can initially use Docker Compose:

```text
docker compose up
```

The goal is to make the entire development environment reproducible.

---

# 14. GitHub

GitHub contains:

- source code
- issues
- pull requests
- Git history
- CI workflows
- Kubernetes manifests
- Terraform

Possible repository structure:

```text
knowdo/
│
├── api/
├── worker/
├── frontend/
│
├── docker/
│
├── terraform/
│   ├── modules/
│   │   ├── network/
│   │   ├── gke/
│   │   └── database/
│   │
│   └── environments/
│       ├── dev/
│       └── production/
│
├── kubernetes/
│   ├── base/
│   └── overlays/
│       ├── dev/
│       └── production/
│
├── argocd/
│   └── applications/
│
├── monitoring/
│   ├── prometheus/
│   └── grafana/
│
└── README.md
```

---

# 15. GitHub Actions

CI should run automatically on pull requests.

Example pipeline:

```text
Pull Request
     │
     ▼
GitHub Actions
     │
     ├── Go tests
     ├── Python tests
     ├── lint
     ├── security checks
     └── Docker build
```

Eventually it can build images:

```text
knowdo-api:1.4.2
knowdo-worker:1.4.2
knowdo-frontend:1.4.2
```

and push them to a container registry.

---

# 16. Terraform

Terraform manages cloud infrastructure.

Potential GCP resources:

```text
GCP
├── VPC
├── GKE cluster
├── IAM/service accounts
├── Cloud Storage
├── PostgreSQL
└── networking
```

The desired state is stored as code.

Typical workflow:

```text
terraform init
terraform plan
terraform apply
terraform destroy
```

The project should eventually be reproducible from Terraform.

---

# 17. GKE / Kubernetes

The production environment runs on GKE.

Possible Kubernetes workloads:

```text
namespace: knowdo

├── frontend
├── api
├── ai-worker
├── postgres
├── redis
├── qdrant
└── monitoring
```

The API might initially run with:

```yaml
replicas: 3
```

This gives an opportunity to experiment with:

- Pods
- Deployments
- Services
- Ingress
- ConfigMaps
- Secrets
- Persistent Volumes
- Health checks
- Resource limits
- Horizontal Pod Autoscaling
- Namespaces

Deliberately break things and observe how Kubernetes responds.

For example:

```text
Delete API pod
     ↓
Kubernetes notices
     ↓
Replacement pod created
     ↓
Desired state restored
```

---

# 18. ArgoCD

Kubernetes configuration is stored in Git.

ArgoCD watches the repository:

```text
Git
 │
 ▼
ArgoCD
 │
 ▼
Kubernetes
```

A deployment might work like:

```text
Developer
    │
    ▼
GitHub
    │
    ▼
GitHub Actions
    │
    ▼
Docker image
    │
    ▼
Update image version in Git
    │
    ▼
ArgoCD
    │
    ▼
GKE
```

This creates a GitOps workflow.

---

# 19. Atlantis

Atlantis manages collaborative Terraform workflows.

Example:

```text
Developer changes Terraform
          │
          ▼
      GitHub PR
          │
          ▼
       Atlantis
          │
          ▼
  terraform plan
          │
          ▼
Plan appears in PR
          │
          ▼
      Code review
          │
          ▼
   terraform apply
```

This allows infrastructure changes to be reviewed similarly to application code.

---

# 20. Prometheus

Prometheus collects metrics.

Useful application metrics:

```text
http_requests_total
http_request_duration_seconds
http_errors_total

ai_requests_total
ai_request_duration_seconds
ai_tokens_used_total

queue_depth
worker_processing_duration

database_connections
```

This allows you to answer questions such as:

- How many requests are we receiving?
- Is the API getting slower?
- How many AI jobs are waiting?
- How long does RAG take?
- How often are requests failing?

---

# 21. Grafana

Grafana visualizes Prometheus metrics.

A possible dashboard:

```text
┌─────────────────────────────────────┐
│ KNOWDO PRODUCTION                   │
├─────────────────────────────────────┤
│ Requests/sec          42             │
│ Error rate           0.4%           │
│ P95 latency          180ms          │
│                                     │
│ AI requests           12/min       │
│ AI latency             4.2s         │
│ Queue depth               3         │
│                                     │
│ API CPU                31%          │
│ Worker CPU             67%          │
└─────────────────────────────────────┘
```

---

# 22. Datadog

Datadog can provide broader observability:

- metrics
- logs
- traces
- APM
- Kubernetes monitoring
- alerts

A distributed request could look like:

```text
POST /ask
     │
     ├── API                  30ms
     │
     ├── Redis                 5ms
     │
     ├── Qdrant               80ms
     │
     └── AI Worker
          │
          └── LLM            3.8s
```

This provides a realistic environment for learning distributed tracing and production debugging.

OpenTelemetry can optionally be introduced as the vendor-neutral instrumentation layer.

---

# 23. Development Roadmap

Do not build everything at once.

## Stage 1 — Application

- [ ] Create Go API
- [ ] Add PostgreSQL
- [ ] Implement tasks CRUD
- [ ] Add basic frontend
- [ ] Add Docker Compose

## Stage 2 — CI

- [ ] Add GitHub repository
- [ ] Add GitHub Actions
- [ ] Run Go tests
- [ ] Run Python tests
- [ ] Add linting
- [ ] Build Docker images

## Stage 3 — AI

- [ ] Create Python worker
- [ ] Add Redis job queue
- [ ] Add document upload
- [ ] Extract document text
- [ ] Chunk documents
- [ ] Generate embeddings
- [ ] Store embeddings in Qdrant
- [ ] Implement basic RAG

## Stage 4 — AI Task Planning

- [ ] Ask AI to create tasks
- [ ] Validate structured AI output
- [ ] Save generated tasks to PostgreSQL
- [ ] Add task planning workflow
- [ ] Introduce LangGraph
- [ ] Add basic evaluation

## Stage 5 — Terraform

- [ ] Create Terraform project
- [ ] Create GCP project/environment
- [ ] Create VPC
- [ ] Create IAM/service accounts
- [ ] Create GKE cluster
- [ ] Create required cloud resources

## Stage 6 — Kubernetes

- [ ] Containerize all services
- [ ] Create Kubernetes Deployments
- [ ] Create Services
- [ ] Add ConfigMaps
- [ ] Add Secrets
- [ ] Add health checks
- [ ] Add resource limits
- [ ] Add persistent storage
- [ ] Deploy to GKE

## Stage 7 — GitOps

- [ ] Install ArgoCD
- [ ] Move Kubernetes configuration to Git
- [ ] Configure ArgoCD application
- [ ] Deploy through Git
- [ ] Test rollback

## Stage 8 — Observability

- [ ] Add Prometheus
- [ ] Expose application metrics
- [ ] Install Grafana
- [ ] Create dashboard
- [ ] Add structured logs
- [ ] Add OpenTelemetry
- [ ] Add Datadog

## Stage 9 — Infrastructure Collaboration

- [ ] Install/configure Atlantis
- [ ] Connect Atlantis to GitHub
- [ ] Run Terraform plans through PRs
- [ ] Review infrastructure changes
- [ ] Apply infrastructure through the workflow

## Stage 10 — Break It

- [ ] Kill API Pods
- [ ] Break a deployment
- [ ] Exhaust worker resources
- [ ] Introduce a slow database query
- [ ] Break a GitHub Actions workflow
- [ ] Create a bad Terraform change
- [ ] Create an ArgoCD drift
- [ ] Investigate everything using monitoring

---

# 24. Final Architecture

```text
                         USER
                           │
                           ▼
                     ┌───────────┐
                     │ Frontend  │
                     └─────┬─────┘
                           │
                           ▼
                     ┌───────────┐
                     │  Go API   │
                     └─────┬─────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
              ▼            ▼            ▼
         PostgreSQL      Redis        Qdrant
                           │
                           │ jobs
                           ▼
                    ┌─────────────┐
                    │ AI Worker   │
                    │   Python    │
                    └──────┬──────┘
                           │
                    ┌──────┴──────┐
                    ▼             ▼
                   RAG           LLM


                    CLOUD / DEVOPS

                         GitHub
                           │
             ┌─────────────┴─────────────┐
             ▼                           ▼
      GitHub Actions                 Atlantis
             │                           │
             ▼                           ▼
      Docker Registry                Terraform
                                         │
                                         ▼
                                        GCP
                                         │
                                        ▼
                                        GKE
                                         │
                                    Kubernetes
                                         │
                                      ArgoCD
                                         │
                                         ▼
                                      KnowDo
                                         │
                              ┌──────────┴──────────┐
                              ▼                     ▼
                         Prometheus             Datadog
                              │
                              ▼
                           Grafana
```

---

# 25. Main Learning Goals

By the end of the project, the goal is to understand not just **what** each technology does, but **why it exists**.

### Backend

- Go
- Python
- PostgreSQL
- Redis
- APIs
- asynchronous jobs
- microservices

### AI

- embeddings
- vector databases
- RAG
- LLM integration
- LangChain
- LangGraph
- agents
- evaluation
- AI observability

### Cloud

- GCP
- IAM
- networking
- GKE
- managed services

### Infrastructure

- Docker
- Terraform
- Atlantis
- Infrastructure as Code

### Deployment

- Kubernetes
- GitHub Actions
- ArgoCD
- GitOps
- rolling deployments
- health checks
- scaling

### Observability

- Prometheus
- Grafana
- Datadog
- OpenTelemetry
- metrics
- logs
- traces
- alerting

The most important principle is:

> **Start with a simple application and progressively make the infrastructure more sophisticated.**

That way every technology has a concrete purpose and you learn how the pieces interact rather than learning them as isolated tools.
