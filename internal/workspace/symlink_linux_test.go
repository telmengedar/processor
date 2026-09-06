package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRejectsAPathLeadingThroughASymbolicLinkToADirectoryOutsideTheRun(t *testing.T) {
	t.Parallel()

	outside := t.TempDir()
	w, dir := openRun(t)

	if err := os.Symlink(outside, filepath.Join(dir, "assets")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	assertRejected(t, w, dir, "assets/style.css", "body{}")

	if _, err := os.Stat(filepath.Join(outside, "style.css")); !os.IsNotExist(err) {
		t.Fatalf("Stat outside the run = %v, want the write never to have landed there", err)
	}
}

func TestWriteRejectsWritingThroughASymbolicLinkStandingWhereTheFileWouldGo(t *testing.T) {
	t.Parallel()

	outside := t.TempDir()
	target := filepath.Join(outside, "index.html")
	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatalf("write the outside file: %v", err)
	}

	w, dir := openRun(t)
	if err := os.Symlink(target, filepath.Join(dir, "index.html")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	assertRejected(t, w, dir, "index.html", "overwritten")

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read the outside file: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("the outside file now reads %q, want %q — the write followed the link", got, "original")
	}
}

func TestWriteAcceptsARunDirectoryReachedThroughASymbolicLinkedRoot(t *testing.T) {
	t.Parallel()

	real := t.TempDir()
	linked := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(real, linked); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	w := New(linked)
	dir, err := w.OpenRun(context.Background())
	if err != nil {
		t.Fatalf("OpenRun: %v", err)
	}

	if _, err := w.Write(context.Background(), dir, "index.html", "ok"); err != nil {
		t.Fatalf("Write: %v — a root the operator gave as a symbolic link is legitimate and must not be refused", err)
	}
}
