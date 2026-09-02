package main

import "fmt"

const (
	envHTTPAddr     = "PROCESSOR_HTTP_ADDR"
	defaultHTTPAddr = "127.0.0.1:8080"

	// envDivoidURL and envDivoidKey are unit A's two boot members (design
	// §8.1): the graph URL and key M0 deliberately left undeclared, and
	// M1 is the milestone that first dereferences them. Both are
	// required — there is no defensible default for a base URL or a key
	// (design §8.1's required/optional table).
	envDivoidURL = "PROCESSOR_DIVOID_URL"
	envDivoidKey = "PROCESSOR_DIVOID_KEY"
)

// bootConfig is the process's configuration, assembled once at startup.
type bootConfig struct {
	httpAddr  string
	divoidURL string
	divoidKey string
}

// lookupFunc mirrors os.LookupEnv: a value and whether the key is present.
type lookupFunc func(key string) (string, bool)

// loadBootConfig builds the boot configuration from lookup. This remains
// the module's single environment read site (design S6): every member is
// read here, and nothing below main reads the environment.
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

	return bootConfig{httpAddr: addr, divoidURL: divoidURL, divoidKey: divoidKey}, nil
}

// requireEnv reads a required configuration member. Absent and
// present-but-empty are both startup errors, and the error names the
// variable, never the value — no secret is ever logged or echoed (design
// §8.1's secrets discipline).
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
