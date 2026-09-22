// Package ledger computes the mechanical predicates a run record decides from its own fields alone.
package ledger

import (
	"fmt"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/runarchive"
)

const unproductiveRoundsNeeded = 2

// Requery is one run's mechanical account of how it handled what retrieval gave it.
type Requery struct {
	// AcceptedInsufficiency is true when a thin block reached the model and the run asked for nothing more.
	AcceptedInsufficiency bool `json:"acceptedInsufficiency"`

	// UnproductiveRequery is true when two or more dispatched rounds each yielded no row the model had not already seen.
	UnproductiveRequery bool `json:"unproductiveRequery"`

	// ProductiveRequery is true when a dispatched round admitted at least one row the model had not already seen.
	ProductiveRequery bool `json:"productiveRequery"`

	// RefusedRequery is true when a recall round carried an error, grounding nothing and reaching the model not at all.
	RefusedRequery bool `json:"refusedRequery"`

	// AdmittedRows is how many rows the initial block admitted.
	AdmittedRows int `json:"admittedRows"`

	// DispatchedRounds is how many recall rounds reached the graph.
	DispatchedRounds int `json:"dispatchedRounds"`

	// ProductiveRounds is how many dispatched rounds admitted a row the model had not already seen.
	ProductiveRounds int `json:"productiveRounds"`

	// BarrenRounds is how many dispatched rounds admitted no row the model had not already seen.
	BarrenRounds int `json:"barrenRounds"`

	// RefusedRounds is how many recall rounds carried an error.
	RefusedRounds int `json:"refusedRounds"`
}

// Entry is the ledger's whole mechanical account of one archived record.
type Entry struct {
	Node int64  `json:"node"`
	Name string `json:"name"`

	// At is the instant the node's name stated, absent when Dated is false.
	At time.Time `json:"at,omitzero"`

	// Dated is true when the run instant is known; when it is false At is unknown rather than early.
	Dated bool `json:"dated"`

	Outcome loop.Outcome `json:"outcome"`
	Requery Requery      `json:"requery"`
}

// Compute derives one entry, recomputing every verdict from the record's own fields rather than reading the values stored beside them.
func Compute(entry runarchive.Entry) Entry {
	return Entry{
		Node:    entry.Node,
		Name:    entry.Name,
		At:      entry.At,
		Dated:   entry.Dated,
		Outcome: loop.ComputeOutcome(entry.Record),
		Requery: ComputeRequery(entry.Record),
	}
}

// Over computes one entry per archived record, in the order the archive holds them.
func Over(archive runarchive.Archive) []Entry {
	entries := make([]Entry, 0, len(archive.Entries))
	for _, entry := range archive.Entries {
		entries = append(entries, Compute(entry))
	}
	return entries
}

// ComputeRequery derives one run's requery account from that record's own fields and the block thinness the loop itself applies.
func ComputeRequery(record loop.Record) Requery {
	visible := make(map[int64]bool, len(record.Candidates))
	requery := Requery{}

	for _, d := range record.Candidates {
		if !d.Included {
			continue
		}
		requery.AdmittedRows++
		visible[d.ID] = true
	}

	for _, call := range record.ToolCalls {
		if !isRecall(call) {
			continue
		}
		if call.Error != "" {
			requery.RefusedRounds++
			continue
		}
		requery.DispatchedRounds++
		if yieldedUnseenRow(call.Results, visible) {
			requery.ProductiveRounds++
			continue
		}
		requery.BarrenRounds++
	}

	requery.AcceptedInsufficiency = requery.AdmittedRows < loop.ThinKnowledgeThreshold &&
		requery.DispatchedRounds == 0 && requery.RefusedRounds == 0
	requery.UnproductiveRequery = requery.BarrenRounds >= unproductiveRoundsNeeded
	requery.ProductiveRequery = requery.ProductiveRounds > 0
	requery.RefusedRequery = requery.RefusedRounds > 0

	return requery
}

func isRecall(call loop.ToolCallRecord) bool {
	return call.Tool == loop.ToolRecall || (call.Tool == "" && call.Query != "")
}

func yieldedUnseenRow(results []loop.Disposition, visible map[int64]bool) bool {
	unseen := false
	for _, d := range results {
		if !d.Included {
			continue
		}
		if !visible[d.ID] {
			unseen = true
		}
		visible[d.ID] = true
	}
	return unseen
}

// Firing is one requery disposition over a selection: how many records fired it, and whether it has ever fired at all.
type Firing struct {
	Disposition string `json:"disposition"`
	Definition  string `json:"definition"`
	FiredIn     int    `json:"firedIn"`
	Unfired     bool   `json:"unfired"`
}

// FiringResult is one reading of the requery dispositions over one named selection, carrying the population no figure of it is quotable without.
type FiringResult struct {
	Selector      string   `json:"selector"`
	Population    int      `json:"population"`
	Firings       []Firing `json:"firings"`
	NoDisposition int      `json:"noDisposition"`
}

// DispositionFiring counts, for every requery disposition, the records in the selection that fired it, and the records that fired none.
func DispositionFiring(archive runarchive.Archive) FiringResult {
	result := FiringResult{
		Selector:   archive.Selector,
		Population: len(archive.Entries),
		Firings:    make([]Firing, 0, len(dispositions)),
	}

	counts := make([]int, len(dispositions))
	for _, entry := range archive.Entries {
		requery := ComputeRequery(entry.Record)
		fired := false
		for i, d := range dispositions {
			if !d.fires(requery) {
				continue
			}
			counts[i]++
			fired = true
		}
		if !fired {
			result.NoDisposition++
		}
	}

	for i, d := range dispositions {
		result.Firings = append(result.Firings, Firing{
			Disposition: d.name,
			Definition:  d.definition,
			FiredIn:     counts[i],
			Unfired:     counts[i] == 0,
		})
	}

	return result
}

// RenderFiring renders the requery dispositions as the text a reader acts on, each stated as the count of records meeting its definition over the population it was computed against.
func RenderFiring(result FiringResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "REQUERY DISPOSITIONS  selector %q, population %d\n\n", result.Selector, result.Population)
	for _, firing := range result.Firings {
		state := fmt.Sprintf("%d of %d", firing.FiredIn, result.Population)
		if firing.Unfired {
			state = fmt.Sprintf("UNFIRED over a population of %d, which is not a rate", result.Population)
		}
		fmt.Fprintf(&b, "  %-22s %s\n  %-22s %s\n", firing.Disposition, state, "", firing.Definition)
	}
	fmt.Fprintf(&b, "\n  %-22s %d of %d\n", "no disposition", result.NoDisposition, result.Population)

	return b.String()
}

type disposition struct {
	name       string
	definition string
	fires      func(Requery) bool
}

var dispositions = []disposition{
	{
		name:       "acceptedInsufficiency",
		definition: "the block admitted fewer rows than the loop calls thin and the run emitted no recall round",
		fires:      func(r Requery) bool { return r.AcceptedInsufficiency },
	},
	{
		name:       "unproductiveRequery",
		definition: "two or more dispatched rounds each admitted no row the model had not already seen",
		fires:      func(r Requery) bool { return r.UnproductiveRequery },
	},
	{
		name:       "productiveRequery",
		definition: "a dispatched round admitted at least one row the model had not already seen",
		fires:      func(r Requery) bool { return r.ProductiveRequery },
	},
	{
		name:       "refusedRequery",
		definition: "a recall round carried an error and reached the graph not at all",
		fires:      func(r Requery) bool { return r.RefusedRequery },
	},
}
