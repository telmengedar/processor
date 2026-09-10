package openaicompat

import (
	"context"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestOpenAICompatJudgeRedactsUserinfoFromProviderEndpointOnTheSuccessPath(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, stopResponse)
	credentialed := "http://alice:s3cr3t@" + strings.TrimPrefix(srv.URL, "http://")

	c := NewClient(credentialed, "model-x", "", loop.Sampling{}, srv.Client())
	result, err := c.Judge(context.Background(), loop.JudgeInput{System: "sys", Block: "block", Input: "in"})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}

	if strings.Contains(result.Provider.Endpoint, "alice") || strings.Contains(result.Provider.Endpoint, "s3cr3t") {
		t.Fatalf("Provider.Endpoint still carries the credential: %q", result.Provider.Endpoint)
	}
	want := "http://redacted@" + strings.TrimPrefix(srv.URL, "http://") + "/chat/completions"
	if result.Provider.Endpoint != want {
		t.Fatalf("Provider.Endpoint = %q, want %q", result.Provider.Endpoint, want)
	}
}
