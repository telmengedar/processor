package process_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/completion"
	"github.com/telmengedar/processor/internal/process"
)

func TestPythonToolJSONBoundaryAndFailureBounds(t *testing.T) {
	t.Setenv("PROCESSOR_TEST_CREDENTIAL_SENTINEL", "must-not-reach-child")
	value, err := (process.Python{}).Run(context.Background(), "import json,os,sys\nv=json.load(sys.stdin)\njson.dump({'cwd':os.getcwd(),'credential':os.environ.get('PROCESSOR_TEST_CREDENTIAL_SENTINEL'),'text':v['text'].upper()},sys.stdout)", map[string]any{"text": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	if result["credential"] != nil || result["text"] != "HELLO" {
		t.Fatalf("child boundary result=%v", result)
	}
	if _, err = os.Stat(result["cwd"].(string)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("temporary working directory remains")
	}
	for _, test := range []struct {
		name, source, code string
		timeout            time.Duration
	}{
		{"invalid", "print('not json')", "tool_invalid_output", time.Second},
		{"multiple", "print('{} {}')", "tool_invalid_output", time.Second},
		{"exit", "raise SystemExit(7)", "tool_failed", time.Second},
		{"stdout", "print('x'*70000)", "tool_output_bound", time.Second},
		{"stderr", "import sys;sys.stderr.write('x'*10000)", "tool_output_bound", time.Second},
		{"timeout", "import time;time.sleep(60)", "tool_timeout", 50 * time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), test.timeout)
			defer cancel()
			_, err := (process.Python{}).Run(ctx, test.source, map[string]any{})
			var typed *process.Error
			if !errors.As(err, &typed) || typed.Code != test.code {
				t.Fatalf("got %v want %s", err, test.code)
			}
		})
	}
	_, err = (process.Python{Executable: "/no/such/python"}).Run(context.Background(), "print('{}')", map[string]any{})
	var typed *process.Error
	if !errors.As(err, &typed) || typed.Code != "tool_unavailable" {
		t.Fatalf("missing interpreter: %v", err)
	}
}

func TestAuthoredPythonGraphReturnsTransformedReply(t *testing.T) {
	t.Parallel()
	g, _ := authored(t, process.PythonExample())
	status, r := request(t, g, nil, `{"process":"Python transformation","arguments":{"text":"hello world"}}`)
	if status != 200 || r.Output["reply"] != "HELLO WORLD" || len(r.Trace) != 2 {
		t.Fatalf("%d %+v", status, r)
	}
}

func TestContextCutsWholeOptionalItemsAndRejectsOversizedRequiredInput(t *testing.T) {
	t.Parallel()
	b := process.GangolfChat("knowledge")
	for i := range b.Nodes {
		if b.Nodes[i].Symbol == "assess_situation" {
			body := b.Nodes[i].Body.(map[string]any)
			body["context"] = []any{map[string]any{"key": "oversized", "value": strings.Repeat("x", 40000), "required": false}, map[string]any{"key": "required", "value": "distinct-required", "required": true}}
		}
	}
	g, _ := authored(t, b)
	calls := 0
	m := modelFunc(func(_ context.Context, r completion.Request) (completion.Result, error) {
		calls++
		raw, _ := json.Marshal(r.Messages)
		if strings.Contains(string(raw), strings.Repeat("x", 100)) {
			t.Fatal("cut context leaked")
		}
		if calls == 1 && !strings.Contains(string(raw), "distinct-required") {
			t.Fatal("required context missing")
		}
		return completion.Result{Text: "reply"}, nil
	})
	status, r := request(t, g, m, `{"process":"Gangolf Chat","arguments":{"message":"hello","persona":90}}`)
	if status != 200 || len(r.Trace[3].Context) != 2 || r.Trace[3].Context[0].Included || !r.Trace[3].Context[1].Included {
		t.Fatalf("%d %+v", status, r)
	}
	body, _ := json.Marshal(map[string]any{"process": "Gangolf Chat", "arguments": map[string]any{"message": strings.Repeat("z", 34000), "persona": 90}})
	before := calls
	status, r = request(t, g, m, string(body))
	if status == 200 || r.Error.Code != "context_bound" || calls != before {
		t.Fatalf("oversized required input accepted: %d %+v", status, r)
	}
}
