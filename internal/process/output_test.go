package process_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/telmengedar/processor/internal/process"
)

func TestProcessOutputStringsRespectVersionAndInvocationValidation(t *testing.T) {
	t.Parallel()
	for _, version := range []int{1, 2} {
		for _, tc := range []struct {
			name      string
			output    map[string]any
			input     string
			errorCode string
		}{
			{"empty", map[string]any{"text": "", "accepted": true}, "message", ""},
			{"whitespace", map[string]any{"text": " \t\n", "accepted": true}, "message", ""},
			{"nonblank", map[string]any{"text": "  unchanged text\n", "accepted": false}, "message", ""},
			{"missing", map[string]any{"accepted": true}, "message", "invalid_output"},
			{"unknown_instead_of_required", map[string]any{"other": "text", "accepted": true}, "message", "invalid_output"},
			{"extra", map[string]any{"text": "text", "accepted": true, "other": "extra"}, "message", "invalid_output"},
			{"wrong_string_type", map[string]any{"text": false, "accepted": true}, "message", "invalid_output"},
			{"null_string", map[string]any{"text": nil, "accepted": true}, "message", "invalid_output"},
			{"wrong_boolean_type", map[string]any{"text": "text", "accepted": "true"}, "message", "invalid_output"},
			{"empty_input", map[string]any{"text": "text", "accepted": true}, "", "invalid_arguments"},
			{"whitespace_input", map[string]any{"text": "text", "accepted": true}, " \t\n", "invalid_arguments"},
		} {
			t.Run(fmt.Sprintf("v%d/%s", version, tc.name), func(t *testing.T) {
				b := process.Blueprint{
					Version: version, Name: "Output validation", Entry: "end",
					Inputs:  map[string]process.Declaration{"message": {Type: "string"}},
					Outputs: map[string]process.Declaration{"text": {Type: "string"}, "accepted": {Type: "boolean"}},
					Nodes: []process.DraftNode{{Symbol: "end", Type: "process-step", Name: "end",
						Body: process.Step{Version: version, Key: "end", Kind: "return", Output: tc.output}}},
				}
				g, p := authored(t, b)
				r := (&process.Engine{Graph: g}).Run(context.Background(), process.Request{
					ProcessID: p.ProcessID, Arguments: map[string]any{"message": tc.input},
				})
				wantError := tc.errorCode
				if version == 1 && (tc.name == "empty" || tc.name == "whitespace") {
					wantError = "invalid_output"
				}
				if wantError == "" {
					if r.Status != "completed" || r.Error != nil || !reflect.DeepEqual(r.Output, tc.output) {
						t.Fatalf("output not retained exactly: %+v", r)
					}
				} else if r.Status != "failed" || r.Error == nil || r.Error.Code != wantError || r.Output != nil {
					t.Fatalf("expected %s with no output: %+v", wantError, r)
				}
				wantVisits := 1
				if wantError == "invalid_arguments" {
					wantVisits = 0
				}
				if len(r.Trace) != wantVisits || g.recalls != 0 {
					t.Fatalf("unexpected execution: visits=%d recalls=%d", len(r.Trace), g.recalls)
				}
			})
		}
	}
}
