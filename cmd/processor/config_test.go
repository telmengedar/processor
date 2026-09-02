package main

import (
	"strings"
	"testing"
)

// fixedLookup turns a plain map into a lookupFunc, so each test can state
// exactly the environment it wants without hand-rolling a closure.
func fixedLookup(values map[string]string) lookupFunc {
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}

// validEnv returns an environment with both required DiVoid members
// present, so tests that are about httpAddr (or about one DiVoid member)
// don't also have to fight the other required member's error. overrides
// wins over the defaults, and a key mapped to "" still counts as present
// (matching os.LookupEnv's own present-but-empty semantics) — only
// deleting the key from the returned map makes it absent.
func validEnv(overrides map[string]string) map[string]string {
	env := map[string]string{
		"PROCESSOR_DIVOID_URL": "https://graph.example/api",
		"PROCESSOR_DIVOID_KEY": "test-key-12345",
	}
	for k, v := range overrides {
		env[k] = v
	}
	return env
}

// --- PROCESSOR_HTTP_ADDR (optional, unchanged from M0) ---

func TestLoadBootConfigDefaultsHTTPAddrWhenAbsent(t *testing.T) {
	t.Parallel()

	cfg, err := loadBootConfig(fixedLookup(validEnv(nil)))
	if err != nil {
		t.Fatalf("loadBootConfig: %v", err)
	}
	const want = "127.0.0.1:8080"
	if cfg.httpAddr != want {
		t.Fatalf("httpAddr = %q, want %q", cfg.httpAddr, want)
	}
}

func TestLoadBootConfigUsesHTTPAddrVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "0.0.0.0:9090"
	env := validEnv(map[string]string{"PROCESSOR_HTTP_ADDR": want})

	cfg, err := loadBootConfig(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadBootConfig: %v", err)
	}
	if cfg.httpAddr != want {
		t.Fatalf("httpAddr = %q, want %q", cfg.httpAddr, want)
	}
}

func TestLoadBootConfigErrorsWhenHTTPAddrPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_HTTP_ADDR": ""})

	_, err := loadBootConfig(fixedLookup(env))
	if err == nil {
		t.Fatal("loadBootConfig returned nil error for an empty PROCESSOR_HTTP_ADDR, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_HTTP_ADDR") {
		t.Fatalf("error = %q, want it to name PROCESSOR_HTTP_ADDR", err.Error())
	}
}

// --- PROCESSOR_DIVOID_URL (required, new in M1) ---

func TestLoadBootConfigErrorsWhenDivoidURLAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_DIVOID_URL")

	_, err := loadBootConfig(fixedLookup(env))
	if err == nil {
		t.Fatal("loadBootConfig returned nil error with PROCESSOR_DIVOID_URL absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
	}
}

func TestLoadBootConfigErrorsWhenDivoidURLPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": ""})

	_, err := loadBootConfig(fixedLookup(env))
	if err == nil {
		t.Fatal("loadBootConfig returned nil error for an empty PROCESSOR_DIVOID_URL, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
	}
}

func TestLoadBootConfigUsesDivoidURLVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "https://custom.graph.internal/api"
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": want})

	cfg, err := loadBootConfig(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadBootConfig: %v", err)
	}
	if cfg.divoidURL != want {
		t.Fatalf("divoidURL = %q, want %q", cfg.divoidURL, want)
	}
}

// --- PROCESSOR_DIVOID_KEY (required, new in M1) ---

func TestLoadBootConfigErrorsWhenDivoidKeyAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_DIVOID_KEY")

	_, err := loadBootConfig(fixedLookup(env))
	if err == nil {
		t.Fatal("loadBootConfig returned nil error with PROCESSOR_DIVOID_KEY absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_KEY") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_KEY", err.Error())
	}
}

func TestLoadBootConfigErrorsWhenDivoidKeyPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_DIVOID_KEY": ""})

	_, err := loadBootConfig(fixedLookup(env))
	if err == nil {
		t.Fatal("loadBootConfig returned nil error for an empty PROCESSOR_DIVOID_KEY, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_KEY") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_KEY", err.Error())
	}
}

func TestLoadBootConfigUsesDivoidKeyVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "sekrit-value-99"
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_KEY": want})

	cfg, err := loadBootConfig(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadBootConfig: %v", err)
	}
	if cfg.divoidKey != want {
		t.Fatalf("divoidKey = %q, want %q", cfg.divoidKey, want)
	}
}

// TestLoadBootConfigErrorNeverContainsAPresentSecretValue pins design
// §8.1's secrets discipline: when a required secret is present but a
// *different* member is what's wrong, the resulting error must not echo
// the secret's value anywhere.
func TestLoadBootConfigErrorNeverContainsAPresentSecretValue(t *testing.T) {
	t.Parallel()

	const secret = "sekrit-value-99"
	env := validEnv(map[string]string{
		"PROCESSOR_DIVOID_KEY": secret,
		"PROCESSOR_HTTP_ADDR":  "", // the unrelated member that fails
	})

	_, err := loadBootConfig(fixedLookup(env))
	if err == nil {
		t.Fatal("loadBootConfig returned nil error for an empty PROCESSOR_HTTP_ADDR, want an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error = %q, must not contain the secret value %q", err.Error(), secret)
	}
}
