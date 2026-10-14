package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

const (
	configNodeID       = 77
	wrongTypeConfigID  = 88
	missingConfigID    = 99
	configSubjectID    = 42
	configPostBodyHead = `{"input":"hello","subject":42,"config":`
)

type configNodeCall struct {
	id     int64
	filter loop.Filter
}

type configGraph struct {
	content     string
	configErr   error
	recallErr   error
	nodeCalls   []configNodeCall
	recallCalls []loop.RecallRequest

	configRoot  int64
	subjectRoot int64
	written     []loop.Record
}

func (g *configGraph) Node(_ context.Context, id int64, filter loop.Filter) (loop.Anchor, bool, error) {
	g.nodeCalls = append(g.nodeCalls, configNodeCall{id: id, filter: filter})

	onConfigType := reflect.DeepEqual(filter["type"], []string{"processor-config"})
	switch id {
	case configNodeID:
		if g.configErr != nil {
			return loop.Anchor{}, false, g.configErr
		}
		if !onConfigType {
			return loop.Anchor{}, false, nil
		}
		return loop.Anchor{ID: id, Type: "processor-config", Name: "gangolf", Content: g.content, RootNodeID: g.configRoot}, true, nil
	case wrongTypeConfigID:
		if onConfigType {
			return loop.Anchor{}, false, nil
		}
		return loop.Anchor{ID: id, Type: "documentation", Name: "not a config", Content: `{"filter": {"type": ["thought"]}}`}, true, nil
	case configSubjectID:
		return loop.Anchor{ID: id, Type: "knowledge", Name: "Subject", Substance: "anchor body", RootNodeID: g.subjectRoot}, true, nil
	default:
		return loop.Anchor{}, false, nil
	}
}

func (g *configGraph) Recall(_ context.Context, request loop.RecallRequest) ([]loop.Candidate, error) {
	g.recallCalls = append(g.recallCalls, request)
	return nil, g.recallErr
}

func (g *configGraph) WriteRun(_ context.Context, record loop.Record) loop.WriteReceipt {
	g.written = append(g.written, record)
	return loop.WriteReceipt{State: loop.Stored, NodeID: 5}
}

func runWithConfig(t *testing.T, graph *configGraph, body string) (*httpResult, *countingModel) {
	t.Helper()

	model := &countingModel{}
	turn := loop.NewTurn(graph, model, nil, fixedSystem("system text"), "test-model", testLogger())
	rec := postRunsVerbose(t, turn, body)
	return &httpResult{code: rec.Code, body: rec.Body.String()}, model
}

type countingModel struct {
	judged  int
	derived int
}

func (m *countingModel) calls() int { return m.judged + m.derived }

func (m *countingModel) Derive(context.Context, string, int) (string, error) {
	m.derived++
	return "", nil
}

func (m *countingModel) Judge(context.Context, loop.JudgeInput) (loop.JudgeResult, error) {
	m.judged++
	return loop.JudgeResult{Reason: loop.Answered, RawReason: "stop", Answer: "an answer"}, nil
}

type httpResult struct {
	code int
	body string
}

func (r *httpResult) errorCode(t *testing.T) string {
	t.Helper()

	var envelope errorEnvelope
	if err := json.Unmarshal([]byte(r.body), &envelope); err != nil {
		t.Fatalf("the body %q is not an error envelope: %v", r.body, err)
	}
	return envelope.Error.Code
}

func (r *httpResult) errorMessage() string {
	var envelope errorEnvelope
	_ = json.Unmarshal([]byte(r.body), &envelope)
	return envelope.Error.Message
}

func TestAConfiguredRunReadsTheConfigUnderAFilterOnItsOwnTypeAndHoldsEveryRecallToTheFilterItCarries(t *testing.T) {
	t.Parallel()

	graph := &configGraph{content: `{"filter": {"type": ["thought", "knowledge"], "nostatus": true}}`}
	result, _ := runWithConfig(t, graph, configPostBodyHead+`77}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", result.code, result.body)
	}

	if len(graph.nodeCalls) < 1 || graph.nodeCalls[0].id != configNodeID {
		t.Fatalf("the first graph read was %+v, want the config's", graph.nodeCalls)
	}
	if got, want := graph.nodeCalls[0].filter, (loop.Filter{"type": {"processor-config"}}); !reflect.DeepEqual(got, want) {
		t.Errorf("the config was read under %v, want %v", got, want)
	}

	want := loop.Filter{"type": {"thought", "knowledge"}, "nostatus": {"true"}}
	if len(graph.recallCalls) == 0 {
		t.Fatal("the run made no recall")
	}
	for i, call := range graph.recallCalls {
		if !reflect.DeepEqual(call.Filter, want) {
			t.Errorf("recall %d carried %v, want %v", i, call.Filter, want)
		}
	}

	var record loop.Record
	if err := json.Unmarshal([]byte(result.body), &record); err != nil {
		t.Fatalf("the verbose body is not a record: %v", err)
	}
	if !reflect.DeepEqual(record.Perspective, &loop.View{Config: configNodeID, Filter: want}) {
		t.Errorf("the record carries the perspective %+v, want the config id and the forwarded filter", record.Perspective)
	}
}

func TestARunWithoutAConfigReadsNoConfigAndSendsNoFilter(t *testing.T) {
	t.Parallel()

	graph := &configGraph{}
	result, _ := runWithConfig(t, graph, `{"input":"hello","subject":42}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", result.code, result.body)
	}
	if len(graph.nodeCalls) != 1 || graph.nodeCalls[0].id != configSubjectID || graph.nodeCalls[0].filter != nil {
		t.Errorf("the graph reads were %+v, want the subject's alone and unfiltered", graph.nodeCalls)
	}
	for i, call := range graph.recallCalls {
		if call.Filter != nil {
			t.Errorf("recall %d carried %v, want no filter", i, call.Filter)
		}
	}
	if strings.Contains(result.body, "perspective") {
		t.Errorf("the record of a run without a config carries a perspective: %s", result.body)
	}
}

func TestAConfigWithoutAFilterRunsUnfilteredAndStillRecordsTheConfigId(t *testing.T) {
	t.Parallel()

	graph := &configGraph{content: `{}`}
	result, _ := runWithConfig(t, graph, configPostBodyHead+`77}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", result.code, result.body)
	}
	for i, call := range graph.recallCalls {
		if call.Filter != nil {
			t.Errorf("recall %d carried %v, want no filter", i, call.Filter)
		}
	}
	var record loop.Record
	if err := json.Unmarshal([]byte(result.body), &record); err != nil {
		t.Fatalf("the verbose body is not a record: %v", err)
	}
	if record.Perspective == nil || record.Perspective.Config != configNodeID {
		t.Errorf("the record carries %+v, want the config id", record.Perspective)
	}
}

func TestAConfigThatIsMissingOrOfAnotherTypeIsNotFoundAndNothingElseRuns(t *testing.T) {
	t.Parallel()

	for name, id := range map[string]int{"a missing id": missingConfigID, "a node of another type holding JSON": wrongTypeConfigID} {
		graph := &configGraph{}
		result, model := runWithConfig(t, graph, fmt.Sprintf("%s%d}", configPostBodyHead, id))

		if result.code != http.StatusNotFound || result.errorCode(t) != "config_not_found" {
			t.Errorf("%s: status = %d code = %q, want 404 config_not_found; body=%s", name, result.code, result.errorCode(t), result.body)
		}
		if len(graph.recallCalls) != 0 || model.calls() != 0 {
			t.Errorf("%s: the run went on: %d recalls, %d model calls", name, len(graph.recallCalls), model.calls())
		}
		if len(graph.nodeCalls) != 1 {
			t.Errorf("%s: the graph was read %d times, want the config read alone", name, len(graph.nodeCalls))
		}
	}
}

func TestAMalformedConfigIsAnInvalidConfigNamingTheFirstViolationAndNothingElseRuns(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		content string
		names   string
	}{
		"not json":               {`this is prose`, ""},
		"an unknown top key":     {`{"filtr": {"type": ["thought"]}}`, "filtr"},
		"an empty filter":        {`{"filter": {}}`, "filter"},
		"an owned key":           {`{"filter": {"Count": 100}}`, "Count"},
		"a misspelt name":        {`{"filter": {"tpye": ["thought"]}}`, "tpye"},
		"a wrongly cased name":   {`{"filter": {"Type": ["thought"]}}`, "Type"},
		"an empty array":         {`{"filter": {"type": []}}`, "type"},
		"a blank value":          {`{"filter": {"type": ""}}`, "type"},
		"a blank element":        {`{"filter": {"type": ["thought", " "]}}`, "type"},
		"an object value":        {`{"filter": {"type": {"a": 1}}}`, "type"},
		"a null value":           {`{"filter": {"type": null}}`, "type"},
		"an id that would widen": {`{"filter": {"id": [5]}}`, "id"},
	}
	for name, c := range cases {
		graph := &configGraph{content: c.content}
		result, model := runWithConfig(t, graph, configPostBodyHead+`77}`)

		if result.code != http.StatusBadRequest || result.errorCode(t) != "invalid_config" {
			t.Errorf("%s: status = %d code = %q, want 400 invalid_config; body=%s", name, result.code, result.errorCode(t), result.body)
			continue
		}
		if c.names != "" && !strings.Contains(result.errorMessage(), c.names) {
			t.Errorf("%s: the message %q does not name %q", name, result.errorMessage(), c.names)
		}
		if len(graph.recallCalls) != 0 || model.calls() != 0 || len(graph.nodeCalls) != 1 {
			t.Errorf("%s: the run went on: %d recalls, %d model calls, %d graph reads", name, len(graph.recallCalls), model.calls(), len(graph.nodeCalls))
		}
	}
}

func TestAConfigReadThatFailsInTransportIsGraphUnavailableAndNothingElseRuns(t *testing.T) {
	t.Parallel()

	graph := &configGraph{configErr: errors.New("connection refused")}
	result, model := runWithConfig(t, graph, configPostBodyHead+`77}`)

	if result.code != http.StatusBadGateway || result.errorCode(t) != "graph_unavailable" {
		t.Fatalf("status = %d code = %q, want 502 graph_unavailable; body=%s", result.code, result.errorCode(t), result.body)
	}
	if len(graph.recallCalls) != 0 || model.calls() != 0 {
		t.Fatalf("the run went on: %d recalls, %d model calls", len(graph.recallCalls), model.calls())
	}
}

func TestAFilterTheGraphRejectsOnTheFirstRecallIsAnInvalidConfigCarryingTheGraphsWords(t *testing.T) {
	t.Parallel()

	graph := &configGraph{
		content:   `{"filter": {"path": "[id:"}}`,
		recallErr: fmt.Errorf("divoid: unexpected status 400: the path could not be parsed: %w", loop.ErrGraphRejected),
	}
	result, model := runWithConfig(t, graph, configPostBodyHead+`77}`)

	if result.code != http.StatusBadRequest || result.errorCode(t) != "invalid_config" {
		t.Fatalf("status = %d code = %q, want 400 invalid_config; body=%s", result.code, result.errorCode(t), result.body)
	}
	if !strings.Contains(result.errorMessage(), "the path could not be parsed") {
		t.Errorf("the message %q does not carry the graph's own words", result.errorMessage())
	}
	if model.judged != 0 {
		t.Errorf("the model judged %d times, want none", model.judged)
	}
}

func TestAFirstRecallThatFailsWithoutBeingARejectionStaysGraphUnavailableUnderAConfig(t *testing.T) {
	t.Parallel()

	graph := &configGraph{content: `{"filter": {"type": ["thought"]}}`, recallErr: errors.New("divoid: unexpected status 503: down")}
	result, _ := runWithConfig(t, graph, configPostBodyHead+`77}`)

	if result.code != http.StatusBadGateway || result.errorCode(t) != "graph_unavailable" {
		t.Fatalf("status = %d code = %q, want 502 graph_unavailable; body=%s", result.code, result.errorCode(t), result.body)
	}
}

func TestAConfigThatIsNotAPositiveNodeIdIsAnInvalidRequest(t *testing.T) {
	t.Parallel()

	for _, body := range []string{configPostBodyHead + `0}`, configPostBodyHead + `-3}`, configPostBodyHead + `"77"}`, configPostBodyHead + `null}`} {
		graph := &configGraph{content: `{}`}
		result, _ := runWithConfig(t, graph, body)

		wantOK := strings.HasSuffix(body, "null}")
		switch {
		case wantOK && result.code != http.StatusOK:
			t.Errorf("%s: status = %d, want 200: a null config is no config; body=%s", body, result.code, result.body)
		case !wantOK && (result.code != http.StatusBadRequest || result.errorCode(t) != "invalid_request"):
			t.Errorf("%s: status = %d code = %q, want 400 invalid_request; body=%s", body, result.code, result.errorCode(t), result.body)
		}
		if !wantOK && len(graph.nodeCalls) != 0 {
			t.Errorf("%s: the graph was read %d times, want none", body, len(graph.nodeCalls))
		}
	}
}

func TestARunUnderAConfigRootedAtANodeRecordsThatRootInItsPerspective(t *testing.T) {
	t.Parallel()

	graph := &configGraph{content: `{}`, configRoot: 9001, subjectRoot: 31}
	result, _ := runWithConfig(t, graph, configPostBodyHead+`77}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", result.code, result.body)
	}
	if len(graph.written) != 1 || graph.written[0].Perspective == nil || graph.written[0].Perspective.Root != 9001 {
		t.Fatalf("the record handed to the write was %+v, want the perspective rooted at the config root", graph.written)
	}
}

func TestARunWithoutAConfigWritesItsRecordUnrootedWhateverRootItsSubjectHas(t *testing.T) {
	t.Parallel()

	graph := &configGraph{subjectRoot: 31}
	result, _ := runWithConfig(t, graph, `{"input":"hello","subject":42}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", result.code, result.body)
	}
	if len(graph.written) != 1 || graph.written[0].Perspective != nil {
		t.Fatalf("the record handed to the write was %+v, want no perspective and so no root", graph.written)
	}
}

func TestARunUnderAConfigWithoutARootIgnoresTheRootOfItsSubject(t *testing.T) {
	t.Parallel()

	graph := &configGraph{content: `{}`, subjectRoot: 31}
	result, _ := runWithConfig(t, graph, configPostBodyHead+`77}`)

	if result.code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", result.code, result.body)
	}
	if len(graph.written) != 1 || graph.written[0].Perspective == nil || graph.written[0].Perspective.Root != 0 {
		t.Fatalf("the record handed to the write was %+v, want a perspective with no root", graph.written)
	}
}
