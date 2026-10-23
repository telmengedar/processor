package loop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
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
	errRecallClosed         = "recall is closed for this turn: two consecutive rounds returned nothing you had not already been shown"
	errReservedCallRefused  = "the turn's last call was reserved for answering, so it offered no tool and dispatched none"
	errNoSuchNode           = "no part of memory has that id"
	errReadOfSubject        = "that id is this request's own subject, and the context block already carries it"
	errAlreadyShownThisTurn = "that part has already been shown to you in this turn"
	errNodeTooLargeFormat   = "that part's %s is %d bytes and one read carries at most %d, so it cannot be shown"
	errQueryNotPhraseFormat = "that query is %d bytes and a recall query is a phrase of at most %d: nothing was looked up"
	errQueryBlank           = "that query is blank: nothing was looked up"
)

type reservation string

const (
	reservedByCallCap      reservation = "callCap"
	reservedByClosedRecall reservation = "closedRecall"
	reservedByRefusedRound reservation = "refusedRound"
)

// CarriedCauseRunes bounds a cause carried to a durable or shared destination.
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

// ErrGraphRejected marks a graph answer of 400 or 414, the statuses the request's own parameters can cause, as distinct from a graph that could not be reached or failed.
var ErrGraphRejected = errors.New("graph rejected the request")

// ErrInvalidConfig marks a configured run whose first recall the graph answered with 400 or 414, which the config's filter may have caused.
var ErrInvalidConfig = errors.New("invalid config")

// ErrWriteRejected marks a write the working directory refused; its message is recorded on the round and not fed back to the following call.
var ErrWriteRejected = errors.New("write rejected")

// RecallRequest is one recall: queries ranked by mean similarity, at most Limit rows counted after exclusion, Exclude ids and every run record left out, a zero Window unbounded, and Filter, nil for none, the run's config filter the graph applies before it counts.
type RecallRequest struct {
	Queries []string
	Limit   int
	Exclude []int64
	Window  UpdateWindow
	Filter  Filter
}

// GraphPort is the seam between the loop and the graph. Declared here,
// implemented by internal/divoid, constructed in main (design §5.2, §8.3).
type GraphPort interface {
	// Node fetches a node by id, with its content; filter, nil for none, is applied by the graph, and found is false when the id resolves to nothing or to a node the filter excludes.
	Node(ctx context.Context, id int64, filter Filter) (anchor Anchor, found bool, err error)

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

	// Derive runs one query-derivation call on the deployment's sampling, which it reports; it takes no temperature, so no run's choice can reach it.
	Derive(ctx context.Context, prompt string, maxOutputTokens int) (DeriveResult, error)
}

// Turn is one run: anchor, recall, assemble, judge, write back.
type Turn struct {
	Graph GraphPort
	Model ModelPort

	// Files is the working directory writes go to; nil leaves the file tool unoffered.
	Files FilePort

	// Notes files the notes a run's model writes; nil never offers the note tool.
	Notes NotePort

	// Fill is the condensation behaviour that creates a missing substance; nil refuses every creation with a recorded reason.
	Fill FillPort

	// System composes the system text of one judgement call from the tools that call offers, whether the turn carries a conversation section, whether the run offers silence, and the form the run renders parts as.
	System func(offered []string, conversation, silence bool, form Form, mode ResponseMode) string

	// ModelID is the model id sent with every judgement step, echoed into Record.Model.
	ModelID string

	// Build is the build identity of the serving binary, echoed into Record.Build; empty for a binary that was not stamped.
	Build string

	// Floors is the deployment's declared model rates the remaining-time guard prices a judgement call at; the zero value is the product's own declaration.
	Floors Floors

	clock func() time.Time

	logger *slog.Logger
}

// NewTurn builds a Turn over graph, model and files, judging with system and modelID.
func NewTurn(graph GraphPort, model ModelPort, files FilePort, system func(offered []string, conversation, silence bool, form Form, mode ResponseMode) string, modelID string, logger *slog.Logger) *Turn {
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
	record, started, err := t.Answer(ctx, options)
	if err != nil {
		return Record{}, WriteReceipt{}, err
	}
	return record, t.File(ctx, record, started), nil
}

// Answer executes one turn as options describe it up to its outcome and issues no write-back, returning the record and the instant the run started.
func (t *Turn) Answer(ctx context.Context, options RunOptions) (Record, time.Time, error) {
	input, subject, history, settings := options.Input, options.Subject, options.History, options.Settings
	settings.Form = options.form()
	filter, silence, identity := options.filter(), options.silence(), options.identity()
	mode := ResponseMode("")
	if options.View != nil {
		mode = options.View.ResponseMode
	}
	started := time.Now()
	ctx = withPrepaid(ctx)
	if err := validateResponseMode(options.View); err != nil {
		return t.failed(subject, started, err)
	}
	if mode == ResponseStructuredRead && t.System == nil {
		return t.failed(subject, started, fmt.Errorf("%w: structuredRead requires a system composer", ErrInvalidConfig))
	}
	t.log().Info("run started", "subject", subject, "inputLength", len(input))

	anchor, found, err := t.Graph.Node(ctx, subject, nil)
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

	queries, window, windowDiscarded, derivationSampling, derivationError := t.derive(ctx, input, subject, now, conversation)
	notPhrase := inputNotPhraseNote(input)

	candidates, err := Retrieve(ctx, t.Graph, anchor, queries, settings.CandidateLimit, window, filter, t.log())
	if err != nil {
		if len(filter) > 0 && errors.Is(err, ErrGraphRejected) {
			return t.failed(subject, started, fmt.Errorf("%w: %v", ErrInvalidConfig, err))
		}
		return t.failed(subject, started, fmt.Errorf("%w: %v", ErrGraphUnavailable, err))
	}

	anchor, candidates, fills := t.fillInitial(ctx, settings, anchor, candidates)

	block, dispositions := Assemble(anchor, candidates, settings.AssemblyByteBudget, settings.Form)

	record := Record{
		Input:              input,
		Subject:            subject,
		Now:                now,
		Query:              firstQuery(input),
		Queries:            queries,
		DerivationError:    derivationError,
		InputNotPhrase:     notPhrase,
		Window:             window,
		WindowDiscarded:    windowDiscarded,
		Anchor:             summarizeAnchor(anchor, settings.Form),
		Candidates:         dispositions,
		Fills:              fills,
		Block:              block,
		History:            used,
		HistoryCut:         len(history) - len(used),
		Limits:             settings.Limits(),
		Perspective:        options.View,
		DerivationSampling: derivationSampling,
	}

	memory := newWorkingMemory(settings.PromptCeiling(), anchor, len(candidates) > 0, includedCandidates(candidates, dispositions), dispositions, settings.Form)
	memory.charge(len(conversation))
	memory.charge(len(identity))
	if mode == ResponseStructuredRead {
		memory.charge(t.structuredReadCharge(conversation != "", silence, settings.Form))
	}

	judged, err := t.judge(ctx, settings, memory, input, queries, conversation, identity, subject, now, window, filter, silence, noteScopeOf(options.View), options.temperature(), dispositions, mode)
	record.Fills = append(record.Fills, judged.fills...)
	if err != nil {
		t.deferFills(record.Fills)
		return t.failed(subject, started, err)
	}

	record.Answer = judged.answer
	record.NonReply = judged.nonReply
	record.Silent = silence && choseSilence(judged.answer)
	record.Model = t.ModelID
	record.Build = t.Build
	record.Provider = judged.provider
	record.ToolCalls = judged.toolCalls
	record.Workspace = judged.workspace
	record.ModelCalls = judged.modelCalls
	record.CapReached = judged.capReached
	record.RecallClosed = judged.recallClosed
	record.TimeShortfall = judged.timeShortfall
	record.ReservedCall = judged.reservedCall
	record.Usage = judged.usages
	record.Trailers = judged.trailers
	record.Offered = judged.offered
	record.StructuredAllowedActions = judged.structuredAllowedActions
	record.NoteAcks = judged.noteAcks
	record.LastResponse = judged.lastResponse
	record.StopReason = judged.stop
	record.Sampling = judged.sampling
	record.Outcome = ComputeOutcome(record)

	return record, started, nil
}

// File files the record of a run that started at started on a context its caller cannot cancel, hands its fills to the deferred worker and reports where it landed.
func (t *Turn) File(ctx context.Context, record Record, started time.Time) WriteReceipt {
	receipt := t.Graph.WriteRun(context.WithoutCancel(ctx), record)
	t.deferFills(record.Fills)

	t.logFinished(record, receipt, time.Since(started))

	return receipt
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

func (t *Turn) derive(ctx context.Context, input string, subject int64, now time.Time, conversation string) ([]string, UpdateWindow, string, *Sampling, string) {
	started := time.Now()
	derived, window, discarded, sampling, err := deriveQueriesWithConversation(ctx, t.Model, input, now, conversation, DerivationBound)
	elapsed := time.Since(started)

	if err != nil {
		t.log().Warn("the query set holds no derived query", "subject", subject, "elapsed", elapsed, "error", err)
		return MergeQueries(input, nil), UpdateWindow{}, "", sampling, BoundCause(err.Error())
	}

	if discarded != "" {
		t.log().Warn("the derived window was discarded and the run is unbounded", "subject", subject, "discarded", discarded)
	}

	queries := MergeQueries(input, derived)
	t.log().Info("queries derived", "subject", subject, "derived", len(derived), "queries", len(queries), "elapsed", elapsed)
	return queries, window, discarded, sampling, ""
}

func firstQuery(input string) string {
	if IsPhrase(input) {
		return input
	}
	return ""
}

func inputNotPhraseNote(input string) string {
	if IsPhrase(input) {
		return ""
	}
	return fmt.Sprintf("the input is %d bytes and not a phrase, which is non-blank and at most %d: it went to the model as content and was not sent to the graph", len(input), MaxPhraseBytes)
}

func (t *Turn) failed(subject int64, started time.Time, err error) (Record, time.Time, error) {
	t.log().Error("run failed", "subject", subject, "elapsed", time.Since(started), "error", err)
	return Record{}, time.Time{}, err
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

	if outcome := record.Outcome; outcome.Verdict == VerdictSilent {
		t.log().Info("the run chose silence: the model declined to reply", "subject", record.Subject, "modelCalls", record.ModelCalls)
	} else if outcome.Verdict != VerdictDelivered {
		t.log().Warn("the run did not deliver: what it obtained is not what a delivered run obtains",
			"subject", record.Subject, "verdict", string(outcome.Verdict),
			"produced", outcome.Produced, "grounded", outcome.Grounded, "curtailed", outcome.Curtailed)
	}

	if record.StopReason.Reason == Truncated {
		t.log().Warn(truncationCause(record),
			"subject", record.Subject, "adapter", record.Provider.Adapter, "rawStop", record.StopReason.Raw, "modelCalls", record.ModelCalls)
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
	nonReply      NonReply
	capReached    bool
	recallClosed  bool
	timeShortfall string

	fills []FillOutcome

	reservedCall ReservedCall

	usages                   []*Usage
	trailers                 []TrailerKind
	offered                  [][]string
	structuredAllowedActions [][]string
	noteAcks                 []NoteAck
	lastResponse             *LastResponse
	sampling                 Sampling
	provider                 Provider
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

	prompt := fmt.Sprintf("%d prompt bytes at the declared floor of %g bytes per second", settings.PromptCeiling(), floors.PromptBytesPerSecond)
	if settings.JudgementBudget > 0 {
		prompt = fmt.Sprintf("%d output tokens at the declared floor of %g tokens per second, plus %s", settings.JudgementBudget, floors.TokensPerSecond, prompt)
	}
	return BoundCause(fmt.Sprintf("%s: %s left, and one call needs %s — %s",
		errRunTimeShort, remaining.Round(time.Millisecond), need.Round(time.Millisecond), prompt))
}

func (t *Turn) judge(ctx context.Context, settings RunSettings, memory *workingMemory, input string, queries []string, conversation, identity string, subject int64, now time.Time, window UpdateWindow, filter Filter, silence bool, note noteScope, temperature *float64, admitted []Disposition, mode ResponseMode) (judgement, error) {
	var judged judgement
	var exchanges []ToolExchange

	account := newYieldAccount(admitted)
	var searches []recallSearch
	reserved := reservation("")
	dispatched := 0
	notesFiled := 0
	ack, ackedText := NoteAckNone, ""
	lastWasToolRequest := false
	unhonoured := ""
	echoes := newEchoReader(input, queries)

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

		offered := t.offeredTools(reserved != "", note.enabled && notesFiled < NotesPerTurn)
		logical := offered
		var allowed []string
		if mode == ResponseStructuredRead {
			logical = structuredReadTools(offered, account.closed())
			allowed = StructuredReadActions(logical)
			offered = []string{}
			judged.structuredAllowedActions = append(judged.structuredAllowedActions, append([]string{}, allowed...))
		}
		search, trailerKind := trailerFor(logical, searches)
		clause := ""
		if trailerKind == TrailerClause {
			clause, search = search, ""
			if mode == ResponseStructuredRead {
				clause = structuredRecallClause
			}
		}
		trailer := composeClosing(search, acknowledgement(ack, ackedText))
		judged.noteAcks = append(judged.noteAcks, ack)
		judged.trailers = append(judged.trailers, trailerKind)
		judged.offered = append(judged.offered, append([]string{}, offered...))

		result, jerr := t.Model.Judge(ctx, JudgeInput{
			System:                   t.composeSystem(identity, logical, conversation != "", silence, settings.Form, mode),
			Block:                    memory.render(),
			Input:                    input,
			Conversation:             conversation,
			Clause:                   clause,
			Trailer:                  trailer,
			Now:                      now,
			Window:                   window,
			MaxOutputTokens:          outputBudget(settings, reserved),
			Offered:                  offered,
			StructuredAllowedActions: allowed,
			Temperature:              temperature,
		})
		if jerr == nil && mode == ResponseStructuredRead {
			result, jerr = decodeReadDecision(result, allowed)
		}
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

		echoShape := ""
		if mode != ResponseStructuredRead {
			result = referenceRead(result, offered, memory)
			result, echoShape = echoes.recovered(result, trailerKind, silence)
		}
		if echoShape != "" {
			t.log().Info("the answer to the recall clause was a query rather than a reply: the loop reads it as the recall the clause asked for",
				"call", judged.modelCalls, "shape", echoShape)
		}

		judged.answer = result.deliveredText()
		lastWasToolRequest = wantsTool(result.Reason)
		unhonoured = result.UnhonouredCall
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

		if result.ToolSource == ToolSourceReference {
			t.log().Info("the answer was only the id of a part shown in its call: the loop reads it as a request for that part",
				"node", result.ReadNodeID, "call", judged.modelCalls)
		}

		if result.ToolSource == ToolSourceTextual {
			t.log().Info("the answer opened with a retrieval call written as text: the adapter recovered it and the loop dispatches it",
				"adapter", result.Provider.Adapter, "tool", toolFor(result.Reason))
		}

		exchange := t.dispatch(ctx, settings, result, &judged.workspace, window, filter, subject, account, memory, note, offersNote(offered))
		exchange.ToolSource = result.ToolSource
		accountForYield(account, &exchange)
		if exchange.Tool == ToolRecall && exchange.Error == "" {
			searches = append(searches, recallSearch{query: exchange.Query, yield: exchange.Yield})
		}
		if retrieves(exchange.Tool) && exchange.Error == "" {
			memory.absorb(exchange.Results, exchange.Dispositions)
		}
		exchanges = append(exchanges, exchange)
		judged.fills = append(judged.fills, exchange.Fills...)
		dispatched++
		if exchange.Tool == ToolNote {
			notesFiled++
			if notesFiled == 1 {
				ack, ackedText = acknowledgementOf(exchange)
			}
		}

		if exchange.Refused {
			reserved = reservedByRefusedRound
			judged.reservedCall = ReservedCall{State: ReservedCallUnmade}
			t.logReservation(settings, reserved, judged.modelCalls+1, dispatched)
		}
	}

	hadEchoRound := slices.ContainsFunc(exchanges, func(e ToolExchange) bool { return e.ToolSource == ToolSourceEcho })
	echoed := func(answer string) bool { return hadEchoRound && echoes.shapeOf(answer, silence) != "" }
	judged.nonReply = classifyNonReply(judged.answer, lastWasToolRequest, unhonoured, memory.displays, echoed)
	if judged.nonReply != "" {
		t.log().Warn("the final answer is not a reply and is withheld", "kind", string(judged.nonReply), "modelCalls", judged.modelCalls)
	}

	judged.toolCalls = toolCallRecords(exchanges)
	return judged, nil
}

func referenceRead(result JudgeResult, offered []string, memory *workingMemory) JudgeResult {
	if result.Reason != Answered || result.UnhonouredCall != "" || !slices.Contains(offered, ToolReadNode) {
		return result
	}
	id, ok := partReferenceOf(result.deliveredText())
	if !ok || !memory.displays(id) {
		return result
	}
	result.Reason = WantsRead
	result.ReadNodeID = id
	result.ToolSource = ToolSourceReference
	return result
}

func (t *Turn) logReservation(settings RunSettings, reserved reservation, call, dispatched int) {
	t.log().Info("the turn reserved its last judgement call for answering: no tool is offered on a call whose tool request it would not dispatch",
		"condition", string(reserved), "call", call, "callCap", settings.MaxModelCalls, "dispatched", dispatched)
}

func (t *Turn) offeredTools(reserved, noteOpen bool) []string {
	if reserved {
		return nil
	}

	offered := []string{ToolRecall, ToolReadNode}
	if t.Files != nil {
		offered = append(offered, ToolWriteFile)
	}
	if noteOpen && t.Notes != nil {
		offered = append(offered, ToolNote)
	}
	return offered
}

const paragraphBreak = "\n\n"

func (t *Turn) composeSystem(identity string, offered []string, conversation, silence bool, form Form, mode ResponseMode) string {
	if t.System == nil {
		return identity
	}
	product := t.System(offered, conversation, silence, form, mode)
	if identity == "" {
		return product
	}
	return identity + paragraphBreak + product
}

func truncationCause(record Record) string {
	limit := record.Limits.JudgementBudget
	if record.ReservedCall.State == ReservedCallCompleted {
		limit = record.Limits.AnsweringBudget
	}
	if limit == 0 {
		return "the endpoint stopped generation at length although processor sent no output limit: a deployment default or the context window cut the answer"
	}
	return "the endpoint stopped generation at length: the output limit the run carried cut the answer"
}

func outputBudget(settings RunSettings, reserved reservation) int {
	if reserved == "" {
		return settings.JudgementBudget
	}
	return settings.AnsweringBudget
}

func wantsTool(reason TerminalReason) bool {
	return reason == WantsRecall || reason == WantsWrite || reason == WantsRead || reason == WantsNote
}

func toolFor(reason TerminalReason) string {
	switch reason {
	case WantsWrite:
		return ToolWriteFile
	case WantsRead:
		return ToolReadNode
	case WantsNote:
		return ToolNote
	default:
		return ToolRecall
	}
}

func undispatchedExchange(result JudgeResult, cause string) ToolExchange {
	if result.ToolError != "" {
		return ToolExchange{Tool: toolFor(result.Reason), Error: BoundCause(result.ToolError)}
	}
	exchange := ToolExchange{
		Tool:   toolFor(result.Reason),
		Query:  result.RecallQuery,
		NodeID: result.ReadNodeID,
		Path:   result.WritePath,
		Bytes:  len(result.WriteContent),
		Error:  cause,
	}
	if result.Reason == WantsNote {
		exchange.Bytes = len(result.NoteText)
		exchange.About = result.NoteAbout
		exchange.Text = result.NoteText
	}
	return exchange
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
	return ToolExchange{Tool: ToolRecall, Query: result.RecallQuery, Error: errRecallClosed, Dispositions: []Disposition{}}
}

func (t *Turn) dispatch(ctx context.Context, settings RunSettings, result JudgeResult, workspace *string, window UpdateWindow, filter Filter, subject int64, account *yieldAccount, memory *workingMemory, note noteScope, noteOffered bool) ToolExchange {
	switch result.Reason {
	case WantsWrite:
		return t.dispatchWrite(ctx, result, workspace)
	case WantsRead:
		return t.dispatchRead(ctx, settings, result, filter, subject, account, memory)
	case WantsNote:
		return t.dispatchNote(ctx, result, note, noteOffered, subject, memory)
	default:
		return t.dispatchRecall(ctx, settings, result, window, filter, subject, memory)
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
		exchange.Refused = true
		return exchange
	}
	if t.Files == nil {
		exchange.Error = errNoWorkingDirectory
		exchange.Refused = true
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
			exchange.Refused = true
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

func (t *Turn) dispatchRecall(ctx context.Context, settings RunSettings, result JudgeResult, window UpdateWindow, filter Filter, subject int64, memory *workingMemory) ToolExchange {
	if result.ToolError != "" {
		return ToolExchange{Tool: ToolRecall, Error: BoundCause(result.ToolError), Dispositions: []Disposition{}, Refused: true}
	}
	if !IsPhrase(result.RecallQuery) {
		return ToolExchange{Tool: ToolRecall, Error: recallQueryRefusal(result.RecallQuery), Dispositions: []Disposition{}, Refused: true}
	}

	candidates, err := t.Graph.Recall(ctx, RecallRequest{Queries: []string{result.RecallQuery}, Limit: settings.CandidateLimit, Exclude: []int64{subject}, Window: window, Filter: filter})
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

	admitted, dispositions := admitUnheld(candidates, memory.room(settings.SupplementaryByteBudget), 0, memory.holds, settings.Form)
	return ToolExchange{Tool: ToolRecall, Query: result.RecallQuery, Results: admitted, Dispositions: dispositions, Fills: fills}
}

func recallQueryRefusal(query string) string {
	if strings.TrimSpace(query) == "" {
		return errQueryBlank
	}
	return fmt.Sprintf(errQueryNotPhraseFormat, len(query), MaxPhraseBytes)
}

func (t *Turn) dispatchRead(ctx context.Context, settings RunSettings, result JudgeResult, filter Filter, subject int64, account *yieldAccount, memory *workingMemory) ToolExchange {
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

	anchor, found, err := t.Graph.Node(ctx, result.ReadNodeID, filter)
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
	admitted, dispositions := admit([]Candidate{read}, room, 0, settings.Form)
	if account.alreadyShown(dispositions[0]) {
		exchange.Error = errAlreadyShownThisTurn
		exchange.Refused = true
		return exchange
	}
	if len(admitted) == 0 {
		exchange.Error = fmt.Sprintf(errNodeTooLargeFormat, readFormName(settings.Form), dispositions[0].RenderedSize, room)
		exchange.Refused = true
		return exchange
	}

	t.log().Info("node read", "node", dispositions[0].ID, "bytes", dispositions[0].RenderedSize)
	exchange.Results = admitted
	exchange.Dispositions = dispositions
	return exchange
}

func readFormName(form Form) Form {
	if form == FormContent {
		return FormContent
	}
	return FormSubstance
}

func toolCallRecords(exchanges []ToolExchange) []ToolCallRecord {
	records := make([]ToolCallRecord, len(exchanges))
	for i, e := range exchanges {
		results := e.Dispositions
		if results == nil {
			results = []Disposition{}
		}
		records[i] = ToolCallRecord{Tool: e.Tool, Source: e.ToolSource, Query: e.Query, NodeID: e.NodeID, Path: e.Path, Bytes: e.Bytes, Error: e.Error, Results: results, Yield: e.Yield,
			Text: e.Text, About: e.About, AboutDropped: e.AboutDropped, Created: e.Created, Written: e.Written}
	}
	return records
}
