package divoid

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/process"
)

func TestMemorizeRequiresExactStoredNodeAndLinksReadback(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"confirmed", "partial", "missing", "body", "type", "root", "link", "about"} {
		t.Run(mode, func(t *testing.T) {
			reads := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					if r.URL.Path == "/api/nodes" {
						_, _ = w.Write([]byte(`{"id":900}`))
						return
					}
					if mode == "partial" && strings.HasSuffix(r.URL.Path, "/links") {
						w.WriteHeader(500)
						return
					}
					_, _ = w.Write([]byte(`{}`))
					return
				}
				reads++
				if r.URL.Path == "/api/nodes/links" {
					edges := []process.Edge{{Source: 900, Target: 701}, {Source: 702, Target: 900}}
					if mode == "link" {
						edges = edges[1:]
					}
					if mode == "about" {
						edges = edges[:1]
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"result": edges})
					return
				}
				row := map[string]any{"id": 900, "type": "fixture-fact", "rootNodeId": 700, "content": "exact synthetic memory"}
				if mode == "body" {
					row["content"] = "other"
				}
				if mode == "type" {
					row["type"] = "other"
				}
				if mode == "root" {
					row["rootNodeId"] = 999
				}
				rows := []any{row}
				if mode == "missing" {
					rows = []any{}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"result": rows})
			}))
			defer srv.Close()
			result, err := NewClient(srv.URL, "", srv.Client(), nil).Memorize(context.Background(), process.MemoryRequest{Target: process.MemoryTarget{Root: 700, Anchor: 701, Type: "fixture-fact"}, Text: "exact synthetic memory", About: 702})
			if mode == "confirmed" {
				if err != nil || !result.Confirmed || result.ContentHash != process.Hash("exact synthetic memory") || result.NodeID != 900 || reads != 2 {
					t.Fatalf("unconfirmed success: %+v %v reads=%d", result, err, reads)
				}
			} else if err == nil || result.Confirmed {
				t.Fatalf("bad readback accepted: %+v %v", result, err)
			}
		})
	}
}

func TestProcessTemplatePublicationPreservesRawMarkdownBytesAndMediaType(t *testing.T) {
	t.Parallel()
	body := "# Template\n{{context.message}}\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual, _ := io.ReadAll(r.Body)
		if string(actual) != body || r.Header.Get("Content-Type") != "text/markdown; charset=utf-8" {
			t.Fatal("template publication changed raw bytes or MIME")
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	if err := NewClient(srv.URL, "", srv.Client(), nil).SetProcessTextContent(context.Background(), 811, body); err != nil {
		t.Fatal(err)
	}
}
