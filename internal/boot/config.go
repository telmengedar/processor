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
// calls but the wrong one here — the graph client appends "/api/nodes" itself, so
// an operator who pastes that value gets ".../api/api/nodes", a 404 the client
// treats as "no results" rather than a boot failure (DiVoid #11328).
//
// Only the parsed path is checked, never the raw string, so a legitimate origin
// that merely contains "api" elsewhere — a host named "api.example.com", a
// deployment mounted under "/graph-api" — stays accepted; matching on the path
// segment boundary ("/api", not just the substring "api") is what keeps
// "/graph-api" out of scope. A trailing slash is trimmed first so "/api/" is
// caught the same as "/api".
func rejectAPIBase(divoidURL string) error {
	trimmed := strings.TrimRight(divoidURL, "/")

	parsed, err := neturl.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		// Not a parseable absolute URL; let the caller (or the client's own
		// request construction) surface that failure instead of guessing here.
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

	corrected := *parsed
	corrected.Path = strings.TrimSuffix(path, badSuffix)
	return fmt.Errorf(
		"%s is %q, which already includes the graph API path (%q); the client appends %q itself, so requests would go to %q — set it to the origin only, e.g. %q",
		envDivoidURL, divoidURL, badSuffix, nodesPathSuffix, trimmed+nodesPathSuffix, corrected.String(),
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
