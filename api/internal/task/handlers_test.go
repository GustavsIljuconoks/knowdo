package task

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeStore is an in-memory Store used so handler tests don't need a
// real Postgres connection.
type fakeStore struct {
	tasks  map[int64]Task
	nextID int64
}

func newFakeStore(seed ...Task) *fakeStore {
	fs := &fakeStore{tasks: map[int64]Task{}}

	for _, t := range seed {
		fs.nextID++
		t.ID = fs.nextID
		fs.tasks[t.ID] = t
	}
	return fs
}

func (fs *fakeStore) Create(ctx context.Context, t *Task) error {
	fs.nextID++
	t.ID = fs.nextID
	t.Status = "open"
	fs.tasks[t.ID] = *t
	return nil
}

func (fs *fakeStore) List(ctx context.Context) ([]Task, error) {
	tasks := make([]Task, 0, len(fs.tasks))

	for _, t := range fs.tasks {
		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (fs *fakeStore) Get(ctx context.Context, id int64) (Task, error) {
	t, ok := fs.tasks[id]

	if !ok {
		return Task{}, ErrNotFound
	}

	return t, nil
}

func (fs *fakeStore) Update(ctx context.Context, id int64, patch Patch) (Task, error) {
	t, ok := fs.tasks[id]

	if !ok {
		return Task{}, ErrNotFound
	}

	if patch.Title != nil {
		t.Title = *patch.Title
	}

	if patch.Description != nil {
		t.Description = *patch.Description
	}

	if patch.Status != nil {
		t.Status = *patch.Status
	}

	if patch.DueDate != nil {
		t.DueDate = patch.DueDate
	}

	fs.tasks[id] = t
	return t, nil
}

func (fs *fakeStore) Delete(ctx context.Context, id int64) error {
	if _, ok := fs.tasks[id]; !ok {
		return ErrNotFound
	}

	delete(fs.tasks, id)
	return nil
}

// do runs handler against a synthetic request, optionally setting the
// {id} path value the way the real mux would after matching a pattern
// like "GET /tasks/{id}".
func do(handler http.HandlerFunc, method, target, body, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if id != "" {
		req.SetPathValue("id", id)
	}

	rec := httptest.NewRecorder()
	handler(rec, req)

	return rec
}

func TestHandleCreate(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"valid", `{"title":"Learn Kubernetes"}`, http.StatusCreated},
		{"missing title", `{"description":"no title"}`, http.StatusBadRequest},
		{"invalid json", `{`, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandlers(newFakeStore())

			rec := do(h.HandleCreate, http.MethodPost, "/tasks", tc.body, "")
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body)
			}
		})
	}
}

func TestHandleList(t *testing.T) {
	h := NewHandlers(newFakeStore(Task{Title: "a"}, Task{Title: "b"}))

	rec := do(h.HandleList, http.MethodGet, "/tasks", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"a"`) || !strings.Contains(rec.Body.String(), `"b"`) {
		t.Fatalf("body missing seeded tasks: %s", rec.Body)
	}
}

func TestHandleGet(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		wantStatus int
	}{
		{"found", "1", http.StatusOK},
		{"not found", "999", http.StatusNotFound},
		{"invalid id", "abc", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandlers(newFakeStore(Task{Title: "a"}))

			rec := do(h.HandleGet, http.MethodGet, "/tasks/"+tc.id, "", tc.id)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body)
			}
		})
	}
}

func TestHandleUpdate(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		body       string
		wantStatus int
	}{
		{"found", "1", `{"status":"done"}`, http.StatusOK},
		{"not found", "999", `{"status":"done"}`, http.StatusNotFound},
		{"invalid json", "1", `{`, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandlers(newFakeStore(Task{Title: "a", Status: "open"}))

			rec := do(h.HandleUpdate, http.MethodPatch, "/tasks/"+tc.id, tc.body, tc.id)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body)
			}
		})
	}

	t.Run("only patches given fields", func(t *testing.T) {
		h := NewHandlers(newFakeStore(Task{Title: "keep me", Status: "open"}))

		rec := do(h.HandleUpdate, http.MethodPatch, "/tasks/1", `{"status":"done"}`, "1")
		if !strings.Contains(rec.Body.String(), `"keep me"`) {
			t.Fatalf("title should be unchanged, got: %s", rec.Body)
		}
	})
}

func TestHandleDelete(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		wantStatus int
	}{
		{"found", "1", http.StatusNoContent},
		{"not found", "999", http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandlers(newFakeStore(Task{Title: "a"}))

			rec := do(h.HandleDelete, http.MethodDelete, "/tasks/"+tc.id, "", tc.id)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body)
			}
		})
	}
}
