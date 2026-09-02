package loop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const (
	CandidateLimit          = 20
	AssemblyByteBudget      = 60_000
	MaxModelCalls           = 3
	SupplementaryByteBudget = 20_000
	MaxOutputTokens         = 4_096
)

const (
	errSupplementaryRecallFailed = "supplementary recall failed"
	errCallCapReached            = "call cap reached"
)

// ErrSubjectNotFound is returned when the subject id resolves to nothing —
// the graph's empty-result shape (design C30), not a transport error.
var ErrSubjectNotFound = errors.New("subject not found")

// ErrGraphUnavailable wraps any failure reading the graph: transport,
// authentication, or a non-2xx status.
var ErrGraphUnavailable = errors.New("graph unavailable")

// ErrModelUnavailable wraps any failure completing the model call itself.
var ErrModelUnavailable = errors.New("model unavailable")

// GraphPort is the seam between the loop and the graph. Declared here,
// implemented by internal/divoid, constructed in main (design §5.2, §8.3).
type GraphPort interface {
	// Node fetches the subject node by id, with its content. found is
	// false when the id resolves to nothing.
	Node(ctx context.Context, id int64) (anchor Anchor, found bool, err error)

	// Recall runs one semantic query and returns up to limit candidates in
	// the rank order the graph returned them. The port does not re-sort.
	Recall(ctx context.Context, query string, limit int) ([]Candidate, error)

	// WriteRun files the record and reports where it landed; the adapter alone chooses type, name and edge.
	WriteRun(ctx context.Context, record Record) WriteReceipt
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

	// System is the system text sent with every judgement step.
	System string

	// ModelID is the model id sent with every judgement step, echoed into Record.Model.
	ModelID string

	logger *slog.Logger
}

// NewTurn builds a Turn over graph and model, judging with system and modelID.
func NewTurn(graph GraphPort, model ModelPort, system, modelID string, logger *slog.Logger) *Turn {
	return &Turn{Graph: graph, Model: model, System: system, ModelID: modelID, logger: logger}
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
		return Record{}, WriteReceipt{}, fmt.Errorf("%w: %v", ErrGraphUnavailable, err)
	}
	if !found {
		return Record{}, WriteReceipt{}, ErrSubjectNotFound
	}

	candidates, err := t.Graph.Recall(ctx, input, CandidateLimit)
	if err != nil {
		return Record{}, WriteReceipt{}, fmt.Errorf("%w: %v", ErrGraphUnavailable, err)
	}

	block, dispositions := Assemble(anchor, candidates, AssemblyByteBudget)

	record := Record{
		Input:      input,
		Subject:    subject,
		Query:      input,
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

	answer, stop, toolCalls, modelCalls, capReached, usages, err := t.judge(ctx, block, input)
	if err != nil {
		return Record{}, WriteReceipt{}, err
	}

	record.Answer = answer
	record.Model = t.ModelID
	record.ToolCalls = toolCalls
	record.ModelCalls = modelCalls
	record.CapReached = capReached
	record.Usage = usages
	record.StopReason = stop

	receipt := t.Graph.WriteRun(context.WithoutCancel(ctx), record)

	t.logFinished(record, receipt, time.Since(started))

	return record, receipt, nil
}

func (t *Turn) logFinished(record Record, receipt WriteReceipt, elapsed time.Duration) {
	reports, inTokens, outTokens := summarizeUsage(record.Usage)

	attrs := []any{
		"subject", record.Subject,
		"receipt", string(receipt.State),
		"candidates", len(record.Candidates),
		"cut", cutCount(record.Candidates),
		"modelCalls", record.ModelCalls,
		"model", record.Model,
		"usageReports", reports,
		"inTokens", inTokens,
		"outTokens", outTokens,
		"elapsed", elapsed,
	}
	if receipt.NodeID != 0 {
		attrs = append(attrs, "node", receipt.NodeID)
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

func (t *Turn) judge(ctx context.Context, block, input string) (answer string, stop StopReason, toolCalls []ToolCallRecord, modelCalls int, capReached bool, usages []*Usage, err error) {
	var recalls []RecallExchange

	for {
		modelCalls++

		result, jerr := t.Model.Judge(ctx, JudgeInput{
			System:       t.System,
			Block:        block,
			Input:        input,
			PriorRecalls: recalls,
		})
		if jerr != nil {
			return "", StopReason{}, nil, 0, false, nil, fmt.Errorf("%w: %v", ErrModelUnavailable, jerr)
		}

		answer = result.Answer
		stop = StopReason{Reason: result.Reason, Raw: result.RawReason}
		usages = append(usages, result.Usage)

		if result.Reason != WantsRecall {
			break
		}
		if modelCalls >= MaxModelCalls {
			capReached = true
			exchange := RecallExchange{Query: result.RecallQuery, Error: errCallCapReached}
			if result.RecallError != "" {
				exchange = RecallExchange{Error: result.RecallError}
			}
			recalls = append(recalls, exchange)
			break
		}

		recalls = append(recalls, t.dispatchRecall(ctx, result))
	}

	return answer, stop, toolCallRecords(recalls), modelCalls, capReached, usages, nil
}

func (t *Turn) dispatchRecall(ctx context.Context, result JudgeResult) RecallExchange {
	if result.RecallError != "" {
		return RecallExchange{Error: result.RecallError, Dispositions: []Disposition{}}
	}

	candidates, err := t.Graph.Recall(ctx, result.RecallQuery, CandidateLimit)
	if err != nil {
		t.log().Error("supplementary recall failed", "query", result.RecallQuery, "error", err)
		return RecallExchange{Query: result.RecallQuery, Error: errSupplementaryRecallFailed, Dispositions: []Disposition{}}
	}

	admitted, dispositions := admit(candidates, SupplementaryByteBudget)
	return RecallExchange{Query: result.RecallQuery, Results: admitted, Dispositions: dispositions}
}

func toolCallRecords(recalls []RecallExchange) []ToolCallRecord {
	records := make([]ToolCallRecord, len(recalls))
	for i, r := range recalls {
		results := r.Dispositions
		if results == nil {
			results = []Disposition{}
		}
		records[i] = ToolCallRecord{Query: r.Query, Error: r.Error, Results: results}
	}
	return records
}
