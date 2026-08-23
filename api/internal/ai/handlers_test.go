package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAsker stands in for the Python service.
type fakeAsker struct {
	question string
	response Response
	err      error
}

func (fa *fakeAsker) Ask(ctx context.Context, question string) (Response, error) {
	fa.question = question

	if fa.err != nil {
		return Response{}, fa.err
	}

	return fa.response, nil
}

func TestHandleAsk(t *testing.T) {
	asker := &fakeAsker{response: Response{Answer: "Pods are ephemeral.", Sources: []int64{3}}}
	h := NewHandlers(asker)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"question":"What are Pods?"}`))
	h.HandleAsk(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body)
	}

	var got Response
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if got.Answer != "Pods are ephemeral." {
		t.Errorf("answer = %q, want %q", got.Answer, "Pods are ephemeral.")
	}

	if asker.question != "What are Pods?" {
		t.Errorf("question passed through = %q, want %q", asker.question, "What are Pods?")
	}
}

func TestHandleAskValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty question", `{"question":""}`},
		{"whitespace question", `{"question":"   "}`},
		{"missing question", `{}`},
		{"malformed json", `{"question":`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(&fakeAsker{})

			rec := httptest.NewRecorder()
			h.HandleAsk(rec, httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(tt.body)))

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

// An unreachable AI service is 503, not 500 — the distinction is the
// one that matters when the stack is half-up locally.
func TestHandleAskServiceUnavailable(t *testing.T) {
	h := NewHandlers(&fakeAsker{err: ErrUnavailable})

	rec := httptest.NewRecorder()
	h.HandleAsk(rec, httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"question":"hi"}`)))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleAskOtherError(t *testing.T) {
	h := NewHandlers(&fakeAsker{err: errors.New("boom")})

	rec := httptest.NewRecorder()
	h.HandleAsk(rec, httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"question":"hi"}`)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// The client half: a real HTTP round trip against a stub server.
func TestClientAsk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ask" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/ask")
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(Response{Answer: "yes", Sources: []int64{1, 2}}); err != nil {
			t.Fatalf("encoding stub response: %v", err)
		}
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL).Ask(context.Background(), "really?")
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}

	if got.Answer != "yes" || len(got.Sources) != 2 {
		t.Errorf("got %+v, want answer 'yes' with 2 sources", got)
	}
}

func TestClientAskUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // closed immediately: nothing is listening

	_, err := NewClient(srv.URL).Ask(context.Background(), "hi")
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("error = %v, want ErrUnavailable", err)
	}
}
