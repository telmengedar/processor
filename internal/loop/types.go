// Package loop implements the Processor turn: context assembly, judgement, and write-back.
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

	// SelfProduced is true when this row is a record this system wrote; the graph adapter sets it.
	SelfProduced bool

	// Sources is every recall that returned this row, in the order the recalls were issued.
	Sources []Source
}

// Source is one recall that returned a candidate: which of the queries Retrieve was given carried it, whether that recall was scoped, and at what rank it came back.
type Source struct {
	Query  int  `json:"query"`
	Scoped bool `json:"scoped,omitempty"`
	Rank   int  `json:"rank"`
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
	Rank        int      `json:"rank"`
	ID          int64    `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Similarity  float64  `json:"similarity"`
	Size        int      `json:"size"`
	ContentHash string   `json:"contentHash"`
	Included    bool     `json:"included"`
	CutReason   string   `json:"cutReason,omitempty"`
	Sources     []Source `json:"sources,omitempty"`
}

// TerminalReason is the loop's own closed set of ways a judgement step can end.
type TerminalReason string

const (
	// Answered is a completed prose answer with no pending tool request.
	Answered TerminalReason = "answered"
	// WantsRecall means the model asked for the supplementary-recall tool.
	WantsRecall TerminalReason = "wantsRecall"
	// Truncated means the answer was cut off before it completed.
	Truncated TerminalReason = "truncated"
	// Refused means the endpoint declined to answer.
	Refused TerminalReason = "refused"
	// Unrecognised means the endpoint reported a terminal state outside this set.
	Unrecognised TerminalReason = "unrecognised"
)

// Usage is the two token counts as the endpoint reported them.
type Usage struct {
	InTokens  int `json:"inTokens"`
	OutTokens int `json:"outTokens"`
}

// ToolCallRecord is one supplementary-recall round as the run record carries it.
type ToolCallRecord struct {
	Query   string        `json:"query,omitempty"`
	Error   string        `json:"error,omitempty"`
	Results []Disposition `json:"results"`
}

// StopReason pairs the loop's own terminal value with the endpoint's verbatim string.
type StopReason struct {
	Reason TerminalReason `json:"reason"`
	Raw    string         `json:"raw"`
}

// WriteState is the closed set of ways one record's filing can end.
type WriteState string

const (
	// Stored means the record is at the node, bodied and linked to its subject.
	Stored WriteState = "stored"
	// Unlinked means the record is at the node and complete, with no edge to its subject.
	Unlinked WriteState = "unlinked"
	// NotStored means no node holds the record.
	NotStored WriteState = "notStored"
)

// WriteReceipt is where a record was filed: a response-only key, never a member of the record.
type WriteReceipt struct {
	State  WriteState `json:"state"`
	NodeID int64      `json:"nodeId,omitempty"`
}

// Limits records the five constants that governed one run.
type Limits struct {
	CandidateLimit          int `json:"candidateLimit"`
	AssemblyByteBudget      int `json:"assemblyByteBudget"`
	SupplementaryByteBudget int `json:"supplementaryByteBudget"`
	MaxModelCalls           int `json:"maxModelCalls"`
	MaxOutputTokens         int `json:"maxOutputTokens"`
}

// Sampling records one run's model-call sampling parameters, nil where left to the endpoint's default.
type Sampling struct {
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"topP,omitempty"`
}

// Record is the outcome of one run.
type Record struct {
	Input      string        `json:"input"`
	Subject    int64         `json:"subject"`
	Query      string        `json:"query"`
	Queries    []string      `json:"queries"`
	Anchor     AnchorSummary `json:"anchor"`
	Candidates []Disposition `json:"candidates"`
	Block      string        `json:"block"`

	Answer     string           `json:"answer"`
	Model      string           `json:"model"`
	ToolCalls  []ToolCallRecord `json:"toolCalls"`
	ModelCalls int              `json:"modelCalls"`
	// CapReached is true exactly when the call cap was hit while the model still wanted recall.
	CapReached bool `json:"capReached"`
	// Usage carries one entry per model call, in call order, nil where the endpoint reported none.
	Usage      []*Usage   `json:"usage"`
	StopReason StopReason `json:"stopReason"`
	Limits     Limits     `json:"limits"`
	Sampling   Sampling   `json:"sampling"`
}

// RecallExchange is one supplementary-recall round already completed in this turn.
type RecallExchange struct {
	Query        string
	Error        string
	Results      []Candidate
	Dispositions []Disposition
}

// JudgeInput is everything one judgement step needs.
type JudgeInput struct {
	System       string
	Block        string
	Input        string
	PriorRecalls []RecallExchange
}

// JudgeResult is one judgement step's outcome.
type JudgeResult struct {
	Answer      string
	Reason      TerminalReason
	RawReason   string
	RecallQuery string
	RecallError string
	Usage       *Usage
	Sampling    Sampling
}
