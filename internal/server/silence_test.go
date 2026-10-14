package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

const silenceMarker = loop.SilenceMarker

func silenceTurn(content string, step func(int) (loop.JudgeResult, error)) (*loop.Turn, *configGraph) {
	graph := &configGraph{content: content}
	return loop.NewTurn(graph, &scriptedModel{step: step}, nil, fixedSystem("system text"), "test-model", testLogger()), graph
}

func TestAChosenSilenceAnswersAnEmptyReplyNotProducedAndSilent(t *testing.T) {
	t.Parallel()

	turn, _ := silenceTurn(`{"silence": true}`, answeringWith(silenceMarker))

	rec := postRuns(t, turn, configPostBodyHead+`77}`)

	members, keys := topLevelKeys(t, rec)
	if want := []string{"produced", "reply", "silent", "written"}; !slices.Equal(keys, want) {
		t.Fatalf("body keys = %q, want %q", keys, want)
	}
	if reply := stringMember(t, members, "reply"); reply != "" {
		t.Errorf("reply = %q, want empty: nothing to post on a chosen silence", reply)
	}
	if boolMember(t, members, "produced") || !boolMember(t, members, "silent") {
		t.Errorf("produced = %v and silent = %v, want false and true", boolMember(t, members, "produced"), boolMember(t, members, "silent"))
	}
}

func TestAChosenSilenceVerboseCarriesAnEmptyReplyTheRawAnswerAndTheSilentFacts(t *testing.T) {
	t.Parallel()

	turn, _ := silenceTurn(`{"silence": true}`, answeringWith("Gangolf: "+silenceMarker))

	rec := postRunsVerbose(t, turn, configPostBodyHead+`77}`)

	var body struct {
		loop.Record
		Reply string `json:"reply"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("the verbose body is not a record: %v; body=%s", err, rec.Body.String())
	}
	if body.Reply != "" || !body.Silent || body.Outcome.Verdict != loop.VerdictSilent || body.Outcome.Produced {
		t.Errorf("reply = %q, silent = %v, outcome = %+v, want an empty reply and a silent outcome", body.Reply, body.Silent, body.Outcome)
	}
	if body.Answer != "Gangolf: "+silenceMarker {
		t.Errorf("answer = %q, want the raw answer kept", body.Answer)
	}
	if body.Perspective == nil || !body.Perspective.Silence {
		t.Errorf("perspective = %+v, want it to say silence was offered", body.Perspective)
	}
}

func TestABlankAnswerUnderSilenceKeepsTheFallbackSentenceAndCarriesNoSilentKey(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"", "  \n "} {
		turn, _ := silenceTurn(`{"silence": true}`, answeringWith(answer))

		members, keys := topLevelKeys(t, postRuns(t, turn, configPostBodyHead+`77}`))

		if want := []string{"produced", "reply", "written"}; !slices.Equal(keys, want) {
			t.Errorf("answer %q: body keys = %q, want %q", answer, keys, want)
		}
		if reply := stringMember(t, members, "reply"); reply != fallbackReply {
			t.Errorf("answer %q: reply = %q, want the fallback sentence", answer, reply)
		}
		if boolMember(t, members, "produced") {
			t.Errorf("answer %q: produced = true, want false", answer)
		}
	}
}

func TestTheMarkerIsDeliveredVerbatimWhenTheConfigDoesNotOfferSilence(t *testing.T) {
	t.Parallel()

	for _, content := range []string{`{}`, `{"silence": false}`} {
		turn, _ := silenceTurn(content, answeringWith("say "+silenceMarker))

		members, keys := topLevelKeys(t, postRuns(t, turn, configPostBodyHead+`77}`))

		if want := []string{"produced", "reply", "written"}; !slices.Equal(keys, want) {
			t.Errorf("config %s: body keys = %q, want %q", content, keys, want)
		}
		if reply := stringMember(t, members, "reply"); reply != "say "+silenceMarker || !boolMember(t, members, "produced") {
			t.Errorf("config %s: reply = %q, produced = %v, want the answer verbatim", content, reply, boolMember(t, members, "produced"))
		}
	}
}

func TestASilenceThatIsNotABooleanIsAnInvalidConfigNamingTheKeyAndNothingElseRuns(t *testing.T) {
	t.Parallel()

	for _, value := range []string{`"true"`, `1`, `null`, `{}`, `[]`} {
		graph := &configGraph{content: `{"silence": ` + value + `}`}

		result, model := runWithConfig(t, graph, configPostBodyHead+`77}`)

		if result.code != http.StatusBadRequest || result.errorCode(t) != "invalid_config" {
			t.Errorf("silence %s: status = %d code = %q, want 400 invalid_config", value, result.code, result.errorCode(t))
			continue
		}
		if msg := result.errorMessage(); !strings.Contains(msg, `"silence"`) {
			t.Errorf("silence %s: the message %q does not name the key", value, msg)
		}
		if len(graph.recallCalls) != 0 || model.calls() != 0 {
			t.Errorf("silence %s: the run went on: %d recalls, %d model calls", value, len(graph.recallCalls), model.calls())
		}
	}
}

func TestAFailedRunUnderSilenceKeepsItsErrorEnvelopeAndCarriesNoSilentKey(t *testing.T) {
	t.Parallel()

	failing := func(int) (loop.JudgeResult, error) { return loop.JudgeResult{}, errors.New("connection reset") }
	turn, _ := silenceTurn(`{"silence": true}`, failing)

	rec := postRuns(t, turn, configPostBodyHead+`77}`)

	if rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "silent") {
		t.Errorf("model failure: status = %d body = %s, want 502 and no silent key", rec.Code, rec.Body.String())
	}

	turn, graph := silenceTurn(`{"silence": true}`, answeringWith(silenceMarker))
	graph.recallErr = errors.New("graph down")
	rec = postRuns(t, turn, configPostBodyHead+`77}`)

	if rec.Code != http.StatusBadGateway || strings.Contains(rec.Body.String(), "silent") {
		t.Errorf("graph failure: status = %d body = %s, want 502 and no silent key", rec.Code, rec.Body.String())
	}
}
