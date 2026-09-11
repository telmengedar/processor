package loop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const (
	CandidateLimit     = 20
	RecallScopeReserve = 3
	// AssemblyByteBudget bounds the block's content bytes: the anchor plus admitted candidates.
	AssemblyByteBudget = 60_000
	// MaxModelCalls caps a turn's model calls: 6 is the measured knee, 17 of 20 sampled tasks reaching their own terminal.
	MaxModelCalls           = 6
	SupplementaryByteBudget = 20_000
	MaxOutputTokens         = 4_096
)

const (
	errCallCapReached     = "call cap reached"
	errNoWorkingDirectory = "no working directory is configured"
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

// ErrWriteRejected marks a write the working directory refused; its message is shown to the model.
var ErrWriteRejected = errors.New("write rejected")

// GraphPort is the seam between the loop and the graph. Declared here,
// implemented by internal/divoid, constructed in main (design §5.2, §8.3).
type GraphPort interface {
	// Node fetches the subject node by id, with its content. found is
	// false when the id resolves to nothing.
	Node(ctx context.Context, id int64) (anchor Anchor, found bool, err error)

	// Recall returns up to limit candidates in the graph's own rank order, never re-sorted; an empty scope ranks the whole graph.
	Recall(ctx context.Context, query string, limit int, scope []int64) ([]Candidate, error)

	// Neighbours returns the other endpoint of every edge incident to id, ascending and deduplicated.
	Neighbours(ctx context.Context, id int64) ([]int64, error)

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
}

// Turn is one run: anchor, recall, assemble, judge, write back.
type Turn struct {
	Graph GraphPort
	Model ModelPort

	// Files is the working directory writes go to; nil refuses every write.
	Files FilePort

	// System is the system text sent with every judgement step.
	System string

	// ModelID is the model id sent with every judgement step, echoed into Record.Model.
	ModelID string

	logger *slog.Logger
}

// NewTurn builds a Turn over graph, model and files, judging with system and modelID.
func NewTurn(graph GraphPort, model ModelPort, files FilePort, system, modelID string, logger *slog.Logger) *Turn {
	return &Turn{Graph: graph, Model: model, Files: files, System: system, ModelID: modelID, logger: logger}
}

func (t *Turn) log() *slog.Logger {
	if t.logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return t.logger
}

// Run executes one turn for input against subject, returning the record and where it was filed.
func (t *Turn) Run(ctx context.Context, input string, subject int64) (Record, WriteReceipt, error) {
	started := time.Now()
	t.log().Info("run started", "subject", subject, "inputLength", len(input))

	anchor, found, err := t.Graph.Node(ctx, subject)
	if err != nil {
		return t.failed(subject, started, fmt.Errorf("%w: %v", ErrGraphUnavailable, err))
	}
	if !found {
		return t.failed(subject, started, ErrSubjectNotFound)
	}

	queries := []string{input}

	candidates, err := Retrieve(ctx, t.Graph, anchor, queries, CandidateLimit, RecallScopeReserve)
	if err != nil {
		return t.failed(subject, started, fmt.Errorf("%w: %v", ErrGraphUnavailable, err))
	}

	block, dispositions := Assemble(anchor, candidates, AssemblyByteBudget)

	record := Record{
		Input:      input,
		Subject:    subject,
		Query:      input,
		Queries:    queries,
		Anchor:     summarizeAnchor(anchor),
		Candidates: dispositions,
		Block:      block,
		Limits: Limits{
			CandidateLimit:          CandidateLimit,
			AssemblyByteBudget:      AssemblyByteBudget,
			SupplementaryByteBudget: SupplementaryByteBudget,
			MaxModelCalls:           MaxModelCalls,
			MaxOutputTokens:         MaxOutputTokens,
		},
	}

	judged, err := t.judge(ctx, block, input)
	if err != nil {
		return t.failed(subject, started, err)
	}

	record.Answer = judged.answer
	record.Model = t.ModelID
	record.Provider = judged.provider
	record.ToolCalls = judged.toolCalls
	record.Workspace = judged.workspace
	record.ModelCalls = judged.modelCalls
	record.CapReached = judged.capReached
	record.Usage = judged.usages
	record.StopReason = judged.stop
	record.Sampling = judged.sampling

	receipt := t.Graph.WriteRun(context.WithoutCancel(ctx), record)

	t.logFinished(record, receipt, time.Since(started))

	return record, receipt, nil
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

	if len(record.Candidates) > 0 && cut == len(record.Candidates) {
		t.log().Warn("assembly admitted no candidate: the block carried the anchor alone", "subject", record.Subject, "candidates", len(record.Candidates))
	}

	t.log().Info("run finished", attrs...)
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
	answer     string
	stop       StopReason
	toolCalls  []ToolCallRecord
	workspace  string
	modelCalls int
	capReached bool
	usages     []*Usage
	sampling   Sampling
	provider   Provider
}

func (t *Turn) judge(ctx context.Context, block, input string) (judgement, error) {
	var judged judgement
	var exchanges []ToolExchange

	for {
		judged.modelCalls++

		result, jerr := t.Model.Judge(ctx, JudgeInput{
			System:     t.System,
			Block:      block,
			Input:      input,
			PriorTools: exchanges,
		})
		if jerr != nil {
			return judgement{}, fmt.Errorf("%w: %v", ErrModelUnavailable, jerr)
		}

		judged.answer = result.Answer
		judged.stop = StopReason{Reason: result.Reason, Raw: result.RawReason}
		judged.usages = append(judged.usages, result.Usage)
		judged.sampling = result.Sampling
		judged.provider = result.Provider

		if !wantsTool(result.Reason) {
			break
		}
		if judged.modelCalls >= MaxModelCalls {
			judged.capReached = true
			capped := cappedExchange(result)
			capped.ToolSource = result.ToolSource
			exchanges = append(exchanges, capped)
			break
		}

		exchange := t.dispatch(ctx, result, &judged.workspace)
		exchange.ToolSource = result.ToolSource
		exchanges = append(exchanges, exchange)
	}

	judged.toolCalls = toolCallRecords(exchanges)
	return judged, nil
}

func wantsTool(reason TerminalReason) bool {
	return reason == WantsRecall || reason == WantsWrite
}

func toolFor(reason TerminalReason) string {
	if reason == WantsWrite {
		return ToolWriteFile
	}
	return ToolRecall
}

func cappedExchange(result JudgeResult) ToolExchange {
	if result.ToolError != "" {
		return ToolExchange{Tool: toolFor(result.Reason), Error: BoundCause(result.ToolError)}
	}
	return ToolExchange{
		Tool:    toolFor(result.Reason),
		Query:   result.RecallQuery,
		Path:    result.WritePath,
		Content: result.WriteContent,
		Bytes:   len(result.WriteContent),
		Error:   errCallCapReached,
	}
}

func (t *Turn) dispatch(ctx context.Context, result JudgeResult, workspace *string) ToolExchange {
	if result.Reason == WantsWrite {
		return t.dispatchWrite(ctx, result, workspace)
	}
	return t.dispatchRecall(ctx, result)
}

func (t *Turn) dispatchWrite(ctx context.Context, result JudgeResult, workspace *string) ToolExchange {
	exchange := ToolExchange{
		Tool:         ToolWriteFile,
		Path:         result.WritePath,
		Content:      result.WriteContent,
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

func (t *Turn) dispatchRecall(ctx context.Context, result JudgeResult) ToolExchange {
	if result.ToolError != "" {
		return ToolExchange{Tool: ToolRecall, Error: BoundCause(result.ToolError), Dispositions: []Disposition{}}
	}

	candidates, err := t.Graph.Recall(ctx, result.RecallQuery, CandidateLimit, nil)
	if err != nil {
		t.log().Error("supplementary recall failed", "query", result.RecallQuery, "error", err)
		return ToolExchange{Tool: ToolRecall, Query: result.RecallQuery, Error: BoundCause(err.Error()), Dispositions: []Disposition{}}
	}

	admitted, dispositions := admit(candidates, SupplementaryByteBudget)
	return ToolExchange{Tool: ToolRecall, Query: result.RecallQuery, Results: admitted, Dispositions: dispositions}
}

func toolCallRecords(exchanges []ToolExchange) []ToolCallRecord {
	records := make([]ToolCallRecord, len(exchanges))
	for i, e := range exchanges {
		results := e.Dispositions
		if results == nil {
			results = []Disposition{}
		}
		records[i] = ToolCallRecord{Tool: e.Tool, Source: e.ToolSource, Query: e.Query, Path: e.Path, Bytes: e.Bytes, Error: e.Error, Results: results}
	}
	return records
}
