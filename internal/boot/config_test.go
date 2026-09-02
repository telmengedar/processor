package boot

import (
	"strings"
	"testing"
)

func fixedLookup(values map[string]string) lookupFunc {
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}

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

func TestLoadHTTPAddrDefaultsWhenAbsent(t *testing.T) {
	t.Parallel()

	addr, err := loadHTTPAddr(fixedLookup(validEnv(nil)))
	if err != nil {
		t.Fatalf("loadHTTPAddr: %v", err)
	}
	const want = "127.0.0.1:8080"
	if addr != want {
		t.Fatalf("httpAddr = %q, want %q", addr, want)
	}
}

func TestLoadHTTPAddrUsesItVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "0.0.0.0:9090"
	env := validEnv(map[string]string{"PROCESSOR_HTTP_ADDR": want})

	addr, err := loadHTTPAddr(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadHTTPAddr: %v", err)
	}
	if addr != want {
		t.Fatalf("httpAddr = %q, want %q", addr, want)
	}
}

func TestLoadHTTPAddrErrorsWhenPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_HTTP_ADDR": ""})

	_, err := loadHTTPAddr(fixedLookup(env))
	if err == nil {
		t.Fatal("loadHTTPAddr returned nil error for an empty PROCESSOR_HTTP_ADDR, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_HTTP_ADDR") {
		t.Fatalf("error = %q, want it to name PROCESSOR_HTTP_ADDR", err.Error())
	}
}

func TestLoadGraphErrorsWhenDivoidURLAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_DIVOID_URL")

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error with PROCESSOR_DIVOID_URL absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
	}
}

func TestLoadGraphErrorsWhenDivoidURLPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": ""})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error for an empty PROCESSOR_DIVOID_URL, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
	}
}

func TestLoadGraphUsesDivoidURLVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "https://custom.graph.internal/api"
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_URL": want})

	cfg, err := loadGraph(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadGraph: %v", err)
	}
	if cfg.URL != want {
		t.Fatalf("divoidURL = %q, want %q", cfg.URL, want)
	}
}

func TestLoadGraphErrorsWhenDivoidKeyAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_DIVOID_KEY")

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error with PROCESSOR_DIVOID_KEY absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_KEY") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_KEY", err.Error())
	}
}

func TestLoadGraphErrorsWhenDivoidKeyPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_DIVOID_KEY": ""})

	_, err := loadGraph(fixedLookup(env))
	if err == nil {
		t.Fatal("loadGraph returned nil error for an empty PROCESSOR_DIVOID_KEY, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_KEY") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_KEY", err.Error())
	}
}

func TestLoadGraphUsesDivoidKeyVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "sekrit-value-99"
	env := validEnv(map[string]string{"PROCESSOR_DIVOID_KEY": want})

	cfg, err := loadGraph(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadGraph: %v", err)
	}
	if cfg.Key != want {
		t.Fatalf("divoidKey = %q, want %q", cfg.Key, want)
	}
}

func TestGraphBootConfigLoadsWhenNoModelVariableIsSet(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"PROCESSOR_DIVOID_URL": "https://graph.example/api",
		"PROCESSOR_DIVOID_KEY": "test-key-12345",
	}

	cfg, err := loadGraph(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadGraph with no model variable set: %v", err)
	}
	if cfg.URL != env["PROCESSOR_DIVOID_URL"] {
		t.Fatalf("divoidURL = %q, want %q", cfg.URL, env["PROCESSOR_DIVOID_URL"])
	}
	if cfg.Key != env["PROCESSOR_DIVOID_KEY"] {
		t.Fatalf("divoidKey = %q, want %q", cfg.Key, env["PROCESSOR_DIVOID_KEY"])
	}
}

func TestBootConfigErrorsNameTheVariableAndNeverItsValue(t *testing.T) {
	t.Parallel()

	const secret = "sekrit-value-99"

	graphEnv := validEnv(map[string]string{
		"PROCESSOR_DIVOID_KEY": secret,
		"PROCESSOR_DIVOID_URL": "",
	})

	_, err := loadGraph(fixedLookup(graphEnv))
	if err == nil {
		t.Fatal("loadGraph returned nil error for an empty PROCESSOR_DIVOID_URL, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_DIVOID_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_DIVOID_URL", err.Error())
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error = %q, must not contain the secret value %q", err.Error(), secret)
	}

	modelEnv := validEnv(map[string]string{
		"PROCESSOR_MODEL_KEY": secret,
		"PROCESSOR_MODEL_ID":  "",
	})

	_, err = loadModel(fixedLookup(modelEnv))
	if err == nil {
		t.Fatal("loadModel returned nil error for an empty PROCESSOR_MODEL_ID, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_ID") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_ID", err.Error())
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error = %q, must not contain the secret value %q", err.Error(), secret)
	}
}

func TestModelBootConfigErrorsWhenTheModelUrlIsAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_MODEL_URL")

	_, err := loadModel(fixedLookup(env))
	if err == nil {
		t.Fatal("loadModel returned nil error with PROCESSOR_MODEL_URL absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_URL", err.Error())
	}
}

func TestLoadModelErrorsWhenModelURLPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_MODEL_URL": ""})

	_, err := loadModel(fixedLookup(env))
	if err == nil {
		t.Fatal("loadModel returned nil error for an empty PROCESSOR_MODEL_URL, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_URL") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_URL", err.Error())
	}
}

func TestLoadModelUsesModelURLVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "https://custom.model.internal/v1"
	env := validEnv(map[string]string{"PROCESSOR_MODEL_URL": want})

	cfg, err := loadModel(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadModel: %v", err)
	}
	if cfg.URL != want {
		t.Fatalf("modelURL = %q, want %q", cfg.URL, want)
	}
}

func TestLoadModelErrorsWhenModelIDAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_MODEL_ID")

	_, err := loadModel(fixedLookup(env))
	if err == nil {
		t.Fatal("loadModel returned nil error with PROCESSOR_MODEL_ID absent, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_ID") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_ID", err.Error())
	}
}

func TestLoadModelErrorsWhenModelIDPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_MODEL_ID": ""})

	_, err := loadModel(fixedLookup(env))
	if err == nil {
		t.Fatal("loadModel returned nil error for an empty PROCESSOR_MODEL_ID, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_ID") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_ID", err.Error())
	}
}

func TestLoadModelUsesModelIDVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "llama-3.1-8b-instruct"
	env := validEnv(map[string]string{"PROCESSOR_MODEL_ID": want})

	cfg, err := loadModel(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadModel: %v", err)
	}
	if cfg.ID != want {
		t.Fatalf("modelID = %q, want %q", cfg.ID, want)
	}
}

func TestLoadModelLeavesModelKeyEmptyWhenAbsent(t *testing.T) {
	t.Parallel()

	env := validEnv(nil)
	delete(env, "PROCESSOR_MODEL_KEY")

	cfg, err := loadModel(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadModel: %v", err)
	}
	if cfg.Key != "" {
		t.Fatalf("modelKey = %q, want empty when PROCESSOR_MODEL_KEY is absent (no Authorization header is sent)", cfg.Key)
	}
}

func TestLoadModelErrorsWhenModelKeyPresentButEmpty(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{"PROCESSOR_MODEL_KEY": ""})

	_, err := loadModel(fixedLookup(env))
	if err == nil {
		t.Fatal("loadModel returned nil error for an empty PROCESSOR_MODEL_KEY, want an error — an empty value is a mistake, never a way to spell \"no auth\"")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_MODEL_KEY") {
		t.Fatalf("error = %q, want it to name PROCESSOR_MODEL_KEY", err.Error())
	}
}

func TestLoadModelUsesModelKeyVerbatimWhenPresent(t *testing.T) {
	t.Parallel()

	const want = "local-runtime-key-1"
	env := validEnv(map[string]string{"PROCESSOR_MODEL_KEY": want})

	cfg, err := loadModel(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadModel: %v", err)
	}
	if cfg.Key != want {
		t.Fatalf("modelKey = %q, want %q", cfg.Key, want)
	}
}

func TestExportedLoadersReadTheProcessEnvironment(t *testing.T) {
	t.Setenv("PROCESSOR_HTTP_ADDR", "0.0.0.0:7070")
	t.Setenv("PROCESSOR_DIVOID_URL", "https://env.graph.example/api")
	t.Setenv("PROCESSOR_DIVOID_KEY", "env-graph-key")
	t.Setenv("PROCESSOR_MODEL_URL", "https://env.model.example/v1")
	t.Setenv("PROCESSOR_MODEL_ID", "env-model-id")
	t.Setenv("PROCESSOR_MODEL_KEY", "env-model-key")

	addr, err := LoadHTTPAddr()
	if err != nil {
		t.Fatalf("LoadHTTPAddr: %v", err)
	}
	if addr != "0.0.0.0:7070" {
		t.Fatalf("httpAddr = %q, want %q", addr, "0.0.0.0:7070")
	}

	graph, err := LoadGraph()
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if graph.URL != "https://env.graph.example/api" || graph.Key != "env-graph-key" {
		t.Fatalf("graph = %+v, want the PROCESSOR_DIVOID_ variables", graph)
	}

	model, err := LoadModel()
	if err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	if model.URL != "https://env.model.example/v1" || model.ID != "env-model-id" || model.Key != "env-model-key" {
		t.Fatalf("model = %+v, want the PROCESSOR_MODEL_ variables", model)
	}
}
