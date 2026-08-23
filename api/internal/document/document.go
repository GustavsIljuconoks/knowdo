package document

import "time"

// Document is a document's metadata. The uploaded bytes live in the
// documents.content column and are deliberately not a field here —
// listing documents must never load every uploaded file into memory.
type Document struct {
	ID          int64     `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	ChunkCount  int       `json:"chunk_count"`
	CreatedAt   time.Time `json:"created_at"`
}
