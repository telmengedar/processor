package main

import (
	"strings"
	"testing"
)

func TestLoadBootConfigDefaultsWhenAbsent(t *testing.T) {
	t.Parallel()

	lookup := func(string) (string, bool) { return "", false }

	cfg, err := loadBootConfig(lookup)
	if err != nil {
		t.Fatalf("loadBootConfig: %v", err)
	}
	const want = "127.0.0.1:8080"
	if cfg.httpAddr != want {
		t.Fatalf("httpAddr = %q, want %q", cfg.httpAddr, want)
	}
}

func TestLoadBootConfigUsesPresentValueVerbatim(t *testing.T) {
	t.Parallel()

	const want = "0.0.0.0:9090"
	lookup := func(key string) (string, bool) {
		if key == "PROCESSOR_HTTP_ADDR" {
			return want, true
		}
		return "", false
	}

	cfg, err := loadBootConfig(lookup)
	if err != nil {
		t.Fatalf("loadBootConfig: %v", err)
	}
	if cfg.httpAddr != want {
		t.Fatalf("httpAddr = %q, want %q", cfg.httpAddr, want)
	}
}

func TestLoadBootConfigErrorsWhenPresentButEmpty(t *testing.T) {
	t.Parallel()

	lookup := func(key string) (string, bool) {
		if key == "PROCESSOR_HTTP_ADDR" {
			return "", true
		}
		return "", false
	}

	_, err := loadBootConfig(lookup)
	if err == nil {
		t.Fatal("loadBootConfig returned nil error for an empty variable, want an error")
	}
	if !strings.Contains(err.Error(), "PROCESSOR_HTTP_ADDR") {
		t.Fatalf("error = %q, want it to name PROCESSOR_HTTP_ADDR", err.Error())
	}
}
