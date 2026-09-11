package ollama

import (
	"bytes"
	"context"
	"errors"
	"net/http"
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

const nativeChatRoute = "/api/chat"

const (
	credentialSentinel   = "sk-secretkey"
	unreachableBase      = "http://" + credentialSentinel + "@127.0.0.1:1/v1"
	unparseableBase      = "http://" + credentialSentinel + "@[::1/v1"
	unreachableAuthority = "127.0.0.1:1"
	unparseableAuthority = "[::1"
)

type refusingTransport struct{}

func (refusingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("transport refused")
}

func refusingClient() *http.Client {
	return &http.Client{Transport: refusingTransport{}}
}

func unredactedReference(t *testing.T, composed string) string {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, composed, bytes.NewReader([]byte("{}")))
	if err != nil {
		return "build request: " + err.Error()
	}
	if _, doErr := refusingClient().Do(req); doErr != nil {
		return "request failed: " + doErr.Error()
	}
	t.Fatalf("the reference request against %q succeeded, so no error path was exercised", composed)
	return ""
}

func assertCredentialRedacted(t *testing.T, err error, composed, mustKeep string) {
	t.Helper()
	if err == nil {
		t.Fatalf("the call against %q returned no error, so no error path was exercised", composed)
	}
	if reference := unredactedReference(t, composed); !strings.Contains(reference, credentialSentinel) {
		t.Fatalf("fixture stopped demonstrating the gap: net/http's own error for %q no longer carries %q: %q", composed, credentialSentinel, reference)
	}
	if strings.Contains(err.Error(), credentialSentinel) {
		t.Fatalf("the returned error still carries the credential: %q", err.Error())
	}
	if !strings.Contains(err.Error(), mustKeep) {
		t.Fatalf("the returned error lost %q, so two failing endpoints are no longer distinguishable: %q", mustKeep, err.Error())
	}
}

func judgeInput() loop.JudgeInput {
	return loop.JudgeInput{System: "sys", Block: "block", Input: "in"}
}

func TestOllamaJudgeRedactsTheCredentialFromTheBuildRequestError(t *testing.T) {
	t.Parallel()

	c := NewClient(unparseableBase, "model-x", "", loop.Sampling{}, refusingClient())
	_, err := c.Judge(context.Background(), judgeInput())
	assertCredentialRedacted(t, err, unparseableBase+nativeChatRoute, unparseableAuthority)
}

func TestOllamaJudgeRedactsTheCredentialFromTheRequestFailedError(t *testing.T) {
	t.Parallel()

	c := NewClient(unreachableBase, "model-x", "", loop.Sampling{}, refusingClient())
	_, err := c.Judge(context.Background(), judgeInput())
	assertCredentialRedacted(t, err, unreachableBase+nativeChatRoute, unreachableAuthority)
}

func TestOllamaCondenseRedactsTheCredentialFromTheBuildRequestError(t *testing.T) {
	t.Parallel()

	c := NewClient(unparseableBase, "model-x", "", loop.Sampling{}, refusingClient())
	_, err := c.Condense(context.Background(), "prompt", 64)
	assertCredentialRedacted(t, err, unparseableBase+nativeChatRoute, unparseableAuthority)
}

func TestOllamaCondenseRedactsTheCredentialFromTheRequestFailedError(t *testing.T) {
	t.Parallel()

	c := NewClient(unreachableBase, "model-x", "", loop.Sampling{}, refusingClient())
	_, err := c.Condense(context.Background(), "prompt", 64)
	assertCredentialRedacted(t, err, unreachableBase+nativeChatRoute, unreachableAuthority)
}

var endpointsCarryingNoUserinfo = []string{
	"http://localhost:11434",
	"http://localhost:11434/api",
	"https://api.openai.com/v1",
	"https://divoid.mamgo.io/api",
	"http://[::1]:11434/api",
	"http://[fe80::1%25eth0]:8080/v1",
	"https://host.example/path/user@example/x",
	"https://host.example/v1?email=foo@bar.com",
	"https://host.example/v1#frag@x",
	"https://host.example/v1?a=b@c#d@e",
	"//host.example/v1",
	"host.example:11434",
	"http:///v1",
	"HTTPS://HOST.EXAMPLE/V1",
	"http://ho st.example/x?email=foo@bar.com",
	"",
	"http://host.example",
	"https://sub.domain-with-dash.example:443/a/b/c?x=1&y=2",
}

func TestOllamaRedactionIsANoOpOnTheErrorTextOfAnEndpointCarryingNoUserinfo(t *testing.T) {
	t.Parallel()

	for _, base := range endpointsCarryingNoUserinfo {
		composed := strings.TrimRight(base, "/") + nativeChatRoute
		want := unredactedReference(t, composed)
		c := NewClient(base, "model-x", "", loop.Sampling{}, refusingClient())

		_, judgeErr := c.Judge(context.Background(), judgeInput())
		carried := errors.Unwrap(judgeErr)
		if carried == nil {
			t.Fatalf("Judge against %q returned %q, which wraps no cause at all", base, judgeErr)
		}
		if carried.Error() != want {
			t.Fatalf("Judge against %q carried the cause %q, want the unredacted text %q", base, carried.Error(), want)
		}

		_, condenseErr := c.Condense(context.Background(), "prompt", 64)
		if condenseErr.Error() != "ollama: "+want {
			t.Fatalf("Condense against %q returned %q, want the unredacted text %q", base, condenseErr.Error(), "ollama: "+want)
		}
	}
}
