// Package server implements the Processor HTTP surface.
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
)

// NewHandler builds the HTTP handler for the Processor API route table.
// turn is the loop that POST /runs delegates to — the handler takes the
// loop itself, not an abstraction of it (design §9.2), and turn arrives as
// the handler-building function's first parameter (design §5.5, #10466
// archetype C).
func NewHandler(turn *loop.Turn) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /runs", handleRuns(turn))
	return mux
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// runRequest is the POST /runs request body.
type runRequest struct {
	Input   string `json:"input"`
	Subject int64  `json:"subject"`
}

const (
	codeInvalidRequest   = "invalid_request"
	codeSubjectNotFound  = "subject_not_found"
	codeGraphUnavailable = "graph_unavailable"
	codeModelUnavailable = "model_unavailable"
)

// maxRequestBodyBytes bounds POST /runs' request body (W-6): generous for
// the two-field {input, subject} shape, small enough that an unbounded
// body can't be used to exhaust memory before decoding ever inspects it.
const maxRequestBodyBytes = 1 << 20 // 1 MiB

func handleRuns(turn *loop.Turn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

		var req runRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "request body could not be parsed as JSON")
			return
		}
		if strings.TrimSpace(req.Input) == "" {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "input must not be empty")
			return
		}
		// design line 374: subject missing or malformed → 400. Absent JSON
		// decodes a non-pointer int64 to 0, and node ids are never <= 0
		// (W-9), so both "missing" and "malformed" (negative) collapse to
		// the same check with no false positive.
		if req.Subject <= 0 {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "subject is required")
			return
		}

		record, err := turn.Run(r.Context(), req.Input, req.Subject)
		if err != nil {
			switch {
			case errors.Is(err, loop.ErrSubjectNotFound):
				writeError(w, http.StatusNotFound, codeSubjectNotFound, "the subject node was not found")
			case errors.Is(err, loop.ErrModelUnavailable):
				writeError(w, http.StatusBadGateway, codeModelUnavailable, "the model call did not complete")
			default:
				writeError(w, http.StatusBadGateway, codeGraphUnavailable, "the graph could not be read")
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(record)
	}
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: errorBody{Code: code, Message: message}})
}
