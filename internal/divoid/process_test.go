package divoid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telmengedar/processor/internal/process"
)

func TestProcessLinksPaginateAndPreserveNativeDirectionAndContext(t *testing.T) {
	t.Parallel()
	calls := 0
	contextText := `{"kind":"process-transition", "default":true}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/nodes/links" || r.URL.Query().Get("ids") != "71" || r.URL.Query().Get("count") != "500" {
			t.Fatal(r.URL.String())
		}
		if calls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{"result": []process.Edge{{Source: 72, Target: 71, LinkType: "Unidirectional", Context: contextText}}, "continue": 500})
		} else {
			if r.URL.Query().Get("continue") != "500" {
				t.Fatal("cursor missing")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": []process.Edge{{Source: 71, Target: 73, LinkType: "Unidirectional", Context: contextText}}})
		}
	}))
	defer srv.Close()
	edges, err := NewClient(srv.URL, "", srv.Client(), nil).Links(context.Background(), 71)
	if err != nil || calls != 2 || len(edges) != 2 || edges[0].Source != 72 || edges[1].Source != 71 || edges[0].Context != contextText {
		t.Fatalf("edges=%+v err=%v", edges, err)
	}
}

func TestProcessNameLookupFiltersWildcardMatchesAcrossPages(t *testing.T) {
	t.Parallel()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("name") != "Chat_%" || r.URL.Query().Get("type") != "process" {
			t.Fatal("missing name/type")
		}
		if calls == 1 {
			_, _ = w.Write([]byte(`{"result":[{"id":1,"type":"process","name":"Chat noise"}],"continue":500}`))
		} else {
			_, _ = w.Write([]byte(`{"result":[{"id":9,"type":"process","name":"Chat_%"}]}`))
		}
	}))
	defer srv.Close()
	nodes, err := NewClient(srv.URL, "", srv.Client(), nil).FindNodes(context.Background(), "Chat_%", "process")
	if err != nil || len(nodes) != 1 || nodes[0].ID != 9 || calls != 2 {
		t.Fatalf("%+v %v", nodes, err)
	}
}

func TestProcessLinkPublicationSendsDirectedMetadataAndBareTarget(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var target int64
		if json.NewDecoder(r.Body).Decode(&target) != nil || target != 82 || r.URL.Path != "/api/nodes/81/links" || r.URL.Query().Get("linkType") != "Unidirectional" || r.URL.Query().Get("context") != `{"kind":"process-transition","default":true}` {
			t.Fatal("incorrect native link publication")
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	err := NewClient(srv.URL, "", srv.Client(), nil).LinkProcessNodes(context.Background(), process.Edge{Source: 81, Target: 82, LinkType: "Unidirectional", Context: `{"kind":"process-transition","default":true}`})
	if err != nil {
		t.Fatal(err)
	}
}
