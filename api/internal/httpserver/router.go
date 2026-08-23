package httpserver

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"knowdo/api/internal/ai"
	"knowdo/api/internal/document"
	"knowdo/api/internal/queue"
	"knowdo/api/internal/task"
)

func NewRouter(pool *pgxpool.Pool, q queue.Queue, aiClient *ai.Client) *http.ServeMux {
	mux := http.NewServeMux()

	taskStore := task.NewPGStore(pool)
	taskHandlers := task.NewHandlers(taskStore)
	documentHandlers := document.NewHandlers(document.NewPGStore(pool), q)
	askHandlers := ai.NewHandlers(aiClient)
	planHandlers := ai.NewPlanHandlers(aiClient, taskStore)

	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /tasks", taskHandlers.HandleCreate)
	mux.HandleFunc("GET /tasks", taskHandlers.HandleList)
	mux.HandleFunc("GET /tasks/{id}", taskHandlers.HandleGet)
	mux.HandleFunc("PATCH /tasks/{id}", taskHandlers.HandleUpdate)
	mux.HandleFunc("DELETE /tasks/{id}", taskHandlers.HandleDelete)
	mux.HandleFunc("POST /documents", documentHandlers.HandleCreate)
	mux.HandleFunc("GET /documents", documentHandlers.HandleList)
	mux.HandleFunc("GET /documents/{id}", documentHandlers.HandleGet)
	mux.HandleFunc("POST /ask", askHandlers.HandleAsk)
	mux.HandleFunc("POST /plan", planHandlers.HandlePlan)
	mux.Handle("/", http.FileServer(http.Dir("static")))

	return mux
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("encoding response: %v", err)
	}
}
