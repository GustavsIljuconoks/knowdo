package task

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

type Handlers struct {
	store Store
}

func NewHandlers(store Store) *Handlers {
	return &Handlers{store: store}
}

func (h *Handlers) HandleList(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.store.List(r.Context())

	if err != nil {
		http.Error(w, "failed to list tasks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (h *Handlers) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string     `json:"title"`
		Description string     `json:"description"`
		DueDate     *time.Time `json:"due_date"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if body.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	t := &Task{
		Title:       body.Title,
		Description: body.Description,
		DueDate:     body.DueDate,
	}

	if err := h.store.Create(r.Context(), t); err != nil {
		http.Error(w, "failed to create task", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

func (h *Handlers) HandleGet(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)

	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	t, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (h *Handlers) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)

	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var patch Patch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	t, err := h.store.Update(r.Context(), id, patch)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (h *Handlers) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.store.Delete(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	http.Error(w, "internal error", http.StatusInternalServerError)
}
