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

	// envModelURL and envModelID are unit B's two required boot members:
	// properties of *which endpoint you pointed this at*, and both vary
	// by construction (design §8.1) — there is no defensible default for
	// either, so absent is an error exactly like the DiVoid members.
	envModelURL = "PROCESSOR_MODEL_URL"
	envModelID  = "PROCESSOR_MODEL_ID"

	// envModelKey is unit B's one optional boot member (design §8.1):
	// absent means "send no Authorization header" — a deliberate
	// statement (a local endpoint commonly needs none), never a default.
	// Present-but-empty is still a startup error, exactly like every
	// required member: empty is a mistake, never a way to spell "no
	// auth" — treating it as absent would be a silent auth downgrade.
	envModelKey = "PROCESSOR_MODEL_KEY"
)

// bootConfig is the process's configuration, assembled once at startup.
type bootConfig struct {
	httpAddr  string
	divoidURL string
	divoidKey string
	modelURL  string
	modelID   string
	modelKey  string // "" means: send no Authorization header (design §8.1)
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

// optionalEnv reads an optional configuration member: absent yields "",
// which the model adapter reads as "send no Authorization header" —
// present-but-empty is still a startup error, naming the variable, never
// the value. This is requireEnv's sibling, not its exception (design
// §8.1): "present-but-empty is an error for every member" is the one
// rule; what "absent" means is the one axis that varies.
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
