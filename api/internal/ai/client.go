// Package ai talks to the internal Python AI service.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ErrUnavailable means the AI service could not be reached or did not
// answer in time — distinct from it answering with an error, because
// only one of those is fixed by starting the container.
var ErrUnavailable = errors.New("ai service unavailable")

// Response is the shape both the Python service and this API return.
type Response struct {
	Answer  string  `json:"answer"`
	Sources []int64 `json:"sources"`
}

// Asker is the boundary the handler depends on, so tests need no server.
type Asker interface {
	Ask(ctx context.Context, question string) (Response, error)
}

// Plan is the structured plan the Python /plan endpoint returns.
type Plan struct {
	Goal  string        `json:"goal"`
	Tasks []PlannedTask `json:"tasks"`
}

// PlannedTask's DueDate is a "2006-01-02" date string — the worker
// already resolved the model's relative day_offset against today, so
// this side just parses it, it doesn't compute it.
type PlannedTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	DueDate     string `json:"due_date"`
}

// Planner is the boundary the handler depends on, so tests need no server.
type Planner interface {
	Plan(ctx context.Context, request string) (Plan, error)
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		// Retrieval plus one LLM call. ponytail: a flat timeout — if
		// streaming arrives in a later stage this becomes a stream proxy
		// and the timeout moves to the first byte.
		http: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Ask(ctx context.Context, question string) (Response, error) {
	var out Response
	err := c.post(ctx, "/ask", map[string]string{"question": question}, &out)
	return out, err
}

func (c *Client) Plan(ctx context.Context, request string) (Plan, error) {
	var out Plan
	err := c.post(ctx, "/plan", map[string]string{"request": request}, &out)
	return out, err
}

// post sends reqBody as JSON to path and decodes the JSON response into out.
func (c *Client) post(ctx context.Context, path string, reqBody, out any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ai service returned %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
