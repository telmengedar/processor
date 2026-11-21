package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/telmengedar/processor/internal/process"
)

func handleProcessRuns(engine *process.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if engine == nil {
			w.Header().Set("Content-Type", jsonContentType)
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(process.Response{Status: "failed", Trace: []process.Trace{}, Error: &process.Error{Code: "process_unavailable", Message: "process engine is not configured"}})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		d.UseNumber()
		var request process.Request
		if d.Decode(&request) != nil || d.Decode(new(any)) != io.EOF || request.Arguments == nil {
			writeError(w, http.StatusBadRequest, codeInvalidRequest, "expected process selector and arguments object")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), RunBound)
		defer cancel()
		response := engine.Run(ctx, request)
		status := http.StatusOK
		if response.Error != nil {
			status = http.StatusUnprocessableEntity
			switch response.Error.Code {
			case "invalid_request", "invalid_arguments", "invalid_definition", "definition_changed", "process_not_unique":
				status = http.StatusBadRequest
			case "graph_unavailable", "model_unavailable":
				status = http.StatusBadGateway
			case "process_cancelled":
				status = http.StatusGatewayTimeout
			}
		}
		w.Header().Set("Content-Type", jsonContentType)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(response)
	}
}
