package ledger

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/runarchive"
)

func admitted(ids ...int64) []loop.Disposition {
	rows := make([]loop.Disposition, 0, len(ids))
	for i, id := range ids {
		rows = append(rows, loop.Disposition{Rank: i + 1, ID: id, Included: true})
	}
	return rows
}

func recall(query string, results []loop.Disposition) loop.ToolCallRecord {
	return loop.ToolCallRecord{Tool: loop.ToolRecall, Query: query, Results: results}
}

func TestAcceptedInsufficiencyFiresOnAThinBlockTheRunAskedNothingMoreAbout(t *testing.T) {
	t.Parallel()

	got := ComputeRequery(loop.Record{Candidates: admitted(1, 2)})

	if !got.AcceptedInsufficiency {
		t.Fatalf("a block of 2 admitted rows with no recall round must be accepted insufficiency, got %+v", got)
	}
}

func TestAcceptedInsufficiencyUsesTheBlockThinnessTheLoopItselfApplies(t *testing.T) {
	t.Parallel()

	thin := make([]int64, loop.ThinKnowledgeThreshold-1)
	for i := range thin {
		thin[i] = int64(i + 1)
	}
	if got := ComputeRequery(loop.Record{Candidates: admitted(thin...)}); !got.AcceptedInsufficiency {
		t.Errorf("a block one row below the loop's own thinness must be accepted insufficiency, got %+v", got)
	}

	atThreshold := append(thin, int64(loop.ThinKnowledgeThreshold))
	if got := ComputeRequery(loop.Record{Candidates: admitted(atThreshold...)}); got.AcceptedInsufficiency {
		t.Errorf("a block at the loop's own thinness is not thin and must not be accepted insufficiency, got %+v", got)
	}
}

func TestAcceptedInsufficiencyDoesNotFireWhenTheRunAskedAndTheRoundWasRefused(t *testing.T) {
	t.Parallel()

	record := loop.Record{
		Candidates: admitted(1),
		ToolCalls:  []loop.ToolCallRecord{{Tool: loop.ToolRecall, Query: "more", Error: "call cap reached"}},
	}

	got := ComputeRequery(record)

	if got.AcceptedInsufficiency {
		t.Fatalf("a run that asked and was refused did not accept its insufficiency, got %+v", got)
	}
	if !got.RefusedRequery || got.RefusedRounds != 1 {
		t.Fatalf("want one refused round, got %+v", got)
	}
}

func TestARefusedRoundIsNeitherDispatchedNorBarren(t *testing.T) {
	t.Parallel()

	record := loop.Record{
		Candidates: admitted(1, 2, 3, 4, 5),
		ToolCalls: []loop.ToolCallRecord{
			{Tool: loop.ToolRecall, Query: "refused", Error: "call cap reached"},
			{Tool: loop.ToolRecall, Query: "also refused", Error: "recall is closed"},
		},
	}

	got := ComputeRequery(record)

	if got.DispatchedRounds != 0 || got.BarrenRounds != 0 {
		t.Fatalf("a refused round reached the graph not at all and yielded nothing to call barren, got %+v", got)
	}
	if got.UnproductiveRequery {
		t.Fatalf("two refused rounds are not two barren rounds, got %+v", got)
	}
	if got.RefusedRounds != 2 {
		t.Fatalf("want two refused rounds, got %+v", got)
	}
}

func TestARoundIsProductiveOnlyForARowTheModelHadNotAlreadySeen(t *testing.T) {
	t.Parallel()

	record := loop.Record{
		Candidates: admitted(6115, 11259, 11398, 12957, 12824, 10994, 10032),
		ToolCalls: []loop.ToolCallRecord{
			recall("retrieval path changes today", admitted(10451, 10423, 10434, 10422, 10455)),
			recall("retrieval path changes the day after", admitted(10423, 10422, 10451, 10455, 10434)),
			recall("retrieval path changes the day after that", admitted(10423, 10422, 10451, 10455, 10434)),
			recall("and the day after that", admitted(10423, 10422, 10451, 10455, 10434)),
			recall("and again", admitted(10423, 10422, 10455, 10451, 10434)),
			{Tool: loop.ToolRecall, Query: "once more", Error: "call cap reached"},
		},
	}

	got := ComputeRequery(record)

	if got.ProductiveRounds != 1 {
		t.Fatalf("only the first round admitted a row the block did not hold, so want 1 productive round, got %+v", got)
	}
	if got.BarrenRounds != 4 {
		t.Fatalf("four rounds returned rows the model had already seen, so want 4 barren rounds, got %+v", got)
	}
	if !got.UnproductiveRequery {
		t.Fatalf("four barren rounds must fire unproductive requery, got %+v", got)
	}
	if !got.ProductiveRequery {
		t.Fatalf("the first round did yield, so productive requery must still hold, got %+v", got)
	}
	if !got.RefusedRequery {
		t.Fatalf("the last round carried an error, so refused requery must hold, got %+v", got)
	}
}

func TestARoundThatAdmitsARowTheBlockDidNotHoldIsProductiveAndNotUnproductive(t *testing.T) {
	t.Parallel()

	record := loop.Record{
		Candidates: admitted(1, 2),
		ToolCalls: []loop.ToolCallRecord{
			recall("something new", admitted(3)),
			recall("something else new", admitted(4)),
		},
	}

	got := ComputeRequery(record)

	if got.ProductiveRounds != 2 || !got.ProductiveRequery {
		t.Fatalf("both rounds yielded an unseen row, got %+v", got)
	}
	if got.BarrenRounds != 0 || got.UnproductiveRequery {
		t.Fatalf("no round was barren, so unproductive requery must not fire, got %+v", got)
	}
}

func TestARowVisibleOnlyFromAnEarlierRoundIsNoLongerAnUnseenYield(t *testing.T) {
	t.Parallel()

	record := loop.Record{
		Candidates: admitted(1),
		ToolCalls: []loop.ToolCallRecord{
			recall("first", admitted(2)),
			recall("second", admitted(2)),
		},
	}

	got := ComputeRequery(record)

	if got.ProductiveRounds != 1 || got.BarrenRounds != 1 {
		t.Fatalf("row 2 was new to the first round and old to the second, got %+v", got)
	}
}

func TestARoundWhoseRowsWereAllCutYieldsNothing(t *testing.T) {
	t.Parallel()

	record := loop.Record{
		Candidates: admitted(1, 2, 3, 4, 5),
		ToolCalls: []loop.ToolCallRecord{
			{Tool: loop.ToolRecall, Query: "all too large", Results: []loop.Disposition{
				{Rank: 1, ID: 9, CutReason: "byte budget exceeded"},
				{Rank: 2, ID: 10, CutReason: "byte budget exceeded"},
			}},
			{Tool: loop.ToolRecall, Query: "all too large again", Results: []loop.Disposition{
				{Rank: 1, ID: 11, CutReason: "byte budget exceeded"},
			}},
		},
	}

	got := ComputeRequery(record)

	if got.ProductiveRounds != 0 || got.BarrenRounds != 2 || !got.UnproductiveRequery {
		t.Fatalf("a round that admitted nothing yielded nothing, got %+v", got)
	}
}

func TestUnproductiveRequeryNeedsTwoBarrenRoundsAndOneIsNotEnough(t *testing.T) {
	t.Parallel()

	record := loop.Record{
		Candidates: admitted(1, 2, 3, 4, 5),
		ToolCalls:  []loop.ToolCallRecord{recall("nothing new", admitted(1))},
	}

	got := ComputeRequery(record)

	if got.BarrenRounds != 1 {
		t.Fatalf("want the single barren round counted, got %+v", got)
	}
	if got.UnproductiveRequery {
		t.Fatalf("one barren round is below the two the disposition needs, got %+v", got)
	}
}

func TestAWriteRoundIsNotARecallRound(t *testing.T) {
	t.Parallel()

	record := loop.Record{
		Candidates: admitted(1),
		ToolCalls:  []loop.ToolCallRecord{{Tool: loop.ToolWriteFile, Path: "notes.md", Bytes: 42}},
	}

	got := ComputeRequery(record)

	if got.DispatchedRounds != 0 || got.RefusedRounds != 0 {
		t.Fatalf("a file write is not a requery, got %+v", got)
	}
	if !got.AcceptedInsufficiency {
		t.Fatalf("the run asked for no more knowledge, so it accepted its insufficiency, got %+v", got)
	}
}

func TestALegacyRoundWithNoToolNameButAQueryCountsAsARecall(t *testing.T) {
	t.Parallel()

	record := loop.Record{
		Candidates: admitted(1),
		ToolCalls:  []loop.ToolCallRecord{{Query: "an older record names no tool", Results: admitted(2)}},
	}

	got := ComputeRequery(record)

	if got.DispatchedRounds != 1 || got.ProductiveRounds != 1 {
		t.Fatalf("an older round carrying a query alone is a recall, got %+v", got)
	}
}

func TestComputeRecomputesTheOutcomeRatherThanReadingTheOneStoredBesideIt(t *testing.T) {
	t.Parallel()

	record := loop.Record{
		Answer:     "an answer",
		Candidates: admitted(1),
		Outcome:    loop.Outcome{Verdict: loop.VerdictEmpty, Produced: false, Grounded: false},
	}

	got := Compute(runarchive.Entry{Node: 7, Record: record})

	if got.Outcome.Verdict != loop.VerdictDelivered {
		t.Fatalf("the stored verdict was poisoned to %q; recomputation must ignore it and reach %q, got %q",
			loop.VerdictEmpty, loop.VerdictDelivered, got.Outcome.Verdict)
	}
}

func TestOverComputesOneEntryPerArchivedRecordInTheOrderTheArchiveHoldsThem(t *testing.T) {
	t.Parallel()

	archive := runarchive.Archive{Entries: []runarchive.Entry{
		{Node: 3, Record: loop.Record{Answer: "one", Candidates: admitted(1)}},
		{Node: 1, Record: loop.Record{Candidates: admitted(1, 2)}},
	}}

	got := Over(archive)

	if len(got) != 2 || got[0].Node != 3 || got[1].Node != 1 {
		t.Fatalf("want one entry per record in archive order, got %+v", got)
	}
	if got[0].Outcome.Verdict != loop.VerdictDelivered || got[1].Outcome.Verdict != loop.VerdictEmpty {
		t.Fatalf("verdicts are %q and %q", got[0].Outcome.Verdict, got[1].Outcome.Verdict)
	}
}

func TestAnUndatedEntrySerialisesWithNoInstantRatherThanAnEarlyOne(t *testing.T) {
	t.Parallel()

	undated := Compute(runarchive.Entry{Node: 7, Record: loop.Record{Answer: "an answer", Candidates: admitted(1)}})

	encoded, err := json.Marshal(undated)
	if err != nil {
		t.Fatalf("encode entry: %v", err)
	}
	if strings.Contains(string(encoded), "0001-01-01") {
		t.Fatalf("an unknown instant must not serialise as an early one, got %s", encoded)
	}
	if !strings.Contains(string(encoded), `"dated":false`) {
		t.Fatalf("an entry must say whether its instant is known, got %s", encoded)
	}

	at := time.Date(2026, 9, 17, 18, 34, 11, 0, time.UTC)
	dated := Compute(runarchive.Entry{Node: 7, At: at, Dated: true, Record: loop.Record{Answer: "an answer", Candidates: admitted(1)}})
	encoded, err = json.Marshal(dated)
	if err != nil {
		t.Fatalf("encode entry: %v", err)
	}
	if !strings.Contains(string(encoded), "2026-09-17T18:34:11Z") || !strings.Contains(string(encoded), `"dated":true`) {
		t.Fatalf("a dated entry carries its instant and says so, got %s", encoded)
	}
}
