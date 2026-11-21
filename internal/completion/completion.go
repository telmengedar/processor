package completion

import (
	"context"
	"encoding/json"

	"github.com/telmengedar/processor/internal/loop"
)

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type Call struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type Message struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Calls    []Call `json:"calls,omitempty"`
	CallID   string `json:"callId,omitempty"`
	ToolName string `json:"toolName,omitempty"`
}

type Request struct {
	Messages    []Message
	Tools       []Tool
	Thinking    bool
	MaxTokens   int
	Temperature *float64
}

type Result struct {
	Text         string
	Calls        []Call
	FinishReason string
	Usage        *loop.Usage
	Provider     loop.Provider
}

type Model interface {
	Complete(context.Context, Request) (Result, error)
}
