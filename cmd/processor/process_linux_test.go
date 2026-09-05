package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// syncBuffer is a concurrency-safe byte sink for a child process's stderr:
// the readiness poll below reads it while the child is still writing to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

const (
	envHTTPAddr  = "PROCESSOR_HTTP_ADDR"
	envDivoidURL = "PROCESSOR_DIVOID_URL"
	envDivoidKey = "PROCESSOR_DIVOID_KEY"
	envModelURL  = "PROCESSOR_MODEL_URL"
	envModelID   = "PROCESSOR_MODEL_ID"
	envModelKey  = "PROCESSOR_MODEL_KEY"

	envModelTemperature = "PROCESSOR_MODEL_TEMPERATURE"
	envModelTopP        = "PROCESSOR_MODEL_TOP_P"
)

const (
	graphKeySentinel = "test-key-12345"
	modelKeySentinel = "test-model-key-67890"
	modelIDSentinel  = "test-model-id"

	temperatureSentinel = 0.37
	topPSentinel        = 0.91
)

func validEnv(overrides map[string]string) map[string]string {
	env := map[string]string{
		envDivoidURL: "https://graph.example",
		envDivoidKey: graphKeySentinel,
		envModelURL:  "https://model.example/v1",
		envModelID:   modelIDSentinel,
	}
	for k, v := range overrides {
		env[k] = v
	}
	return env
}

func harnessEnv(addr string) []string {
	return harnessEnvWith(map[string]string{envHTTPAddr: addr})
}

func harnessEnvWith(overrides map[string]string) []string {
	m := validEnv(overrides)
	env := make([]string, 0, len(m))
	for k, v := range m {
		env = append(env, k+"="+v)
	}
	return env
}

// buildProcessorBinary builds the package under test into a per-test
// temporary directory and returns the path to the built binary.
func buildProcessorBinary(t *testing.T) string {
	t.Helper()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("LookPath(go): %v", err)
	}

	binPath := filepath.Join(t.TempDir(), "processor")
	cmd := exec.Command(goBin, "build", "-o", binPath, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return binPath
}

// servingProcess is a launched binary that has been observed to serve.
type servingProcess struct {
	cmd    *exec.Cmd
	stderr *syncBuffer
	addr   string
}

// startServingProcess launches bin with an explicit, minimal environment —
// the ambient environment of the test process is never inherited — and
// waits for it to prove it is serving before returning. Readiness is two
// steps for two different reasons: first the actual bound address is read
// out of the child's "listening" record (the only way the address becomes
// known at all, since every scenario binds 127.0.0.1:0), then a real
// GET /health is polled until it answers — establishing that the process
// genuinely served before any caller asserts that it later stopped.
func startServingProcess(t *testing.T, bin string, env []string) *servingProcess {
	t.Helper()

	stderr := &syncBuffer{}
	cmd := exec.Command(bin)
	cmd.Env = env
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	listeningRE := regexp.MustCompile(`msg=listening addr=(\S+)`)

	addr := ""
	addrDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(addrDeadline) {
		if m := listeningRE.FindStringSubmatch(stderr.String()); m != nil {
			addr = m[1]
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if addr == "" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("process never logged a listening address within 5s; stderr:\n%s", stderr.String())
	}

	// The default http.Client has no timeout, so a single call to Get would
	// block forever if nothing ever answers (e.g. the listener is bound but
	// nothing accepts on it). readyDeadline only bounds the number of
	// retries, not any one blocking call, so each request gets its own
	// fixed 500ms timeout — the worst case is therefore readyDeadline plus
	// up to one more in-flight request, not readyDeadline alone.
	client := &http.Client{Timeout: 500 * time.Millisecond}

	readyDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(readyDeadline) {
		resp, err := client.Get("http://" + addr + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return &servingProcess{cmd: cmd, stderr: stderr, addr: addr}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	t.Fatalf("GET /health against %s never returned 200 within 5s; stderr:\n%s", addr, stderr.String())
	return nil
}

func awaitStderr(t *testing.T, stderr *syncBuffer, want string, deadline time.Duration) {
	t.Helper()

	until := time.Now().Add(deadline)
	for time.Now().Before(until) {
		if strings.Contains(stderr.String(), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("stderr never carried %q within %s; stderr:\n%s", want, deadline, stderr.String())
}

// awaitExit waits for cmd to exit within deadline. On timeout it kills and
// reaps the process and fails naming the exit that never came — not
// "timeout" — because the configuration-error-ignored mutant this harness
// must kill does not exit at all (it binds every interface and serves), and
// a bare timeout message would not distinguish that from a slow machine.
func awaitExit(t *testing.T, cmd *exec.Cmd, deadline time.Duration, stderr *syncBuffer) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(deadline):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("process did not exit within %s; stderr so far:\n%s", deadline, stderr.String())
	}
}

// TestGracefulShutdown covers G1 and G2 (design §6.1): an interrupt and a
// SIGTERM each make the process run the graceful path and exit 0, with the
// ordered shutdown records on stderr. One instrument, one body, one subtest
// per signal — SIGTERM is the leg Windows can never deliver.
func TestGracefulShutdown(t *testing.T) {
	t.Parallel()

	bin := buildProcessorBinary(t)

	cases := []struct {
		name   string
		signal os.Signal
	}{
		{"Interrupt", os.Interrupt},
		{"SIGTERM", syscall.SIGTERM},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rp := startServingProcess(t, bin, harnessEnv("127.0.0.1:0"))

			if err := rp.cmd.Process.Signal(tc.signal); err != nil {
				t.Fatalf("Signal(%v): %v", tc.signal, err)
			}

			awaitExit(t, rp.cmd, 10*time.Second, rp.stderr)

			// A signal-terminated process reports ExitCode() == -1, never
			// 0 (Go returns -1 whenever Exited() is false), so exit 0 by
			// itself is an unambiguous "handled the signal" discriminator
			// on Linux.
			if code := rp.cmd.ProcessState.ExitCode(); code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, rp.stderr.String())
			}

			out := rp.stderr.String()
			idxListening := strings.Index(out, "msg=listening")
			idxStarted := strings.Index(out, `msg="shutdown started"`)
			idxComplete := strings.Index(out, `msg="shutdown complete"`)
			if idxListening == -1 || idxStarted == -1 || idxComplete == -1 {
				t.Fatalf("missing shutdown lifecycle record(s); stderr:\n%s", out)
			}
			if !(idxListening < idxStarted && idxStarted < idxComplete) {
				t.Fatalf("shutdown lifecycle records out of order; stderr:\n%s", out)
			}
		})
	}
}

// TestConfigurationError covers G3 (design §6.2): the configuration-error
// branch of run() exits non-zero and names the offending variable.
func TestConfigurationError(t *testing.T) {
	t.Parallel()

	bin := buildProcessorBinary(t)

	stderr := &syncBuffer{}
	cmd := exec.Command(bin)
	cmd.Env = []string{"PROCESSOR_HTTP_ADDR="}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// M18: the configuration-error-ignored mutant does not exit — it binds
	// every interface and serves until killed. This wait must stay bounded.
	awaitExit(t, cmd, 5*time.Second, stderr)

	if code := cmd.ProcessState.ExitCode(); code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr:\n%s", code, stderr.String())
	}

	out := stderr.String()
	// A record is identified by its level and its message together (design
	// §8.3); asserting the message alone lets a demoted Info record survive.
	if !strings.Contains(out, `level=ERROR msg="boot configuration"`) {
		t.Fatalf("stderr missing boot configuration error record; stderr:\n%s", out)
	}
	if !strings.Contains(out, "PROCESSOR_HTTP_ADDR") {
		t.Fatalf("boot configuration record does not name PROCESSOR_HTTP_ADDR; stderr:\n%s", out)
	}
}

// TestBindError covers G4 (design §6.3): the bind-error branch of run()
// exits non-zero. A first instance on port 0 makes a known-occupied address
// available without ever naming a fixed port; a second instance is pointed
// at that address and is expected to fail to bind it.
func TestBindError(t *testing.T) {
	t.Parallel()

	bin := buildProcessorBinary(t)

	first := startServingProcess(t, bin, harnessEnv("127.0.0.1:0"))
	defer func() {
		_ = first.cmd.Process.Kill()
		_ = first.cmd.Wait()
	}()

	stderr := &syncBuffer{}
	cmd := exec.Command(bin)
	cmd.Env = harnessEnv(first.addr)
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	awaitExit(t, cmd, 5*time.Second, stderr)

	if code := cmd.ProcessState.ExitCode(); code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr:\n%s", code, stderr.String())
	}

	out := stderr.String()
	// Two assertions, not one combined string: pinning addr as the record's
	// first attribute false-fails on benign reorderings (an attribute
	// inserted before addr, or addr moved after another field) even though
	// the record is present and does carry addr. Checking the message
	// identity ("msg=listen" alone also matches "msg=listening", hence the
	// trailing space) and the addr field separately survives those
	// reorderings while still requiring both to be present.
	if !strings.Contains(out, "level=ERROR msg=listen ") {
		t.Fatalf("stderr missing listen error record; stderr:\n%s", out)
	}
	if !strings.Contains(out, "addr="+first.addr) {
		t.Fatalf("stderr missing listen error record carrying addr=%s; stderr:\n%s", first.addr, out)
	}
}

func assertEachEndpointReceivedItsOwnCredential(t *testing.T, graphAuth, modelAuth *authRecorder) {
	t.Helper()

	if graphKeySentinel == modelKeySentinel {
		t.Fatalf("the sentinels are identical, so a crossed credential is indistinguishable from a correct one")
	}

	if got, want := graphAuth.get(), "Bearer "+graphKeySentinel; got != want {
		t.Errorf("the graph endpoint was called with Authorization %q, want %q — the graph client was not built with the graph key", got, want)
	}
	if got, want := modelAuth.get(), "Bearer "+modelKeySentinel; got != want {
		t.Errorf("the model endpoint was called with Authorization %q, want %q — the model client was not built with the model key", got, want)
	}
}

type runCallResult struct {
	status int
	body   []byte
	err    error
}

func TestSIGTERMDrainsAnInFlightRunInsteadOfDroppingIt(t *testing.T) {
	t.Parallel()

	bin := buildProcessorBinary(t)

	stored := &storedBody{}
	graphSrv, graphAuth := graphServer(t, stored, "")

	const modelDelay = 6 * time.Second
	entered := make(chan struct{}, 1)
	modelAuth := &authRecorder{}
	modelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		modelAuth.record(r)

		select {
		case entered <- struct{}{}:
		default:
		}
		time.Sleep(modelDelay)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"the drained answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":22}}`))
	}))
	t.Cleanup(modelSrv.Close)

	rp := startServingProcess(t, bin, harnessEnvWith(map[string]string{
		envHTTPAddr:  "127.0.0.1:0",
		envDivoidURL: graphSrv.URL,
		envModelURL:  modelSrv.URL,
		envModelKey:  modelKeySentinel,
	}))

	done := make(chan runCallResult, 1)
	go func() {
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Post("http://"+rp.addr+"/runs", "application/json", strings.NewReader(`{"input":"what changed","subject":42}`))
		if err != nil {
			done <- runCallResult{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		done <- runCallResult{status: resp.StatusCode, body: body, err: err}
	}()

	select {
	case <-entered:
	case <-time.After(15 * time.Second):
		_ = rp.cmd.Process.Kill()
		_ = rp.cmd.Wait()
		t.Fatalf("the model endpoint was never reached, so no run was in flight when the signal was sent; stderr:\n%s", rp.stderr.String())
	}

	if err := rp.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Signal(SIGTERM): %v", err)
	}

	var got runCallResult
	select {
	case got = <-done:
	case <-time.After(60 * time.Second):
		_ = rp.cmd.Process.Kill()
		_ = rp.cmd.Wait()
		t.Fatalf("the in-flight run never produced a response after SIGTERM; stderr:\n%s", rp.stderr.String())
	}

	if got.err != nil {
		t.Fatalf("POST /runs across the drain failed: %v; stderr:\n%s", got.err, rp.stderr.String())
	}
	if got.status != http.StatusOK {
		t.Fatalf("POST /runs status = %d, want %d; body=%s", got.status, http.StatusOK, got.body)
	}
	if !strings.Contains(string(got.body), `"answer":"the drained answer"`) {
		t.Fatalf("the drained response does not carry the model's answer; body=%s", got.body)
	}
	if !strings.Contains(string(got.body), `"written":{"state":"stored"`) {
		t.Fatalf("the drained response does not report the record as stored; body=%s", got.body)
	}
	if len(stored.get()) == 0 {
		t.Fatalf("the graph never received the run record, so the drain delivered the answer but not the second copy; stderr:\n%s", rp.stderr.String())
	}

	assertEachEndpointReceivedItsOwnCredential(t, graphAuth, modelAuth)

	awaitExit(t, rp.cmd, 30*time.Second, rp.stderr)

	if code := rp.cmd.ProcessState.ExitCode(); code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, rp.stderr.String())
	}

	out := rp.stderr.String()
	idxStarted := strings.Index(out, `msg="shutdown started"`)
	idxComplete := strings.Index(out, `msg="shutdown complete"`)
	if idxStarted == -1 || idxComplete == -1 || idxStarted > idxComplete {
		t.Fatalf("shutdown lifecycle records missing or out of order across a drained run; stderr:\n%s", out)
	}
}

func TestACompletedRunEmitsTheStartedAndFinishedPairInOrder(t *testing.T) {
	t.Parallel()

	bin := buildProcessorBinary(t)

	stored := &storedBody{}
	graphSrv, graphAuth := graphServer(t, stored, "")

	modelAuth := &authRecorder{}
	modelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		modelAuth.record(r)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"the answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":22}}`))
	}))
	t.Cleanup(modelSrv.Close)

	rp := startServingProcess(t, bin, harnessEnvWith(map[string]string{
		envHTTPAddr:  "127.0.0.1:0",
		envDivoidURL: graphSrv.URL,
		envModelURL:  modelSrv.URL,
		envModelKey:  modelKeySentinel,
	}))
	defer func() {
		_ = rp.cmd.Process.Kill()
		_ = rp.cmd.Wait()
	}()

	const input = "INPUT-TEXT-MARKER what changed"
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post("http://"+rp.addr+"/runs", "application/json", strings.NewReader(`{"input":"`+input+`","subject":42}`))
	if err != nil {
		t.Fatalf("POST /runs: %v; stderr:\n%s", err, rp.stderr.String())
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /runs status = %d, want %d; stderr:\n%s", resp.StatusCode, http.StatusOK, rp.stderr.String())
	}

	awaitStderr(t, rp.stderr, `msg="run started"`, 5*time.Second)
	awaitStderr(t, rp.stderr, `msg="run finished"`, 5*time.Second)

	out := rp.stderr.String()
	idxStarted := strings.Index(out, `msg="run started"`)
	idxFinished := strings.Index(out, `msg="run finished"`)
	if idxStarted > idxFinished {
		t.Fatalf("the run-finished record precedes the run-started record; stderr:\n%s", out)
	}
	if strings.Contains(out, "INPUT-TEXT-MARKER") {
		t.Fatalf("stderr carries the input text; stderr:\n%s", out)
	}
	if !strings.Contains(out, "inputLength=30") {
		t.Fatalf("run-started record does not carry inputLength=%d; stderr:\n%s", len(input), out)
	}
	if !strings.Contains(out, "receipt=stored") {
		t.Fatalf("run-finished record does not carry the write receipt; stderr:\n%s", out)
	}
	if !strings.Contains(out, "elapsed=") {
		t.Fatalf("run-finished record does not carry the wall clock; stderr:\n%s", out)
	}
	if !strings.Contains(out, "model="+modelIDSentinel) {
		t.Fatalf("run-finished record does not carry model=%s, so the model id the turn was built with is unpinned; stderr:\n%s", modelIDSentinel, out)
	}

	assertEachEndpointReceivedItsOwnCredential(t, graphAuth, modelAuth)
}

type requestBodyRecorder struct {
	mu   sync.Mutex
	body []byte
}

func (b *requestBodyRecorder) record(r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.body = body
}

func (b *requestBodyRecorder) get() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.body
}

func TestTheConfiguredSamplingReachesTheModelEndpointUncrossed(t *testing.T) {
	t.Parallel()

	if temperatureSentinel == topPSentinel {
		t.Fatal("the sampling sentinels are identical, so a crossed pair is indistinguishable from a correct one")
	}
	if temperatureSentinel == 0 || topPSentinel == 0 {
		t.Fatal("a zero sampling sentinel is indistinguishable from a binary that configures no sampling at all")
	}

	bin := buildProcessorBinary(t)

	stored := &storedBody{}
	graphSrv, _ := graphServer(t, stored, "")

	modelRequest := &requestBodyRecorder{}
	modelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		modelRequest.record(r)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"the answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":22}}`))
	}))
	t.Cleanup(modelSrv.Close)

	rp := startServingProcess(t, bin, harnessEnvWith(map[string]string{
		envHTTPAddr:         "127.0.0.1:0",
		envDivoidURL:        graphSrv.URL,
		envModelURL:         modelSrv.URL,
		envModelKey:         modelKeySentinel,
		envModelTemperature: strconv.FormatFloat(temperatureSentinel, 'f', -1, 64),
		envModelTopP:        strconv.FormatFloat(topPSentinel, 'f', -1, 64),
	}))
	defer func() {
		_ = rp.cmd.Process.Kill()
		_ = rp.cmd.Wait()
	}()

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post("http://"+rp.addr+"/runs", "application/json", strings.NewReader(`{"input":"what changed","subject":42}`))
	if err != nil {
		t.Fatalf("POST /runs: %v; stderr:\n%s", err, rp.stderr.String())
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /runs status = %d, want %d; stderr:\n%s", resp.StatusCode, http.StatusOK, rp.stderr.String())
	}

	sent := modelRequest.get()
	if len(sent) == 0 {
		t.Fatalf("the model endpoint recorded no request body, so nothing about the request the binary sends is pinned; stderr:\n%s", rp.stderr.String())
	}

	var wire struct {
		Temperature *float64 `json:"temperature"`
		TopP        *float64 `json:"top_p"`
	}
	if err := json.Unmarshal(sent, &wire); err != nil {
		t.Fatalf("decode the request the model endpoint received: %v; body=%s", err, sent)
	}

	if wire.Temperature == nil {
		t.Fatalf("the model endpoint received no temperature, want %v — %s never reached the wire; body=%s", temperatureSentinel, envModelTemperature, sent)
	}
	if *wire.Temperature != temperatureSentinel {
		t.Fatalf("the model endpoint received temperature %v, want %v — %s reached the wire as some other value; body=%s", *wire.Temperature, temperatureSentinel, envModelTemperature, sent)
	}
	if wire.TopP == nil {
		t.Fatalf("the model endpoint received no top_p, want %v — %s never reached the wire; body=%s", topPSentinel, envModelTopP, sent)
	}
	if *wire.TopP != topPSentinel {
		t.Fatalf("the model endpoint received top_p %v, want %v — %s reached the wire as some other value; body=%s", *wire.TopP, topPSentinel, envModelTopP, sent)
	}
}
