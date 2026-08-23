package document

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"knowdo/api/internal/queue"
)

// maxUploadBytes caps an upload because Create reads the whole file into
// memory before inserting it. ponytail: in-memory read, 10MB ceiling —
// switch to a streaming large-object write if real documents outgrow it.
const maxUploadBytes = 10 << 20

type Handlers struct {
	store Store
	queue queue.Queue
}

func NewHandlers(store Store, q queue.Queue) *Handlers {
	return &Handlers{store: store, queue: q}
}

// HandleCreate accepts a multipart upload, stores it as 'pending', and
// queues the ingest job. It returns 202, not 201: the document exists,
// but it is not yet answerable — the client polls GET /documents/{id}
// for the status to reach 'ready'.
func (h *Handlers) HandleCreate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "a 'file' upload is required", http.StatusBadRequest)
		return
	}

	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read upload", http.StatusBadRequest)
		return
	}

	if len(content) == 0 {
		http.Error(w, "uploaded file is empty", http.StatusBadRequest)
		return
	}

	d := &Document{
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
	}

	if err := h.store.Create(r.Context(), d, content); err != nil {
		http.Error(w, "failed to store document", http.StatusInternalServerError)
		return
	}

	// A stored-but-unqueued document would sit at 'pending' forever
	// with no worker coming, so a failed enqueue is a failed request.
	if err := h.queue.EnqueueIngest(r.Context(), d.ID); err != nil {
		log.Printf("enqueueing ingest job for document %d: %v", d.ID, err)
		http.Error(w, "failed to queue document for processing", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(d); err != nil {
		log.Printf("encoding response: %v", err)
	}
}

func (h *Handlers) HandleList(w http.ResponseWriter, r *http.Request) {
	docs, err := h.store.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list documents", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(docs); err != nil {
		log.Printf("encoding response: %v", err)
	}
}

func (h *Handlers) HandleGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	d, err := h.store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "document not found", http.StatusNotFound)
			return
		}

		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(d); err != nil {
		log.Printf("encoding response: %v", err)
	}
}
