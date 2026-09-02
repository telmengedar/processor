// Package loop implements the Processor turn: mechanical context assembly
// and, in a later unit, the model call and write-back. Unit A implements
// assembly only — see docs/architecture/m1-skeleton-loop.md §2.3.
package loop

// Anchor is the subject node a run is about, fetched with its full body.
type Anchor struct {
	ID      int64
	Type    string
	Name    string
	Content string
}

// Candidate is one row recall returned, in the rank order the graph
// reported it — before assembly has decided whether it fits the budget.
type Candidate struct {
	ID         int64
	Type       string
	Name       string
	Similarity float64
	Content    string
}

// AnchorSummary is the anchor's entry in a run record: enough to identify
// and size it without repeating the full body (the full body is inside
// Record.Block instead).
type AnchorSummary struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Size        int    `json:"size"`
	ContentHash string `json:"contentHash"`
}

// Disposition is one candidate's entry in a run record: whatever recall
// returned about it, plus assembly's admit-or-cut decision and why. Design
// §9.4 obligation 1: every candidate the query returned is recorded here,
// not only the ones admitted into the block — otherwise recall@k is
// uncomputable for every run ever written, retroactively.
type Disposition struct {
	Rank        int     `json:"rank"`
	ID          int64   `json:"id"`
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Similarity  float64 `json:"similarity"`
	Size        int     `json:"size"`
	ContentHash string  `json:"contentHash"`
	Included    bool    `json:"included"`
	CutReason   string  `json:"cutReason,omitempty"`
}

// Record is the outcome of one run. Unit A populates every field below.
// answer, toolCalls, modelCalls, usage, stopReason and written are unit
// B's (design §8.2) — a future member is a list in prose, never a
// declared member (#1220 §2), so they are not declared here.
type Record struct {
	Input      string        `json:"input"`
	Subject    int64         `json:"subject"`
	Query      string        `json:"query"`
	Anchor     AnchorSummary `json:"anchor"`
	Candidates []Disposition `json:"candidates"`
	Block      string        `json:"block"`
}
