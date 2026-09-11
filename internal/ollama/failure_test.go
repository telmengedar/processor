package ollama

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

const upstreamRejection = "The model produced output that does not match the expected peg-native format"

func respondingServer(t *testing.T, status int, body string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Path = r.URL.Path
		captured.Body, _ = readAllBody(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func readAllBody(r *http.Request) ([]byte, error) {
	defer func() { _ = r.Body.Close() }()
	buf := make([]byte, 0, 1024)
	chunk := make([]byte, 1024)
	for {
		n, err := r.Body.Read(chunk)
		buf = append(buf, chunk[:n]...)
		if err != nil {
			return buf, nil
		}
	}
}

func notANumber() *float64 {
	nan := math.NaN()
	return &nan
}

func judgeFailure(t *testing.T, c *Client) error {
	t.Helper()
	_, err := c.Judge(context.Background(), judgeInput())
	if err == nil {
		t.Fatal("Judge returned no error although the call was made to fail")
	}
	return err
}

var ollamaPreamble = regexp.MustCompile(`^ollama: model=([^ ]*) endpoint=(\S+) request=(\d+) B elapsed=(\S+) \(client bound ([^)]*)\): `)

func preambleFields(t *testing.T, err error) []string {
	t.Helper()
	fields := ollamaPreamble.FindStringSubmatch(err.Error())
	if fields == nil {
		t.Fatalf("the failure carries no preamble at its outermost point: %q", err.Error())
	}
	return fields
}

func TestOllamaJudgeAttachesTheFailurePreambleToEveryFailureMode(t *testing.T) {
	t.Parallel()

	badJSON, _ := respondingServer(t, http.StatusOK, `{"message":`)
	rejecting, _ := respondingServer(t, http.StatusInternalServerError, `{"error":"`+upstreamRejection+`"}`)

	modes := []struct {
		mode      string
		client    *Client
		wantCause string
	}{
		{"encode", NewClient("http://host.example", "model-x", "", loop.Sampling{Temperature: notANumber()}, refusingClient()), "encode request: "},
		{"build", NewClient(unparseableBase, "model-x", "", loop.Sampling{}, refusingClient()), "build request: "},
		{"transport", NewClient("http://host.example", "model-x", "", loop.Sampling{}, refusingClient()), "request failed: "},
		{"non-2xx", NewClient(rejecting.URL, "model-x", "", loop.Sampling{}, rejecting.Client()), "unexpected status 500: "},
		{"decode", NewClient(badJSON.URL, "model-x", "", loop.Sampling{}, badJSON.Client()), "decode response: "},
	}

	for _, mode := range modes {
		t.Run(mode.mode, func(t *testing.T) {
			err := judgeFailure(t, mode.client)
			if !ollamaPreamble.MatchString(err.Error()) {
				t.Fatalf("the %s failure carries no preamble, so the wrap covers only the modes it sits inside: %q", mode.mode, err.Error())
			}
			cause := ollamaPreamble.ReplaceAllString(err.Error(), "")
			if !strings.HasPrefix(cause, mode.wantCause) {
				t.Fatalf("the fixture named %s reached a different branch: cause is %q, want one opening %q", mode.mode, cause, mode.wantCause)
			}
		})
	}
}

func TestOllamaJudgeCarriesTheUpstreamsOwnSentenceBehindThePreamble(t *testing.T) {
	t.Parallel()

	srv, _ := respondingServer(t, http.StatusInternalServerError, `{"error":"`+upstreamRejection+`"}`)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	err := judgeFailure(t, c)
	if !strings.Contains(err.Error(), upstreamRejection) {
		t.Fatalf("the failure discarded the upstream's own sentence, which is the whole of the information: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "unexpected status 500") {
		t.Fatalf("the failure does not name the status the endpoint returned: %q", err.Error())
	}
}

func TestOllamaJudgeFailurePreambleNamesTheModelItAskedFor(t *testing.T) {
	t.Parallel()

	c := NewClient("http://host.example", "qwen3-coder:30b", "", loop.Sampling{}, refusingClient())

	if got := preambleFields(t, judgeFailure(t, c))[1]; got != "qwen3-coder:30b" {
		t.Fatalf("the preamble names model %q, want the model id the client was built with", got)
	}
}

func TestOllamaJudgeFailurePreambleNamesTheRouteBeneathTheBaseURL(t *testing.T) {
	t.Parallel()

	c := NewClient("http://host.example/prefix", "model-x", "", loop.Sampling{}, refusingClient())

	want := "http://host.example/prefix" + nativeChatRoute
	if got := preambleFields(t, judgeFailure(t, c))[2]; got != want {
		t.Fatalf("the preamble names endpoint %q, want the composed endpoint %q", got, want)
	}
}

func TestOllamaJudgeFailurePreambleRedactsUserinfoFromTheEndpointItNames(t *testing.T) {
	t.Parallel()

	c := NewClient(unreachableBase, "model-x", "", loop.Sampling{}, refusingClient())
	endpoint := preambleFields(t, judgeFailure(t, c))[2]

	if strings.Contains(endpoint, credentialSentinel) {
		t.Fatalf("the preamble's endpoint still carries the credential: %q", endpoint)
	}
	if !strings.Contains(endpoint, unreachableAuthority) {
		t.Fatalf("the preamble's endpoint lost %q, so two failing endpoints are no longer distinguishable: %q", unreachableAuthority, endpoint)
	}
}

func TestOllamaJudgeFailurePreambleSizesTheRequestTheEndpointActuallyReceived(t *testing.T) {
	t.Parallel()

	srv, captured := respondingServer(t, http.StatusInternalServerError, `{"error":"nope"}`)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	reported := preambleFields(t, judgeFailure(t, c))[3]
	if reported != strconv.Itoa(len(captured.Body)) {
		t.Fatalf("the preamble reports request=%s B, want the %d bytes the endpoint actually received", reported, len(captured.Body))
	}
	if len(captured.Body) == 0 {
		t.Fatal("the endpoint received an empty body, so the size assertion above cannot discriminate")
	}
}

func TestOllamaJudgeFailurePreambleNamesTheBoundTheSuppliedClientActuallyCarries(t *testing.T) {
	t.Parallel()

	c := NewClient("http://host.example", "model-x", "", loop.Sampling{}, &http.Client{Transport: refusingTransport{}, Timeout: 90 * time.Second})

	if got := preambleFields(t, judgeFailure(t, c))[5]; got != "1m30s" {
		t.Fatalf("the preamble names client bound %q, want the supplied client's own 90s timeout — an injected client must not make the message lie", got)
	}
}

func TestOllamaJudgeFailurePreambleNamesTheDefaultBoundWhenNoClientIsSupplied(t *testing.T) {
	t.Parallel()

	c := NewClient("http://127.0.0.1:1", "model-x", "", loop.Sampling{}, nil)

	if got := preambleFields(t, judgeFailure(t, c))[5]; got != DefaultTimeout.String() {
		t.Fatalf("the preamble names client bound %q, want the package default %q the client is actually running under", got, DefaultTimeout)
	}
}

const upstreamDelay = 40 * time.Millisecond

func TestOllamaJudgeFailurePreambleTimesTheCallAgainstTheBoundItNames(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(upstreamDelay)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	elapsed := preambleFields(t, judgeFailure(t, c))[4]
	measured, err := time.ParseDuration(elapsed)
	if err != nil {
		t.Fatalf("the preamble reports elapsed=%q, which is not a duration a reader can compare against the bound: %v", elapsed, err)
	}
	if measured < upstreamDelay {
		t.Fatalf("the preamble reports elapsed=%s for a call the endpoint held for %s, so the number is not the wall clock of this call", measured, upstreamDelay)
	}
	if measured > time.Minute {
		t.Fatalf("the preamble reports elapsed=%s for a call against a local endpoint, want a plausible wall clock", measured)
	}
}

func TestOllamaJudgeAddsNoPreambleToACallThatSucceeds(t *testing.T) {
	t.Parallel()

	srv, _ := capturingServer(t, doneResponse)
	c := NewClient(srv.URL, "model-x", "", loop.Sampling{}, srv.Client())

	result := judgeOnce(t, c)
	if result.Answer != "the answer" {
		t.Fatalf("Answer = %q, want the endpoint's own answer", result.Answer)
	}
}
