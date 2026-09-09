package server

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

const scriptsDir = "../../scripts"

var shutdownGraceDeclaration = regexp.MustCompile(`^SHUTDOWN_GRACE_S\s*(?::[^=]*)?=\s*(.*)$`)
var drainGraceDeclaration = regexp.MustCompile(`^DRAIN_GRACE_S\s*(?::[^=]*)?=\s*(.*)$`)
var bareDecimalInteger = regexp.MustCompile(`^[0-9]+$`)
var mirrorPlusPositiveMargin = regexp.MustCompile(`^SHUTDOWN_GRACE_S\s*\+\s*([0-9]+)$`)

func stripPythonComment(s string) string {
	if idx := strings.Index(s, "#"); idx != -1 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func parseBareDecimalInteger(s string) (int, bool) {
	if !bareDecimalInteger.MatchString(s) {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func scanScriptFileForGraceDeclarations(t *testing.T, path string, wantSeconds int) (mirrorFound, derivationFound bool) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if match := shutdownGraceDeclaration.FindStringSubmatch(line); match != nil {
			mirrorFound = true
			value := stripPythonComment(match[1])
			got, ok := parseBareDecimalInteger(value)
			if !ok {
				t.Errorf("%s declares a SHUTDOWN_GRACE_S line that does not satisfy design §7.1: %q", path, line)
			} else if got != wantSeconds {
				t.Errorf("%s declares SHUTDOWN_GRACE_S = %d, but shutdownGrace is %d seconds", path, got, wantSeconds)
			}
		}

		if match := drainGraceDeclaration.FindStringSubmatch(line); match != nil {
			derivationFound = true
			value := stripPythonComment(match[1])
			addend := mirrorPlusPositiveMargin.FindStringSubmatch(value)
			margin, ok := 0, false
			if addend != nil {
				margin, ok = parseBareDecimalInteger(addend[1])
			}
			if !ok || margin < 1 {
				t.Errorf("%s declares a DRAIN_GRACE_S line that does not satisfy design §7.1: %q", path, line)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanning %s: %v", path, err)
	}
	return mirrorFound, derivationFound
}

func TestEveryDeclaredHarnessGraceEqualsShutdownGraceAndEveryDeclaredWaitIsThatMirrorPlusAPositiveMargin(t *testing.T) {
	if shutdownGrace%time.Second != 0 {
		t.Fatalf("shutdownGrace = %v is not a whole number of seconds", shutdownGrace)
	}
	wantSeconds := int(shutdownGrace / time.Second)

	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		t.Fatalf("reading %s: %v", scriptsDir, err)
	}

	mirrorsFound := 0
	derivationsFound := 0

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".py" {
			continue
		}
		path := filepath.Join(scriptsDir, entry.Name())
		mirrorFound, derivationFound := scanScriptFileForGraceDeclarations(t, path, wantSeconds)
		if mirrorFound {
			mirrorsFound++
		}
		if derivationFound {
			derivationsFound++
		}
	}

	if mirrorsFound == 0 {
		t.Fatalf("found no SHUTDOWN_GRACE_S declaration under %s", scriptsDir)
	}
	if derivationsFound == 0 {
		t.Fatalf("found no DRAIN_GRACE_S declaration under %s", scriptsDir)
	}
}
