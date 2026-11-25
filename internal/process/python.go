package process

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

type Python struct {
	Executable string
}

type cappedOutput struct {
	mu       sync.Mutex
	data     bytes.Buffer
	limit    int
	exceeded bool
	kill     func()
}

func (w *cappedOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	remaining := w.limit - w.data.Len()
	if len(p) > remaining {
		_, _ = w.data.Write(p[:remaining])
		w.exceeded = true
		w.kill()
		return len(p), nil
	}
	return w.data.Write(p)
}

func (p Python) Run(ctx context.Context, source string, input map[string]any) (any, error) {
	executable := p.Executable
	if executable == "" {
		var err error
		executable, err = exec.LookPath("python3")
		if err != nil {
			return nil, failure("tool_unavailable", "python3 was not found")
		}
	}
	dir, err := os.MkdirTemp("", "processor-tool-")
	if err != nil {
		return nil, failure("tool_unavailable", "tool directory could not be created")
	}
	defer os.RemoveAll(dir)
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(bounded, executable, "-I", "-c", source)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}
	body, err := json.Marshal(input)
	if err != nil {
		return nil, failure("tool_input", "input could not be serialized")
	}
	cmd.Stdin = bytes.NewReader(body)
	kill := configureChild(cmd)
	out := &cappedOutput{limit: 64 << 10, kill: kill}
	diagnostic := &cappedOutput{limit: 8 << 10, kill: kill}
	cmd.Stdout = out
	cmd.Stderr = diagnostic
	cmd.WaitDelay = time.Second
	err = cmd.Run()
	kill()
	if out.exceeded || diagnostic.exceeded {
		return nil, failure("tool_output_bound", "Python output exceeded its limit")
	}
	if bounded.Err() != nil {
		return nil, failure("tool_timeout", "Python tool deadline expired")
	}
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return nil, failure("tool_unavailable", "Python interpreter unavailable")
		}
		return nil, failure("tool_failed", "Python tool exited unsuccessfully")
	}
	d := json.NewDecoder(bytes.NewReader(out.data.Bytes()))
	d.UseNumber()
	var value any
	if d.Decode(&value) != nil || d.Decode(new(any)) != io.EOF {
		return nil, failure("tool_invalid_output", "Python must emit exactly one JSON value")
	}
	return value, nil
}
