// Package condense derives a node's substance from its content, for the offline command and for the turn's fill.
package condense

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	skipNodeAbsent        = "node absent"
	skipContentAbsent     = "content absent"
	skipSubstancePresent  = "substance present"
	skipSelfProduced      = "self-produced"
	skipNonProseContent   = "non-prose content type"
	skipEmptyCondensation = "condensation empty"
	skipPreamble          = "condensation opens with a preamble"
	skipTruncated         = "condensation truncated"
	skipContentMoved      = "content moved during condensation"
	skipReadFailed        = "graph read failed"
	skipModelFailed       = "model call failed"
	skipWriteFailed       = "substance write failed"
)

const (
	originGenerated   = "generated"
	originPreExisting = "pre-existing"
	originNotWritten  = "not-written"
	originAbsent      = "absent"
)

const (
	minOutputTokens = 2048
	bytesPerToken   = 4.2
)

const finishReasonStop = "stop"

// Node is one graph row as the pass reads it.
type Node struct {
	ID           int64
	Type         string
	Name         string
	ContentType  string
	Content      string
	Substance    string
	SelfProduced bool
}

// Sampling is what one condensation call carried on the wire.
type Sampling struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"topP,omitempty"`
	FrequencyPenalty float64  `json:"frequencyPenalty"`
	PresencePenalty  float64  `json:"presencePenalty"`
	MaxTokens        int      `json:"maxTokens"`
}

// Completion is one condensation call's outcome.
type Completion struct {
	Text         string
	FinishReason string
	Model        string
	Sampling     Sampling
}

// GraphPort is the seam between the pass and the graph, and exposes no way to write content or to create or delete a node.
type GraphPort interface {
	NodeWithSubstance(ctx context.Context, id int64) (Node, bool, error)
	Content(ctx context.Context, id int64) (string, bool, error)
	SetSubstance(ctx context.Context, id int64, substance string) error
}

// ModelPort is the seam between the pass and the model.
type ModelPort interface {
	Condense(ctx context.Context, prompt string, maxOutputTokens int) (Completion, error)
}

// Target is one node the pass was asked to condense, with whatever pre-registered reason the corpus attached to it.
type Target struct {
	Node int64    `json:"node"`
	Rows []string `json:"rows,omitempty"`
	Why  []string `json:"why,omitempty"`
}

// Options are the two switches the pass takes: re-derive over an existing substance, and run without writing.
type Options struct {
	Force  bool
	DryRun bool
}

// Provenance is what produced one substance, and the content it was derived from.
type Provenance struct {
	Node          int64     `json:"node"`
	Name          string    `json:"name"`
	ContentHash   string    `json:"contentHash"`
	ContentSize   int       `json:"contentSize"`
	SubstanceSize int       `json:"substanceSize"`
	Ratio         float64   `json:"ratio"`
	Model         string    `json:"model"`
	Sampling      Sampling  `json:"sampling"`
	Clauses       []string  `json:"clauses"`
	Written       bool      `json:"written"`
	At            time.Time `json:"at"`
}

// Skip is one node the pass did not write to, and the rule that stopped it.
type Skip struct {
	Node   int64  `json:"node"`
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

// AuditEntry pairs one required node's pre-registered reason with the substance the node now carries.
type AuditEntry struct {
	Node          int64    `json:"node"`
	Name          string   `json:"name"`
	Rows          []string `json:"rows"`
	Why           []string `json:"why"`
	Origin        string   `json:"origin"`
	ContentSize   int      `json:"contentSize"`
	SubstanceSize int      `json:"substanceSize"`
	Ratio         float64  `json:"ratio"`
	Substance     string   `json:"substance"`
}

// RatioSummary is the distribution of substance size over content size across everything one pass generated.
type RatioSummary struct {
	Count  int     `json:"count"`
	Min    float64 `json:"min"`
	Median float64 `json:"median"`
	Mean   float64 `json:"mean"`
	Max    float64 `json:"max"`
}

// Result is one pass's whole output: provenance for what it wrote, a reason for everything it did not, and the audit bundle.
type Result struct {
	RanAt       time.Time    `json:"ranAt"`
	DryRun      bool         `json:"dryRun"`
	Forced      bool         `json:"forced"`
	TargetCount int          `json:"targetCount"`
	Provenance  []Provenance `json:"provenance"`
	Skipped     []Skip       `json:"skipped"`
	Audit       []AuditEntry `json:"audit"`
	Ratio       RatioSummary `json:"ratio"`
}

// OperationalFailures counts the skips that are a failure of the pass rather than a rule it applied.
func (r Result) OperationalFailures() int {
	failures := 0
	for _, s := range r.Skipped {
		if isOperationalFailure(s.Reason) {
			failures++
		}
	}
	return failures
}

func isOperationalFailure(reason string) bool {
	switch reason {
	case skipNodeAbsent, skipContentAbsent, skipSubstancePresent, skipSelfProduced,
		skipNonProseContent, skipEmptyCondensation, skipPreamble, skipContentMoved:
		return false
	}
	return true
}

// Condensation is one node's outcome: its provenance when a substance was produced, its skip when not, and an audit entry carrying the text produced or already present.
type Condensation struct {
	Provenance *Provenance
	Skip       *Skip
	Audit      AuditEntry
}

// Present returns the substance the node already carried, and whether that is why it was skipped.
func (c Condensation) Present() (string, bool) {
	if c.Skip == nil || c.Skip.Reason != skipSubstancePresent {
		return "", false
	}
	return c.Audit.Substance, true
}

// Run condenses every target whose content is present and whose substance is absent, isolating each node's failure from the rest of the pass.
func Run(ctx context.Context, graph GraphPort, model ModelPort, targets []Target, opts Options, now func() time.Time) Result {
	result := Result{
		RanAt:       now().UTC(),
		DryRun:      opts.DryRun,
		Forced:      opts.Force,
		TargetCount: len(targets),
		Provenance:  make([]Provenance, 0, len(targets)),
		Skipped:     make([]Skip, 0, len(targets)),
		Audit:       make([]AuditEntry, 0, len(targets)),
	}

	ratios := make([]float64, 0, len(targets))
	for _, target := range targets {
		out := Condense(ctx, graph, model, target, opts, now)
		if out.Provenance != nil {
			result.Provenance = append(result.Provenance, *out.Provenance)
			ratios = append(ratios, out.Provenance.Ratio)
		}
		if out.Skip != nil {
			result.Skipped = append(result.Skipped, *out.Skip)
		}
		if len(target.Why) > 0 {
			result.Audit = append(result.Audit, out.Audit)
		}
	}

	result.Ratio = summarize(ratios)
	return result
}

// Condense derives one target's substance and stores it unless the options or a gate forbid it, and returns the text with its provenance.
func Condense(ctx context.Context, graph GraphPort, model ModelPort, target Target, opts Options, now func() time.Time) Condensation {
	audit := AuditEntry{Node: target.Node, Rows: target.Rows, Why: target.Why, Origin: originAbsent}

	node, found, err := graph.NodeWithSubstance(ctx, target.Node)
	if err != nil {
		return Condensation{Skip: &Skip{Node: target.Node, Reason: skipReadFailed, Detail: err.Error()}, Audit: audit}
	}
	if !found {
		return Condensation{Skip: &Skip{Node: target.Node, Reason: skipNodeAbsent}, Audit: audit}
	}

	audit.Name = node.Name
	audit.ContentSize = len(node.Content)

	if reason := gate(node, opts); reason != "" {
		if reason == skipSubstancePresent {
			audit.Origin = originPreExisting
			audit.Substance = node.Substance
			audit.SubstanceSize = len(node.Substance)
			audit.Ratio = ratio(len(node.Substance), len(node.Content))
		}
		return Condensation{Skip: &Skip{Node: target.Node, Reason: reason}, Audit: audit}
	}

	hash := contentHash(node.Content)
	completion, err := model.Condense(ctx, Prompt(node.Name, node.Content), maxOutputTokens(node.Content))
	if err != nil {
		return Condensation{Skip: &Skip{Node: target.Node, Reason: skipModelFailed, Detail: err.Error()}, Audit: audit}
	}
	if completion.FinishReason != finishReasonStop {
		return Condensation{Skip: &Skip{Node: target.Node, Reason: skipTruncated, Detail: completion.FinishReason}, Audit: audit}
	}

	substance := Postprocess(completion.Text)
	if reason := Defect(substance); reason != "" {
		return Condensation{Skip: &Skip{Node: target.Node, Reason: reason}, Audit: audit}
	}

	audit.Substance = substance
	audit.SubstanceSize = len(substance)
	audit.Ratio = ratio(len(substance), len(node.Content))
	audit.Origin = originNotWritten

	provenance := Provenance{
		Node:          node.ID,
		Name:          node.Name,
		ContentHash:   hash,
		ContentSize:   len(node.Content),
		SubstanceSize: len(substance),
		Ratio:         audit.Ratio,
		Model:         completion.Model,
		Sampling:      completion.Sampling,
		Clauses:       Clauses(node.Content),
		At:            now().UTC(),
	}

	if opts.DryRun {
		return Condensation{Provenance: &provenance, Audit: audit}
	}

	live, found, err := graph.Content(ctx, node.ID)
	if err != nil {
		return Condensation{Skip: &Skip{Node: target.Node, Reason: skipReadFailed, Detail: err.Error()}, Audit: audit}
	}
	if !found || contentHash(live) != hash {
		return Condensation{Skip: &Skip{Node: target.Node, Reason: skipContentMoved}, Audit: audit}
	}

	if err := graph.SetSubstance(ctx, node.ID, substance); err != nil {
		return Condensation{Skip: &Skip{Node: target.Node, Reason: skipWriteFailed, Detail: err.Error()}, Audit: audit}
	}

	provenance.Written = true
	audit.Origin = originGenerated
	return Condensation{Provenance: &provenance, Audit: audit}
}

func gate(node Node, opts Options) string {
	switch {
	case node.SelfProduced:
		return skipSelfProduced
	case strings.TrimSpace(node.Content) == "":
		return skipContentAbsent
	case !isProse(node.ContentType):
		return skipNonProseContent
	case !opts.Force && strings.TrimSpace(node.Substance) != "":
		return skipSubstancePresent
	default:
		return ""
	}
}

func isProse(contentType string) bool {
	normalised := strings.ToLower(strings.TrimSpace(contentType))
	return normalised == "" || strings.HasPrefix(normalised, "text/")
}

// OutputBudget is the output budget one condensation of contentBytes of content is issued with.
func OutputBudget(contentBytes int) int {
	estimate := int(math.Ceil(float64(contentBytes) / bytesPerToken))
	if estimate < minOutputTokens {
		return minOutputTokens
	}
	return estimate
}

func maxOutputTokens(content string) int {
	return OutputBudget(len(content))
}

func ratio(substanceSize, contentSize int) float64 {
	if contentSize == 0 {
		return 0
	}
	return float64(substanceSize) / float64(contentSize)
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func summarize(ratios []float64) RatioSummary {
	if len(ratios) == 0 {
		return RatioSummary{}
	}

	sorted := append([]float64(nil), ratios...)
	sort.Float64s(sorted)

	total := 0.0
	for _, r := range sorted {
		total += r
	}

	return RatioSummary{
		Count:  len(sorted),
		Min:    sorted[0],
		Median: median(sorted),
		Mean:   total / float64(len(sorted)),
		Max:    sorted[len(sorted)-1],
	}
}

func median(sorted []float64) float64 {
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}
