package divoid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
)

func TestDecisionNoteScopeProofRequiresCanonicalSingletonLinkedTo(t *testing.T) {
	t.Parallel()
	for name, filter := range map[string]loop.Filter{
		"canonical":              {"linkedto": {"1090"}},
		"additional AND filters": {"linkedto": {"1090"}, "path": {"[id:3]/[]"}, "type": {"thought"}, "rootNodeId": {"902"}},
		"absent":                 nil, "empty": {"linkedto": {}}, "multi": {"linkedto": {"1090", "7"}}, "comma": {"linkedto": {"1090,7"}}, "wrong": {"linkedto": {"7"}}, "space": {"linkedto": {" 1090"}}, "plus": {"linkedto": {"+1090"}}, "leading zero": {"linkedto": {"01090"}}, "alternate key": {"linkedTo": {"1090"}}, "case": {"LINKEDTO": {"1090"}}, "path only": {"path": {"[id:1090]/[]"}},
	} {
		t.Run(name, func(t *testing.T) {
			eligible := name == "canonical" || name == "additional AND filters"
			var got url.Values
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				got = r.URL.Query()
				_, _ = w.Write([]byte(`{"result":[{"id":71,"type":"thought","name":"exact","similarity":0.91,"content":"full","substance":"short","rootNodeId":902}],"total":1}`))
			}))
			defer server.Close()
			client := NewClient(server.URL, "key", server.Client(), testLogger())
			rows, err := client.Recall(context.Background(), loop.RecallRequest{Queries: []string{"verbatim, query"}, Limit: 6, Exclude: []int64{1090}, Filter: filter, NoteContextSubject: 1090})
			if err != nil || calls != 1 || len(rows) != 1 {
				t.Fatalf("recall=%+v err=%v calls=%d", rows, err, calls)
			}
			fields := "id,type,name,similarity,content,substance"
			root, subject := int64(0), int64(0)
			if eligible {
				fields += ",rootNodeId"
				root = 902
				subject = 1090
			}
			want := url.Values{"query": {"verbatim, query"}, "count": {"6"}, "excludeId": {"1090"}, "excludeName": {"processor-run%"}, "fields": {fields}}
			for k, v := range filter {
				if len(v) == 0 {
					continue
				}
				want[k] = v
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("query changed: got=%v want=%v", got, want)
			}
			if rows[0].RootNodeID != root || rows[0].DirectSubjectID != subject || rows[0].Content != "full" || rows[0].Substance != "short" || rows[0].Similarity != 0.91 {
				t.Fatalf("evidence/payload=%+v", rows[0])
			}
		})
	}
}

func TestDecisionNoteProjectionAddsOnlyRootScalarAndNeverAdjacency(t *testing.T) {
	t.Parallel()
	for _, subject := range []int64{0, -1, 1090} {
		client, rec := recordingClient(t)
		_, err := client.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 2, Filter: loop.Filter{"linkedto": {"1090"}}, NoteContextSubject: subject})
		fields := "id,type,name,similarity,content,substance"
		if subject > 0 {
			fields += ",rootNodeId"
		}
		if err != nil || rec.requests != 1 || queryOf(t, rec.query).Get("fields") != fields {
			t.Fatalf("subject=%d err=%v request=%+v", subject, err, rec)
		}
	}
}

func TestDecisionScopeRootProofUsesOnlyReturnedScalar(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":1,"rootNodeId":902},{"id":2,"rootNodeId":null},{"id":3},{"id":4,"rootNodeId":-1}],"total":4}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, "key", server.Client(), testLogger())
	rows, err := client.Recall(context.Background(), loop.RecallRequest{Queries: []string{"q"}, Limit: 4, NoteContextSubject: 1090, Filter: loop.Filter{"linkedto": {"1090"}, "rootNodeId": {"902"}}})
	if err != nil || len(rows) != 4 {
		t.Fatalf("recall=%+v %v", rows, err)
	}
	for i, want := range []int64{902, 0, 0, -1} {
		if rows[i].RootNodeID != want || rows[i].DirectSubjectID != 1090 {
			t.Fatalf("row%d=%+v", i, rows[i])
		}
	}
}

type exactCopyModel struct {
	calls []loop.JudgeInput
}

func (m *exactCopyModel) Judge(_ context.Context, input loop.JudgeInput) (loop.JudgeResult, error) {
	m.calls = append(m.calls, input)
	return loop.JudgeResult{Answer: `{"action":"write","text":"nora seems genuinely dedicated to community work, like the bicycle repairs.","about":71}`, Reason: loop.Answered}, nil
}

func (*exactCopyModel) Derive(context.Context, string, int) (loop.DeriveResult, error) {
	return loop.DeriveResult{}, nil
}

func TestAdapterScopeProofReachesTheAdmittedCopyRefusal(t *testing.T) {
	t.Parallel()
	var requests []url.Values
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes++
			t.Errorf("unexpected storage request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		q := r.URL.Query()
		requests = append(requests, q)
		if q.Get("id") == "1090" {
			_, _ = w.Write([]byte(`{"result":[{"id":1090,"type":"identity","content":"anchor","substance":"anchor","rootNodeId":902}],"total":1}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":[{"id":71,"type":"thought","name":"Nora","similarity":0.93,"content":"nora seems genuinely dedicated to community work, like the bicycle repairs.","substance":"community work","rootNodeId":902}],"total":1}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, "key", server.Client(), testLogger())
	model := &exactCopyModel{}
	turn := loop.NewTurn(client, model, nil, nil, "fake", testLogger())
	turn.Notes = client
	record, _, err := turn.Answer(context.Background(), loop.RunOptions{Input: "review", Subject: 1090, Settings: loop.DefaultSettings(), View: &loop.View{Root: 902, Form: loop.FormContent, Note: &loop.NoteConfig{Type: "thought", Mode: loop.NoteModeDecision}, Filter: loop.Filter{"linkedto": {"1090"}}}})
	if err != nil || len(requests) != 2 || requests[1].Get("linkedto") != "1090" || requests[1].Get("fields") != "id,type,name,similarity,content,substance,rootNodeId" || len(model.calls) != 1 || len(record.Candidates) != 1 || !record.Candidates[0].Included {
		t.Fatalf("missing admitted actual-adapter evidence: err=%v requests=%v record=%+v", err, requests, record)
	}
	if writes != 0 || record.NoteDecision == nil || record.NoteDecision.State != "rejected" || record.NoteDecision.Created != 0 || len(record.ToolCalls) != 1 || record.ToolCalls[0].Source != "structured" || record.ToolCalls[0].Error != "that exact note is already supplied for this subject: nothing was stored" || record.Answer != "" || record.Outcome.Produced {
		t.Fatalf("false refusal: writes=%d record=%+v", writes, record)
	}
}
