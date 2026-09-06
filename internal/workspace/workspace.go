// Package workspace confines a run's file writes to a directory beneath one root.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/telmengedar/processor/internal/loop"
)

const (
	runDirPrefix = "run-"
	dirMode      = 0o755
	fileMode     = 0o644
)

const (
	reasonEmpty       = "path must not be empty"
	reasonNullByte    = "path must not contain a null byte"
	reasonColon       = "path must not contain a colon"
	reasonNotRelative = "path must be relative to the working directory"
	reasonNotAFile    = "path must name a file, not the working directory itself"
	reasonLeaves      = "path must not leave the working directory"
	reasonOutside     = "path resolves outside the working directory"
	reasonRunDir      = "the working directory is not one this workspace opened"
)

var _ loop.FilePort = (*Workspace)(nil)

// Workspace creates one working directory per run beneath a root the operator chose.
type Workspace struct {
	root string
}

// New builds a Workspace whose run directories are created beneath root.
func New(root string) *Workspace {
	return &Workspace{root: root}
}

// OpenRun creates a working directory for one run and returns its absolute path.
func (w *Workspace) OpenRun(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	root, err := w.absoluteRoot()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, dirMode); err != nil {
		return "", fmt.Errorf("workspace: create root: %w", err)
	}

	dir, err := os.MkdirTemp(root, runDirPrefix)
	if err != nil {
		return "", fmt.Errorf("workspace: create run directory: %w", err)
	}
	return dir, nil
}

// Write writes content at path inside dir and returns the bytes written.
func (w *Workspace) Write(ctx context.Context, dir, path, content string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	clean, err := cleanRelative(path)
	if err != nil {
		return 0, err
	}

	root, err := w.absoluteRoot()
	if err != nil {
		return 0, err
	}
	runName, err := runNameUnder(root, dir)
	if err != nil {
		return 0, err
	}

	confined, err := os.OpenRoot(root)
	if err != nil {
		return 0, fmt.Errorf("workspace: open root: %w", err)
	}
	defer func() { _ = confined.Close() }()

	target := filepath.Join(runName, clean)
	err = confined.WriteFile(target, []byte(content), fileMode)
	if errors.Is(err, fs.ErrNotExist) {
		if mkErr := confined.MkdirAll(filepath.Dir(target), dirMode); mkErr != nil {
			return 0, classify(mkErr)
		}
		err = confined.WriteFile(target, []byte(content), fileMode)
	}
	if err != nil {
		return 0, classify(err)
	}
	return len(content), nil
}

func (w *Workspace) absoluteRoot() (string, error) {
	root, err := filepath.Abs(w.root)
	if err != nil {
		return "", fmt.Errorf("workspace: resolve root: %w", err)
	}
	return root, nil
}

func runNameUnder(root, dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", rejected(reasonRunDir)
	}

	rel, err := filepath.Rel(root, absolute)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || strings.ContainsRune(rel, filepath.Separator) {
		return "", rejected(reasonRunDir)
	}
	return rel, nil
}

func cleanRelative(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", rejected(reasonEmpty)
	}
	if strings.ContainsRune(trimmed, 0) {
		return "", rejected(reasonNullByte)
	}
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, `\`) {
		return "", rejected(reasonNotRelative)
	}
	if strings.ContainsRune(trimmed, ':') {
		return "", rejected(reasonColon)
	}

	clean := filepath.Clean(trimmed)
	if clean == "." {
		return "", rejected(reasonNotAFile)
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", rejected(reasonLeaves)
	}
	return clean, nil
}

func classify(err error) error {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return fmt.Errorf("workspace: write file: %w", err)
	}
	return rejected(reasonOutside)
}

func rejected(reason string) error {
	return fmt.Errorf("%w: %s", loop.ErrWriteRejected, reason)
}
