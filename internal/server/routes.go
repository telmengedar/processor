// Package server implements the Processor HTTP surface.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/runconfig"
)

// NewHandler builds the routes for GET /health and POST /runs over turn.
func NewHandler(turn *loop.Turn) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth(turn.Build))
	mux.HandleFunc("POST /runs", handleRuns(turn))
	return mux
}

type healthResponse struct {
	Status string `json:"status"`
	Build  string `json:"build,omitempty"`
}

func handleHealth(build string) http.HandlerFunc {
	body, _ := json.Marshal(healthResponse{Status: "ok", Build: build})
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
}

type runRequest struct {
	Input   string          `json:"input"`
	Subject int64           `json:"subject"`
	History []loop.Exchange `json:"history"`

	Config *int64 `json:"config"`
}

const jsonContentType = "application/json; charset=utf-8"

const (
	codeInvalidRequest      = "invalid_request"
	codeConfigNotFound      = "config_not_found"
	codeInvalidConfig       = "invalid_config"
	codeSubjectNotFound     = "subject_not_found"
	codeGraphUnavailable    = "graph_unavailable"
	codeModelUnavailable    = "model_unavailable"
	codeRunDeadlineExceeded = "run_deadline_exceeded"
)

const messageInvalidHistory = "history must be an array of exchanges, each with a non-blank string input and an optional string reply"

const fallbackReply = "Sorry — I couldn't put an answer together for that just now. Could you say a bit more, or ask it another way?"

type chatResponse struct {
	Reply    string `json:"reply"`
	Produced bool   `json:"produced"`
	Silent   bool   `json:"silent,omitempty"`
}

type verboseResponse struct {
	loop.Record
	Written loop.WriteReceipt `json:"written"`
	Reply   string            `json:"reply"`
}

func replyFor(record loop.Record) string {
	if record.Outcome.Silent {
		return ""
	}
	if record.Outcome.Produced {
		return record.Answer
	}
	return fallbackReply
}

func verboseRequested(r *http.Request) (bool, error) {
	values, present := r.URL.Query()["verbose"]
	if !present || values[0] == "" {
		return present, nil
	}
	return strconv.ParseBool(values[0])
}

const maxRequestBodyBytes = 1 << 20

func handleRuns(turn *loop.Turn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		verbose, err := verboseRequested(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "verbose must be a boolean")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

		var req runRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			var mistyped *json.UnmarshalTypeError
			if errors.As(err, &mistyped) && strings.HasPrefix(mistyped.Field, "history") {
				writeError(w, http.StatusBadRequest, codeInvalidRequest, messageInvalidHistory)
				return
			}
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "request body could not be parsed as JSON")
			return
		}
		if strings.TrimSpace(req.Input) == "" {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "input must not be empty")
			return
		}
		if req.Subject <= 0 {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "subject is required")
			return
		}

		if slices.ContainsFunc(req.History, func(e loop.Exchange) bool { return strings.TrimSpace(e.Input) == "" }) {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, messageInvalidHistory)
			return
		}

		if req.Config != nil && *req.Config <= 0 {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "config must be a node id")
			return
		}

		runCtx, cancel := context.WithTimeout(r.Context(), RunBound)
		defer cancel()

		var view *loop.View
		if req.Config != nil {
			var failed bool
			if view, failed = resolveConfig(runCtx, w, turn, *req.Config); failed {
				return
			}
		}

		record, started, err := turn.Answer(runCtx, loop.RunOptions{Input: req.Input, Subject: req.Subject, History: req.History, Settings: loop.DefaultSettings(), View: view})
		if err != nil {
			switch {
			case errors.Is(runCtx.Err(), context.DeadlineExceeded):
				writeError(w, http.StatusGatewayTimeout, codeRunDeadlineExceeded, "the run did not produce an answer within the service's time limit")
			case errors.Is(err, loop.ErrSubjectNotFound):
				writeError(w, http.StatusNotFound, codeSubjectNotFound, "the subject node was not found")
			case errors.Is(err, loop.ErrRunTimeExhausted):
				writeError(w, http.StatusGatewayTimeout, codeRunDeadlineExceeded, withCause("the run had too little time left to afford a judgement call, so none was made", err, loop.ErrRunTimeExhausted))
			case errors.Is(err, loop.ErrInvalidConfig):
				writeError(w, http.StatusBadRequest, codeInvalidConfig, withCause("the graph answered 400 or 414 to the first recall of a run holding a config filter", err, loop.ErrInvalidConfig))
			case errors.Is(err, loop.ErrModelUnavailable):
				writeError(w, http.StatusBadGateway, codeModelUnavailable, withCause("the model call did not complete", err, loop.ErrModelUnavailable))
			default:
				writeError(w, http.StatusBadGateway, codeGraphUnavailable, withCause("the graph could not be read", err, loop.ErrGraphUnavailable))
			}
			return
		}

		reply := replyFor(record)
		if verbose {
			receipt := turn.File(runCtx, record, started)
			w.Header().Set("Content-Type", jsonContentType)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(verboseResponse{Record: record, Written: receipt, Reply: reply})
			return
		}

		sendComplete(w, chatResponse{Reply: reply, Produced: record.Outcome.Produced, Silent: record.Outcome.Silent})
		turn.File(runCtx, record, started)
	}
}

func sendComplete(w http.ResponseWriter, body any) {
	var buffer bytes.Buffer
	_ = json.NewEncoder(&buffer).Encode(body)
	w.Header().Set("Content-Type", jsonContentType)
	w.Header().Set("Content-Length", strconv.Itoa(buffer.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buffer.Bytes())
	_ = http.NewResponseController(w).Flush()
}

func resolveConfig(ctx context.Context, w http.ResponseWriter, turn *loop.Turn, id int64) (view *loop.View, failed bool) {
	node, found, err := turn.Graph.Node(ctx, id, runconfig.ReadFilter())
	if err != nil {
		writeError(w, http.StatusBadGateway, codeGraphUnavailable, "the graph could not be read: "+loop.BoundCause(err.Error()))
		return nil, true
	}
	if !found {
		writeError(w, http.StatusNotFound, codeConfigNotFound, "the config node was not found")
		return nil, true
	}

	config, err := runconfig.Decode(node.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidConfig, loop.BoundCause(err.Error()))
		return nil, true
	}
	return &loop.View{Config: id, Filter: config.Filter, Silence: config.Silence, Identity: config.Identity, Note: config.Note, Temperature: config.Temperature, Form: config.Form, Root: node.RootNodeID}, false
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func withCause(class string, err, sentinel error) string {
	text := err.Error()
	if text == sentinel.Error() {
		return class
	}
	return class + ": " + loop.BoundCause(strings.TrimPrefix(text, sentinel.Error()+": "))
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: errorBody{Code: code, Message: message}})
}
