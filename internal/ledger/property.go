package ledger

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/runarchive"
)

const (
	propertyDialExercise         = "dial exercise"
	propertyMechanismObservation = "mechanism observation"
)

const instantLayout = "2006-01-02T15:04:05Z"

// DialVerdict is the closed set of what a dial's recorded values over a selection can show.
type DialVerdict string

const (
	// DialUnrecorded is a dial no record in the selection carries a value for, which is a louder failure than an unswept one.
	DialUnrecorded DialVerdict = "unrecorded"
	// DialUnexercised is a dial every record that carries it recorded the same value for.
	DialUnexercised DialVerdict = "unexercised"
	// DialExercised is a dial the selection recorded at least two distinct values for.
	DialExercised DialVerdict = "exercised"
)

// Succession is the closed set of how a dial's distinct recorded values sit in time relative to one another.
type Succession string

const (
	// SuccessionSingleValue is a dial the selection recorded fewer than two distinct values for.
	SuccessionSingleValue Succession = "single value"
	// SuccessionUndatable is a dial at least one of whose recorded values sits in a record that states no instant.
	SuccessionUndatable Succession = "undatable"
	// SuccessionSequential is a dial whose values occupy disjoint instant ranges, which an edited constant produces as readily as a sweep does.
	SuccessionSequential Succession = "sequential"
	// SuccessionInterleaved is a dial whose values occupy overlapping instant ranges, which only varying it inside one span produces.
	SuccessionInterleaved Succession = "interleaved"
)

// Failure is one archive-level property that did not hold, named so it reads without the code beside it.
type Failure struct {
	Property   string `json:"property"`
	Subject    string `json:"subject"`
	Detail     string `json:"detail"`
	Selector   string `json:"selector"`
	Population int    `json:"population"`
}

// DialValue is one value a dial was recorded at: how many records carried it, the instant range those records span, and how many stated no instant.
type DialValue struct {
	Value      string    `json:"value"`
	RecordedIn int       `json:"recordedIn"`
	From       time.Time `json:"from"`
	To         time.Time `json:"to"`
	Undated    int       `json:"undated"`
}

// DialReading is one dial's exercise over a selection: its verdict, every value recorded verbatim with the span it was recorded over, and how those spans sit against one another.
type DialReading struct {
	Dial       string      `json:"dial"`
	Field      string      `json:"field"`
	Verdict    DialVerdict `json:"verdict"`
	Values     []DialValue `json:"values"`
	Succession Succession  `json:"succession"`
	RecordedIn int         `json:"recordedIn"`
}

// DialExerciseResult is one run of the dial-exercise property over one named selection.
type DialExerciseResult struct {
	Selector   string        `json:"selector"`
	Population int           `json:"population"`
	Readings   []DialReading `json:"readings"`
	Unreadable []int64       `json:"unreadable"`
	Failures   []Failure     `json:"failures"`
}

// MechanismReading is one merged mechanism's observation over a selection.
type MechanismReading struct {
	Mechanism  string `json:"mechanism"`
	Field      string `json:"field"`
	ObservedIn int    `json:"observedIn"`
}

// MechanismObservationResult is one run of the mechanism-observation property over one named selection.
type MechanismObservationResult struct {
	Selector   string             `json:"selector"`
	Population int                `json:"population"`
	Readings   []MechanismReading `json:"readings"`
	Failures   []Failure          `json:"failures"`
}

type dial struct {
	name  string
	field string
}

type mechanism struct {
	name     string
	field    string
	observed func(loop.Record) bool
}

type span struct {
	count   int
	from    time.Time
	to      time.Time
	undated int
}

var dials = []dial{
	{name: "CandidateLimit", field: "candidateLimit"},
	{name: "AssemblyByteBudget", field: "assemblyByteBudget"},
	{name: "SupplementaryByteBudget", field: "supplementaryByteBudget"},
	{name: "MaxModelCalls", field: "maxModelCalls"},
	{name: "DerivationBudget", field: "derivationBudget"},
	{name: "JudgementBudget", field: "judgementBudget"},
	{name: "RelevanceFloor", field: "relevanceFloor"},
	{name: "MaxFills", field: "maxFills"},
	{name: "FillSizeFloor", field: "fillSizeFloor"},
	{name: "MaxFillContentBytes", field: "maxFillContentBytes"},
	{name: "SubstanceRatioThreshold", field: "substanceRatioThreshold"},
	{name: "BlockOccupancy"},
}

var mechanisms = []mechanism{
	{name: "on-demand fill", field: "fills", observed: observedFill},
	{name: "substance form", field: "candidates[].form", observed: observedSubstanceForm},
}

// DialExercise reads every declared dial's recorded values across the selection, counting a value only where the record itself carries the field and naming the span it was recorded over.
func DialExercise(archive runarchive.Archive) DialExerciseResult {
	result := DialExerciseResult{
		Selector:   archive.Selector,
		Population: len(archive.Entries),
		Readings:   make([]DialReading, 0, len(dials)),
		Unreadable: []int64{},
		Failures:   make([]Failure, 0, len(dials)),
	}

	readable := make([]runarchive.Entry, 0, len(archive.Entries))
	limits := make([]map[string]string, 0, len(archive.Entries))
	for _, entry := range archive.Entries {
		recorded, ok := recordedLimits(entry.Raw)
		if !ok {
			result.Unreadable = append(result.Unreadable, entry.Node)
			result.Failures = append(result.Failures, Failure{
				Property:   propertyDialExercise,
				Subject:    fmt.Sprintf("record %d", entry.Node),
				Detail:     "the record's own bytes carry no readable limits object, so this record contributes to no dial's reading",
				Selector:   result.Selector,
				Population: len(archive.Entries),
			})
			continue
		}
		readable = append(readable, entry)
		limits = append(limits, recorded)
	}

	for _, d := range dials {
		reading := DialReading{Dial: d.name, Field: d.field, Values: []DialValue{}, Succession: SuccessionSingleValue}

		if d.field != "" {
			spans := make(map[string]*span)
			for i, recorded := range limits {
				value, ok := recorded[d.field]
				if !ok {
					continue
				}
				reading.RecordedIn++
				widen(spans, value, readable[i])
			}
			reading.Values = valuesOf(spans)
			reading.Succession = successionOf(reading.Values)
		}

		reading.Verdict = verdictFor(d, reading)
		result.Readings = append(result.Readings, reading)

		if reading.Verdict == DialExercised {
			continue
		}
		result.Failures = append(result.Failures, Failure{
			Property:   propertyDialExercise,
			Subject:    d.name,
			Detail:     dialDetail(d, reading),
			Selector:   result.Selector,
			Population: result.Population,
		})
	}

	return result
}

// MechanismObservation counts, for every declared merged mechanism, the records in the selection whose own field observes it.
func MechanismObservation(archive runarchive.Archive) MechanismObservationResult {
	result := MechanismObservationResult{
		Selector:   archive.Selector,
		Population: len(archive.Entries),
		Readings:   make([]MechanismReading, 0, len(mechanisms)),
		Failures:   make([]Failure, 0, len(mechanisms)),
	}

	for _, m := range mechanisms {
		reading := MechanismReading{Mechanism: m.name, Field: m.field}
		for _, entry := range archive.Entries {
			if m.observed(entry.Record) {
				reading.ObservedIn++
			}
		}
		result.Readings = append(result.Readings, reading)

		if reading.ObservedIn > 0 {
			continue
		}
		result.Failures = append(result.Failures, Failure{
			Property:   propertyMechanismObservation,
			Subject:    m.name,
			Detail:     fmt.Sprintf("%s is empty in every one of %d records; a merged mechanism never observed in a run is a defect of the merge", m.field, result.Population),
			Selector:   result.Selector,
			Population: result.Population,
		})
	}

	return result
}

// RenderProperties renders both archive-level properties as the text a reader acts on, every failure named before the readings it came from.
func RenderProperties(dialResult DialExerciseResult, mechanismResult MechanismObservationResult) string {
	var b strings.Builder

	failures := append(append([]Failure{}, dialResult.Failures...), mechanismResult.Failures...)
	fmt.Fprintf(&b, "ARCHIVE PROPERTIES  selector %q, population %d, %d failed\n\n", dialResult.Selector, dialResult.Population, len(failures))
	for _, f := range failures {
		fmt.Fprintf(&b, "  FAIL  %s: %s\n        %s\n", f.Property, f.Subject, f.Detail)
	}

	fmt.Fprintf(&b, "\n%s\n", propertyDialExercise)
	for _, r := range dialResult.Readings {
		fmt.Fprintf(&b, "  %-24s %-12s recorded in %2d/%2d  %s\n", r.Dial, r.Verdict, r.RecordedIn, dialResult.Population, renderValues(r.Values))
		if caveat := successionCaveat(r); caveat != "" {
			fmt.Fprintf(&b, "  %-24s %s\n", "", caveat)
		}
	}

	fmt.Fprintf(&b, "\n%s\n", propertyMechanismObservation)
	for _, r := range mechanismResult.Readings {
		fmt.Fprintf(&b, "  %-24s observed in %2d/%2d  field %s\n", r.Mechanism, r.ObservedIn, mechanismResult.Population, r.Field)
	}

	return b.String()
}

func widen(spans map[string]*span, value string, entry runarchive.Entry) {
	s := spans[value]
	if s == nil {
		s = &span{}
		spans[value] = s
	}
	s.count++
	if !entry.Dated {
		s.undated++
		return
	}
	if s.from.IsZero() || entry.At.Before(s.from) {
		s.from = entry.At
	}
	if entry.At.After(s.to) {
		s.to = entry.At
	}
}

func valuesOf(spans map[string]*span) []DialValue {
	values := make([]DialValue, 0, len(spans))
	for value, s := range spans {
		values = append(values, DialValue{Value: value, RecordedIn: s.count, From: s.from, To: s.to, Undated: s.undated})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].From.Equal(values[j].From) {
			return values[i].Value < values[j].Value
		}
		return values[i].From.Before(values[j].From)
	})
	return values
}

func successionOf(values []DialValue) Succession {
	if len(values) < 2 {
		return SuccessionSingleValue
	}
	for _, v := range values {
		if v.Undated > 0 {
			return SuccessionUndatable
		}
	}
	for i := 1; i < len(values); i++ {
		if !values[i-1].To.Before(values[i].From) {
			return SuccessionInterleaved
		}
	}
	return SuccessionSequential
}

func renderValues(values []DialValue) string {
	if len(values) == 0 {
		return "no recorded value"
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, fmt.Sprintf("%s in %d %s", v.Value, v.RecordedIn, renderSpan(v)))
	}
	return strings.Join(parts, " | ")
}

func renderSpan(v DialValue) string {
	if v.Undated == v.RecordedIn {
		return "(no record states an instant)"
	}
	dated := fmt.Sprintf("(%s … %s)", v.From.Format(instantLayout), v.To.Format(instantLayout))
	if v.Undated > 0 {
		return fmt.Sprintf("%s and %d stating no instant", dated, v.Undated)
	}
	return dated
}

func successionCaveat(r DialReading) string {
	if r.Verdict != DialExercised {
		return ""
	}
	switch r.Succession {
	case SuccessionSequential:
		return "CAVEAT: each value occupies its own span, which a constant edited between runs produces as readily as a sweep does"
	case SuccessionUndatable:
		return "CAVEAT: a record carrying one of these values states no instant, so the spans do not order"
	default:
		return ""
	}
}

func verdictFor(d dial, reading DialReading) DialVerdict {
	switch {
	case d.field == "" || reading.RecordedIn == 0:
		return DialUnrecorded
	case len(reading.Values) < 2:
		return DialUnexercised
	default:
		return DialExercised
	}
}

func dialDetail(d dial, reading DialReading) string {
	if d.field == "" {
		return "no record field carries this dial, so no selection can ever show it exercised"
	}
	if reading.RecordedIn == 0 {
		return fmt.Sprintf("no record carries %q, so any value read for it would be an absent field rather than a recorded one", d.field)
	}
	return fmt.Sprintf("%q is recorded in %d records and took one value there, %s", d.field, reading.RecordedIn, renderValues(reading.Values))
}

func recordedLimits(raw []byte) (map[string]string, bool) {
	var envelope struct {
		Limits map[string]json.RawMessage `json:"limits"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, false
	}

	values := make(map[string]string, len(envelope.Limits))
	for field, value := range envelope.Limits {
		values[field] = string(value)
	}
	return values, true
}

func observedFill(record loop.Record) bool {
	return len(record.Fills) > 0
}

func observedSubstanceForm(record loop.Record) bool {
	if anySubstanceForm(record.Candidates) {
		return true
	}
	for _, call := range record.ToolCalls {
		if anySubstanceForm(call.Results) {
			return true
		}
	}
	return false
}

func anySubstanceForm(dispositions []loop.Disposition) bool {
	for _, d := range dispositions {
		if d.Form == loop.FormSubstance {
			return true
		}
	}
	return false
}
