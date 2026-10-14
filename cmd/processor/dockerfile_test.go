package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTheDockerfileStampsTheBuildIntoTheBinaryAndRefusesAnEmptyOne(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	text := string(raw)

	if !regexp.MustCompile(`(?m)^ARG PROCESSOR_BUILD$`).MatchString(text) {
		t.Fatalf("the Dockerfile does not declare ARG PROCESSOR_BUILD")
	}
	if !regexp.MustCompile(`(?m)^RUN test -n "\$\{PROCESSOR_BUILD\}" \|\| .*PROCESSOR_BUILD is required.*exit 1`).MatchString(text) {
		t.Fatalf("the Dockerfile does not refuse an empty PROCESSOR_BUILD with a message naming the flag")
	}

	var goBuild string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "RUN ") && strings.Contains(line, "go build") {
			goBuild = line
		}
	}
	if goBuild == "" {
		t.Fatal("the Dockerfile has no RUN line with go build")
	}
	if !strings.Contains(goBuild, "-X main.build=${PROCESSOR_BUILD}") {
		t.Fatalf("the go build line does not stamp main.build from PROCESSOR_BUILD: %s", goBuild)
	}
}

func TestTheDockerfileStampsTheSymbolTheBinaryDeclares(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !regexp.MustCompile(`(?m)^var build string$`).Match(raw) {
		t.Fatal("main.go does not declare the package-level variable build the Dockerfile stamps")
	}
}
