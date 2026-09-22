// Package boot reads the process environment into the configuration each binary needs.
package boot

import (
	"fmt"
	neturl "net/url"
	"os"
	"strconv"
	"strings"

	"github.com/telmengedar/processor/internal/redacturl"
)

const (
	envHTTPAddr     = "PROCESSOR_HTTP_ADDR"
	defaultHTTPAddr = "127.0.0.1:8080"

	envDivoidURL = "PROCESSOR_DIVOID_URL"
	envDivoidKey = "PROCESSOR_DIVOID_KEY"

	envModelProtocol = "PROCESSOR_MODEL_PROTOCOL"

	envModelURL = "PROCESSOR_MODEL_URL"
	envModelID  = "PROCESSOR_MODEL_ID"

	envModelKey = "PROCESSOR_MODEL_KEY"

	envWorkspaceDir = "PROCESSOR_WORKSPACE_DIR"

	envModelTemperature     = "PROCESSOR_MODEL_TEMPERATURE"
	defaultModelTemperature = 0.2

	envModelTopP = "PROCESSOR_MODEL_TOP_P"

	envFloorTokensPerSecond      = "PROCESSOR_MODEL_FLOOR_TOKENS_PER_SECOND"
	envFloorPromptBytesPerSecond = "PROCESSOR_MODEL_FLOOR_PROMPT_BYTES_PER_SECOND"

	envCondenseModelProtocol    = "PROCESSOR_CONDENSE_MODEL_PROTOCOL"
	envCondenseModelURL         = "PROCESSOR_CONDENSE_MODEL_URL"
	envCondenseModelID          = "PROCESSOR_CONDENSE_MODEL_ID"
	envCondenseModelKey         = "PROCESSOR_CONDENSE_MODEL_KEY"
	envCondenseModelTemperature = "PROCESSOR_CONDENSE_MODEL_TEMPERATURE"
	envCondenseModelTopP        = "PROCESSOR_CONDENSE_MODEL_TOP_P"

	apiPathSuffix   = "/api"
	nodesPathSuffix = "/api/nodes"
)

const (
	// ProtocolOpenAICompat selects the OpenAI-compatible chat-completions adapter.
	ProtocolOpenAICompat = "openai-compat"
	// ProtocolOllama selects the adapter speaking ollama's native chat protocol.
	ProtocolOllama = "ollama"
)

// GraphConfig is what a graph client needs to reach the graph.
type GraphConfig struct {
	URL string
	Key string
}

// ModelConfig is what a model client needs to reach a model endpoint.
type ModelConfig struct {
	Protocol    string
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

// LoadCondenseModel returns the fill's model configuration and whether one is configured at all; PROCESSOR_CONDENSE_MODEL_URL absent means configured is false, with a nil error and no fallback to PROCESSOR_MODEL_*.
func LoadCondenseModel() (cfg ModelConfig, configured bool, err error) {
	return loadCondenseModel(lookupEnv)
}

// FloorsConfig is what the deployment declares its model endpoint delivers, nil in either member where the operator declared nothing.
type FloorsConfig struct {
	TokensPerSecond      *float64
	PromptBytesPerSecond *float64
}

// LoadModelFloors returns the deployment's declared generation and prompt-processing floors; a non-positive declaration is a startup error, because it asserts an endpoint that never finishes.
func LoadModelFloors() (FloorsConfig, error) {
	return loadModelFloors(lookupEnv)
}

func loadModelFloors(lookup lookupFunc) (FloorsConfig, error) {
	tokens, err := positiveFloatEnv(lookup, envFloorTokensPerSecond)
	if err != nil {
		return FloorsConfig{}, err
	}

	promptBytes, err := positiveFloatEnv(lookup, envFloorPromptBytesPerSecond)
	if err != nil {
		return FloorsConfig{}, err
	}

	return FloorsConfig{TokensPerSecond: tokens, PromptBytesPerSecond: promptBytes}, nil
}

func positiveFloatEnv(lookup lookupFunc, key string) (*float64, error) {
	val, err := optionalFloatEnv(lookup, key)
	if err != nil {
		return nil, err
	}
	if val != nil && *val <= 0 {
		return nil, fmt.Errorf("%s is %v, which is not a rate a model endpoint can deliver at; declare the tokens or bytes per second this deployment asserts its endpoint sustains, or leave it unset for the product's own declaration", key, *val)
	}
	return val, nil
}

// LoadWorkspaceDir returns the root for run working directories, empty when the variable is absent.
func LoadWorkspaceDir() (string, error) {
	return loadWorkspaceDir(lookupEnv)
}

func loadWorkspaceDir(lookup lookupFunc) (string, error) {
	return optionalEnv(lookup, envWorkspaceDir)
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
		envDivoidURL, redacturl.URL(parsed.String()), badSuffix, nodesPathSuffix, redacturl.URL(wouldRequest.String()), redacturl.URL(corrected.String()),
	)
}

func loadModel(lookup lookupFunc) (ModelConfig, error) {
	protocol, err := loadProtocol(lookup, envModelProtocol)
	if err != nil {
		return ModelConfig{}, err
	}

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

	temperature, err := loadTemperature(lookup, envModelTemperature)
	if err != nil {
		return ModelConfig{}, err
	}

	topP, err := optionalFloatEnv(lookup, envModelTopP)
	if err != nil {
		return ModelConfig{}, err
	}

	return ModelConfig{Protocol: protocol, URL: url, ID: id, Key: key, Temperature: temperature, TopP: topP}, nil
}

func loadCondenseModel(lookup lookupFunc) (ModelConfig, bool, error) {
	url, present := lookup(envCondenseModelURL)
	if !present {
		return ModelConfig{}, false, nil
	}
	if url == "" {
		return ModelConfig{}, false, fmt.Errorf("%s is set but empty", envCondenseModelURL)
	}

	protocol, err := loadProtocol(lookup, envCondenseModelProtocol)
	if err != nil {
		return ModelConfig{}, false, err
	}

	id, err := requireEnv(lookup, envCondenseModelID)
	if err != nil {
		return ModelConfig{}, false, err
	}

	key, err := optionalEnv(lookup, envCondenseModelKey)
	if err != nil {
		return ModelConfig{}, false, err
	}

	temperature, err := loadTemperature(lookup, envCondenseModelTemperature)
	if err != nil {
		return ModelConfig{}, false, err
	}

	topP, err := optionalFloatEnv(lookup, envCondenseModelTopP)
	if err != nil {
		return ModelConfig{}, false, err
	}

	return ModelConfig{Protocol: protocol, URL: url, ID: id, Key: key, Temperature: temperature, TopP: topP}, true, nil
}

func loadProtocol(lookup lookupFunc, key string) (string, error) {
	protocol, present := lookup(key)
	if !present {
		return ProtocolOpenAICompat, nil
	}
	if protocol == "" {
		return "", fmt.Errorf("%s is set but empty", key)
	}
	if protocol != ProtocolOpenAICompat && protocol != ProtocolOllama {
		return "", fmt.Errorf("%s is %q, which is not a protocol this service has an adapter for; set it to %q or %q, or leave it unset for %q",
			key, protocol, ProtocolOpenAICompat, ProtocolOllama, ProtocolOpenAICompat)
	}
	return protocol, nil
}

func loadTemperature(lookup lookupFunc, key string) (*float64, error) {
	val, err := optionalFloatEnv(lookup, key)
	if err != nil {
		return nil, err
	}
	if val == nil {
		d := defaultModelTemperature
		return &d, nil
	}
	return val, nil
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
