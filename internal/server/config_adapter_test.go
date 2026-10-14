package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/loop"
)

func graphAnswering(t *testing.T, recallStatus int) *divoid.Client {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case q.Get("query") != "" || len(q["queries"]) > 0:
			w.WriteHeader(recallStatus)
			_, _ = w.Write([]byte(`{"text":"refused"}`))
		case q.Get("id") == "77":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": []map[string]any{{"id": 77, "type": "processor-config", "name": "c", "content": `{"filter": {"type": ["thought"]}}`}}, "total": 1})
		default:
			_, _ = w.Write([]byte(`{"result":[{"id":42,"type":"knowledge","name":"s","content":"b","substance":"b"}],"total":1}`))
		}
	}))
	t.Cleanup(srv.Close)
	return divoid.NewClient(srv.URL, "k", srv.Client(), testLogger())
}

func TestOnlyA400Or414OnTheFirstConfiguredRecallIsAnInvalidConfigAndEveryOther4xxStaysAnUnavailableGraph(t *testing.T) {
	t.Parallel()

	cases := map[int]struct {
		status int
		code   string
	}{
		http.StatusBadRequest:          {http.StatusBadRequest, "invalid_config"},
		http.StatusRequestURITooLong:   {http.StatusBadRequest, "invalid_config"},
		http.StatusUnauthorized:        {http.StatusBadGateway, "graph_unavailable"},
		http.StatusForbidden:           {http.StatusBadGateway, "graph_unavailable"},
		http.StatusRequestTimeout:      {http.StatusBadGateway, "graph_unavailable"},
		http.StatusTooManyRequests:     {http.StatusBadGateway, "graph_unavailable"},
		http.StatusInternalServerError: {http.StatusBadGateway, "graph_unavailable"},
	}
	for answered, want := range cases {
		model := &countingModel{}
		turn := loop.NewTurn(graphAnswering(t, answered), model, nil, fixedSystem("system"), "test-model", testLogger())
		rec := postRuns(t, turn, configPostBodyHead+`77}`)
		result := &httpResult{code: rec.Code, body: rec.Body.String()}

		if result.code != want.status || result.errorCode(t) != want.code {
			t.Errorf("DiVoid %d on the first recall: status = %d code = %q, want %d %s; body=%s", answered, result.code, result.errorCode(t), want.status, want.code, result.body)
		}
		if model.judged != 0 {
			t.Errorf("DiVoid %d on the first recall: the model judged %d times, want none", answered, model.judged)
		}
	}
}
