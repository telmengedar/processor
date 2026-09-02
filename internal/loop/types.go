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

// TerminalReason is the loop's own closed set of ways a judgement step can
// end (design §8.3). The model adapter maps a provider's finish-reason
// vocabulary into this set; the loop branches only on these values and
// never on a provider's own strings — the whole of what "the loop never
// sees a provider's vocabulary" means (design §9.2's neutrality falsifier).
type TerminalReason string

const (
	// Answered is a completed prose answer with no pending tool request.
	Answered TerminalReason = "answered"
	// WantsRecall means the model asked to use the supplementary-recall
	// tool. Not a final outcome by itself: Turn.Run either dispatches it
	// and judges again, or — if the call cap is reached — the turn ends
	// here anyway (design §6.4).
	WantsRecall TerminalReason = "wantsRecall"
	// Truncated means the answer was cut off, typically by the output-token
	// cap (design §8.4).
	Truncated TerminalReason = "truncated"
	// Refused means the endpoint declined to answer.
	Refused TerminalReason = "refused"
	// Unrecognised means the endpoint reported a terminal state outside
	// this set. The raw value survives in the record's stopReason.raw
	// (design §8.3) even though the loop never branches on it.
	Unrecognised TerminalReason = "unrecognised"
)

// Usage is the two token counts as the endpoint reported them, named for
// the direction of travel — tokens in, tokens out (design §8.3, revision 3
// from #10821 W-4). There is deliberately no total: it is the sum of the
// other two on every endpoint that reports it, so it carries nothing a
// reader cannot compute, and carrying it forced a second adapter to
// fabricate it on any endpoint that reports two counts and no total. A run
// whose endpoint reported no usage object carries a nil *Usage in that
// call's slot — absent, never zero-filled (design §6.5, §6.6): a zero is a
// measurement, an absence is the truth.
type Usage struct {
	InTokens  int `json:"inTokens"`
	OutTokens int `json:"outTokens"`
}

// ToolCallRecord is one supplementary-recall round in the run record: the
// query the model asked and what came back. Error is set instead of
// Results when the round produced nothing usable — a malformed tool
// request, or the recall itself failing — because design §6.4 requires
// such a round to be counted, not dropped, and this is how it is counted.
//
// Results carries the same columns Candidates does — rank, id, type, name,
// similarity, size, content hash, included-or-cut and the cut reason — for
// every row the round returned, not only the admitted subset (design §8.2,
// §9.4 obligation 3, revision 3 from #10821 W-7: this was previously an id
// and a score only, so a run that used the tool could not be reconstructed
// and could not even be detected as unreconstructible).
type ToolCallRecord struct {
	Query   string        `json:"query,omitempty"`
	Error   string        `json:"error,omitempty"`
	Results []Disposition `json:"results"`
}

// StopReason carries both of design §8.3's two deliberate values: Reason
// is the loop's own neutral value, which is what Turn.Run branched on;
// Raw is the endpoint's own terminal-reason string, verbatim, kept because
// it is the evidence for whenever a mapping turns out to be wrong. The
// loop never branches on Raw.
type StopReason struct {
	Reason TerminalReason `json:"reason"`
	Raw    string         `json:"raw"`
}

// WriteOutcome is the run record's own write-back result: the node id the
// record was written to, or the reason it was not (design §8.2). A write
// failure is not a run failure (design §6.5) — the expensive artifact
// already exists in the response body, so the graph write is the second
// copy, not the first.
type WriteOutcome struct {
	NodeID int64  `json:"nodeId,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Limits records the five constants that governed one run: the candidate
// limit, the assembly byte budget, the supplementary byte budget, the
// model-call cap and the output-token cap (design §8.2, new in revision 3
// from #10821 CF-4). Recall@k is uncomputable without k, and candidates[]
// and toolCalls[] are unreadable as a ranking without knowing what governed
// admission. Milestone 2 is expected to change three of these, so the
// corpus will span the change and every record written without this field
// is uninterpretable after it does.
type Limits struct {
	CandidateLimit          int `json:"candidateLimit"`
	AssemblyByteBudget      int `json:"assemblyByteBudget"`
	SupplementaryByteBudget int `json:"supplementaryByteBudget"`
	MaxModelCalls           int `json:"maxModelCalls"`
	MaxOutputTokens         int `json:"maxOutputTokens"`
}

// Record is the outcome of one run. Unit A populates the first six fields
// below. The rest — answer, model, toolCalls, modelCalls, capReached,
// usage, stopReason, written, limits — are unit B's (design §8.2).
type Record struct {
	Input      string        `json:"input"`
	Subject    int64         `json:"subject"`
	Query      string        `json:"query"`
	Anchor     AnchorSummary `json:"anchor"`
	Candidates []Disposition `json:"candidates"`
	Block      string        `json:"block"`

	Answer     string           `json:"answer"`
	Model      string           `json:"model"`
	ToolCalls  []ToolCallRecord `json:"toolCalls"`
	ModelCalls int              `json:"modelCalls"`
	// CapReached is true exactly when MaxModelCalls was reached while the
	// model still wanted supplementary recall (design §8.2, revision 3 from
	// #10821 W-7). Stated explicitly rather than left derivable: a reader
	// would need MaxModelCalls, which the record does not otherwise carry,
	// and the derivation's validity depends on which CF-3 remedy was chosen
	// — contingent on an implementation choice, and therefore not a fact
	// this record can promise on its own.
	CapReached bool `json:"capReached"`
	// Usage carries one entry per model call, in call order, so its length
	// always equals ModelCalls (design §8.2, revision 3 from #10821 W-1). A
	// nil entry means that call's endpoint reported no usage object —
	// absent, never zero-filled. The loop aggregates nothing; a run total
	// is the reader's sum of the present entries.
	Usage      []*Usage     `json:"usage"`
	StopReason StopReason   `json:"stopReason"`
	Written    WriteOutcome `json:"written"`
	// Limits is absent (the zero Limits{}) on every unit A record — it
	// arrives with unit B even though two of its members governed unit A's
	// own assembly, because the corpus milestone 2 reads begins when the
	// loop closes, not at PR 1 (design §8.2).
	Limits Limits `json:"limits"`
}

// RecallExchange is one supplementary-recall round already completed in
// this turn, handed back to the model port on the next Judge call so a
// stateless adapter can reconstruct the whole conversation from scratch
// every time (design §9.1 stage 3c — the adapter, not the loop, owns how
// this becomes wire messages). Error is set instead of Results exactly
// when the round produced nothing usable; Query is empty only when the
// tool request itself could not be parsed.
//
// Results carries only the admitted subset, in rank order, with full
// bodies — what the model actually saw (design §6.4a: the loop admits, the
// adapter renders). Dispositions carries every row the round returned,
// admitted or cut, with the same columns Candidates does — the record's
// source for ToolCallRecord.Results (design §8.2, §9.4 obligation 3).
type RecallExchange struct {
	Query        string
	Error        string
	Results      []Candidate
	Dispositions []Disposition
}

// JudgeInput is everything one judgement step needs (design §8.3's model
// port, "judge" operation, In column): the system framing, the assembled
// block, the caller's raw input, and every supplementary-recall round
// already completed this turn.
type JudgeInput struct {
	System       string
	Block        string
	Input        string
	PriorRecalls []RecallExchange
}

// JudgeResult is one judgement step's outcome (design §8.3's model port,
// Out column). RecallQuery is set only when Reason == WantsRecall and the
// tool request parsed; RecallError is set when Reason == WantsRecall but
// the request could not be used — design §6.4's malformed-tool-input case.
// Nothing in this type is a provider's word: the adapter has already
// translated it (design §9.1 stage 3c, §9.2's neutrality falsifier).
type JudgeResult struct {
	Answer      string
	Reason      TerminalReason
	RawReason   string
	RecallQuery string
	RecallError string
	Usage       *Usage
}
