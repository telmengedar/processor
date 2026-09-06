// Package workspace confines a run's file writes to a directory beneath one root.
package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
)

const (
	runDirPrefix = "run-"
	dirMode      = 0o755
	fileMode     = 0o644
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

	parts, err := relativeParts(path)
	if err != nil {
		return 0, err
	}
	if err := w.confirmRunDir(dir); err != nil {
		return 0, err
	}
	if err := confirmNoSymlink(dir, parts); err != nil {
		return 0, err
	}

	target := filepath.Join(append([]string{dir}, parts...)...)
	if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
		return 0, fmt.Errorf("workspace: create parent directory: %w", err)
	}
	if err := os.WriteFile(target, []byte(content), fileMode); err != nil {
		return 0, fmt.Errorf("workspace: write file: %w", err)
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

func (w *Workspace) confirmRunDir(dir string) error {
	root, err := w.absoluteRoot()
	if err != nil {
		return err
	}

	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("workspace: resolve root: %w", err)
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("workspace: resolve run directory: %w", err)
	}

	if !contains(resolvedRoot, resolvedDir) {
		return rejected("the working directory is outside the workspace root")
	}
	return nil
}

func confirmNoSymlink(dir string, parts []string) error {
	current := dir
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("workspace: inspect path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return rejected("path passes through a symbolic link")
		}
	}
	return nil
}

func relativeParts(path string) ([]string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, rejected("path must not be empty")
	}
	if strings.ContainsRune(trimmed, 0) {
		return nil, rejected("path must not contain a null byte")
	}
	if strings.ContainsRune(trimmed, ':') {
		return nil, rejected("path must not contain a colon")
	}
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, `\`) {
		return nil, rejected("path must be relative to the working directory")
	}

	clean := filepath.Clean(trimmed)
	if clean == "." {
		return nil, rejected("path must name a file, not the working directory itself")
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, rejected("path must not leave the working directory")
	}

	return strings.Split(clean, string(filepath.Separator)), nil
}

func contains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func rejected(reason string) error {
	return fmt.Errorf("%w: %s", loop.ErrWriteRejected, reason)
}
