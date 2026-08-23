package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"knowdo/api/internal/task"
)

// fakePlanner stands in for the Python service.
type fakePlanner struct {
	request string
	plan    Plan
	err     error
}

func (fp *fakePlanner) Plan(ctx context.Context, request string) (Plan, error) {
	fp.request = request

	if fp.err != nil {
		return Plan{}, fp.err
	}

	return fp.plan, nil
}

// fakeTaskStore is enough of task.Store to observe what HandlePlan saved.
type fakeTaskStore struct {
	created []task.Task
	err     error
}

func (f *fakeTaskStore) Create(ctx context.Context, t *task.Task) error {
	if f.err != nil {
		return f.err
	}

	t.ID = int64(len(f.created) + 1)
	t.Status = "pending"
	f.created = append(f.created, *t)

	return nil
}

func (f *fakeTaskStore) List(ctx context.Context) ([]task.Task, error) { return f.created, nil }

func (f *fakeTaskStore) Get(ctx context.Context, id int64) (task.Task, error) {
	return task.Task{}, task.ErrNotFound
}

func (f *fakeTaskStore) Update(ctx context.Context, id int64, patch task.Patch) (task.Task, error) {
	return task.Task{}, task.ErrNotFound
}

func (f *fakeTaskStore) Delete(ctx context.Context, id int64) error { return task.ErrNotFound }

func TestHandlePlan(t *testing.T) {
	planner := &fakePlanner{plan: Plan{
		Goal: "learn kubernetes",
		Tasks: []PlannedTask{
			{Title: "pods", Description: "learn pods", DueDate: "2026-01-03"},
			{Title: "deployments", Description: "learn deployments", DueDate: "2026-01-05"},
		},
	}}
	store := &fakeTaskStore{}
	h := NewPlanHandlers(planner, store)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(`{"request":"learn kubernetes"}`))
	h.HandlePlan(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body)
	}

	var got []task.Task
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("created %d tasks, want 2", len(got))
	}

	if got[0].Title != "pods" || got[0].DueDate == nil || !got[0].DueDate.Equal(time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("task[0] = %+v, want title=pods due_date=2026-01-03", got[0])
	}

	if len(store.created) != 2 {
		t.Errorf("store saved %d tasks, want 2", len(store.created))
	}

	if planner.request != "learn kubernetes" {
		t.Errorf("request passed through = %q, want %q", planner.request, "learn kubernetes")
	}
}

func TestHandlePlanValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty request", `{"request":""}`},
		{"whitespace request", `{"request":"   "}`},
		{"missing request", `{}`},
		{"malformed json", `{"request":`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewPlanHandlers(&fakePlanner{}, &fakeTaskStore{})

			rec := httptest.NewRecorder()
			h.HandlePlan(rec, httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(tt.body)))

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandlePlanServiceUnavailable(t *testing.T) {
	h := NewPlanHandlers(&fakePlanner{err: ErrUnavailable}, &fakeTaskStore{})

	rec := httptest.NewRecorder()
	h.HandlePlan(rec, httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(`{"request":"hi"}`)))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandlePlanOtherError(t *testing.T) {
	h := NewPlanHandlers(&fakePlanner{err: errors.New("boom")}, &fakeTaskStore{})

	rec := httptest.NewRecorder()
	h.HandlePlan(rec, httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(`{"request":"hi"}`)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandlePlanInvalidDueDate(t *testing.T) {
	planner := &fakePlanner{plan: Plan{
		Goal:  "x",
		Tasks: []PlannedTask{{Title: "bad", DueDate: "not-a-date"}},
	}}
	h := NewPlanHandlers(planner, &fakeTaskStore{})

	rec := httptest.NewRecorder()
	h.HandlePlan(rec, httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(`{"request":"hi"}`)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandlePlanStoreError(t *testing.T) {
	planner := &fakePlanner{plan: Plan{
		Goal:  "x",
		Tasks: []PlannedTask{{Title: "a", DueDate: "2026-01-03"}},
	}}
	store := &fakeTaskStore{err: errors.New("db down")}
	h := NewPlanHandlers(planner, store)

	rec := httptest.NewRecorder()
	h.HandlePlan(rec, httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(`{"request":"hi"}`)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// The client half: a real HTTP round trip against a stub server.
func TestClientPlan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/plan" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/plan")
		}

		w.Header().Set("Content-Type", "application/json")
		out := Plan{Goal: "learn k8s", Tasks: []PlannedTask{{Title: "pods", DueDate: "2026-01-03"}}}
		if err := json.NewEncoder(w).Encode(out); err != nil {
			t.Fatalf("encoding stub response: %v", err)
		}
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL).Plan(context.Background(), "learn kubernetes")
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}

	if got.Goal != "learn k8s" || len(got.Tasks) != 1 {
		t.Errorf("got %+v, want goal 'learn k8s' with 1 task", got)
	}
}

func TestClientPlanUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // closed immediately: nothing is listening

	_, err := NewClient(srv.URL).Plan(context.Background(), "hi")
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("error = %v, want ErrUnavailable", err)
	}
}
