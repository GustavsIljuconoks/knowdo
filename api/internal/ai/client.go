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
	body, err := json.Marshal(map[string]string{"question": question})
	if err != nil {
		return Response{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ask", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("ai service returned %s", resp.Status)
	}

	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, err
	}

	return out, nil
}
