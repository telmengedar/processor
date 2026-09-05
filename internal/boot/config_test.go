package boot

import (
	"fmt"
	"strings"
	"testing"
)

// quoted renders s the way %q would render it inside the boot package's own
// error text. A bare strings.Contains(err, s) is not discriminating here: the
// rejectAPIBase error deliberately renders three URL-shaped values that are
// each other's prefixes by construction (the corrected origin is a prefix of
// the supplied value with its bad suffix trimmed; the "would request" value
// is the supplied value with more appended) — so an assertion against the
// unquoted text can pass even when the argument that was supposed to fill
// that slot has been swapped out. Requiring the closing quote immediately
// after the value pins it to one specific %q slot instead of any substring
// occurrence.
func quoted(s string) string {
	return fmt.Sprintf("%q", s)
}

func fixedLookup(values map[string]string) lookupFunc {
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}

func validEnv(overrides map[string]string) map[string]string {
	env := map[string]string{
		// Origin only: the graph client appends "/api/nodes" itself (#11328).
		// A "/api" suffix here is the credentials-file trap, not a valid base.
		"PROCESSOR_DIVOID_URL": "https://graph.example",
		"PROCESSOR_DIVOID_KEY": "test-key-12345",
		"PROCESSOR_MODEL_URL":  "https://model.example/v1",
		"PROCESSOR_MODEL_ID":   "test-model-id",
	}
	for k, v := range overrides {
		env[k] = v
	}
	return env
}

func TestLoadHTTPAddrDefaultsWhenAbsent(t *testing.T) {
	t.Parallel()

	addr, err := loadHTTPAddr(fixedLookup(validEnv(nil)))
	if err != nil {
		t.Fatalf("loadHTTPAddr: %v", err)
	}
	const want = "127.0.0.1:8080"
	if addr != want {
		t.Fatalf("httpAddr = %q, want %q", addr, want)
	}
}

func TestLoadHTTPAddrUsesItVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "0.0.0.0:9090"
	env := validEnv(map[string]string{"PROCESSOR_HTTP_ADDR": want})

	addr, err := loadHTTPAddr(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadHTTPAddr: %v", err)
	}
	if addr != want {
		t.Fatalf("httpAddr = %q, want %q", addr, want)
	}
}

func TestLoadHTTPAddrErrorsWhenPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_HTTP_ADDR": ""})

	_, err := loadHTTPAddr(fixedLookup(env))
	if err == nil {
		t.Fatal("loadHTTPAddr returned nil error for an empty PROCESSOR_HTTP_ADDR, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_HTTP_ADDR") {
		t.Fatalf("error = %q, want it to name PROCESSOR_HTTP_ADDR", err.Error())
	}
}

func TestLoadGraphErrorsWhenDivoidURLAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_DIVOID_URL")

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error with PROCESSOR_DIVOID_URL absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
	}
}

func TestLoadGraphErrorsWhenDivoidURLPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": ""})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error for an empty PROCESSOR_DIVOID_URL, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
	}
}

func TestLoadGraphUsesDivoidURLVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "https://custom.graph.internal"
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": want})

	cfg, err := loadGraph(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadGraph: %v", err)
	}
	if cfg.URL != want {
		t.Fatalf("divoidURL = %q, want %q", cfg.URL, want)
	}
}

func TestLoadGraphRejectsDivoidURLWhosePathEndsInAPI(t *testing.T) {
	t.Parallel()

	// This is the credentials-file trap (#11328): ~/.claude/secrets/.divoid-online's
	// Url= line is the correct base for direct REST calls, but the graph client
	// appends "/api/nodes" itself, so pasting that value here would silently
	// double the path.
	const bad = "https://graph.example/api"
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": bad})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatalf("loadGraph returned nil error for %q, want an error naming the /api trap", bad)
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
	}
	// quoted, not a bare Contains(err, bad): the "requests would go to" clause
	// always renders bad as a literal prefix of its own text (it is bad with
	// "/api/nodes" appended), so an unquoted assertion here would still pass
	// even if the code stopped naming the supplied value in its own slot.
	if !strings.Contains(err.Error(), quoted(bad)) {
		t.Fatalf("error = %q, want it to name the supplied value %s", err.Error(), quoted(bad))
	}
}

func TestLoadGraphRejectAPIBaseErrorNamesTheDoubledPathThatWouldActuallyBeRequested(t *testing.T) {
	t.Parallel()

	// The third of the three URL-shaped values in the error, and the one
	// naming the actual failure this whole check exists to prevent. Nothing
	// else pinned it: mutating wouldRequest.Path from "path + nodesPathSuffix"
	// down to just "path" left the rest of the suite green.
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": "https://graph.example/api"})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error, want an error naming the doubled path")
	}
	const doubled = "https://graph.example/api/api/nodes"
	if !strings.Contains(err.Error(), quoted(doubled)) {
		t.Fatalf("error = %q, want it to name the doubled path %s that would actually be requested", err.Error(), quoted(doubled))
	}
}

func TestLoadGraphRejectsDivoidURLWhosePathEndsInAPIWithTrailingSlash(t *testing.T) {
	t.Parallel()

	const bad = "https://graph.example/api/"
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": bad})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatalf("loadGraph returned nil error for %q, want an error naming the /api trap", bad)
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
	}
}

func TestLoadGraphRejectsDivoidURLThatAlreadyContainsAPINodes(t *testing.T) {
	t.Parallel()

	const bad = "https://graph.example/api/nodes"
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": bad})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatalf("loadGraph returned nil error for %q, want an error naming the /api/nodes trap", bad)
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
	}
	// Pins what this branch actually computes, not just that it rejected:
	// nothing previously checked the /api/nodes branch's own suggestion.
	const suggestion = "https://graph.example"
	if !strings.Contains(err.Error(), quoted(suggestion)) {
		t.Fatalf("error = %q, want it to suggest the corrected value %s", err.Error(), quoted(suggestion))
	}
}

func TestLoadGraphRejectAPIBaseTrimsTheFullAPINodesTailNotJustTheConstantSuffix(t *testing.T) {
	t.Parallel()

	// A path that contains "/api/nodes" without ending on it exactly (a node
	// id appended after it, e.g. a pasted single-node REST URL) still names
	// the full offending tail. A mutant that hard-codes badSuffix to the
	// "/api/nodes" constant instead of computing it from the match index
	// would call strings.TrimSuffix with a suffix that is not actually at
	// the end of this path, leave it untouched, and suggest back the very
	// value that was just rejected — the divergence the two branches
	// (Contains vs. HasSuffix) only produce on a fixture shaped like this.
	const bad = "https://graph.example/api/nodes/123"
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": bad})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatalf("loadGraph returned nil error for %q, want an error naming the /api/nodes trap", bad)
	}
	const suggestion = "https://graph.example"
	if !strings.Contains(err.Error(), quoted(suggestion)) {
		t.Fatalf("error = %q, want it to suggest the corrected value %s, not the rejected value unchanged", err.Error(), quoted(suggestion))
	}
}

func TestLoadGraphRejectAPIBaseErrorSuggestsTheOriginWithTheSuffixStripped(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": "https://graph.example/api"})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error, want an error suggesting the corrected value")
	}
	// quoted, not a bare Contains: the corrected origin is a prefix of both
	// the supplied value and the "would request" value by construction (it
	// is the supplied value with the bad suffix trimmed off), so an unquoted
	// assertion is satisfied by either of those and never actually checks
	// that the corrected value's own slot was filled correctly — proved by
	// mutation: swapping corrected.Redacted() for a constant leaves an
	// unquoted Contains(err, "https://graph.example") passing, because that
	// text still appears (as a prefix) inside the supplied and would-request
	// renderings.
	const suggestion = "https://graph.example"
	if !strings.Contains(err.Error(), quoted(suggestion)) {
		t.Fatalf("error = %q, want it to suggest the corrected value %s", err.Error(), quoted(suggestion))
	}
}

func TestLoadGraphRejectAPIBaseErrorSuggestsTheOriginWithAMountedPathPreserved(t *testing.T) {
	t.Parallel()

	// The bad suffix ("/api") sits at the end of a longer, legitimate path
	// prefix here, so the correction must trim only the suffix and keep the
	// mount point — the code handles this correctly, but nothing pinned it.
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": "https://graph.example/divoid/api"})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error, want an error suggesting the corrected value")
	}
	const suggestion = "https://graph.example/divoid"
	if !strings.Contains(err.Error(), quoted(suggestion)) {
		t.Fatalf("error = %q, want it to suggest the corrected value %s, preserving the /divoid mount", err.Error(), quoted(suggestion))
	}
}

func TestLoadGraphRejectAPIBaseDoesNotRejectAHostThatMerelyContainsAPI(t *testing.T) {
	t.Parallel()

	// "api" appearing somewhere in the URL is not the trap; only the path
	// segment "/api" (or "/api/nodes") is. A host like api.example.com, or a
	// path like "/graph-api" that merely ends with the substring "api"
	// without the segment boundary, must stay accepted.
	for _, want := range []string{
		"https://api.example.com",
		"https://graph.example/graph-api",
		"https://graph.example/apis",
	} {
		env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": want})

		cfg, err := loadGraph(fixedLookup(env))
		if err != nil {
			t.Fatalf("loadGraph(%q): %v, want no error — it does not literally end in the /api segment", want, err)
		}
		if cfg.URL != want {
			t.Fatalf("divoidURL = %q, want %q", cfg.URL, want)
		}
	}
}

func TestLoadGraphAcceptsLegitimateDivoidURLShapes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		url  string
	}{
		{"bare origin", "https://graph.example"},
		{"trailing slash", "https://graph.example/"},
		{"explicit port", "https://graph.example:8443"},
		{"http scheme", "http://graph.internal"},
		{"non-api path prefix", "https://graph.example/graph"},
		{"localhost with port and path prefix", "http://localhost:9000/graph"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": tc.url})

			cfg, err := loadGraph(fixedLookup(env))
			if err != nil {
				t.Fatalf("loadGraph(%q): %v, want a legitimate origin to be accepted", tc.url, err)
			}
			if cfg.URL != tc.url {
				t.Fatalf("divoidURL = %q, want %q (used verbatim)", cfg.URL, tc.url)
			}
		})
	}
}

func TestLoadGraphRejectAPIBaseCatchesTheTrapEvenWithSurroundingWhitespace(t *testing.T) {
	t.Parallel()

	// The trap's source is a credentials file, so a pasted value realistically
	// carries a stray leading space, a trailing space, a trailing "\t", or a
	// trailing "\r" (a CRLF-saved file read on a line-oriented parser). Left
	// unstripped, a trailing space in particular would reach the HTTP client,
	// get percent-escaped to "%20", and reproduce this exact failure under a
	// value that superficially looks like it was rejected already.
	for _, tc := range []struct {
		name string
		url  string
	}{
		{"trailing space", "https://graph.example/api "},
		{"leading space", " https://graph.example/api"},
		{"trailing carriage return", "https://graph.example/api\r"},
		{"trailing tab", "https://graph.example/api\t"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": tc.url})

			_, err := loadGraph(fixedLookup(env))
			if err == nil {
				t.Fatalf("loadGraph(%q) returned nil error, want the /api trap caught despite the whitespace", tc.url)
			}
			if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
				t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
			}
		})
	}
}

func TestLoadGraphAcceptsADivoidURLWithNoSchemeEvenThoughItsPathEndsInAPI(t *testing.T) {
	t.Parallel()

	// A bare host-and-path with no scheme — exactly the credentials file's
	// Url= value pasted without its "https://" — parses via net/url as a
	// relative reference with an empty Host, so rejectAPIBase's guard clause
	// lets it through untouched: this is the documented, deliberate limit of
	// the check (see the comment on the parse-failure branch in config.go),
	// not an oversight. Pinning it here means a future change to that branch
	// is a visible, deliberate decision instead of a silent regression.
	const want = "divoid.mamgo.io/api"
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": want})

	cfg, err := loadGraph(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadGraph(%q): %v, want no error — a scheme-less value is accepted by design, not validated", want, err)
	}
	if cfg.URL != want {
		t.Fatalf("divoidURL = %q, want %q (used verbatim)", cfg.URL, want)
	}
}

func TestLoadGraphRejectAPIBaseErrorOmitsAUserinfoPasswordFromEveryRendering(t *testing.T) {
	t.Parallel()

	// config.go builds all three URL-shaped values in the error (the supplied
	// value, the doubled path, the corrected suggestion) as *url.URL copies
	// rendered via Redacted() specifically so a userinfo password never
	// reaches the error text — a sibling guarantee to the one already pinned
	// for PROCESSOR_DIVOID_KEY (TestBootConfigErrorsNameTheVariableAndNeverItsValue),
	// but previously nothing held it for this path.
	const password = "hunter2"
	env := validEnv(map[string]string{
		"PROCESSOR_DIVOID_URL": "https://user:" + password + "@graph.example/api",
	})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error, want an error rejecting the /api trap")
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("error = %q, must not contain the userinfo password %q", err.Error(), password)
	}
}

func TestLoadGraphErrorsWhenDivoidKeyAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_DIVOID_KEY")

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error with PROCESSOR_DIVOID_KEY absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_KEY") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_KEY", err.Error())
	}
}

func TestLoadGraphErrorsWhenDivoidKeyPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_DIVOID_KEY": ""})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error for an empty PROCESSOR_DIVOID_KEY, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_KEY") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_KEY", err.Error())
	}
}

func TestLoadGraphUsesDivoidKeyVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "sekrit-value-99"
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_KEY": want})

	cfg, err := loadGraph(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadGraph: %v", err)
	}
	if cfg.Key != want {
		t.Fatalf("divoidKey = %q, want %q", cfg.Key, want)
	}
}

func TestGraphBootConfigLoadsWhenNoModelVariableIsSet(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"PROCESSOR_DIVOID_URL": "https://graph.example",
		"PROCESSOR_DIVOID_KEY": "test-key-12345",
	}

	cfg, err := loadGraph(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadGraph with no model variable set: %v", err)
	}
	if cfg.URL != env["PROCESSOR_DIVOID_URL"] {
		t.Fatalf("divoidURL = %q, want %q", cfg.URL, env["PROCESSOR_DIVOID_URL"])
	}
	if cfg.Key != env["PROCESSOR_DIVOID_KEY"] {
		t.Fatalf("divoidKey = %q, want %q", cfg.Key, env["PROCESSOR_DIVOID_KEY"])
	}
}

func TestBootConfigErrorsNameTheVariableAndNeverItsValue(t *testing.T) {
	t.Parallel()

	const secret = "sekrit-value-99"

	graphEnv := validEnv(map[string]string{
		"PROCESSOR_DIVOID_KEY": secret,
		"PROCESSOR_DIVOID_URL": "",
	})

	_, err := loadGraph(fixedLookup(graphEnv))
	if err == nil {
		t.Fatal("loadGraph returned nil error for an empty PROCESSOR_DIVOID_URL, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error = %q, must not contain the secret value %q", err.Error(), secret)
	}

	modelEnv := validEnv(map[string]string{
		"PROCESSOR_MODEL_KEY": secret,
		"PROCESSOR_MODEL_ID":  "",
	})

	_, err = loadModel(fixedLookup(modelEnv))
	if err == nil {
		t.Fatal("loadModel returned nil error for an empty PROCESSOR_MODEL_ID, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_ID") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_ID", err.Error())
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error = %q, must not contain the secret value %q", err.Error(), secret)
	}
}

func TestModelBootConfigErrorsWhenTheModelUrlIsAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_MODEL_URL")

	_, err := loadModel(fixedLookup(env))
	if err == nil {
		t.Fatal("loadModel returned nil error with PROCESSOR_MODEL_URL absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_URL", err.Error())
	}
}

func TestLoadModelErrorsWhenModelURLPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_MODEL_URL": ""})

	_, err := loadModel(fixedLookup(env))
	if err == nil {
		t.Fatal("loadModel returned nil error for an empty PROCESSOR_MODEL_URL, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_URL", err.Error())
	}
}

func TestLoadModelUsesModelURLVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "https://custom.model.internal/v1"
	env := validEnv(map[string]string{"PROCESSOR_MODEL_URL": want})

	cfg, err := loadModel(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadModel: %v", err)
	}
	if cfg.URL != want {
		t.Fatalf("modelURL = %q, want %q", cfg.URL, want)
	}
}

func TestLoadModelErrorsWhenModelIDAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_MODEL_ID")

	_, err := loadModel(fixedLookup(env))
	if err == nil {
		t.Fatal("loadModel returned nil error with PROCESSOR_MODEL_ID absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_ID") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_ID", err.Error())
	}
}

func TestLoadModelErrorsWhenModelIDPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_MODEL_ID": ""})

	_, err := loadModel(fixedLookup(env))
	if err == nil {
		t.Fatal("loadModel returned nil error for an empty PROCESSOR_MODEL_ID, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_ID") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_ID", err.Error())
	}
}

func TestLoadModelUsesModelIDVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "llama-3.1-8b-instruct"
	env := validEnv(map[string]string{"PROCESSOR_MODEL_ID": want})

	cfg, err := loadModel(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadModel: %v", err)
	}
	if cfg.ID != want {
		t.Fatalf("modelID = %q, want %q", cfg.ID, want)
	}
}

func TestLoadModelLeavesModelKeyEmptyWhenAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_MODEL_KEY")

	cfg, err := loadModel(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadModel: %v", err)
	}
	if cfg.Key != "" {
		t.Fatalf("modelKey = %q, want empty when PROCESSOR_MODEL_KEY is absent (no Authorization header is sent)", cfg.Key)
	}
}

func TestLoadModelErrorsWhenModelKeyPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_MODEL_KEY": ""})

	_, err := loadModel(fixedLookup(env))
	if err == nil {
		t.Fatal("loadModel returned nil error for an empty PROCESSOR_MODEL_KEY, want an error — an empty value is a mistake, never a way to spell \"no auth\"")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_KEY") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_KEY", err.Error())
	}
}

func TestLoadModelUsesModelKeyVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "local-runtime-key-1"
	env := validEnv(map[string]string{"PROCESSOR_MODEL_KEY": want})

	cfg, err := loadModel(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadModel: %v", err)
	}
	if cfg.Key != want {
		t.Fatalf("modelKey = %q, want %q", cfg.Key, want)
	}
}

func TestExportedLoadersReadTheProcessEnvironment(t *testing.T) {
	t.Setenv("PROCESSOR_HTTP_ADDR", "0.0.0.0:7070")
	t.Setenv("PROCESSOR_DIVOID_URL", "https://env.graph.example")
	t.Setenv("PROCESSOR_DIVOID_KEY", "env-graph-key")
	t.Setenv("PROCESSOR_MODEL_URL", "https://env.model.example/v1")
	t.Setenv("PROCESSOR_MODEL_ID", "env-model-id")
	t.Setenv("PROCESSOR_MODEL_KEY", "env-model-key")

	addr, err := LoadHTTPAddr()
	if err != nil {
		t.Fatalf("LoadHTTPAddr: %v", err)
	}
	if addr != "0.0.0.0:7070" {
		t.Fatalf("httpAddr = %q, want %q", addr, "0.0.0.0:7070")
	}

	graph, err := LoadGraph()
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if graph.URL != "https://env.graph.example" || graph.Key != "env-graph-key" {
		t.Fatalf("graph = %+v, want the PROCESSOR_DIVOID_ variables", graph)
	}

	model, err := LoadModel()
	if err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	if model.URL != "https://env.model.example/v1" || model.ID != "env-model-id" || model.Key != "env-model-key" {
		t.Fatalf("model = %+v, want the PROCESSOR_MODEL_ variables", model)
	}
}
