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

// validEnv returns an environment with every required member present
// (both DiVoid members and unit B's two required model members —
// PROCESSOR_MODEL_KEY is optional and deliberately left absent here), so
// tests that are about one member don't also have to fight another
// required member's error. overrides wins over the defaults, and a key
// mapped to "" still counts as present (matching os.LookupEnv's own
// present-but-empty semantics) — only deleting the key from the returned
// map makes it absent.
//
// This is also the process-boundary harness's boot environment
// (process_linux_test.go's harnessEnv reads it directly) — the coupling
// this doc comment exists to make discoverable: config.go growing a new
// required boot member without a matching addition here fails every
// "happy path" test below immediately, on every platform this package
// builds on, not just on the Linux-only container gate that hosts the
// real subprocess tests. Keep it in sync with loadBootConfig.
func validEnv(overrides map[string]string) map[string]string {
	env := map[string]string{
		"PROCESSOR_DIVOID_URL": "https://graph.example/api",
		"PROCESSOR_DIVOID_KEY": "test-key-12345",
		"PROCESSOR_MODEL_URL":  "https://model.example/v1",
		"PROCESSOR_MODEL_ID":   "test-model-id",
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

// --- PROCESSOR_MODEL_URL (required, new in unit B) ---

func TestLoadBootConfigErrorsWhenModelURLAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_MODEL_URL")

	_, err := loadBootConfig(fixedLookup(env))
	if err == nil {
		t.Fatal("loadBootConfig returned nil error with PROCESSOR_MODEL_URL absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_URL", err.Error())
	}
}

func TestLoadBootConfigErrorsWhenModelURLPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_MODEL_URL": ""})

	_, err := loadBootConfig(fixedLookup(env))
	if err == nil {
		t.Fatal("loadBootConfig returned nil error for an empty PROCESSOR_MODEL_URL, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_URL", err.Error())
	}
}

func TestLoadBootConfigUsesModelURLVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "https://custom.model.internal/v1"
	env := validEnv(map[string]string{"PROCESSOR_MODEL_URL": want})

	cfg, err := loadBootConfig(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadBootConfig: %v", err)
	}
	if cfg.modelURL != want {
		t.Fatalf("modelURL = %q, want %q", cfg.modelURL, want)
	}
}

// --- PROCESSOR_MODEL_ID (required, new in unit B) ---

func TestLoadBootConfigErrorsWhenModelIDAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_MODEL_ID")

	_, err := loadBootConfig(fixedLookup(env))
	if err == nil {
		t.Fatal("loadBootConfig returned nil error with PROCESSOR_MODEL_ID absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_ID") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_ID", err.Error())
	}
}

func TestLoadBootConfigErrorsWhenModelIDPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_MODEL_ID": ""})

	_, err := loadBootConfig(fixedLookup(env))
	if err == nil {
		t.Fatal("loadBootConfig returned nil error for an empty PROCESSOR_MODEL_ID, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_ID") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_ID", err.Error())
	}
}

func TestLoadBootConfigUsesModelIDVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "llama-3.1-8b-instruct"
	env := validEnv(map[string]string{"PROCESSOR_MODEL_ID": want})

	cfg, err := loadBootConfig(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadBootConfig: %v", err)
	}
	if cfg.modelID != want {
		t.Fatalf("modelID = %q, want %q", cfg.modelID, want)
	}
}

// --- PROCESSOR_MODEL_KEY (optional, new in unit B) ---
//
// design §8.1's whole point: present-but-empty is an error for every
// member, without exception — what differs is only what *absent* means.
// A test that only covers absent-and-present would not catch a silent
// auth downgrade (design §14 step 7), so all three cases are pinned here.

func TestLoadBootConfigLeavesModelKeyEmptyWhenAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_MODEL_KEY")

	cfg, err := loadBootConfig(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadBootConfig: %v", err)
	}
	if cfg.modelKey != "" {
		t.Fatalf("modelKey = %q, want empty when PROCESSOR_MODEL_KEY is absent (no Authorization header is sent)", cfg.modelKey)
	}
}

func TestLoadBootConfigErrorsWhenModelKeyPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_MODEL_KEY": ""})

	_, err := loadBootConfig(fixedLookup(env))
	if err == nil {
		t.Fatal("loadBootConfig returned nil error for an empty PROCESSOR_MODEL_KEY, want an error — an empty value is a mistake, never a way to spell \"no auth\"")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_KEY") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_KEY", err.Error())
	}
}

func TestLoadBootConfigUsesModelKeyVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "local-runtime-key-1"
	env := validEnv(map[string]string{"PROCESSOR_MODEL_KEY": want})

	cfg, err := loadBootConfig(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadBootConfig: %v", err)
	}
	if cfg.modelKey != want {
		t.Fatalf("modelKey = %q, want %q", cfg.modelKey, want)
	}
}
