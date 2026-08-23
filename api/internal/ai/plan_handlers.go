package ai

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"knowdo/api/internal/task"
)

type PlanHandlers struct {
	planner Planner
	tasks   task.Store
}

func NewPlanHandlers(planner Planner, tasks task.Store) *PlanHandlers {
	return &PlanHandlers{planner: planner, tasks: tasks}
}

func (h *PlanHandlers) HandlePlan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Request string `json:"request"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	request := strings.TrimSpace(body.Request)
	if request == "" {
		http.Error(w, "request is required", http.StatusBadRequest)
		return
	}

	generated, err := h.planner.Plan(r.Context(), request)
	if err != nil {
		log.Printf("generating plan: %v", err)

		if errors.Is(err, ErrUnavailable) {
			http.Error(w, "ai service unavailable", http.StatusServiceUnavailable)
			return
		}

		http.Error(w, "failed to generate plan", http.StatusInternalServerError)
		return
	}

	// ponytail: no transaction around the batch insert — a failure
	// partway through leaves the earlier tasks saved. Wrap in a
	// transaction if a partial save is ever observed to cause trouble.
	created := make([]task.Task, 0, len(generated.Tasks))
	for _, pt := range generated.Tasks {
		dueDate, err := time.Parse("2006-01-02", pt.DueDate)
		if err != nil {
			log.Printf("parsing due_date %q: %v", pt.DueDate, err)
			http.Error(w, "ai service returned an invalid plan", http.StatusInternalServerError)
			return
		}

		t := &task.Task{Title: pt.Title, Description: pt.Description, DueDate: &dueDate}
		if err := h.tasks.Create(r.Context(), t); err != nil {
			log.Printf("saving generated task: %v", err)
			http.Error(w, "failed to save generated tasks", http.StatusInternalServerError)
			return
		}

		created = append(created, *t)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(created); err != nil {
		log.Printf("encoding response: %v", err)
	}
}
