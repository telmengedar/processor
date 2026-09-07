package ollama_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/ollama"
)

const cannedDoneResponse = `{"message":{"role":"assistant","content":"served by the supplied transport"},"done":true,"done_reason":"stop"}`

type recordingTransport struct {
	calls int
	url   string
	auth  string
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.calls++
	rt.url = req.URL.String()
	rt.auth = req.Header.Get("Authorization")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(cannedDoneResponse))),
		Request:    req,
	}, nil
}

func TestOllamaClientBuiltByAnExternalKeyedLiteralSurvivesTheNilHTTPClientBranch(t *testing.T) {
	t.Parallel()

	c := ollama.Client{}

	result, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"})
	if err == nil {
		t.Fatal("Judge on a zero-value Client returned no error, want a transport error")
	}
	const wantPrefix = "ollama: request failed:"
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Fatalf("Judge on a zero-value Client failed with %q, want a %q error — any other prefix means it never reached the guarded call", err, wantPrefix)
	}
	if result.Reason != "" || result.Answer != "" {
		t.Fatalf("Judge on a zero-value Client returned %+v, want the zero result alongside its error", result)
	}
}

func TestOllamaJudgeSendsItsRequestThroughTheHTTPClientSuppliedToNewClient(t *testing.T) {
	t.Parallel()

	rt := &recordingTransport{}
	c := ollama.NewClient("http://model.invalid", "m", "supplied-key", loop.Sampling{}, &http.Client{Transport: rt})

	result, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if result.Answer != "served by the supplied transport" {
		t.Fatalf("Answer = %q, want the answer the supplied transport served", result.Answer)
	}
	if rt.calls != 1 {
		t.Fatalf("supplied transport saw %d calls, want 1", rt.calls)
	}
	if rt.url != "http://model.invalid/api/chat" {
		t.Fatalf("supplied transport saw URL %q, want the native route beneath the base given to NewClient", rt.url)
	}
	if rt.auth != "Bearer supplied-key" {
		t.Fatalf("supplied transport saw Authorization %q, want the key given to NewClient", rt.auth)
	}
}
