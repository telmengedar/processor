package loop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTurnRunRecordsTheLastResponseExactlyWhenThatCallAnswered(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name string
		turn func(t *testing.T) *Turn
		want bool
	}{
		{
			name: "an ordinary call ends the turn",
			turn: func(t *testing.T) *Turn {
				return NewTurn(baseGraph(), &fakeModel{results: []JudgeResult{answered("hi")}}, nil, "system", "test-model", testLogger())
			},
			want: true,
		},
		{
			name: "the reserved call is issued at the call cap and completes",
			turn: func(t *testing.T) *Turn {
				return NewTurn(graphYieldingNewRowsToEveryRecall(), &fakeModel{results: researchThenAnswer("done")}, nil, "system", "test-model", testLogger())
			},
			want: true,
		},
		{
			name: "the reserved call follows a closed recall and completes",
			turn: func(t *testing.T) *Turn {
				return NewTurn(graphReturningTheSameRowsToEveryRecall(), &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}, nil, "system", "test-model", testLogger())
			},
			want: true,
		},
		{
			name: "the reserved call is issued at the call cap and fails",
			turn: func(t *testing.T) *Turn {
				return NewTurn(graphYieldingNewRowsToEveryRecall(), &fakeModel{results: researchToTheCap(), failOn: MaxModelCalls, failErr: errors.New("connection reset")}, nil, "system", "test-model", testLogger())
			},
			want: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			record, _, err := scenario.turn(t).Run(context.Background(), "hello", 42)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if got := record.LastResponse != nil; got != scenario.want {
				t.Fatalf("record.LastResponse present = %v, want %v", got, scenario.want)
			}
		})
	}
}

func TestTurnRunRecordOmitsTheLastResponseKeyWhenNoResponseArrived(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: researchToTheCap(), failOn: MaxModelCalls, failErr: errors.New("connection reset")}
	turn := NewTurn(graphYieldingNewRowsToEveryRecall(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	body, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	if strings.Contains(string(body), `"lastResponse"`) {
		t.Fatalf("record JSON contains %q on a reserved call that failed; want no response fact carried for a call that returned no response; body=%s", `"lastResponse"`, body)
	}
}

func TestTurnRunRecordCarriesTheLastResponseKeyWhenACallAnswered(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{answered("hi")}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	body, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	if !strings.Contains(string(body), `"lastResponse":{"reasoningBytes":0,"unofferedToolCalls":0}`) {
		t.Fatalf("record JSON = %s, want a present lastResponse member carrying both counts even when both are a measured zero", body)
	}
}

func TestTurnRunRecordOmitsTheReservedCallKeyWhenNothingWasReserved(t *testing.T) {
	t.Parallel()

	model := &fakeModel{results: []JudgeResult{answered("hi")}}
	turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

	record, _, err := turn.Run(context.Background(), "hello", 42)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	body, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	if strings.Contains(string(body), `"reservedCall"`) {
		t.Fatalf("record JSON contains %q on a run that reserved no call; body=%s", `"reservedCall"`, body)
	}
}

func TestTurnRunEveryReservingConditionRecordsAReservedCallState(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name string
		run  func(t *testing.T) Record
	}{
		{
			name: "the call budget was spent, and the reserved call completes",
			run: func(t *testing.T) Record {
				turn := NewTurn(graphYieldingNewRowsToEveryRecall(), &fakeModel{results: researchToTheCap()}, nil, "system", "test-model", testLogger())
				record, _, err := turn.Run(context.Background(), "hello", 42)
				if err != nil {
					t.Fatalf("Run: %v", err)
				}
				return record
			},
		},
		{
			name: "recall closed on consecutive barren rounds, and the reserved call completes",
			run: func(t *testing.T) Record {
				turn := NewTurn(graphReturningTheSameRowsToEveryRecall(), &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}, nil, "system", "test-model", testLogger())
				record, _, err := turn.Run(context.Background(), "hello", 42)
				if err != nil {
					t.Fatalf("Run: %v", err)
				}
				return record
			},
		},
		{
			name: "recall closed, and the remaining time stops the run before the reserved call is issued: the state must already be set at the reservation site, since the turn can stop before the reserved call is ever issued",
			run: func(t *testing.T) Record {
				now := guardInstant
				model := &fakeModel{results: recallsDifferingOnlyByADateSuffix(MaxModelCalls)}
				model.beforeReturn = func() {
					if len(model.calls) >= barrenRoundsToClose+2 {
						now = guardInstant.Add(9*time.Minute + 59*time.Second)
					}
				}
				turn := turnWithClock(model, graphReturningTheSameRowsToEveryRecall(), func() time.Time { return now })

				ctx, cancel := context.WithDeadline(context.Background(), guardInstant.Add(10*time.Minute))
				defer cancel()

				record, _, err := turn.Run(ctx, "hello", 42)
				if err != nil {
					t.Fatalf("Run: %v", err)
				}
				if record.TimeShortfall == "" {
					t.Fatalf("the scenario did not reach the path under test: TimeShortfall is empty")
				}
				return record
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			record := scenario.run(t)
			if record.ReservedCall.State == "" {
				t.Fatalf("record.ReservedCall.State is empty on a turn that reserved its last call under %q", scenario.name)
			}
		})
	}
}

func TestTurnRunRecordsOneUsageEntryPerModelCall(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name string
		turn func(t *testing.T) *Turn
	}{
		{
			name: "a turn that reserves and completes",
			turn: func(t *testing.T) *Turn {
				return NewTurn(graphYieldingNewRowsToEveryRecall(), &fakeModel{results: researchThenAnswer("done")}, nil, "system", "test-model", testLogger())
			},
		},
		{
			name: "a turn whose reserved call fails",
			turn: func(t *testing.T) *Turn {
				return NewTurn(graphYieldingNewRowsToEveryRecall(), &fakeModel{results: researchToTheCap(), failOn: MaxModelCalls, failErr: errors.New("connection reset")}, nil, "system", "test-model", testLogger())
			},
		},
		{
			name: "a turn that ends ordinarily",
			turn: func(t *testing.T) *Turn {
				return NewTurn(baseGraph(), &fakeModel{results: []JudgeResult{answered("hi")}}, nil, "system", "test-model", testLogger())
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			record, _, err := scenario.turn(t).Run(context.Background(), "hello", 42)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if len(record.Usage) != record.ModelCalls {
				t.Fatalf("len(record.Usage) = %d, record.ModelCalls = %d, want them equal under %q", len(record.Usage), record.ModelCalls, scenario.name)
			}
		})
	}
}

func TestTurnJudgeEndsTheTurnOnEveryTerminalThatWantsNoTool(t *testing.T) {
	t.Parallel()

	nonToolWanting := []TerminalReason{Answered, Truncated, Refused, Unrecognised}

	for _, reason := range nonToolWanting {
		t.Run(string(reason), func(t *testing.T) {
			t.Parallel()

			model := &fakeModel{results: []JudgeResult{
				{Answer: "first call's prose", Reason: reason, RawReason: "raw", ReasoningBytes: 7, UnofferedToolCalls: 2},
				{Reason: WantsRecall, RawReason: "tool_calls", RecallQuery: "would be a second call"},
			}}
			turn := NewTurn(baseGraph(), model, nil, "system", "test-model", testLogger())

			record, _, err := turn.Run(context.Background(), "hello", 42)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if len(model.calls) != 1 {
				t.Fatalf("the model was called %d times, want 1: a terminal that wants no tool must end the turn regardless of what a further call was queued to return", len(model.calls))
			}
			if record.LastResponse == nil || record.LastResponse.ReasoningBytes != 7 || record.LastResponse.UnofferedToolCalls != 2 {
				t.Fatalf("record.LastResponse = %+v, want the first and only call's own facts: every call that can carry them is, by this property, the turn's last", record.LastResponse)
			}
		})
	}
}
