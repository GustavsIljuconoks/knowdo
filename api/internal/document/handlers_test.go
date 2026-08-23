package document

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeStore is an in-memory Store so handler tests don't need Postgres.
type fakeStore struct {
	docs     map[int64]Document
	contents map[int64][]byte
	nextID   int64
}

func newFakeStore(seed ...Document) *fakeStore {
	fs := &fakeStore{docs: map[int64]Document{}, contents: map[int64][]byte{}}

	for _, d := range seed {
		fs.nextID++
		d.ID = fs.nextID
		fs.docs[d.ID] = d
	}
	return fs
}

func (fs *fakeStore) Create(ctx context.Context, d *Document, content []byte) error {
	fs.nextID++
	d.ID = fs.nextID
	d.Status = "pending"
	fs.docs[d.ID] = *d
	fs.contents[d.ID] = content
	return nil
}

func (fs *fakeStore) List(ctx context.Context) ([]Document, error) {
	docs := make([]Document, 0, len(fs.docs))

	for _, d := range fs.docs {
		docs = append(docs, d)
	}

	return docs, nil
}

func (fs *fakeStore) Get(ctx context.Context, id int64) (Document, error) {
	d, ok := fs.docs[id]

	if !ok {
		return Document{}, ErrNotFound
	}

	return d, nil
}

// fakeQueue records what was enqueued instead of talking to Redis.
type fakeQueue struct {
	enqueued []int64
	err      error
}

func (fq *fakeQueue) EnqueueIngest(ctx context.Context, documentID int64) error {
	if fq.err != nil {
		return fq.err
	}

	fq.enqueued = append(fq.enqueued, documentID)
	return nil
}

// uploadRequest builds a multipart POST /documents request.
func uploadRequest(t *testing.T, field, filename string, content []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	part, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("creating form file: %v", err)
	}

	if _, err := part.Write(content); err != nil {
		t.Fatalf("writing form file: %v", err)
	}

	if err := mw.Close(); err != nil {
		t.Fatalf("closing multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/documents", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestHandleCreate(t *testing.T) {
	store := newFakeStore()
	q := &fakeQueue{}
	h := NewHandlers(store, q)

	rec := httptest.NewRecorder()
	h.HandleCreate(rec, uploadRequest(t, "file", "notes.md", []byte("# Kubernetes")))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusAccepted, rec.Body)
	}

	var got Document
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if got.Filename != "notes.md" {
		t.Errorf("filename = %q, want %q", got.Filename, "notes.md")
	}

	if got.Status != "pending" {
		t.Errorf("status = %q, want %q", got.Status, "pending")
	}

	if len(q.enqueued) != 1 || q.enqueued[0] != got.ID {
		t.Errorf("enqueued = %v, want [%d]", q.enqueued, got.ID)
	}

	if string(store.contents[got.ID]) != "# Kubernetes" {
		t.Errorf("stored content = %q, want %q", store.contents[got.ID], "# Kubernetes")
	}
}

func TestHandleCreateMissingFile(t *testing.T) {
	h := NewHandlers(newFakeStore(), &fakeQueue{})

	// A multipart body with the wrong field name — no "file" part.
	rec := httptest.NewRecorder()
	h.HandleCreate(rec, uploadRequest(t, "wrongfield", "notes.md", []byte("x")))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// A document that was stored but could not be queued is a lie waiting to
// happen: it would sit at 'pending' forever with no worker coming. The
// handler must report that as a failure, not a 202.
func TestHandleCreateEnqueueFailure(t *testing.T) {
	h := NewHandlers(newFakeStore(), &fakeQueue{err: errors.New("redis down")})

	rec := httptest.NewRecorder()
	h.HandleCreate(rec, uploadRequest(t, "file", "notes.md", []byte("x")))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleList(t *testing.T) {
	h := NewHandlers(newFakeStore(Document{Filename: "a.md"}, Document{Filename: "b.pdf"}), &fakeQueue{})

	rec := httptest.NewRecorder()
	h.HandleList(rec, httptest.NewRequest(http.MethodGet, "/documents", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got []Document
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("len(documents) = %d, want 2", len(got))
	}
}

func TestHandleGet(t *testing.T) {
	h := NewHandlers(newFakeStore(Document{Filename: "a.md", Status: "ready", ChunkCount: 3}), &fakeQueue{})

	tests := []struct {
		name     string
		id       string
		wantCode int
	}{
		{"found", "1", http.StatusOK},
		{"not found", "99", http.StatusNotFound},
		{"invalid id", "abc", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/documents/"+tt.id, nil)
			req.SetPathValue("id", tt.id)

			rec := httptest.NewRecorder()
			h.HandleGet(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantCode)
			}
		})
	}
}
