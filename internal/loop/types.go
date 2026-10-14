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

// Filter is the graph filter a run's config carries and every read that can admit a node forwards: opaque to the loop, whose names and values belong to the graph adapter's vocabulary alone.
type Filter map[string][]string

// NoteConfig is the note capability a config enables: the node type every note of the run is filed as.
type NoteConfig struct {
	Type string `json:"type"`
}

// View is the config a run was held to: the node that supplied it, the filter it forwarded, whether it offered the model a chosen silence, the identity text it put first, verbatim, the note capability it enabled, nil for none, and the temperature it asked its judgement calls to be sent with, nil where it asked none.
type View struct {
	Config   int64       `json:"config"`
	Filter   Filter      `json:"filter,omitempty"`
	Silence  bool        `json:"silence,omitempty"`
	Identity string      `json:"identity,omitempty"`
	Note     *NoteConfig `json:"note,omitempty"`

	// Temperature is the temperature the config asked the run's judgement calls for, nil when it asked none and the deployment's applies.
	Temperature *float64 `json:"temperature,omitempty"`

	// Root is the node the config is rooted at, which the run record files itself under; zero means none.
	Root int64 `json:"root,omitempty"`
}

// Anchor is the subject node a run is about.
type Anchor struct {
	ID      int64
	Type    string
	Name    string
	Content string

	// Substance is the node's condensed form, empty when none has been generated.
	Substance string

	// RootNodeID is the node this one is rooted at, zero when it has no root.
	RootNodeID int64
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

	// Created is true when the turn itself produced Substance.
	Created bool

	// SelfProduced is true when this row is a record this system wrote; the graph adapter sets it and only the retrieval net reads it.
	SelfProduced bool

	// Read marks a row fetched by id, which no ranking scored.
	Read bool

	// Sources is the recall that returned this row.
	Sources []Source
}

// Source is the recall that returned a candidate: its position in that recall's list.
type Source struct {
	Rank int `json:"rank"`
}

// AnchorSummary is the anchor's entry in a run record: enough to identify and size it.
type AnchorSummary struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Size        int    `json:"size"`
	ContentHash string `json:"contentHash"`

	// Form is the representation the anchor rendered as.
	Form Form `json:"form"`

	// RenderedSize is the byte length of that representation.
	RenderedSize int `json:"renderedSize"`
}

// UnmarshalJSON decodes one anchor summary, restoring the rendered size on a record written before the anchor rendered its substance: the anchor of such a record was charged its content's own byte length.
func (a *AnchorSummary) UnmarshalJSON(data []byte) error {
	type wire AnchorSummary

	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	if decoded.Form == "" && decoded.RenderedSize == 0 && decoded.Size != 0 {
		decoded.RenderedSize = decoded.Size
	}

	*a = AnchorSummary(decoded)
	return nil
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

	// SubstanceAvailable is true when this candidate carried a substance at retrieval, before any creation.
	SubstanceAvailable bool `json:"substanceAvailable"`

	// SubstanceSize is that substance's byte length, zero when SubstanceAvailable is false.
	SubstanceSize int `json:"substanceSize"`

	// Form is the representation the form rule selected for this candidate.
	Form Form `json:"form"`

	// RenderedSize is the byte length of the selected form; admission charged it unless Held.
	RenderedSize int `json:"renderedSize"`

	// Held is true when the working memory already held this row in this form with these exact rendered bytes, so the round charged it nothing and it was Included whatever its rank or similarity.
	Held bool `json:"held,omitempty"`
}

// UnmarshalJSON decodes one disposition, restoring the rendered size on a record written before the form rule existed: such a record carries no rendered size at all, and every candidate in it was charged its content's own byte length.
func (d *Disposition) UnmarshalJSON(data []byte) error {
	type wire Disposition

	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	if decoded.Form == "" && decoded.RenderedSize == 0 && decoded.Size != 0 {
		decoded.RenderedSize = decoded.Size
	}

	*d = Disposition(decoded)
	return nil
}

// Form is the closed set of representations a block can render a candidate as.
type Form string

const (
	// FormSubstance is the candidate's condensed substance.
	FormSubstance Form = "substance"
	// FormUnavailable is a candidate whose substance could not be produced, rendered as its header alone.
	FormUnavailable Form = "unavailable"
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
	// WantsRead means the model asked for the addressed-read tool.
	WantsRead TerminalReason = "wantsRead"
	// WantsNote means the model asked for the note tool.
	WantsNote TerminalReason = "wantsNote"
	// Unrecognised means the endpoint reported a terminal state outside this set.
	Unrecognised TerminalReason = "unrecognised"
)

const (
	// ToolRecall is the supplementary-recall tool's name in a run record.
	ToolRecall = "recall"
	// ToolWriteFile is the file-write tool's name in a run record.
	ToolWriteFile = "writeFile"
	// ToolReadNode is the addressed-read tool's name in a run record.
	ToolReadNode = "readNode"
	// ToolNote is the note tool's name in a run record.
	ToolNote = "note"
)

// ToolSource is the closed set of ways an adapter can have obtained a tool call from one response.
type ToolSource string

const (
	// ToolSourceNative is a call the endpoint itself reported in its own tool-call field.
	ToolSourceNative ToolSource = "native"
	// ToolSourceContent is a call the adapter recovered from the response text because the endpoint reported none.
	ToolSourceContent ToolSource = "content"
	// ToolSourceTextual is a call the adapter recovered from an answer that opened with an offered retrieval tool's name followed by a colon or a brace, with no call markup.
	ToolSourceTextual ToolSource = "textual"
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
	Source ToolSource `json:"source,omitempty"`
	Query  string     `json:"query,omitempty"`
	Path   string     `json:"path,omitempty"`
	Bytes  int        `json:"bytes"`

	// NodeID is the id an addressed read asked for, absent on every round that was not one.
	NodeID int64 `json:"nodeId,omitempty"`

	Error   string        `json:"error,omitempty"`
	Results []Disposition `json:"results"`

	// Yield is how many rows this round put in front of the model for the first time in the turn; zero on every round that was not a dispatched recall.
	Yield int `json:"yield"`

	// Text is the note as the model wrote it, absent on every round that was not a note round.
	Text string `json:"text,omitempty"`

	// About is the id a note round asked to link, absent on every round that asked for none.
	About int64 `json:"about,omitempty"`

	// AboutDropped is true when About was not linked because it had not been shown in the turn.
	AboutDropped bool `json:"aboutDropped,omitempty"`

	// Created is the id of the node a note round filed, absent when none was.
	Created int64 `json:"created,omitempty"`

	// Written is where a note round's filing ended, absent when the note never reached the graph.
	Written WriteState `json:"written,omitempty"`
}

// ReservedCallState is the loop's closed set of fates the one judgement call a turn reserves for answering can have.
type ReservedCallState string

const (
	// ReservedCallUnmade is a reserved answering call the turn stopped before it was ever issued.
	ReservedCallUnmade ReservedCallState = "unmade"
	// ReservedCallCompleted is a reserved answering call the endpoint answered, whatever it answered with.
	ReservedCallCompleted ReservedCallState = "completed"
	// ReservedCallFailed is a reserved answering call that was issued and did not complete.
	ReservedCallFailed ReservedCallState = "failed"
)

// ReservedCall pairs the fate of a turn's reserved answering call with the cause where it did not complete.
type ReservedCall struct {
	State ReservedCallState `json:"state"`
	Error string            `json:"error,omitempty"`
}

// StopReason pairs the loop's own terminal value with the endpoint's verbatim string.
type StopReason struct {
	Reason TerminalReason `json:"reason"`
	Raw    string         `json:"raw"`
}

// LastResponse is the two facts the turn's last judgement call's response carried on channels every request suppresses and no caller reads.
type LastResponse struct {
	ReasoningBytes     int `json:"reasoningBytes"`
	UnofferedToolCalls int `json:"unofferedToolCalls"`
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
	ThinKnowledgeSimilarity float64 `json:"thinKnowledgeSimilarity"`

	// DerivationBudget is the output budget the query-derivation call was issued with; absent: no limit was sent.
	DerivationBudget int `json:"derivationBudget,omitempty"`
	// JudgementBudget is the output budget each judgement call was issued with; absent: no limit was sent.
	JudgementBudget int `json:"judgementBudget,omitempty"`
	// AnsweringBudget is the output budget a reserved answering call is issued with; absent: no limit was sent.
	AnsweringBudget int `json:"answeringBudget,omitempty"`

	// FillWait is how long one substance step waits for its creations, in nanoseconds.
	FillWait time.Duration `json:"fillWait"`

	// HistoryExchangeLimit is how many of the newest earlier exchanges a turn is shown.
	HistoryExchangeLimit int `json:"historyExchangeLimit"`
	// HistorySafetyLimit is the byte size past which the oldest whole exchanges are dropped.
	HistorySafetyLimit int `json:"historySafetyLimit"`
}

// UnmarshalJSON decodes the limits of one run, reading the single output limit a record written before the per-site budgets carried as the budget of every call site it governed: derivation, judgement and the reserved answering call.
func (l *Limits) UnmarshalJSON(data []byte) error {
	type plain Limits
	decoded := struct {
		plain
		LegacyMaxOutputTokens int `json:"maxOutputTokens"`
	}{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*l = Limits(decoded.plain)
	if decoded.LegacyMaxOutputTokens > 0 && l.DerivationBudget == 0 && l.JudgementBudget == 0 && l.AnsweringBudget == 0 {
		l.DerivationBudget = decoded.LegacyMaxOutputTokens
		l.JudgementBudget = decoded.LegacyMaxOutputTokens
		l.AnsweringBudget = decoded.LegacyMaxOutputTokens
	}
	return nil
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
	// DerivationError is why the query set holds no derived query, empty when queries were derived.
	DerivationError string `json:"derivationError,omitempty"`
	// InputNotPhrase is why the input is not among the queries and why Query is empty: it was not a phrase, so it went to the model and not to the graph; absent when the input was a query.
	InputNotPhrase string `json:"inputNotPhrase,omitempty"`
	// Window is the update-time bound retrieval was held to, absent when the run's input expressed no time constraint.
	Window UpdateWindow `json:"window,omitzero"`
	// WindowDiscarded is the range the derivation wrote and the rule it failed, absent when no range was discarded.
	WindowDiscarded string        `json:"windowDiscarded,omitempty"`
	Anchor          AnchorSummary `json:"anchor"`
	Candidates      []Disposition `json:"candidates"`
	// Fills is one entry per creation the turn attempted, in every phase: filled, or the reason it was not.
	Fills []FillOutcome `json:"fills"`
	Block string        `json:"block"`

	// Perspective is the config the run was held to, absent on a run that had none.
	Perspective *View `json:"perspective,omitempty"`

	// History is the earlier exchanges the turn was shown, oldest first, absent when none were used.
	History []Exchange `json:"history,omitempty"`
	// HistoryCut is how many supplied exchanges the turn was not shown, absent when none were cut.
	HistoryCut int `json:"historyCut,omitempty"`

	Answer string `json:"answer"`
	// NonReply is why the answer, kept verbatim, was not delivered as a reply, absent when it was one.
	NonReply NonReply `json:"nonReply,omitempty"`
	// Silent is true when silence was offered and the answer carried the marker, fixed at run time so a later change of marker reclassifies no archived run.
	Silent bool   `json:"silent,omitempty"`
	Model  string `json:"model"`
	// Build is the git commit the serving binary was built from, absent when the binary carried no build identity or the record predates the field.
	Build string `json:"build,omitempty"`
	// Provider is the adapter and endpoint the model calls went through, as the adapter itself reported them.
	Provider  Provider         `json:"provider"`
	ToolCalls []ToolCallRecord `json:"toolCalls"`
	// Workspace is the run's working directory, absent when the run attempted no file write.
	Workspace  string `json:"workspace,omitempty"`
	ModelCalls int    `json:"modelCalls"`
	// CapReached is true exactly when the turn's last judgement call was reserved for answering because the call budget was spent.
	CapReached bool `json:"capReached"`
	// RecallClosed is true exactly when consecutive barren rounds closed recall for the turn while the model still wanted it.
	RecallClosed bool `json:"recallClosed"`
	// TimeShortfall is the arithmetic that stopped the turn where the run's remaining time could not afford another judgement call, empty on every run that was not stopped that way.
	TimeShortfall string `json:"timeShortfall,omitempty"`
	// ReservedCall is what became of the judgement call this turn reserved for answering, absent on a turn that reserved none.
	ReservedCall ReservedCall `json:"reservedCall,omitzero"`
	// Usage carries one entry per model call, in call order, nil where the endpoint reported none.
	Usage []*Usage `json:"usage"`
	// Trailers carries one entry per model call, in call order: the search state that call was made in, which decides its search paragraph.
	Trailers []TrailerKind `json:"trailers"`
	// Offered carries one entry per model call, in call order: the tools that call offered, in the loop's tool names and in offer order, an empty list for a call that offered none, absent on a record that predates the field.
	Offered [][]string `json:"offered"`
	// NoteAcks carries one entry per model call, in call order: which acknowledgement of a note round that call's closing carried, absent on a record that predates the field.
	NoteAcks []NoteAck `json:"noteAcks"`
	// LastResponse is present exactly when the turn's last judgement call returned a response; absent means either that call returned none, or the record predates this field.
	LastResponse *LastResponse `json:"lastResponse,omitempty"`
	StopReason   StopReason    `json:"stopReason"`
	Limits       Limits        `json:"limits"`
	// Sampling is what the run's judgement calls were sent with.
	Sampling Sampling `json:"sampling"`
	// DerivationSampling is the sampling the derivation request carried, absent when no request was issued and on a record that predates the field, whose derivation used Sampling.
	DerivationSampling *Sampling `json:"derivationSampling,omitempty"`

	// Outcome is the loop's own account of what this run obtained, recomputable from the rest of this record.
	Outcome Outcome `json:"outcome"`
}

// ToolExchange is one tool round already completed in this turn.
type ToolExchange struct {
	Tool         string
	ToolSource   ToolSource
	Query        string
	NodeID       int64
	Path         string
	Bytes        int
	Error        string
	Results      []Candidate
	Dispositions []Disposition

	// Fills is every creation the round attempted before admitting its rows.
	Fills []FillOutcome

	// Yield is how many rows this round put in front of the model for the first time in the turn.
	Yield int

	// Text, About, AboutDropped, Created and Written are a note round's account of itself.
	Text         string
	About        int64
	AboutDropped bool
	Created      int64
	Written      WriteState

	// Refused is true when the loop declined a retrieval request for a reason that holds for the same request against the same memory, as opposed to a graph failure a repeat may clear.
	Refused bool
}

// JudgeInput is everything one judgement step needs.
type JudgeInput struct {
	System string
	Block  string
	Input  string

	// Conversation is the rendered earlier exchanges, identical on every call of a turn, empty when the turn has none.
	Conversation string

	// Trailer is the closing of the user message: the search paragraph, then the acknowledgement of a note round, each its own paragraph, empty when the call carries neither.
	Trailer string

	// Now is the instant the prompt states, zero when the caller supplies none.
	Now time.Time

	// Window is the update-time bound the run's supplementary recalls are held to, zero when unbounded.
	Window UpdateWindow

	// MaxOutputTokens is the call's output limit, 0 for none; an adapter sends no limit key for 0.
	MaxOutputTokens int

	// Offered is the loop's names of the tools this call offers and the only ones it can dispatch; empty leaves prose as the only terminal the call can reach.
	Offered []string

	// Temperature replaces the adapter's configured temperature for this call alone, nil for the configured one; 0 is a temperature.
	Temperature *float64
}

// DeriveResult is one query-derivation call's outcome.
type DeriveResult struct {
	Text string

	// Sampling is the sampling the call's request carried, nil where no request was issued.
	Sampling *Sampling
}

// JudgeResult is one judgement step's outcome.
type JudgeResult struct {
	Answer       string
	Reason       TerminalReason
	RawReason    string
	RecallQuery  string
	ReadNodeID   int64
	WritePath    string
	WriteContent string
	NoteText     string
	NoteAbout    int64
	ToolError    string
	ToolSource   ToolSource
	Usage        *Usage
	Provider     Provider

	// Sampling is the sampling the call's request carried, which is the call's own temperature where it supplied one.
	Sampling Sampling

	// ReasoningBytes is how much arrived on the response channel every request suppresses and no caller reads, zero when the suppression held.
	ReasoningBytes int

	// UnofferedToolCalls is how many tool calls the endpoint reported naming a tool the request did not offer, none of which the adapter honoured.
	UnofferedToolCalls int

	// UnhonouredCall is the loop's name of the tool an answer wrote as its opening call text and the adapter did not honour, empty otherwise.
	UnhonouredCall string
}
