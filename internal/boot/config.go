// Package boot reads the process environment into the configuration each binary needs.
package boot

import (
	"fmt"
	neturl "net/url"
	"os"
	"strings"
)

const (
	envHTTPAddr     = "PROCESSOR_HTTP_ADDR"
	defaultHTTPAddr = "127.0.0.1:8080"

	envDivoidURL = "PROCESSOR_DIVOID_URL"
	envDivoidKey = "PROCESSOR_DIVOID_KEY"

	envModelURL = "PROCESSOR_MODEL_URL"
	envModelID  = "PROCESSOR_MODEL_ID"

	envModelKey = "PROCESSOR_MODEL_KEY"

	// apiPathSuffix and nodesPathSuffix mirror internal/divoid's own
	// nodesPath ("/api/nodes"); boot does not import that package (it must
	// stay free of the client it configures), so the two shapes it rejects
	// are named again here, deliberately, rather than shared.
	apiPathSuffix   = "/api"
	nodesPathSuffix = "/api/nodes"
)

// GraphConfig is what a graph client needs to reach the graph.
type GraphConfig struct {
	URL string
	Key string
}

// ModelConfig is what a model client needs to reach a chat-completions endpoint.
type ModelConfig struct {
	URL string
	ID  string
	Key string
}

type lookupFunc func(key string) (string, bool)

func lookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

// LoadHTTPAddr returns the listen address, defaulting when the variable is absent.
func LoadHTTPAddr() (string, error) {
	return loadHTTPAddr(lookupEnv)
}

// LoadGraph returns the graph half of the boot configuration.
func LoadGraph() (GraphConfig, error) {
	return loadGraph(lookupEnv)
}

// LoadModel returns the model half of the boot configuration.
func LoadModel() (ModelConfig, error) {
	return loadModel(lookupEnv)
}

func loadHTTPAddr(lookup lookupFunc) (string, error) {
	addr, present := lookup(envHTTPAddr)
	if !present {
		return defaultHTTPAddr, nil
	}
	if addr == "" {
		return "", fmt.Errorf("%s is set but empty", envHTTPAddr)
	}
	return addr, nil
}

func loadGraph(lookup lookupFunc) (GraphConfig, error) {
	url, err := requireEnv(lookup, envDivoidURL)
	if err != nil {
		return GraphConfig{}, err
	}
	if err := rejectAPIBase(url); err != nil {
		return GraphConfig{}, err
	}

	key, err := requireEnv(lookup, envDivoidKey)
	if err != nil {
		return GraphConfig{}, err
	}

	return GraphConfig{URL: url, Key: key}, nil
}

// rejectAPIBase catches the credentials-file trap: ~/.claude/secrets/.divoid-online
// holds a Url= line ending in "/api", which is the correct base for direct REST
// calls but the wrong one here — the graph client (internal/divoid) appends
// "/api/nodes" itself, so an operator who pastes that value would get
// ".../api/api/nodes" (DiVoid #11328). What that doubled path actually does at
// request time has not been measured and is deliberately not claimed here —
// the only defensible statement is that the value is wrong and boot is the
// cheap place to catch it, before whatever happens downstream happens.
//
// Two separate things keep this from misfiring on a legitimate origin that
// merely contains "api": checking only the parsed path — never the host, never
// the raw string — is what leaves a host like "api.example.com" untouched;
// matching the path segment boundary ("/api" as a suffix, never the bare
// substring "api") is what leaves a path like "/graph-api" or "/apis"
// untouched.
//
// Whitespace is trimmed before a trailing slash, and both before parsing —
// but only for the purpose of this check. The value returned to the caller
// below is the untrimmed input; whitespace in an otherwise-accepted value
// still reaches the client unchanged, which is a separate, pre-existing gap
// this change does not close. The trim here exists solely so a stray leading
// space or a trailing "\r"/space/tab on an /api-suffixed value — a realistic
// paste artefact from a credentials file — cannot slip past this check by
// accident of looking like a different path than "/api".
func rejectAPIBase(divoidURL string) error {
	trimmed := strings.TrimRight(strings.TrimSpace(divoidURL), "/")

	parsed, err := neturl.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		// Not a parseable absolute URL; let the caller (or the client's own
		// request construction) surface that failure instead of guessing here.
		// This is deliberate, not an oversight: a scheme-less value such as
		// "divoid.mamgo.io/api" (the credentials file's Url= pasted without
		// its scheme) parses with an empty Host and is accepted rather than
		// rejected. Teaching this check to also resolve scheme-less
		// references is how a config validator grows into a URL parser.
		return nil
	}

	path := strings.TrimRight(parsed.Path, "/")

	var badSuffix string
	switch {
	case strings.Contains(path, nodesPathSuffix):
		badSuffix = path[strings.Index(path, nodesPathSuffix):]
	case strings.HasSuffix(path, apiPathSuffix):
		badSuffix = apiPathSuffix
	default:
		return nil
	}

	// wouldRequest and corrected are built from copies of the parsed URL,
	// not string concatenation, so Redacted() can strip any userinfo
	// password from all three renderings below — DiVoid itself uses bearer
	// auth, not userinfo, but this check runs on operator-supplied input in
	// general and must not be the thing that echoes a credential into a log.
	// This makes the "set it to the origin only, e.g." suggestion literally
	// correct as a redaction and imprecise as a suggestion: for a userinfo
	// URL it renders as "https://user:xxxxx@graph.example", and pasting that
	// back verbatim would not restore the real password. Accepted as-is —
	// the shape does not occur for DiVoid's own bearer auth, and the
	// alternative is printing a live credential into an error message.
	wouldRequest := *parsed
	wouldRequest.Path = path + nodesPathSuffix

	corrected := *parsed
	corrected.Path = strings.TrimSuffix(path, badSuffix)

	return fmt.Errorf(
		"%s is %q, which already includes the graph API path (%q); the client appends %q itself, so requests would go to %q — set it to the origin only, e.g. %q",
		envDivoidURL, parsed.Redacted(), badSuffix, nodesPathSuffix, wouldRequest.Redacted(), corrected.Redacted(),
	)
}

func loadModel(lookup lookupFunc) (ModelConfig, error) {
	url, err := requireEnv(lookup, envModelURL)
	if err != nil {
		return ModelConfig{}, err
	}

	id, err := requireEnv(lookup, envModelID)
	if err != nil {
		return ModelConfig{}, err
	}

	key, err := optionalEnv(lookup, envModelKey)
	if err != nil {
		return ModelConfig{}, err
	}

	return ModelConfig{URL: url, ID: id, Key: key}, nil
}

func requireEnv(lookup lookupFunc, key string) (string, error) {
	val, present := lookup(key)
	if !present {
		return "", fmt.Errorf("%s is required but not set", key)
	}
	if val == "" {
		return "", fmt.Errorf("%s is set but empty", key)
	}
	return val, nil
}

func optionalEnv(lookup lookupFunc, key string) (string, error) {
	val, present := lookup(key)
	if !present {
		return "", nil
	}
	if val == "" {
		return "", fmt.Errorf("%s is set but empty", key)
	}
	return val, nil
}
