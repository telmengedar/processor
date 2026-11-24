package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/process"
)

func TestPrintAuthoredExamplesDoesNotRequireGraphConfiguration(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"gangolf-chat", "python"} {
		var output, diagnostic bytes.Buffer
		if exit := run([]string{"-example", name, "-persona-type", "knowledge", "-print"}, strings.NewReader(""), &output, &diagnostic); exit != 0 {
			t.Fatal(diagnostic.String())
		}
		var b process.Blueprint
		if json.Unmarshal(output.Bytes(), &b) != nil || len(b.Nodes) < 3 || len(b.Edges) == 0 {
			t.Fatal("blueprint not usable")
		}
		if name == "gangolf-chat" && b.Inputs["persona"].NodeType != "knowledge" {
			t.Fatal("persona node type changed")
		}
	}
}
