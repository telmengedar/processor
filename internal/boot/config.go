// Package boot reads the process environment into the configuration each binary needs.
package boot

import (
	"fmt"
	neturl "net/url"
	"os"
	"strconv"
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

	envModelTemperature     = "PROCESSOR_MODEL_TEMPERATURE"
	defaultModelTemperature = 0.0

	envModelTopP = "PROCESSOR_MODEL_TOP_P"

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
	URL         string
	ID          string
	Key         string
	Temperature *float64
	TopP        *float64
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

func rejectAPIBase(divoidURL string) error {
	trimmed := strings.TrimRight(strings.TrimSpace(divoidURL), "/")

	parsed, err := neturl.Parse(trimmed)
	if err != nil || parsed.Host == "" {
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

	temperature, err := loadModelTemperature(lookup)
	if err != nil {
		return ModelConfig{}, err
	}

	topP, err := loadModelTopP(lookup)
	if err != nil {
		return ModelConfig{}, err
	}

	return ModelConfig{URL: url, ID: id, Key: key, Temperature: temperature, TopP: topP}, nil
}

func loadModelTemperature(lookup lookupFunc) (*float64, error) {
	val, err := optionalFloatEnv(lookup, envModelTemperature)
	if err != nil {
		return nil, err
	}
	if val == nil {
		d := defaultModelTemperature
		return &d, nil
	}
	return val, nil
}

func loadModelTopP(lookup lookupFunc) (*float64, error) {
	return optionalFloatEnv(lookup, envModelTopP)
}

func optionalFloatEnv(lookup lookupFunc, key string) (*float64, error) {
	val, present := lookup(key)
	if !present {
		return nil, nil
	}
	if val == "" {
		return nil, fmt.Errorf("%s is set but empty", key)
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return nil, fmt.Errorf("%s is %q, which is not a valid number: %w", key, val, err)
	}
	return &f, nil
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
