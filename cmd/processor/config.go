package main

import "fmt"

const (
	envHTTPAddr     = "PROCESSOR_HTTP_ADDR"
	defaultHTTPAddr = "127.0.0.1:8080"

	envDivoidURL = "PROCESSOR_DIVOID_URL"
	envDivoidKey = "PROCESSOR_DIVOID_KEY"

	envModelURL = "PROCESSOR_MODEL_URL"
	envModelID  = "PROCESSOR_MODEL_ID"

	envModelKey = "PROCESSOR_MODEL_KEY"
)

type bootConfig struct {
	httpAddr  string
	divoidURL string
	divoidKey string
	modelURL  string
	modelID   string
	modelKey  string
}

type lookupFunc func(key string) (string, bool)

func loadBootConfig(lookup lookupFunc) (bootConfig, error) {
	addr, present := lookup(envHTTPAddr)
	if !present {
		addr = defaultHTTPAddr
	} else if addr == "" {
		return bootConfig{}, fmt.Errorf("%s is set but empty", envHTTPAddr)
	}

	divoidURL, err := requireEnv(lookup, envDivoidURL)
	if err != nil {
		return bootConfig{}, err
	}

	divoidKey, err := requireEnv(lookup, envDivoidKey)
	if err != nil {
		return bootConfig{}, err
	}

	modelURL, err := requireEnv(lookup, envModelURL)
	if err != nil {
		return bootConfig{}, err
	}

	modelID, err := requireEnv(lookup, envModelID)
	if err != nil {
		return bootConfig{}, err
	}

	modelKey, err := optionalEnv(lookup, envModelKey)
	if err != nil {
		return bootConfig{}, err
	}

	return bootConfig{
		httpAddr:  addr,
		divoidURL: divoidURL,
		divoidKey: divoidKey,
		modelURL:  modelURL,
		modelID:   modelID,
		modelKey:  modelKey,
	}, nil
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
