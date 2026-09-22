// Package loop implements the Processor turn: context assembly, judgement, and write-back.
package loop

import (
	"encoding/json"
	"time"
)

// UpdateWindow is a closed instant range over the graph's last-update field, resolved in the run's own zone; both bounds zero means unbounded.
type UpdateWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// IsZero reports whether the window is unbounded.
func (w UpdateWindow) IsZero() bool {
	return w.From.IsZero() && w.To.IsZero()
}

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

	// Substance is the node's condensed form, empty when none has been generated.
	Substance string

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

	// SubstanceAvailable is true when this candidate carried a substance at recall time.
	SubstanceAvailable bool `json:"substanceAvailable"`

	// SubstanceSize is that substance's byte length, zero when SubstanceAvailable is false.
	SubstanceSize int `json:"substanceSize"`

	// PayloadCap is the per-candidate rendered-payload ceiling in force when this row was judged, absent when no ceiling was.
	PayloadCap int `json:"payloadCap,omitempty"`

	// Form is the representation the form rule selected for this candidate.
	Form Form `json:"form"`

	// RenderedSize is the byte length of the selected form, and it is what admission charged and the ceiling refused against.
	RenderedSize int `json:"renderedSize"`
}

// UnmarshalJSON decodes one disposition, restoring the rendered size on a record written before the form rule existed: such a record carries no rendered size at all, and every candidate in it was charged its content's own byte length.
func (d *Disposition) UnmarshalJSON(data []byte) error {
	type wire Disposition

	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	if decoded.RenderedSize == 0 && decoded.Size != 0 {
		decoded.RenderedSize = decoded.Size
	}

	*d = Disposition(decoded)
	return nil
}

// Form is the closed set of representations a block can render a candidate as.
type Form string

const (
	// FormContent is the candidate's full content.
	FormContent Form = "content"
	// FormSubstance is the candidate's condensed substance.
	FormSubstance Form = "substance"
)

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
	// WantsWrite means the model asked for the file-write tool.
	WantsWrite TerminalReason = "wantsWrite"
	// Unrecognised means the endpoint reported a terminal state outside this set.
	Unrecognised TerminalReason = "unrecognised"
)

const (
	// ToolRecall is the supplementary-recall tool's name in a run record.
	ToolRecall = "recall"
	// ToolWriteFile is the file-write tool's name in a run record.
	ToolWriteFile = "writeFile"
)

// ToolSource is the closed set of ways an adapter can have obtained a tool call from one response.
type ToolSource string

const (
	// ToolSourceNative is a call the endpoint itself reported in its own tool-call field.
	ToolSourceNative ToolSource = "native"
	// ToolSourceContent is a call the adapter recovered from the response text because the endpoint reported none.
	ToolSourceContent ToolSource = "content"
)

// Provider names the adapter and the endpoint that served a run's model calls.
type Provider struct {
	Adapter  string `json:"adapter"`
	Endpoint string `json:"endpoint"`
}

// Usage is the two token counts as the endpoint reported them.
type Usage struct {
	InTokens  int `json:"inTokens"`
	OutTokens int `json:"outTokens"`
}

// ToolCallRecord is one tool round as the run record carries it.
type ToolCallRecord struct {
	Tool string `json:"tool"`
	// Source is how the adapter obtained this call, absent on a round that reached no adapter.
	Source  ToolSource    `json:"source,omitempty"`
	Query   string        `json:"query,omitempty"`
	Path    string        `json:"path,omitempty"`
	Bytes   int           `json:"bytes"`
	Error   string        `json:"error,omitempty"`
	Results []Disposition `json:"results"`

	// Yield is how many rows this round put in front of the model for the first time in the turn; zero on every round that was not a dispatched recall.
	Yield int `json:"yield"`
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

// Limits records the constants that governed one run.
type Limits struct {
	CandidateLimit          int     `json:"candidateLimit"`
	AssemblyByteBudget      int     `json:"assemblyByteBudget"`
	SupplementaryByteBudget int     `json:"supplementaryByteBudget"`
	MaxModelCalls           int     `json:"maxModelCalls"`
	RelevanceFloor          float64 `json:"relevanceFloor"`

	// DerivationBudget is the output budget the query-derivation call was issued with.
	DerivationBudget int `json:"derivationBudget"`
	// JudgementBudget is the output budget each judgement call was issued with.
	JudgementBudget int `json:"judgementBudget"`

	// MaxFills is the turn's own fill ceiling, never MaxModelCalls.
	MaxFills int `json:"maxFills"`
	// FillSizeFloor is the fill's size gate.
	FillSizeFloor int `json:"fillSizeFloor"`
	// MaxFillContentBytes is the fast-refusal ceiling for oversized content.
	MaxFillContentBytes int `json:"maxFillContentBytes"`

	// SubstanceRatioThreshold is the form rule's dial as this run was configured.
	SubstanceRatioThreshold SubstanceRatio `json:"substanceRatioThreshold"`
}

// Sampling records one run's model-call sampling parameters, nil where left to the endpoint's default.
type Sampling struct {
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"topP,omitempty"`
}

// Record is the outcome of one run.
type Record struct {
	Input   string `json:"input"`
	Subject int64  `json:"subject"`

	// Now is the instant the assembled prompt states, in the zone it was read; absent when the prompt states none.
	Now time.Time `json:"now,omitzero"`

	Query   string   `json:"query"`
	Queries []string `json:"queries"`
	// DerivationError is why the query set is the raw input alone, empty when queries were derived.
	DerivationError string `json:"derivationError,omitempty"`
	// Window is the update-time bound retrieval was held to, absent when the run's input expressed no time constraint.
	Window     UpdateWindow  `json:"window,omitzero"`
	Anchor     AnchorSummary `json:"anchor"`
	Candidates []Disposition `json:"candidates"`
	// Fills is one entry per candidate that lacked a substance at initial assembly, never the supplementary round: filled, or the reason it was not.
	Fills []FillOutcome `json:"fills"`
	Block string        `json:"block"`

	Answer string `json:"answer"`
	Model  string `json:"model"`
	// Provider is the adapter and endpoint the model calls went through, as the adapter itself reported them.
	Provider  Provider         `json:"provider"`
	ToolCalls []ToolCallRecord `json:"toolCalls"`
	// Workspace is the run's working directory, absent when the run attempted no file write.
	Workspace  string `json:"workspace,omitempty"`
	ModelCalls int    `json:"modelCalls"`
	// CapReached is true exactly when the call cap was hit while the model still wanted a tool.
	CapReached bool `json:"capReached"`
	// RecallClosed is true exactly when consecutive barren rounds closed recall for the turn while the model still wanted it.
	RecallClosed bool `json:"recallClosed"`
	// TimeShortfall is the arithmetic that stopped the turn where the run's remaining time could not afford another judgement call, empty on every run that was not stopped that way.
	TimeShortfall string `json:"timeShortfall,omitempty"`
	// Usage carries one entry per model call, in call order, nil where the endpoint reported none.
	Usage      []*Usage   `json:"usage"`
	StopReason StopReason `json:"stopReason"`
	Limits     Limits     `json:"limits"`
	Sampling   Sampling   `json:"sampling"`

	// Outcome is the loop's own account of what this run obtained, recomputable from the rest of this record.
	Outcome Outcome `json:"outcome"`
}

// ToolExchange is one tool round already completed in this turn.
type ToolExchange struct {
	Tool         string
	ToolSource   ToolSource
	Query        string
	Path         string
	Content      string
	Bytes        int
	Error        string
	Results      []Candidate
	Dispositions []Disposition

	// SubstanceRatioThreshold is the dial this round's admission charged at, and the one its results render at.
	SubstanceRatioThreshold SubstanceRatio

	// Yield is how many rows this round put in front of the model for the first time in the turn.
	Yield int

	// NothingNew is true when this round admitted rows and the model had already been shown every one of them.
	NothingNew bool
}

// JudgeInput is everything one judgement step needs.
type JudgeInput struct {
	System     string
	Block      string
	Input      string
	PriorTools []ToolExchange

	// Now is the instant the prompt states, zero when the caller supplies none.
	Now time.Time

	// Window is the update-time bound the run's supplementary recalls are held to, zero when unbounded.
	Window UpdateWindow

	// MaxOutputTokens is this call site's own output budget; an adapter refuses a call that carries none.
	MaxOutputTokens int
}

// JudgeResult is one judgement step's outcome.
type JudgeResult struct {
	Answer       string
	Reason       TerminalReason
	RawReason    string
	RecallQuery  string
	WritePath    string
	WriteContent string
	ToolError    string
	ToolSource   ToolSource
	Usage        *Usage
	Sampling     Sampling
	Provider     Provider

	// ReasoningBytes is how much arrived on the response channel every request suppresses and no caller reads, zero when the suppression held.
	ReasoningBytes int
}
