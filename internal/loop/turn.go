package loop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
)

const (
	// CandidateLimit is how many top-ranked rows one recall asks the graph for.
	CandidateLimit = 6
	// AssemblyByteBudget bounds the block's content bytes: the anchor plus admitted candidates.
	AssemblyByteBudget = 60_000
	// MaxModelCalls caps a turn's model calls: 6 is the measured knee, 17 of 20 sampled tasks reaching their own terminal.
	MaxModelCalls           = 6
	SupplementaryByteBudget = 20_000
)

const (
	errRunTimeShort         = "the run's remaining time cannot afford another judgement call"
	errNoWorkingDirectory   = "no working directory is configured"
	errRecallClosedToModel  = "recall is closed for this turn: two consecutive rounds returned nothing you had not already been shown"
	errReservedCallRefused  = "the turn's last call was reserved for answering, so it offered no tool and dispatched none"
	errNoSuchNode           = "no part of memory has that id"
	errReadOfSubject        = "that id is this request's own subject, and the context block already carries it"
	errAlreadyShownThisTurn = "that part has already been shown to you in this turn"
	errNodeTooLargeFormat   = "that part's substance is %d bytes and one read carries at most %d, so it cannot be shown"
)

type reservation string

const (
	reservedByCallCap      reservation = "callCap"
	reservedByClosedRecall reservation = "closedRecall"
	reservedByRefusedRound reservation = "refusedRound"
)

// CarriedCauseRunes bounds a cause carried to a durable, shared or prompt-bearing destination.
const CarriedCauseRunes = 512

// BoundCause returns cause bounded to CarriedCauseRunes, the whole of it when it already fits.
func BoundCause(cause string) string {
	runes := []rune(cause)
	if len(runes) <= CarriedCauseRunes {
		return cause
	}
	return string(runes[:CarriedCauseRunes-1]) + "…"
}

// ErrSubjectNotFound is returned when the subject id resolves to nothing —
// the graph's empty-result shape (design C30), not a transport error.
var ErrSubjectNotFound = errors.New("subject not found")

// ErrGraphUnavailable wraps any failure reading the graph: transport,
// authentication, or a non-2xx status.
var ErrGraphUnavailable = errors.New("graph unavailable")

// ErrModelUnavailable wraps any failure completing the model call itself.
var ErrModelUnavailable = errors.New("model unavailable")

// ErrRunTimeExhausted marks a judgement call the run's remaining time could not afford: a call that was never made, and therefore not a model failure.
var ErrRunTimeExhausted = errors.New("run time exhausted")

// ErrWriteRejected marks a write the working directory refused; its message is shown to the model.
var ErrWriteRejected = errors.New("write rejected")

// RecallRequest is one recall: queries ranked by mean similarity, at most Limit rows counted after exclusion, Exclude ids and every run record left out, and a zero Window unbounded.
type RecallRequest struct {
	Queries []string
	Limit   int
	Exclude []int64
	Window  UpdateWindow
}

// GraphPort is the seam between the loop and the graph. Declared here,
// implemented by internal/divoid, constructed in main (design §5.2, §8.3).
type GraphPort interface {
	// Node fetches a node by id, with its content; found is false when the id resolves to nothing.
	Node(ctx context.Context, id int64) (anchor Anchor, found bool, err error)

	// Recall returns the request's candidates in the graph's own rank order, never re-sorted, each carrying the mean similarity over its queries.
	Recall(ctx context.Context, request RecallRequest) ([]Candidate, error)

	// WriteRun files the record and reports where it landed; the adapter alone chooses type, name and edge.
	WriteRun(ctx context.Context, record Record) WriteReceipt
}

// FilePort is the seam between the loop and the run's working directory.
type FilePort interface {
	// OpenRun creates a working directory for one run and returns its absolute path.
	OpenRun(ctx context.Context) (string, error)

	// Write writes content at path inside dir and returns the bytes written; a refusal wraps ErrWriteRejected.
	Write(ctx context.Context, dir, path, content string) (int, error)
}

// ModelPort is the seam between the loop and the model.
type ModelPort interface {
	// Judge runs one judgement step. One attempt; no retry.
	Judge(ctx context.Context, in JudgeInput) (JudgeResult, error)

	// Derive runs one query-derivation call, returning the completion text alone.
	Derive(ctx context.Context, prompt string, maxOutputTokens int) (string, error)
}

// Turn is one run: anchor, recall, assemble, judge, write back.
type Turn struct {
	Graph GraphPort
	Model ModelPort

	// Files is the working directory writes go to; nil refuses every write.
	Files FilePort

	// Fill is the condensation behaviour that creates a missing substance; nil refuses every creation with a recorded reason.
	Fill FillPort

	// System composes the system text of one judgement call from the tools that call offers and whether the turn carries a conversation section.
	System func(offered []string, conversation bool) string

	// ModelID is the model id sent with every judgement step, echoed into Record.Model.
	ModelID string

	// Floors is the deployment's declared model rates the remaining-time guard prices a judgement call at; the zero value is the product's own declaration.
	Floors Floors

	clock func() time.Time

	logger *slog.Logger
}

// NewTurn builds a Turn over graph, model and files, judging with system and modelID.
func NewTurn(graph GraphPort, model ModelPort, files FilePort, system func(offered []string, conversation bool) string, modelID string, logger *slog.Logger) *Turn {
	return &Turn{Graph: graph, Model: model, Files: files, System: system, ModelID: modelID, clock: time.Now, logger: logger}
}

func (t *Turn) log() *slog.Logger {
	if t.logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return t.logger
}

func (t *Turn) now() time.Time {
	if t.clock == nil {
		return time.Now()
	}
	return t.clock()
}

// Run executes one turn for input against subject with no earlier conversation.
func (t *Turn) Run(ctx context.Context, input string, subject int64) (Record, WriteReceipt, error) {
	return t.RunWithHistory(ctx, input, subject, nil)
}

// RunWithHistory executes one turn for input against subject after the supplied earlier exchanges, oldest first, returning the record and where it was filed.
func (t *Turn) RunWithHistory(ctx context.Context, input string, subject int64, history []Exchange) (Record, WriteReceipt, error) {
	return t.RunWithOptions(ctx, RunOptions{Input: input, Subject: subject, History: history, Settings: DefaultSettings()})
}

// RunWithOptions executes one turn as options describe it, returning the record and where it was filed.
func (t *Turn) RunWithOptions(ctx context.Context, options RunOptions) (Record, WriteReceipt, error) {
	input, subject, history, settings := options.Input, options.Subject, options.History, options.Settings
	started := time.Now()
	ctx = withPrepaid(ctx)
	t.log().Info("run started", "subject", subject, "inputLength", len(input))

	anchor, found, err := t.Graph.Node(ctx, subject)
	if err != nil {
		return t.failed(subject, started, fmt.Errorf("%w: %v", ErrGraphUnavailable, err))
	}
	if !found {
		return t.failed(subject, started, ErrSubjectNotFound)
	}

	now := t.now()

	used, conversation, safetyCut := selectExchanges(history, settings)
	if safetyCut > 0 {
		t.log().Warn("the conversation exceeded its safety limit: the oldest whole exchanges were dropped",
			"subject", subject, "supplied", len(history), "used", len(used), "bytes", len(conversation), "safetyLimit", settings.HistorySafetyLimit)
	}

	queries, window, derivationError := t.derive(ctx, input, subject, now)

	candidates, err := Retrieve(ctx, t.Graph, anchor, queries, settings.CandidateLimit, window, t.log())
	if err != nil {
		return t.failed(subject, started, fmt.Errorf("%w: %v", ErrGraphUnavailable, err))
	}

	anchor, candidates, fills := t.fillInitial(ctx, settings, anchor, candidates)

	block, dispositions := Assemble(anchor, candidates, settings.AssemblyByteBudget)

	record := Record{
		Input:           input,
		Subject:         subject,
		Now:             now,
		Query:           input,
		Queries:         queries,
		DerivationError: derivationError,
		Window:          window,
		Anchor:          summarizeAnchor(anchor),
		Candidates:      dispositions,
		Fills:           fills,
		Block:           block,
		History:         used,
		HistoryCut:      len(history) - len(used),
		Limits:          settings.Limits(),
	}

	memory := newWorkingMemory(settings.PromptCeiling(), anchor, len(candidates) > 0, includedCandidates(candidates, dispositions), dispositions)
	memory.charge(len(conversation))

	judged, err := t.judge(ctx, settings, memory, input, conversation, subject, now, window, dispositions)
	record.Fills = append(record.Fills, judged.fills...)
	if err != nil {
		t.deferFills(record.Fills)
		return t.failed(subject, started, err)
	}

	record.Answer = judged.answer
	record.Model = t.ModelID
	record.Provider = judged.provider
	record.ToolCalls = judged.toolCalls
	record.Workspace = judged.workspace
	record.ModelCalls = judged.modelCalls
	record.CapReached = judged.capReached
	record.RecallClosed = judged.recallClosed
	record.TimeShortfall = judged.timeShortfall
	record.ReservedCall = judged.reservedCall
	record.Usage = judged.usages
	record.LastResponse = judged.lastResponse
	record.StopReason = judged.stop
	record.Sampling = judged.sampling
	record.Outcome = ComputeOutcome(record)

	receipt := t.Graph.WriteRun(context.WithoutCancel(ctx), record)
	t.deferFills(record.Fills)

	t.logFinished(record, receipt, time.Since(started))

	return record, receipt, nil
}

func (t *Turn) fillInitial(ctx context.Context, settings RunSettings, anchor Anchor, candidates []Candidate) (Anchor, []Candidate, []FillOutcome) {
	var ids []int64
	if anchor.Substance == "" {
		ids = append(ids, anchor.ID)
	}
	ids = append(ids, missingSubstances(candidates, nil)...)

	created, fills := t.createSubstances(ctx, settings, phaseInitial, ids)
	if text, ok := created[anchor.ID]; ok && anchor.Substance == "" {
		anchor.Substance = text
	}
	return anchor, withCreated(candidates, created), fills
}

func (t *Turn) derive(ctx context.Context, input string, subject int64, now time.Time) ([]string, UpdateWindow, string) {
	started := time.Now()
	derived, window, err := DeriveQueries(ctx, t.Model, input, now)
	elapsed := time.Since(started)

	if err != nil {
		t.log().Warn("the query set fell back to the raw input alone", "subject", subject, "elapsed", elapsed, "error", err)
		return []string{input}, UpdateWindow{}, BoundCause(err.Error())
	}

	queries := MergeQueries(input, derived)
	t.log().Info("queries derived", "subject", subject, "derived", len(derived), "queries", len(queries), "elapsed", elapsed)
	return queries, window, ""
}

func (t *Turn) failed(subject int64, started time.Time, err error) (Record, WriteReceipt, error) {
	t.log().Error("run failed", "subject", subject, "elapsed", time.Since(started), "error", err)
	return Record{}, WriteReceipt{}, err
}

func (t *Turn) logFinished(record Record, receipt WriteReceipt, elapsed time.Duration) {
	reports, inTokens, outTokens := summarizeUsage(record.Usage)
	cut := cutCount(record.Candidates)

	attrs := []any{
		"subject", record.Subject,
		"receipt", string(receipt.State),
		"candidates", len(record.Candidates),
		"cut", cut,
		"modelCalls", record.ModelCalls,
		"model", record.Model,
		"adapter", record.Provider.Adapter,
		"endpoint", record.Provider.Endpoint,
		"usageReports", reports,
		"inTokens", inTokens,
		"outTokens", outTokens,
		"elapsed", elapsed,
	}
	if receipt.NodeID != 0 {
		attrs = append(attrs, "node", receipt.NodeID)
	}

	if outcome := record.Outcome; outcome.Verdict != VerdictDelivered {
		t.log().Warn("the run did not deliver: what it obtained is not what a delivered run obtains",
			"subject", record.Subject, "verdict", string(outcome.Verdict),
			"produced", outcome.Produced, "grounded", outcome.Grounded, "curtailed", outcome.Curtailed)
	}

	if record.RecallClosed {
		t.log().Warn("recall was closed for the turn: consecutive rounds returned nothing the model had not already been shown",
			"subject", record.Subject, "rounds", len(record.ToolCalls), "modelCalls", record.ModelCalls)
	}

	if len(record.Candidates) > 0 && cut == len(record.Candidates) {
		t.log().Warn("assembly admitted no candidate: the block carried the anchor alone", "subject", record.Subject, "candidates", len(record.Candidates))
	}

	if top, ok := topRankedDisposition(record.Candidates); ok && top.CutReason == cutReasonByteBudget {
		remaining := summaryRemainingAfterAnchor(record.Limits, record.Anchor.RenderedSize)
		t.log().Warn("the top-ranked candidate was cut for want of room: the best match the graph found did not reach the model",
			"subject", record.Subject, "candidateId", top.ID, "candidateName", top.Name, "candidateSize", top.RenderedSize, "cutReason", top.CutReason, "remaining", remaining)
	}

	if len(record.Candidates) < record.Limits.CandidateLimit {
		t.log().Warn("the candidate aperture under-delivered: retrieval returned fewer rows than the limit holds", "subject", record.Subject, "candidates", len(record.Candidates), "candidateLimit", record.Limits.CandidateLimit)
	}

	t.log().Info("run finished", attrs...)
}

func topRankedDisposition(dispositions []Disposition) (Disposition, bool) {
	for _, d := range dispositions {
		if d.Rank == 1 {
			return d, true
		}
	}
	return Disposition{}, false
}

func cutCount(dispositions []Disposition) int {
	cut := 0
	for _, d := range dispositions {
		if !d.Included {
			cut++
		}
	}
	return cut
}

func summarizeUsage(usages []*Usage) (reports, inTokens, outTokens int) {
	for _, u := range usages {
		if u == nil {
			continue
		}
		reports++
		inTokens += u.InTokens
		outTokens += u.OutTokens
	}
	return reports, inTokens, outTokens
}

type judgement struct {
	answer        string
	stop          StopReason
	toolCalls     []ToolCallRecord
	workspace     string
	modelCalls    int
	capReached    bool
	recallClosed  bool
	timeShortfall string

	fills []FillOutcome

	reservedCall ReservedCall

	usages       []*Usage
	lastResponse *LastResponse
	sampling     Sampling
	provider     Provider
}

func (t *Turn) judgementShortfall(ctx context.Context, settings RunSettings) string {
	deadline, ok := ctx.Deadline()
	if !ok {
		return ""
	}

	floors := t.Floors.Resolved()
	need := settings.JudgementCost(floors)
	remaining := deadline.Sub(t.now())
	phase, granted := prepaidFrom(ctx).spend()
	if remaining >= need {
		return ""
	}
	if granted && ctx.Err() == nil {
		t.log().Info("the next judgement call was paid for when the substance step began waiting, and is issued on that decision",
			"remaining", remaining.Round(time.Millisecond), "need", need.Round(time.Millisecond), "phase", phase)
		return ""
	}

	return BoundCause(fmt.Sprintf("%s: %s left, and one call needs %s — %d output tokens at the declared floor of %g tokens per second, plus %d prompt bytes at the declared floor of %g bytes per second",
		errRunTimeShort, remaining.Round(time.Millisecond), need.Round(time.Millisecond),
		settings.JudgementBudget, floors.TokensPerSecond, settings.PromptCeiling(), floors.PromptBytesPerSecond))
}

func (t *Turn) judge(ctx context.Context, settings RunSettings, memory *workingMemory, input, conversation string, subject int64, now time.Time, window UpdateWindow, admitted []Disposition) (judgement, error) {
	var judged judgement
	var exchanges []ToolExchange

	account := newYieldAccount(admitted)
	reserved := reservation("")
	dispatched := 0

	for {
		if shortfall := t.judgementShortfall(ctx, settings); shortfall != "" {
			if judged.modelCalls == 0 {
				return judgement{fills: judged.fills}, fmt.Errorf("%w: %s", ErrRunTimeExhausted, shortfall)
			}
			t.log().Warn("the run stopped short of its call cap: its remaining time cannot afford another judgement call at the declared floor",
				"modelCalls", judged.modelCalls, "shortfall", shortfall)
			judged.timeShortfall = shortfall
			break
		}

		if reserved == "" && judged.modelCalls+1 >= settings.MaxModelCalls {
			reserved = reservedByCallCap
			judged.capReached = true
			judged.reservedCall = ReservedCall{State: ReservedCallUnmade}
			t.logReservation(settings, reserved, judged.modelCalls+1, dispatched)
		}

		judged.modelCalls++

		offered := t.offeredTools(reserved != "")

		result, jerr := t.Model.Judge(ctx, JudgeInput{
			System:          t.composeSystem(offered, conversation != ""),
			Block:           memory.render(offersRetrieval(offered)),
			Input:           input,
			Conversation:    conversation,
			Now:             now,
			Window:          window,
			MaxOutputTokens: outputBudget(settings, reserved),
			Offered:         offered,
		})
		if jerr != nil {
			if reserved == "" {
				return judgement{fills: judged.fills}, fmt.Errorf("%w: %v", ErrModelUnavailable, jerr)
			}
			judged.answer = ""
			judged.usages = append(judged.usages, nil)
			judged.lastResponse = nil
			judged.reservedCall = ReservedCall{State: ReservedCallFailed, Error: BoundCause(jerr.Error())}
			t.log().Warn("the reserved answering call failed: the turn ends with the record it already holds rather than failing the run",
				"condition", string(reserved), "modelCalls", judged.modelCalls, "error", jerr)
			break
		}

		if reserved != "" {
			judged.reservedCall.State = ReservedCallCompleted
		}

		if result.ReasoningBytes > 0 {
			t.log().Warn("the response carried reasoning on a request that suppressed it: the endpoint took the control and ignored it",
				"adapter", result.Provider.Adapter, "reasoningBytes", result.ReasoningBytes)
		}

		if result.UnofferedToolCalls > 0 {
			t.log().Warn("the response carried a tool call the request did not offer: the endpoint answered a tool list it was never sent",
				"adapter", result.Provider.Adapter, "unofferedToolCalls", result.UnofferedToolCalls)
		}

		judged.answer = result.Answer
		judged.stop = StopReason{Reason: result.Reason, Raw: result.RawReason}
		judged.usages = append(judged.usages, result.Usage)
		judged.lastResponse = &LastResponse{ReasoningBytes: result.ReasoningBytes, UnofferedToolCalls: result.UnofferedToolCalls}
		judged.sampling = result.Sampling
		judged.provider = result.Provider

		if reserved != "" {
			if wantsTool(result.Reason) {
				refused := undispatchedExchange(result, errReservedCallRefused)
				refused.ToolSource = result.ToolSource
				exchanges = append(exchanges, refused)
				t.log().Warn("the reserved answering call returned a tool-wanting terminal: the adapter honoured a tool list the request did not carry",
					"adapter", result.Provider.Adapter, "condition", string(reserved), "tool", toolFor(result.Reason))
			}
			break
		}
		if !wantsTool(result.Reason) {
			break
		}
		if result.Reason == WantsRecall && account.closed() {
			judged.recallClosed = true
			closed := closedRecallExchange(result)
			closed.ToolSource = result.ToolSource
			exchanges = append(exchanges, closed)
			reserved = reservedByClosedRecall
			judged.reservedCall = ReservedCall{State: ReservedCallUnmade}
			t.logReservation(settings, reserved, judged.modelCalls+1, dispatched)
			continue
		}

		exchange := t.dispatch(ctx, settings, result, &judged.workspace, window, subject, account, memory)
		exchange.ToolSource = result.ToolSource
		accountForYield(account, &exchange)
		if retrieves(exchange.Tool) && exchange.Error == "" {
			memory.absorb(exchange.Results, exchange.Dispositions)
		}
		exchanges = append(exchanges, exchange)
		judged.fills = append(judged.fills, exchange.Fills...)
		dispatched++

		if exchange.Refused {
			reserved = reservedByRefusedRound
			judged.reservedCall = ReservedCall{State: ReservedCallUnmade}
			t.logReservation(settings, reserved, judged.modelCalls+1, dispatched)
		}
	}

	judged.toolCalls = toolCallRecords(exchanges)
	return judged, nil
}

func (t *Turn) logReservation(settings RunSettings, reserved reservation, call, dispatched int) {
	t.log().Info("the turn reserved its last judgement call for answering: no tool is offered on a call whose tool request it would not dispatch",
		"condition", string(reserved), "call", call, "callCap", settings.MaxModelCalls, "dispatched", dispatched, "budget", settings.AnsweringBudget)
}

func (t *Turn) offeredTools(reserved bool) []string {
	if reserved {
		return nil
	}

	offered := []string{ToolRecall, ToolReadNode}
	if t.Files != nil {
		offered = append(offered, ToolWriteFile)
	}
	return offered
}

func (t *Turn) composeSystem(offered []string, conversation bool) string {
	if t.System == nil {
		return ""
	}
	return t.System(offered, conversation)
}

func offersRetrieval(offered []string) bool {
	return slices.Contains(offered, ToolRecall) || slices.Contains(offered, ToolReadNode)
}

func outputBudget(settings RunSettings, reserved reservation) int {
	if reserved == "" {
		return settings.JudgementBudget
	}
	return settings.AnsweringBudget
}

func wantsTool(reason TerminalReason) bool {
	return reason == WantsRecall || reason == WantsWrite || reason == WantsRead
}

func toolFor(reason TerminalReason) string {
	switch reason {
	case WantsWrite:
		return ToolWriteFile
	case WantsRead:
		return ToolReadNode
	default:
		return ToolRecall
	}
}

func undispatchedExchange(result JudgeResult, cause string) ToolExchange {
	if result.ToolError != "" {
		return ToolExchange{Tool: toolFor(result.Reason), Error: BoundCause(result.ToolError)}
	}
	return ToolExchange{
		Tool:   toolFor(result.Reason),
		Query:  result.RecallQuery,
		NodeID: result.ReadNodeID,
		Path:   result.WritePath,
		Bytes:  len(result.WriteContent),
		Error:  cause,
	}
}

func accountForYield(account *yieldAccount, exchange *ToolExchange) {
	if !retrieves(exchange.Tool) || exchange.Error != "" {
		return
	}
	exchange.Yield = account.round(exchange.Dispositions)
}

func retrieves(tool string) bool {
	return tool == ToolRecall || tool == ToolReadNode
}

func closedRecallExchange(result JudgeResult) ToolExchange {
	return ToolExchange{Tool: ToolRecall, Query: result.RecallQuery, Error: errRecallClosedToModel, Dispositions: []Disposition{}}
}

func (t *Turn) dispatch(ctx context.Context, settings RunSettings, result JudgeResult, workspace *string, window UpdateWindow, subject int64, account *yieldAccount, memory *workingMemory) ToolExchange {
	switch result.Reason {
	case WantsWrite:
		return t.dispatchWrite(ctx, result, workspace)
	case WantsRead:
		return t.dispatchRead(ctx, settings, result, subject, account, memory)
	default:
		return t.dispatchRecall(ctx, settings, result, window, subject, memory)
	}
}

func (t *Turn) dispatchWrite(ctx context.Context, result JudgeResult, workspace *string) ToolExchange {
	exchange := ToolExchange{
		Tool:         ToolWriteFile,
		Path:         result.WritePath,
		Bytes:        len(result.WriteContent),
		Dispositions: []Disposition{},
	}

	if result.ToolError != "" {
		exchange.Error = BoundCause(result.ToolError)
		return exchange
	}
	if t.Files == nil {
		exchange.Error = errNoWorkingDirectory
		return exchange
	}

	if *workspace == "" {
		dir, err := t.Files.OpenRun(ctx)
		if err != nil {
			t.log().Error("opening the run working directory failed", "error", err)
			exchange.Error = BoundCause(err.Error())
			return exchange
		}
		*workspace = dir
	}

	written, err := t.Files.Write(ctx, *workspace, result.WritePath, result.WriteContent)
	if err != nil {
		if errors.Is(err, ErrWriteRejected) {
			exchange.Error = BoundCause(err.Error())
			return exchange
		}
		t.log().Error("file write failed", "path", result.WritePath, "error", err)
		exchange.Error = BoundCause(err.Error())
		return exchange
	}

	exchange.Bytes = written
	t.log().Info("file written", "dir", *workspace, "path", result.WritePath, "bytes", written)
	return exchange
}

func (t *Turn) dispatchRecall(ctx context.Context, settings RunSettings, result JudgeResult, window UpdateWindow, subject int64, memory *workingMemory) ToolExchange {
	if result.ToolError != "" {
		return ToolExchange{Tool: ToolRecall, Error: BoundCause(result.ToolError), Dispositions: []Disposition{}, Refused: true}
	}

	candidates, err := t.Graph.Recall(ctx, RecallRequest{Queries: []string{result.RecallQuery}, Limit: settings.CandidateLimit, Exclude: []int64{subject}, Window: window})
	if err != nil {
		t.log().Error("supplementary recall failed", "query", result.RecallQuery, "error", err)
		return ToolExchange{Tool: ToolRecall, Query: result.RecallQuery, Error: BoundCause(err.Error()), Dispositions: []Disposition{}}
	}

	candidates, leaked := usableRows(candidates, subject)
	if leaked > 0 {
		t.log().Warn(logLeakedRows, "subject", subject, "rows", leaked)
	}

	created, fills := t.createSubstances(ctx, settings, phaseRecall, missingSubstances(candidates, func(c Candidate) bool { return memory.holds(c) }))
	candidates = withCreated(candidates, created)

	admitted, dispositions := admitUnheld(candidates, memory.room(settings.SupplementaryByteBudget), 0, memory.holds)
	return ToolExchange{Tool: ToolRecall, Query: result.RecallQuery, Results: admitted, Dispositions: dispositions, Fills: fills}
}

func (t *Turn) dispatchRead(ctx context.Context, settings RunSettings, result JudgeResult, subject int64, account *yieldAccount, memory *workingMemory) ToolExchange {
	exchange := ToolExchange{Tool: ToolReadNode, NodeID: result.ReadNodeID, Dispositions: []Disposition{}}

	if result.ToolError != "" {
		exchange.Error = BoundCause(result.ToolError)
		exchange.Refused = true
		return exchange
	}
	if result.ReadNodeID == subject {
		exchange.Error = errReadOfSubject
		exchange.Refused = true
		return exchange
	}

	anchor, found, err := t.Graph.Node(ctx, result.ReadNodeID)
	if err != nil {
		t.log().Error("addressed read failed", "node", result.ReadNodeID, "error", err)
		exchange.Error = BoundCause(err.Error())
		return exchange
	}
	if !found {
		exchange.Error = errNoSuchNode
		exchange.Refused = true
		return exchange
	}

	createdHere := false
	if anchor.Substance == "" {
		created, fills := t.createSubstances(ctx, settings, phaseRead, []int64{anchor.ID})
		exchange.Fills = fills
		anchor.Substance = created[anchor.ID]
		createdHere = anchor.Substance != ""
	}

	room := memory.roomToReplace(result.ReadNodeID, settings.SupplementaryByteBudget)
	read := candidateFromAnchor(anchor)
	read.Created = createdHere
	admitted, dispositions := admit([]Candidate{read}, room, 0)
	if account.alreadyShown(dispositions[0]) {
		exchange.Error = errAlreadyShownThisTurn
		exchange.Refused = true
		return exchange
	}
	if len(admitted) == 0 {
		exchange.Error = fmt.Sprintf(errNodeTooLargeFormat, dispositions[0].RenderedSize, room)
		exchange.Refused = true
		return exchange
	}

	t.log().Info("node read", "node", dispositions[0].ID, "bytes", dispositions[0].RenderedSize)
	exchange.Results = admitted
	exchange.Dispositions = dispositions
	return exchange
}

func toolCallRecords(exchanges []ToolExchange) []ToolCallRecord {
	records := make([]ToolCallRecord, len(exchanges))
	for i, e := range exchanges {
		results := e.Dispositions
		if results == nil {
			results = []Disposition{}
		}
		records[i] = ToolCallRecord{Tool: e.Tool, Source: e.ToolSource, Query: e.Query, NodeID: e.NodeID, Path: e.Path, Bytes: e.Bytes, Error: e.Error, Results: results, Yield: e.Yield}
	}
	return records
}
