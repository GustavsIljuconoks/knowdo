package httpserver

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"knowdo/api/internal/task"
)

func NewRouter(pool *pgxpool.Pool) *http.ServeMux {
	mux := http.NewServeMux()

	taskHandlers := task.NewHandlers(task.NewPGStore(pool))

	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /tasks", taskHandlers.HandleCreate)
	mux.HandleFunc("GET /tasks", taskHandlers.HandleList)
	mux.HandleFunc("GET /tasks/{id}", taskHandlers.HandleGet)
	mux.HandleFunc("PATCH /tasks/{id}", taskHandlers.HandleUpdate)
	mux.HandleFunc("DELETE /tasks/{id}", taskHandlers.HandleDelete)
	mux.Handle("/", http.FileServer(http.Dir("static")))

	return mux
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("encoding response: %v", err)
	}
}
