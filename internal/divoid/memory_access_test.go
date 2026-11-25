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

func TestMemorizeExplicitAccessIsCreatedAndConfirmedWithoutCoercion(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested string
		observed  string
		linkFails bool
		confirmed bool
	}{
		{"omitted", "", "", false, true},
		{"private_name", "0", `"None"`, false, true},
		{"private_number", "0", "0", false, true},
		{"private_escaped_name", "0", `"\u004eone"`, false, true},
		{"read", "1", `"Read"`, false, true},
		{"write", "2", `"Write"`, false, true},
		{"both", "3", `"Read, Write"`, false, true},
		{"both_number", "3", "3", false, true},
		{"mismatch", "0", `"Read, Write"`, false, false},
		{"missing", "0", "", false, false},
		{"null", "0", "null", false, false},
		{"unknown_name", "0", `"ReadWrite"`, false, false},
		{"unknown_bits", "0", "4", false, false},
		{"boolean", "0", "false", false, false},
		{"numeric_string", "0", `"0"`, false, false},
		{"unknown_combination", "0", `"Read, Delete"`, false, false},
		{"none_with_grant", "0", `"None, Read"`, false, false},
		{"partial_links", "0", `"None"`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			creates, mutations, nodeReads := 0, 0, 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					mutations++
					if r.URL.Path == "/api/nodes" {
						creates++
						body, _ := io.ReadAll(r.Body)
						want := `{"type":"fixture-fact","name":"exact synthetic memory","rootNodeId":700`
						if tc.requested != "" {
							want += `,"access":` + tc.requested
						}
						want += "}"
						if string(body) != want || mutations != 1 {
							t.Errorf("first create payload=%s want=%s mutation=%d", body, want, mutations)
						}
						_, _ = w.Write([]byte(`{"id":900}`))
						return
					}
					if tc.linkFails && strings.HasSuffix(r.URL.Path, "/links") {
						w.WriteHeader(http.StatusServiceUnavailable)
					}
					return
				}
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method %s", r.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				if r.URL.Path == "/api/nodes/links" {
					_, _ = w.Write([]byte(`{"result":[{"sourceId":900,"targetId":701}]}`))
					return
				}
				nodeReads++
				wantFields := nodeFields
				if tc.requested != "" {
					wantFields += ",access"
				}
				if r.URL.Query().Get("fields") != wantFields {
					t.Errorf("fields=%q want=%q", r.URL.Query().Get("fields"), wantFields)
				}
				row := `{"id":900,"type":"fixture-fact","rootNodeId":700,"content":"exact synthetic memory"`
				if tc.observed != "" {
					row += `,"access":` + tc.observed
				}
				_, _ = w.Write([]byte(`{"result":[` + row + `}]}`))
			}))
			defer srv.Close()
			wire := `{"root":700,"type":"fixture-fact","anchor":701`
			if tc.requested != "" {
				wire += `,"access":` + tc.requested
			}
			var target process.MemoryTarget
			if err := json.Unmarshal([]byte(wire+"}"), &target); err != nil {
				t.Fatal(err)
			}
			result, err := NewClient(srv.URL, "", srv.Client(), nil).Memorize(context.Background(), process.MemoryRequest{Target: target, Text: "exact synthetic memory"})
			if creates != 1 || mutations != 3 || result.NodeID != 900 || result.Confirmed != tc.confirmed {
				t.Fatalf("incorrect write/confirmation: %+v err=%v creates=%d mutations=%d", result, err, creates, mutations)
			}
			if tc.confirmed {
				if err != nil || result.State != "stored" || result.ContentHash != process.Hash("exact synthetic memory") {
					t.Fatalf("valid access unconfirmed: %+v %v", result, err)
				}
			} else if err == nil || result.ContentHash != "" {
				t.Fatalf("invalid access or partial write confirmed: %+v %v", result, err)
			}
			wantReads := 1
			if tc.linkFails {
				wantReads = 0
				if result.State != "unlinked" {
					t.Fatalf("partial allocation lost: %+v", result)
				}
			}
			if nodeReads != wantReads {
				t.Fatalf("readback retried or skipped: reads=%d want=%d", nodeReads, wantReads)
			}
		})
	}
}

func TestMemorizeRejectsInvalidRequestedAccessBeforeHTTP(t *testing.T) {
	for _, access := range []string{"-1", "4", "1.5", `"None"`, "true", "null"} {
		t.Run(access, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()
			var target process.MemoryTarget
			err := json.Unmarshal([]byte(`{"root":700,"type":"fixture-fact","anchor":701,"access":`+access+`}`), &target)
			if err == nil {
				result, writeErr := NewClient(srv.URL, "", srv.Client(), nil).Memorize(context.Background(), process.MemoryRequest{Target: target, Text: "synthetic memory"})
				if writeErr == nil || result.Confirmed || result.NodeID != 0 {
					t.Fatalf("invalid request accepted: %+v %v", result, writeErr)
				}
			}
			if calls != 0 {
				t.Fatalf("invalid access reached HTTP: calls=%d", calls)
			}
		})
	}
}
