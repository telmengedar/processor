package main

import "fmt"

const (
	envHTTPAddr     = "PROCESSOR_HTTP_ADDR"
	defaultHTTPAddr = "127.0.0.1:8080"
)

// bootConfig is the process's configuration, assembled once at startup.
type bootConfig struct {
	httpAddr string
}

// lookupFunc mirrors os.LookupEnv: a value and whether the key is present.
type lookupFunc func(key string) (string, bool)

// loadBootConfig builds the boot configuration from lookup.
func loadBootConfig(lookup lookupFunc) (bootConfig, error) {
	addr, present := lookup(envHTTPAddr)
	if !present {
		return bootConfig{httpAddr: defaultHTTPAddr}, nil
	}
	if addr == "" {
		return bootConfig{}, fmt.Errorf("%s is set but empty", envHTTPAddr)
	}
	return bootConfig{httpAddr: addr}, nil
}
