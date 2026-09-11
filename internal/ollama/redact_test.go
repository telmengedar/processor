package ollama

import (
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestOllamaJudgeRedactsUserinfoFromProviderEndpointOnTheSuccessPath(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, doneResponse)
	credentialed := "http://alice:s3cr3t@" + strings.TrimPrefix(srv.URL, "http://")

	c := NewClient(credentialed, "model-x", "", loop.Sampling{}, srv.Client())
	result := judgeOnce(t, c)

	if strings.Contains(result.Provider.Endpoint, "alice") || strings.Contains(result.Provider.Endpoint, "s3cr3t") {
		t.Fatalf("Provider.Endpoint still carries the credential: %q", result.Provider.Endpoint)
	}
	want := "http://redacted@" + strings.TrimPrefix(srv.URL, "http://") + "/api/chat"
	if result.Provider.Endpoint != want {
		t.Fatalf("Provider.Endpoint = %q, want %q", result.Provider.Endpoint, want)
	}
}
