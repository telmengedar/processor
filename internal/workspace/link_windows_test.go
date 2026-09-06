package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func junction(t *testing.T, link, target string) {
	t.Helper()

	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Fatalf("mklink /J %q %q: %v; output: %s", link, target, err, out)
	}
}

func TestWriteRejectsAPathLeadingThroughAJunctionToADirectoryOutsideTheRun(t *testing.T) {
	t.Parallel()

	outside := t.TempDir()
	w, dir := openRun(t)
	junction(t, filepath.Join(dir, "assets"), outside)

	assertRejected(t, w, dir, "assets/style.css", "body{}", "path resolves outside the working directory")

	if _, err := os.Stat(filepath.Join(outside, "style.css")); !os.IsNotExist(err) {
		t.Fatalf("Stat outside the run = %v, want the write never to have landed there", err)
	}
}

func TestWriteRejectsAJunctionStandingWhereTheParentDirectoryWouldGo(t *testing.T) {
	t.Parallel()

	outside := t.TempDir()
	target := filepath.Join(outside, "index.html")
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatalf("write the outside file: %v", err)
	}

	w, dir := openRun(t)
	junction(t, filepath.Join(dir, "site"), outside)

	assertRejected(t, w, dir, "site/index.html", "overwritten", "path resolves outside the working directory")

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read the outside file: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("the outside file now reads %q, want %q — the write followed the junction", got, "original")
	}
}

func TestWriteAcceptsARealDirectoryWhereAJunctionWouldHaveBeenRefused(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	if _, err := w.Write(context.Background(), dir, "assets/style.css", "body{}"); err != nil {
		t.Fatalf("Write: %v — an ordinary directory of the same name is legitimate and must not be refused", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "style.css")); err != nil {
		t.Fatalf("Stat the nested file: %v", err)
	}
}

func TestWriteRefusesTheNullDeviceInsteadOfDiscardingTheBytesAndReportingSuccess(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)
	assertRejected(t, w, dir, "NUL", "sixsix", "path resolves outside the working directory")
}

func TestWriteRefusesTheNullDeviceReachedThroughASubdirectory(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)
	assertRejected(t, w, dir, "sub/NUL", "sixsix", "path resolves outside the working directory")
}

func TestWriteRefusesTheNullDeviceSpelledInLowerCase(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)
	assertRejected(t, w, dir, "nul", "sixsix", "path resolves outside the working directory")
}
