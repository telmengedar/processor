package process

import (
	"context"
	"encoding/json"

	"github.com/telmengedar/processor/internal/completion"
	"github.com/telmengedar/processor/internal/loop"
)

const MaxMembers = 128
const MaxTransitions = 256
const MaxDefinitionBytes = 1 << 20
const MaxVisits = 32
const MaxJudgements = 8
const MaxDispatches = 16
const MaxContextBytes = 32 << 10

type Graph interface {
	Node(context.Context, int64, loop.Filter) (loop.Anchor, bool, error)
	Recall(context.Context, loop.RecallRequest) ([]loop.Candidate, error)
	FindNodes(context.Context, string, string) ([]loop.Anchor, error)
	Links(context.Context, int64) ([]Edge, error)
}

type Edge struct {
	Source   int64  `json:"sourceId"`
	Target   int64  `json:"targetId"`
	LinkType string `json:"linkType"`
	Context  string `json:"context"`
}

type Member struct {
	ID     int64  `json:"id"`
	SHA256 string `json:"sha256"`
}

type Declaration struct {
	Type     string `json:"type"`
	NodeType string `json:"nodeType,omitempty"`
}

type Definition struct {
	Version           int                    `json:"version"`
	Entry             int64                  `json:"entry"`
	Inputs            map[string]Declaration `json:"inputs"`
	Outputs           map[string]Declaration `json:"outputs"`
	Members           []Member               `json:"members"`
	TransitionsSHA256 string                 `json:"transitionsSha256"`
}

type ContextItem struct {
	Key        string `json:"key"`
	Value      any    `json:"value,omitempty"`
	Node       int64  `json:"node,omitempty"`
	Required   bool   `json:"required"`
	ToolResult bool   `json:"toolResult,omitempty"`
}

type Step struct {
	Version    int           `json:"version"`
	Key        string        `json:"key"`
	Kind       string        `json:"kind"`
	Input      any           `json:"input,omitempty"`
	Context    []ContextItem `json:"context,omitempty"`
	Tools      []int64       `json:"tools,omitempty"`
	Prompt     string        `json:"prompt,omitempty"`
	Tool       int64         `json:"tool,omitempty"`
	Output     any           `json:"output,omitempty"`
	Exports    any           `json:"exports,omitempty"`
	templateID int64
}

type Tool struct {
	Version     int           `json:"version"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Schema      Schema        `json:"schema"`
	Builtin     string        `json:"builtin,omitempty"`
	Python      string        `json:"python,omitempty"`
	Memory      *MemoryTarget `json:"memory,omitempty"`
}

type Predicate struct {
	Path   string `json:"path"`
	Equals any    `json:"equals"`
}

type Transition struct {
	Kind    string     `json:"kind"`
	When    *Predicate `json:"when,omitempty"`
	Default bool       `json:"default,omitempty"`
}

type Snapshot struct {
	ID         int64
	Name       string
	Hash       string
	Definition Definition
	Steps      map[int64]Step
	Tools      map[int64]Tool
	Nodes      map[int64]loop.Anchor
	Hashes     map[int64]string
	Edges      map[int64][]Edge
}

type Request struct {
	ProcessID int64          `json:"processId,omitempty"`
	Process   string         `json:"process,omitempty"`
	Arguments map[string]any `json:"arguments"`
}

type Identity struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Hash string `json:"hash"`
}

type Admission struct {
	Key      string `json:"key"`
	Node     int64  `json:"node,omitempty"`
	Hash     string `json:"hash"`
	Included bool   `json:"included"`
}

type Trace struct {
	Ordinal       int             `json:"ordinal"`
	StepID        int64           `json:"stepId"`
	Key           string          `json:"key"`
	Kind          string          `json:"kind"`
	Hash          string          `json:"hash"`
	InputHash     string          `json:"inputHash"`
	Context       []Admission     `json:"context"`
	Offered       []int64         `json:"offered"`
	Outcome       string          `json:"outcome"`
	Error         *Error          `json:"error,omitempty"`
	Edge          *Edge           `json:"edge,omitempty"`
	Usage         *loop.Usage     `json:"usage,omitempty"`
	Provider      *loop.Provider  `json:"provider,omitempty"`
	ElapsedMS     int64           `json:"elapsedMs"`
	TemplateHash  string          `json:"templateHash,omitempty"`
	TransportHash string          `json:"transportHash,omitempty"`
	Receipts      []MemoryReceipt `json:"receipts,omitempty"`
	Repair        *MemoryResult   `json:"repair,omitempty"`
}

type MemoryTarget struct {
	Root   int64  `json:"root"`
	Type   string `json:"type"`
	Anchor int64  `json:"anchor"`
}

type MemoryRequest struct {
	Target MemoryTarget
	Text   string
	About  int64
}

type MemoryResult struct {
	State       string `json:"state"`
	NodeID      int64  `json:"nodeId,omitempty"`
	ContentHash string `json:"contentHash,omitempty"`
	Confirmed   bool   `json:"confirmed"`
}

type MemoryWriter interface {
	Memorize(context.Context, MemoryRequest) (MemoryResult, error)
}

type MemoryReceipt struct {
	Operation   string `json:"operation"`
	NodeID      int64  `json:"nodeId"`
	ContentHash string `json:"contentHash"`
	StepID      int64  `json:"stepId"`
	Visit       int    `json:"visit"`
}

type Response struct {
	Process Identity       `json:"process"`
	Status  string         `json:"status"`
	Output  map[string]any `json:"output,omitempty"`
	Trace   []Trace        `json:"trace"`
	Error   *Error         `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func failure(code, message string) *Error {
	if len(message) > 512 {
		message = message[:512]
	}
	return &Error{Code: code, Message: message}
}

type Engine struct {
	Graph  Graph
	Model  completion.Model
	Python Python
}

type selectedCall struct {
	StepID    int64
	ToolID    int64
	Arguments map[string]any
	Call      completion.Call
	Text      string
	Allowed   []int64
	Result    json.RawMessage
	Exposed   []int64
}
