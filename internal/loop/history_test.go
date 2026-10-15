package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
)

var historyTestInstant = time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)

const (
	historyTestBanner = "===== EARLIER IN THIS CONVERSATION ====="

	userContentBeforeHistory = "===== INPUT =====\nwhat about him\n\n===== NOW =====\n2026-10-12T18:00:00Z\n\n\nBLOCK\n\n===== INPUT =====\nwhat about him"
	datelessBeforeHistory    = "===== INPUT =====\nwhat about him\n\n\nBLOCK\n\n===== INPUT =====\nwhat about him"
)

func exchangesNumbered(from, to int) []Exchange {
	exchanges := make([]Exchange, 0, to-from+1)
	for i := from; i <= to; i++ {
		exchanges = append(exchanges, Exchange{Input: fmt.Sprintf("said %d", i), Reply: fmt.Sprintf("replied %d", i)})
	}
	return exchanges
}

func runWithHistory(t *testing.T, history []Exchange) (Record, *fakeModel) {
	t.Helper()

	model := &fakeModel{results: []JudgeResult{answered("done")}}
	record, _, err := turnComposing(baseGraph(), model, nil).RunWithHistory(context.Background(), "what about him", 42, history)
	if err != nil {
		t.Fatalf("RunWithHistory: %v", err)
	}
	if len(model.calls) == 0 {
		t.Fatalf("the model was never called")
	}
	return record, model
}

func TestARunWithoutHistoryRendersTheSameUserContentAsBefore(t *testing.T) {
	t.Parallel()

	if got := RenderUserContent("BLOCK\n", "what about him", historyTestInstant, UpdateWindow{}, "", "", ""); got != userContentBeforeHistory {
		t.Errorf("user content with an instant =\n%q\nwant the layout from before history existed\n%q", got, userContentBeforeHistory)
	}
	if got := RenderUserContent("BLOCK\n", "what about him", time.Time{}, UpdateWindow{}, "", "", ""); got != datelessBeforeHistory {
		t.Errorf("user content with no instant =\n%q\nwant the layout from before history existed\n%q", got, datelessBeforeHistory)
	}

	for name, history := range map[string][]Exchange{"absent": nil, "empty": {}} {
		record, model := runWithHistory(t, history)
		for i, call := range model.calls {
			if call.Conversation != "" {
				t.Errorf("%s history: call %d carries the conversation %q, want none", name, i+1, call.Conversation)
			}
		}
		if len(record.History) != 0 || record.HistoryCut != 0 {
			t.Errorf("%s history: the record carries history %v cut %d, want neither", name, record.History, record.HistoryCut)
		}
	}
}

func TestTheConversationSectionSitsBetweenTheBlockAndTheClosingInputOldestFirst(t *testing.T) {
	t.Parallel()

	section := renderConversation([]Exchange{
		{Input: "rugenforth is on the ice wall again", Reply: "third try? bold."},
		{Input: "he drops instead of holding his grip", Reply: "stamina bar as a suggestion."},
	})
	wantSection := historyTestBanner + "\nuser said: rugenforth is on the ice wall again\nyou replied: third try? bold.\nuser said: he drops instead of holding his grip\nyou replied: stamina bar as a suggestion."
	if section != wantSection {
		t.Fatalf("section =\n%q\nwant\n%q", section, wantSection)
	}

	window := UpdateWindow{From: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)}

	for _, tc := range []struct {
		name    string
		now     time.Time
		window  UpdateWindow
		content string
	}{
		{"with an instant", historyTestInstant, UpdateWindow{}, "===== INPUT =====\nwhat about him\n\n===== NOW =====\n2026-10-12T18:00:00Z\n\n\nBLOCK\n\n\n" + wantSection + "\n\n===== INPUT =====\nwhat about him"},
		{"with a window", historyTestInstant, window, "===== INPUT =====\nwhat about him\n\n===== NOW =====\n2026-10-12T18:00:00Z\nretrieval is limited to nodes updated 2026-10-01 … 2026-10-07\n\n\nBLOCK\n\n\n" + wantSection + "\n\n===== INPUT =====\nwhat about him"},
		{"with no instant", time.Time{}, UpdateWindow{}, "===== INPUT =====\nwhat about him\n\n\nBLOCK\n\n\n" + wantSection + "\n\n===== INPUT =====\nwhat about him"},
	} {
		if got := RenderUserContent("BLOCK\n", "what about him", tc.now, tc.window, section, "", ""); got != tc.content {
			t.Errorf("%s: user content =\n%q\nwant\n%q", tc.name, got, tc.content)
		}
	}
}

func TestEveryJudgementCallOfAConversationTurnIsComposedWithTheConversationPresent(t *testing.T) {
	t.Parallel()

	oversize := []Exchange{{Input: strings.Repeat("a long remark ", HistorySafetyLimit), Reply: "noted"}}

	for _, tc := range []struct {
		name    string
		history []Exchange
		want    string
	}{
		{"history present", exchangesNumbered(1, 2), ";conversation"},
		{"history absent", nil, ""},
		{"history empty", []Exchange{}, ""},
		{"history cut away by the safety limit", oversize, ""},
	} {
		model := &fakeModel{results: researchToTheCap()}
		turn := turnComposing(graphYieldingNewRowsToEveryRecall(), model, nil)

		record, _, err := turn.RunWithHistory(context.Background(), "what about him", 42, tc.history)
		if err != nil {
			t.Fatalf("%s: RunWithHistory: %v", tc.name, err)
		}
		if len(model.calls) != MaxModelCalls {
			t.Fatalf("%s: the model was called %d times, want %d: the cases need tool rounds and the reserved answering call", tc.name, len(model.calls), MaxModelCalls)
		}
		if tc.want == "" && len(record.History) != 0 {
			t.Fatalf("%s: the record holds %d exchanges, but the case must reach the model with no section", tc.name, len(record.History))
		}

		for i, call := range model.calls {
			offered := "offers:recall,readNode"
			if i == MaxModelCalls-1 {
				offered = "offers:"
			}
			if want := offered + tc.want; call.System != want {
				t.Errorf("%s: call %d has system text %q, want %q", tc.name, i+1, call.System, want)
			}
		}
	}
}

func TestAnExchangeWithoutAReplyRendersOnlyItsUserLine(t *testing.T) {
	t.Parallel()

	got := renderConversation([]Exchange{{Input: "an unanswered channel line"}, {Input: "asked", Reply: "answered"}})
	want := historyTestBanner + "\nuser said: an unanswered channel line\nuser said: asked\nyou replied: answered"
	if got != want {
		t.Fatalf("section =\n%q\nwant\n%q", got, want)
	}
}

func TestTheConversationKeepsTheNewestExchangesUpToTheLimit(t *testing.T) {
	t.Parallel()

	supplied := exchangesNumbered(1, 6)
	record, model := runWithHistory(t, supplied)

	if !slices.Equal(record.History, supplied[2:]) {
		t.Fatalf("the record holds %v, want the newest four exchanges %v oldest first", record.History, supplied[2:])
	}
	section := model.calls[0].Conversation
	for _, kept := range []string{"said 3", "said 4", "said 5", "said 6"} {
		if !strings.Contains(section, kept) {
			t.Errorf("the section lacks %q:\n%s", kept, section)
		}
	}
	for _, cut := range []string{"said 1\n", "said 2\n"} {
		if strings.Contains(section, cut) {
			t.Errorf("the section still carries %q:\n%s", strings.TrimSpace(cut), section)
		}
	}
	if strings.Index(section, "said 3") > strings.Index(section, "said 6") {
		t.Errorf("the section is not oldest first:\n%s", section)
	}
}

func TestSmallExchangesBeyondTheLimitAreCutWhileFarBelowTheSafetyLimit(t *testing.T) {
	t.Parallel()

	record, model := runWithHistory(t, exchangesNumbered(1, 7))

	if len(record.History) != 4 || record.HistoryCut != 3 {
		t.Fatalf("the record holds %d exchanges and cut %d, want 4 held and 3 cut", len(record.History), record.HistoryCut)
	}
	if size := len(model.calls[0].Conversation); size*10 > 8_000 {
		t.Fatalf("the section is %d bytes, not far below the 8000 safety limit, so the count was not what cut", size)
	}
}

func TestTheSafetyLimitDropsTheOldestWholeExchangesAndNeverLeavesAGap(t *testing.T) {
	t.Parallel()

	large := strings.Repeat("L", 7_950)

	for _, tc := range []struct {
		name     string
		supplied []Exchange
		want     []string
	}{
		{"a large exchange between small ones", []Exchange{{Input: "old a"}, {Input: "old b"}, {Input: large}, {Input: "newest"}}, []string{"newest"}},
		{"a large oldest exchange", []Exchange{{Input: large}, {Input: "b"}, {Input: "c"}, {Input: "d"}}, []string{"b", "c", "d"}},
	} {
		record, model := runWithHistory(t, tc.supplied)

		var got []string
		for _, e := range record.History {
			got = append(got, e.Input)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: the turn kept %v, want %v", tc.name, got, tc.want)
		}
		if size := len(model.calls[0].Conversation); size > 8_000 {
			t.Errorf("%s: the section is %d bytes, over the 8000 safety limit", tc.name, size)
		}
		if record.HistoryCut != len(tc.supplied)-len(tc.want) {
			t.Errorf("%s: the record cut %d, want %d", tc.name, record.HistoryCut, len(tc.supplied)-len(tc.want))
		}
	}
}

func TestTheSafetyCutIsLoggedAtWarn(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		supplied []Exchange
		warns    bool
	}{
		{"a safety cut", []Exchange{{Input: strings.Repeat("L", 7_950)}, {Input: "newest"}}, true},
		{"a count cut alone", exchangesNumbered(1, 6), false},
	} {
		var logged bytes.Buffer
		model := &fakeModel{results: []JudgeResult{answered("done")}}
		turn := NewTurn(baseGraph(), model, nil, fixedSystem("system"), "test-model", slog.New(slog.NewTextHandler(&logged, nil)))

		if _, _, err := turn.RunWithHistory(context.Background(), "hello", 42, tc.supplied); err != nil {
			t.Fatalf("%s: RunWithHistory: %v", tc.name, err)
		}
		var safetyLines []string
		for _, line := range strings.Split(logged.String(), "\n") {
			if strings.Contains(line, "safety limit") {
				safetyLines = append(safetyLines, line)
			}
		}
		if !tc.warns {
			if len(safetyLines) != 0 {
				t.Errorf("%s: the log carries a safety-limit line, want none: %q", tc.name, safetyLines)
			}
			continue
		}
		if len(safetyLines) != 1 {
			t.Errorf("%s: the log carries %d safety-limit lines, want 1; log:\n%s", tc.name, len(safetyLines), logged.String())
			continue
		}
		for _, want := range []string{"level=WARN", "supplied=2", "used=1"} {
			if !strings.Contains(safetyLines[0], want) {
				t.Errorf("%s: the safety-limit line lacks %q: %s", tc.name, want, safetyLines[0])
			}
		}
	}
}

func TestAnOversizeNewestExchangeLeavesNoHistoryAndCutsAll(t *testing.T) {
	t.Parallel()

	record, model := runWithHistory(t, []Exchange{{Input: "older"}, {Input: strings.Repeat("N", 9_000)}})

	if len(record.History) != 0 || record.HistoryCut != 2 {
		t.Fatalf("the record holds %d exchanges and cut %d, want none held and both cut", len(record.History), record.HistoryCut)
	}
	if got := model.calls[0].Conversation; got != "" {
		t.Fatalf("the section is %d bytes, want none: an exchange is never truncated to fit", len(got))
	}
}

func TestTheHistoryIsChargedToTheWorkingMemory(t *testing.T) {
	t.Parallel()

	const (
		anchorBytes = len("anchor body")
		rowBytes    = 14_990
	)
	history := []Exchange{
		{Input: strings.Repeat("é", 750), Reply: strings.Repeat("b", 1_500)},
		{Input: strings.Repeat("c", 1_500), Reply: strings.Repeat("d", 1_500)},
	}
	section := renderConversation(history)
	room := 80_000 - (anchorBytes + 4*rowBytes + len(section))
	if room <= 0 || room >= 20_000 {
		t.Fatalf("test setup error: the first round's room is %d, want it between the section's charge and the supplementary budget", room)
	}

	body := func(n int) string { return strings.Repeat("#", n) }

	for _, tc := range []struct {
		name         string
		history      []Exchange
		wantLargeRow bool
		wantSmallRow bool
	}{
		{"with the conversation charged", history, true, false},
		{"without a conversation", nil, true, true},
	} {
		graph := baseGraph()
		graph.recallQueue = []recallResponse{
			{Candidates: []Candidate{rowOf(11, body(rowBytes)), rowOf(12, body(rowBytes)), rowOf(13, body(rowBytes)), rowOf(14, body(rowBytes))}},
			{Candidates: []Candidate{rowOf(21, body(room)), rowOf(22, body(1))}},
		}
		model := &fakeModel{results: []JudgeResult{recallFor("one"), answered("done")}}

		record, _, err := turnComposing(graph, model, nil).RunWithHistory(context.Background(), "hello", 42, tc.history)
		if err != nil {
			t.Fatalf("%s: RunWithHistory: %v", tc.name, err)
		}

		round := record.ToolCalls[0].Results
		if len(round) != 2 {
			t.Fatalf("%s: the round recorded %d rows, want the 2 it returned", tc.name, len(round))
		}
		if round[0].Included != tc.wantLargeRow || round[1].Included != tc.wantSmallRow {
			t.Errorf("%s: the %d-byte row was Included=%v and the 1-byte row Included=%v, want %v and %v: the memory had %d bytes of room left after the block and the conversation",
				tc.name, room, round[0].Included, round[1].Included, tc.wantLargeRow, tc.wantSmallRow, room)
		}
	}
}

func TestEveryJudgementCallOfATurnCarriesTheSameConversation(t *testing.T) {
	t.Parallel()

	history := exchangesNumbered(1, 2)
	model := &fakeModel{results: researchToTheCap()}

	if _, _, err := turnComposing(graphYieldingNewRowsToEveryRecall(), model, nil).RunWithHistory(context.Background(), "hello", 42, history); err != nil {
		t.Fatalf("RunWithHistory: %v", err)
	}
	if len(model.calls) != MaxModelCalls {
		t.Fatalf("the model was called %d times, want %d: the tool rounds and the reserved answering call", len(model.calls), MaxModelCalls)
	}

	want := historyTestBanner + "\nuser said: said 1\nyou replied: replied 1\nuser said: said 2\nyou replied: replied 2"
	for i, call := range model.calls {
		if call.Conversation != want {
			t.Errorf("call %d carries the conversation %q, want %q", i+1, call.Conversation, want)
		}
	}
}

func TestTheRecordCarriesTheHistoryItUsedAndHowManyItCut(t *testing.T) {
	t.Parallel()

	withNone, _ := runWithHistory(t, nil)
	encoded, err := json.Marshal(withNone)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &members); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"history", "historyCut"} {
		if _, present := members[key]; present {
			t.Errorf("a record of a turn with no history carries %q", key)
		}
	}

	supplied := exchangesNumbered(1, 6)
	record, _ := runWithHistory(t, supplied)
	encoded, err = json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		History []struct {
			Input string `json:"input"`
			Reply string `json:"reply"`
		} `json:"history"`
		HistoryCut int `json:"historyCut"`
		Limits     struct {
			HistoryExchangeLimit int `json:"historyExchangeLimit"`
			HistorySafetyLimit   int `json:"historySafetyLimit"`
		} `json:"limits"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.History) != 4 || decoded.History[0].Input != "said 3" || decoded.History[0].Reply != "replied 3" || decoded.History[3].Input != "said 6" {
		t.Errorf("history on the wire = %+v, want the newest four exchanges oldest first", decoded.History)
	}
	if decoded.HistoryCut != 2 {
		t.Errorf("historyCut on the wire = %d, want 2", decoded.HistoryCut)
	}
	if decoded.Limits.HistoryExchangeLimit != 4 || decoded.Limits.HistorySafetyLimit != 8_000 {
		t.Errorf("limits on the wire = %+v, want 4 exchanges and 8000 bytes", decoded.Limits)
	}
}

func TestTheSummaryReportsHistoryCountsAndNoneOfItsText(t *testing.T) {
	t.Parallel()

	record, _ := runWithHistory(t, []Exchange{{Input: "dropped"}, {Input: "zebra-token first", Reply: "okapi-token second"}, {Input: "third"}, {Input: "fourth"}, {Input: "fifth"}})

	summary := RenderSummary(record, historyTestInstant)
	if !strings.Contains(summary, "\nHISTORY  4 of 5 exchanges, 153 B\n") {
		t.Errorf("the summary carries no history line of 4 of 5 exchanges and 153 B:\n%s", summary)
	}
	for _, token := range []string{"zebra-token", "okapi-token", "third", "fifth"} {
		if strings.Contains(summary, token) {
			t.Errorf("the summary carries %q from an exchange:\n%s", token, summary)
		}
	}

	bare, _ := runWithHistory(t, nil)
	if strings.Contains(RenderSummary(bare, historyTestInstant), "HISTORY") {
		t.Errorf("the summary of a turn without history carries a history line")
	}
}

func TestASectionOfExactlyTheSafetyLimitIsKeptAndOneByteMoreIsNot(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		atLimit     string
		oneByteOver string
	}{
		{"ASCII", strings.Repeat("x", 7_948), strings.Repeat("x", 7_949)},
		{"multi-byte characters, whose byte length is not their rune count", strings.Repeat("é", 3_974), strings.Repeat("é", 3_974) + "x"},
	} {
		atLimit, model := runWithHistory(t, []Exchange{{Input: tc.atLimit}})
		if len(atLimit.History) != 1 || len(model.calls[0].Conversation) != 8_000 {
			t.Errorf("%s: a section of exactly 8000 bytes was kept as %d exchanges of %d bytes, want 1 exchange of 8000", tc.name, len(atLimit.History), len(model.calls[0].Conversation))
		}

		over, model := runWithHistory(t, []Exchange{{Input: tc.oneByteOver}})
		if len(over.History) != 0 || model.calls[0].Conversation != "" {
			t.Errorf("%s: a section of 8001 bytes was kept as %d exchanges, want it cut whole", tc.name, len(over.History))
		}
	}
}

func TestTheHistoryLabelsReadAsWhoSaidWhatAndAreTheOnesTheRendererWrites(t *testing.T) {
	t.Parallel()

	if LabelUserSaid != "user said:" || LabelYouReplied != "you replied:" {
		t.Fatalf("labels = %q and %q, want %q and %q", LabelUserSaid, LabelYouReplied, "user said:", "you replied:")
	}

	got := renderConversation([]Exchange{{Input: "hello", Reply: "hi"}})
	want := historyTestBanner + "\n" + LabelUserSaid + " hello\n" + LabelYouReplied + " hi"
	if got != want {
		t.Fatalf("section =\n%q\nwant\n%q", got, want)
	}
	for _, former := range []string{"\nuser: ", "\nyou: "} {
		if strings.Contains(got, former) {
			t.Errorf("section carries the former label %q, which reads as the model's own voice", former)
		}
	}
}
