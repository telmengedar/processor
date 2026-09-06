package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func openRun(t *testing.T) (*Workspace, string) {
	t.Helper()

	w := New(t.TempDir())
	dir, err := w.OpenRun(context.Background())
	if err != nil {
		t.Fatalf("OpenRun: %v", err)
	}
	return w, dir
}

func TestOpenRunCreatesADistinctDirectoryPerCallBeneathTheRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	w := New(root)

	first, err := w.OpenRun(context.Background())
	if err != nil {
		t.Fatalf("first OpenRun: %v", err)
	}
	second, err := w.OpenRun(context.Background())
	if err != nil {
		t.Fatalf("second OpenRun: %v", err)
	}

	if first == second {
		t.Fatalf("both runs opened %q, want a directory each so concurrent runs cannot overwrite each other", first)
	}
	for _, dir := range []string{first, second} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Fatalf("Stat(%q) = %v, %v, want an existing directory", dir, info, err)
		}
		assertUnderRoot(t, root, dir)
	}
}

func TestOpenRunCreatesARootThatDoesNotExistYet(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "not", "created", "yet")
	dir, err := New(root).OpenRun(context.Background())
	if err != nil {
		t.Fatalf("OpenRun: %v", err)
	}
	assertUnderRoot(t, root, dir)
}

func TestWriteCreatesTheFileWithTheContentGivenInsideTheRunDirectory(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)

	const content = "<!doctype html><title>hi</title>"
	written, err := w.Write(context.Background(), dir, "index.html", content)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if written != len(content) {
		t.Fatalf("Write returned %d, want %d", written, len(content))
	}

	got, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != content {
		t.Fatalf("file content = %q, want %q", got, content)
	}
}

func TestWriteReportsBytesNotRunesForMultiByteContent(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)

	const content = "größer — ünïcode"
	written, err := w.Write(context.Background(), dir, "page.html", content)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if written == len([]rune(content)) {
		t.Fatalf("Write returned %d, which is the rune count; want the byte count %d", written, len(content))
	}
	if written != len(content) {
		t.Fatalf("Write returned %d, want %d bytes", written, len(content))
	}
}

func TestWriteCreatesMissingParentDirectoriesInsideTheRunDirectory(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)

	if _, err := w.Write(context.Background(), dir, "site/assets/style.css", "body{}"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "site", "assets", "style.css")); err != nil {
		t.Fatalf("Stat the nested file: %v", err)
	}
}

func TestWriteAcceptsATraversalThatNormalisesBackInsideTheRunDirectory(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)

	if _, err := w.Write(context.Background(), dir, "site/../page.html", "ok"); err != nil {
		t.Fatalf("Write: %v — a path containing .. that resolves back inside is legitimate and must not be refused", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "page.html")); err != nil {
		t.Fatalf("Stat the normalised target: %v", err)
	}
}

func TestWriteRejectsATraversalThatLeavesTheRunDirectoryAndCreatesNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	w := New(root)
	dir, err := w.OpenRun(context.Background())
	if err != nil {
		t.Fatalf("OpenRun: %v", err)
	}

	assertRejected(t, w, dir, "../escaped.html", "should not exist", "path must not leave the working directory")

	if _, err := os.Stat(filepath.Join(root, "escaped.html")); !os.IsNotExist(err) {
		t.Fatalf("Stat the escape target = %v, want it never to have been created", err)
	}
}

func TestWriteRejectsADeepTraversalThatLeavesTheRunDirectory(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)
	assertRejected(t, w, dir, "site/../../../escaped.html", "x", "path must not leave the working directory")
}

func TestWriteRejectsAnAbsolutePath(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)

	absolute := filepath.Join(t.TempDir(), "absolute.html")
	assertRejected(t, w, dir, absolute, "x", "path must be relative to the working directory")

	if _, err := os.Stat(absolute); !os.IsNotExist(err) {
		t.Fatalf("Stat the absolute target = %v, want it never to have been created", err)
	}
}

func TestWriteRejectsASlashRootedPathOnEveryPlatform(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)
	assertRejected(t, w, dir, "/etc/passwd", "x", "path must be relative to the working directory")
}

func TestWriteRejectsAPathContainingAColonOnEveryPlatform(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)
	assertRejected(t, w, dir, `C:evil.html`, "x", "path must not contain a colon")
}

func TestWriteRejectsAnEmptyPath(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)
	assertRejected(t, w, dir, "   ", "x", "path must not be empty")
}

func TestWriteRejectsAPathContainingANullByteWithItsOwnReasonNotAScrubbedFailure(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)
	assertRejected(t, w, dir, "page"+string(rune(0))+".html", "x", "path must not contain a null byte")
}

func TestWriteRejectsAPathNamingTheRunDirectoryItself(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)
	assertRejected(t, w, dir, ".", "x", "path must name a file, not the working directory itself")
}

func TestWriteRejectsARunDirectoryThatIsNotBeneathTheWorkspaceRoot(t *testing.T) {
	t.Parallel()

	w := New(t.TempDir())
	assertRejected(t, w, t.TempDir(), "index.html", "x", "the working directory is not one this workspace opened")
}

func TestWriteReturnsAFailureThatIsNotARejectionWhenTheFilesystemRefuses(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)

	if _, err := w.Write(context.Background(), dir, "occupied", "x"); err != nil {
		t.Fatalf("Write the blocking file: %v", err)
	}

	_, err := w.Write(context.Background(), dir, "occupied/child.html", "x")
	if err == nil {
		t.Fatal("Write succeeded through a regular file used as a directory, want a failure")
	}
	if errors.Is(err, loop.ErrWriteRejected) {
		t.Fatalf("Write failed with %v, want a filesystem failure rather than a path rejection — the two reach the model differently", err)
	}
}

func TestWriteRejectionsWrapTheSentinelTheTurnMatchesOn(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)

	_, err := w.Write(context.Background(), dir, "../escaped.html", "x")
	if !errors.Is(err, loop.ErrWriteRejected) {
		t.Fatalf("Write returned %v, want an error wrapping the rejection sentinel", err)
	}
	if !strings.Contains(err.Error(), "working directory") {
		t.Fatalf("rejection message = %q, want it to say what rule refused the write", err)
	}
}

func TestWriteDoesNothingOnAnAlreadyCancelledContext(t *testing.T) {
	t.Parallel()

	w, dir := openRun(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := w.Write(ctx, dir, "index.html", "x"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Write returned %v, want the cancellation", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); !os.IsNotExist(err) {
		t.Fatalf("Stat = %v, want no file written on a cancelled context", err)
	}
}

func TestOpenRunDoesNothingOnAnAlreadyCancelledContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := New(root).OpenRun(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("OpenRun returned %v, want the cancellation", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("root holds %d entries, want none on a cancelled context", len(entries))
	}
}

func assertRejected(t *testing.T, w *Workspace, dir, path, content, wantReason string) {
	t.Helper()

	_, err := w.Write(context.Background(), dir, path, content)
	if !errors.Is(err, loop.ErrWriteRejected) {
		t.Fatalf("Write(%q) returned %v, want a rejection", path, err)
	}
	if !strings.Contains(err.Error(), wantReason) {
		t.Fatalf("Write(%q) was refused with %q, want the reason %q — a rejection that does not name its rule lets a masked guard survive deletion", path, err, wantReason)
	}
}

func assertUnderRoot(t *testing.T, root, dir string) {
	t.Helper()

	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		t.Fatalf("run directory %q is not beneath the root %q (rel=%q, err=%v)", dir, root, rel, err)
	}
}
