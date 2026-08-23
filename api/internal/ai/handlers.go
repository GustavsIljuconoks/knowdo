package ai

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
)

type Handlers struct {
	asker Asker
}

func NewHandlers(asker Asker) *Handlers {
	return &Handlers{asker: asker}
}

func (h *Handlers) HandleAsk(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Question string `json:"question"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	question := strings.TrimSpace(body.Question)
	if question == "" {
		http.Error(w, "question is required", http.StatusBadRequest)
		return
	}

	answer, err := h.asker.Ask(r.Context(), question)
	if err != nil {
		log.Printf("asking ai service: %v", err)

		if errors.Is(err, ErrUnavailable) {
			http.Error(w, "ai service unavailable", http.StatusServiceUnavailable)
			return
		}

		http.Error(w, "failed to answer question", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(answer); err != nil {
		log.Printf("encoding response: %v", err)
	}
}
