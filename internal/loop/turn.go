package loop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// CandidateLimit and AssemblyByteBudget are constants, not configuration
// (design §8.4): no operator tunes them, and milestone 2 is the named
// event at which they become measurable.
//
// MaxModelCalls is the model call cap per run (design §8.4): one
// judgement call plus two supplementary rounds. Reaching it while the
// model still wants recall is not an error — the turn ends there and the
// record shows modelCalls == MaxModelCalls (design §6.4).
//
// SupplementaryByteBudget bounds one supplementary-recall round (design
// §6.4a, new in revision 3 from #10821 CF-4): the same admission rule as
// AssemblyByteBudget — rank order, stop rather than skip, no back-fill, no
// exemption — applied per round rather than per run. Derived, not picked:
// it is the largest per-round figure that keeps a whole run's
// graph-derived prompt inside a 32,768-token window with the output cap
// reserved (design §8.4's window table).
//
// MaxOutputTokens is the output-token cap sent as the model call's
// max_tokens. Design §8.4 lists it among "Constants, not configuration" —
// a milestone constant, not an adapter detail — and §8.2 requires the run
// record's Limits to carry it; internal/loop is where the record is built,
// so this is where the constant belongs. Only its wire spelling
// (max_tokens vs. max_completion_tokens vs. maxOutputTokens) is a provider
// detail, and that stays in internal/openaicompat's struct tag. Living
// here also avoids an import cycle reaching the record — internal/openaicompat
// already imports internal/loop, never the reverse — but that is a
// consequence of the design's placement, not the reason for it.
const (
	CandidateLimit          = 20
	AssemblyByteBudget      = 60_000
	MaxModelCalls           = 3
	SupplementaryByteBudget = 20_000
	MaxOutputTokens         = 4_096
)

// errSupplementaryRecallFailed is the generic message a supplementary
// recall's transport failure surfaces to the model prompt and the written
// record (design §6.4a, §8.5's rule extended). It is bounded to a short
// sentence and carries no address — the underlying error, which can embed
// the graph's request URL, goes to the operator's log instead (W-6's
// original fix made that error entirely unrecoverable because turn.go had
// no logger at all; #10821's open finding requires it restored to a
// diagnostic channel rather than merely deleted).
const errSupplementaryRecallFailed = "supplementary recall failed"

// ErrSubjectNotFound is returned when the subject id resolves to nothing —
// the graph's empty-result shape (design C30), not a transport error.
var ErrSubjectNotFound = errors.New("subject not found")

// ErrGraphUnavailable wraps any failure reading the graph: transport,
// authentication, or a non-2xx status.
var ErrGraphUnavailable = errors.New("graph unavailable")

// ErrModelUnavailable wraps any failure completing the model call:
// transport, a non-2xx status, or a response body that will not decode
// (design §6.5). It never wraps a *content* outcome — refused, truncated,
// or unrecognised are all successful calls from the port's point of view
// and are recorded, not returned as errors.
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

	// WriteRun records what happened. The adapter alone chooses the
	// written node's type, name and edge — the loop supplies no structure,
	// only the record (design §8.3's write port, and the milestone's own
	// expression of "agents emit observations, not nodes").
	WriteRun(ctx context.Context, record Record) (nodeID int64, err error)
}

// ModelPort is the seam between the loop and the model (design §8.3's
// model port). Declared here, implemented by internal/openaicompat,
// constructed in main. Its vocabulary is the loop's own — TerminalReason,
// not a provider's finish-reason string — which is the whole of what
// provider-agnosticism means at this boundary (design §9.2).
type ModelPort interface {
	// Judge runs one judgement step and returns its outcome. One attempt;
	// no retry (design §10.5).
	Judge(ctx context.Context, in JudgeInput) (JudgeResult, error)
}

// Turn is one run: fetch the anchor, recall candidates, assemble the
// block, judge (dispatching the supplementary-recall tool as the model
// asks for it, up to MaxModelCalls), and write the record back. A Turn
// holds no mutable state, so two runs never interfere (design §6.5
// falsifier: any package-level mutable variable in internal/loop).
type Turn struct {
	Graph GraphPort
	Model ModelPort

	// System is the system text sent with every judgement step — a value
	// constructed in main, not read from anywhere (design §8.6).
	System string

	// ModelID is the model id sent with every judgement step, and the
	// value recorded in Record.Model. It is echoed from configuration,
	// not from the endpoint's response: design §6.6 does not depend on a
	// response carrying a model field, so the record states what was
	// *sent*, which is the only thing guaranteed to be known.
	ModelID string

	// logger is the operator's diagnostic channel — local stderr, per the
	// M0 design §10.3 — for failures whose detail must not reach the model
	// prompt or the written record (design §6.4a, §8.5's rule; #10821's
	// open finding). Required, like server.Serve's logger: NewTurn never
	// nil-checks it, and it is unexported precisely so NewTurn is the only
	// way to set it (design review #10829 W-4) — a bare Turn{Graph: g,
	// Model: m} built outside the package cannot name this field, so it
	// stays at its zero value, nil. log() below is what makes that zero
	// value harmless instead of a months-later panic on dispatchRecall's
	// rare failure branch.
	logger *slog.Logger
}

// NewTurn builds a Turn over graph and model, judging with system and
// modelID on every call. logger is the operator's diagnostic channel for
// detail that must not reach the model prompt or the written record — the
// same threading pattern server.Serve already uses (design §10.3).
func NewTurn(graph GraphPort, model ModelPort, system, modelID string, logger *slog.Logger) *Turn {
	return &Turn{Graph: graph, Model: model, System: system, ModelID: modelID, logger: logger}
}

// log returns t.logger, falling back to a discarding logger when it is
// nil. NewTurn's contract is that logger is required and never defaulted
// (ruling on design review #10829, upheld against server.Serve's own
// precedent) — this is not a relaxation of that contract, only a guard
// against the one construction path NewTurn does not control: a bare
// Turn{} composite literal built outside the package, where the
// unexported logger field cannot be set at all and is left nil (W-4). A
// silently discarded diagnostic line on that path is strictly better than
// a panic on dispatchRecall's least-exercised branch.
func (t *Turn) log() *slog.Logger {
	if t.logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return t.logger
}

// Run executes one turn for input against subject. Request decoding and
// validation are the caller's job (design §5.5 — the handler owns policy,
// the loop does not); Run starts from an already-validated input and
// subject id.
//
// The query text passed to recall is input, verbatim — no rewriting, no
// expansion, no model (design S3).
//
// Steps 1-6 of design §6.1 (anchor, recall, assemble) contain no model and
// no branch a model can influence — that sentence is the milestone. Only
// after the block is rendered does a model enter the turn at all.
func (t *Turn) Run(ctx context.Context, input string, subject int64) (Record, error) {
	anchor, found, err := t.Graph.Node(ctx, subject)
	if err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrGraphUnavailable, err)
	}
	if !found {
		return Record{}, ErrSubjectNotFound
	}

	candidates, err := t.Graph.Recall(ctx, input, CandidateLimit)
	if err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrGraphUnavailable, err)
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
		return Record{}, err
	}

	record.Answer = answer
	record.Model = t.ModelID
	record.ToolCalls = toolCalls
	record.ModelCalls = modelCalls
	record.CapReached = capReached
	record.Usage = usages
	record.StopReason = stop

	if nodeID, werr := t.Graph.WriteRun(ctx, record); werr != nil {
		record.Written = WriteOutcome{Error: werr.Error()}
	} else {
		record.Written = WriteOutcome{NodeID: nodeID}
	}

	return record, nil
}

// judge runs the judgement loop: one call, then — while the model asks
// for supplementary recall and the call cap allows another round —
// dispatch the tool and call again (design §4, §6.4). It returns once the
// model reaches a non-WantsRecall outcome or the cap is reached.
// capReached is true exactly when the loop ended because MaxModelCalls was
// reached while the model still wanted recall (design §8.2, W-7). usages
// carries one entry per Judge call, in call order, so its length always
// equals modelCalls (design §8.2, W-1) — nil entries mean that call's
// endpoint reported no usage object.
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
			// The cap is reached while the model still wants recall. Not an
			// error (design §6.4): the turn ends here, and record.modelCalls
			// == MaxModelCalls is how the record shows the cap fired, and
			// record.capReached is set explicitly (design §8.2, W-7) rather
			// than left for a reader to derive from a constant the record
			// does not carry. The round is still recorded — counted, not
			// dispatched — so its query is never silently dropped (CF-3):
			// §2.4's whole justification for the tool is measuring what the
			// model asked for, and this is exactly the run where it asked
			// hardest for it.
			capReached = true
			exchange := RecallExchange{Query: result.RecallQuery, Error: "call cap reached"}
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

// dispatchRecall turns one WantsRecall result into a completed
// RecallExchange. A malformed tool request (RecallError set) and a
// transport failure on the recall itself are both recorded as an
// error-flagged round rather than aborting the turn — design §6.4 states
// this explicitly for the malformed case ("counted, not dropped"), and a
// recall transport failure mid-turn is treated the same way: the
// already-obtained judgement context is not discarded over a single
// supplementary lookup failing.
//
// A successful recall is admitted under SupplementaryByteBudget before
// anything is handed back (design §6.4a): the loop decides admission: rank
// order, stop rather than skip, no back-fill, no exemption — identical to
// §6.3's rule, applied per round. Results carries only the admitted
// subset, in rank order, for the adapter to render; Dispositions carries
// every row, admitted or cut, for the record (design §8.2, §9.4 obligation
// 3). A round that admits nothing is not an error: it is recorded with
// every row cut, and the adapter's own empty-result rendering is what
// tells the model the round produced nothing usable (design §6.5).
func (t *Turn) dispatchRecall(ctx context.Context, result JudgeResult) RecallExchange {
	if result.RecallError != "" {
		return RecallExchange{Error: result.RecallError, Dispositions: []Disposition{}}
	}

	candidates, err := t.Graph.Recall(ctx, result.RecallQuery, CandidateLimit)
	if err != nil {
		// A generic message, not err.Error() verbatim (W-6): a transport
		// error can embed the graph's request URL, and while that is not
		// a credential — the DiVoid key travels as a header, never a
		// query parameter — it is still an internal address that should
		// not reach the model prompt or the written record (design §6.4a,
		// §8.5's rule). The detail goes to the operator's log instead —
		// local stderr, the process's existing diagnostic channel — so a
		// DNS failure, a timeout and a 500 remain distinguishable to
		// whoever operates the deployment, even though the model and the
		// shared graph see only the generic sentence (#10821's open
		// finding).
		t.log().Error("supplementary recall failed", "query", result.RecallQuery, "error", err)
		return RecallExchange{Query: result.RecallQuery, Error: errSupplementaryRecallFailed, Dispositions: []Disposition{}}
	}

	admitted, dispositions := admit(candidates, SupplementaryByteBudget)
	return RecallExchange{Query: result.RecallQuery, Results: admitted, Dispositions: dispositions}
}

// toolCallRecords projects the turn's internal recall history into the
// run record's shape (design §8.2's toolCalls[]): every row each round
// returned, admitted or cut, with the same columns Candidates carries
// (design §9.4 obligation 3, W-7). Always non-nil, so an empty history
// serializes as [] rather than null (the same convention Assemble already
// applies to Candidates), and each entry's Results is likewise non-nil
// even for an error-flagged round that never reached admission.
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
