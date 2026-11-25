package process

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/telmengedar/processor/internal/loop"
	"sort"
	"strings"
)

func Hash(data string) string { sum := sha256.Sum256([]byte(data)); return hex.EncodeToString(sum[:]) }

func hashJSON(v any) string { b, _ := json.Marshal(v); return Hash(string(b)) }

func TransitionHash(edges []Edge) string {
	rows := make([][]any, 0, len(edges))
	for _, edge := range edges {
		rows = append(rows, []any{edge.Source, edge.Target, edge.Context})
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a[0] != b[0] {
			return a[0].(int64) < b[0].(int64)
		}
		if a[1] != b[1] {
			return a[1].(int64) < b[1].(int64)
		}
		return a[2].(string) < b[2].(string)
	})
	return hashJSON(rows)
}

func transition(edge Edge) (Transition, bool, error) {
	var marker map[string]json.RawMessage
	if json.Unmarshal([]byte(edge.Context), &marker) != nil {
		if strings.Contains(edge.Context, "process-transition") {
			return Transition{}, true, fmt.Errorf("malformed transition")
		}
		return Transition{}, false, nil
	}
	var kind string
	_ = json.Unmarshal(marker["kind"], &kind)
	if kind != "process-transition" {
		return Transition{}, false, nil
	}
	var t Transition
	if err := decode(edge.Context, &t); err != nil {
		return t, true, err
	}
	if edge.LinkType != "Unidirectional" || edge.Source == edge.Target || t.Default == (t.When != nil) {
		return t, true, fmt.Errorf("invalid directed transition")
	}
	if t.When != nil {
		if t.When.Path == "" {
			return t, true, fmt.Errorf("empty predicate path")
		}
		for _, k := range strings.Split(t.When.Path, ".") {
			if !pathKey.MatchString(k) {
				return t, true, fmt.Errorf("invalid predicate path")
			}
		}
		if _, ok := marker["when"]; !ok {
			return t, true, fmt.Errorf("missing predicate")
		}
		var p map[string]json.RawMessage
		_ = json.Unmarshal(marker["when"], &p)
		if _, ok := p["equals"]; !ok {
			return t, true, fmt.Errorf("missing predicate value")
		}
	}
	return t, true, nil
}

func Load(ctx context.Context, g Graph, request Request) (*Snapshot, error) {
	id := request.ProcessID
	if (id > 0) == (strings.TrimSpace(request.Process) != "") || id < 0 {
		return nil, failure("invalid_request", "select exactly one process ID or exact name")
	}
	if request.Process != "" {
		nodes, err := g.FindNodes(ctx, request.Process, "process")
		if err != nil {
			return nil, failure("graph_unavailable", "process lookup failed")
		}
		if len(nodes) != 1 {
			return nil, failure("process_not_unique", "process name must resolve exactly once")
		}
		id = nodes[0].ID
	}
	root, found, err := g.Node(ctx, id, nil)
	if err != nil {
		return nil, failure("graph_unavailable", "process read failed")
	}
	if !found || root.Type != "process" {
		return nil, failure("invalid_definition", "process node missing or wrong type")
	}
	if len(root.Content) > MaxDefinitionBytes {
		return nil, failure("invalid_definition", "process body exceeds definition byte bound")
	}
	s := &Snapshot{ID: id, Name: root.Name, Hash: Hash(root.Content), Steps: map[int64]Step{}, Tools: map[int64]Tool{}, Nodes: map[int64]loop.Anchor{}, Hashes: map[int64]string{}, Edges: map[int64][]Edge{}}
	if err := decode(root.Content, &s.Definition); err != nil {
		return nil, failure("invalid_definition", "invalid process body")
	}
	d := s.Definition
	if (d.Version != 1 && d.Version != 2) || len(d.Members) == 0 || len(d.Members) > MaxMembers || len(d.Inputs) == 0 || len(d.Outputs) == 0 {
		return nil, failure("invalid_definition", "unsupported version or declaration bounds")
	}
	for _, decls := range []map[string]Declaration{d.Inputs, d.Outputs} {
		for key, decl := range decls {
			if !pathKey.MatchString(key) {
				return nil, failure("invalid_definition", "invalid argument key")
			}
			if decl.Type == "nodeRef" {
				if strings.TrimSpace(decl.NodeType) == "" {
					return nil, failure("invalid_definition", "node reference requires node type")
				}
			} else if decl.NodeType != "" || !(decl.Type == "string" || decl.Type == "number" || decl.Type == "integer" || decl.Type == "boolean" || decl.Type == "object" || decl.Type == "array") {
				return nil, failure("invalid_definition", "unsupported declaration")
			}
		}
	}
	total := len(root.Content)
	for _, member := range d.Members {
		if member.ID <= 0 || member.ID == id || s.Hashes[member.ID] != "" {
			return nil, failure("invalid_definition", "duplicate or invalid member")
		}
		node, ok, err := g.Node(ctx, member.ID, nil)
		if err != nil {
			return nil, failure("graph_unavailable", "member read failed")
		}
		if !ok || Hash(node.Content) != member.SHA256 {
			return nil, failure("definition_changed", "member content does not match publication")
		}
		total += len(node.Content)
		if total > MaxDefinitionBytes {
			return nil, failure("invalid_definition", "definition byte bound exceeded")
		}
		s.Nodes[member.ID] = node
		s.Hashes[member.ID] = member.SHA256
		switch node.Type {
		case "process-step":
			var step Step
			if decode(node.Content, &step) != nil || step.Version != d.Version || !pathKey.MatchString(step.Key) {
				return nil, failure("invalid_definition", "invalid step")
			}
			s.Steps[member.ID] = step
		case "process-tool":
			var tool Tool
			if decode(node.Content, &tool) != nil || tool.Version != d.Version || !pathKey.MatchString(tool.Name) || strings.TrimSpace(tool.Description) == "" || tool.Schema.Type != "object" || tool.Schema.check() != nil || (tool.Builtin == "") == (tool.Python == "") || d.Version == 2 && tool.Python != "" || d.Version == 1 && tool.Memory != nil {
				return nil, failure("invalid_definition", "invalid tool")
			}
			switch tool.Builtin {
			case "", "read_node", "recall", "select", "execute_selected_tool":
			case "memorize":
				if d.Version != 2 || tool.Memory == nil {
					return nil, failure("invalid_definition", "memorize requires v2 authored placement")
				}
			default:
				return nil, failure("invalid_definition", "unknown builtin")
			}
			s.Tools[member.ID] = tool
		case "process-prompt-template":
			if d.Version != 2 {
				return nil, failure("invalid_definition", "template requires v2")
			}
			if _, err := templatePaths(node.Content); err != nil {
				return nil, failure("invalid_definition", "invalid prompt template")
			}
		}
	}
	if _, ok := s.Steps[d.Entry]; !ok {
		return nil, failure("invalid_definition", "entry step missing")
	}
	keys := map[string]bool{}
	for _, step := range s.Steps {
		if keys[step.Key] {
			return nil, failure("invalid_definition", "duplicate step key")
		}
		keys[step.Key] = true
	}
	var edges []Edge
	pairs := map[[2]int64]bool{}
	if d.Version == 2 {
		refs, err := resolveReferences(ctx, g, s)
		if err != nil {
			return nil, err
		}
		edges = append(edges, refs...)
		for _, edge := range refs {
			pairs[[2]int64{min(edge.Source, edge.Target), max(edge.Source, edge.Target)}] = true
			total += len(edge.Context)
		}
		if len(edges) > MaxTransitions || total > MaxDefinitionBytes {
			return nil, failure("invalid_definition", "reference bounds exceeded")
		}
	}
	for id, step := range s.Steps {
		if err := validateStep(step, s, keys); err != nil {
			return nil, failure("invalid_definition", err.Error())
		}
		links, err := g.Links(ctx, id)
		if err != nil {
			return nil, failure("graph_unavailable", "transition read failed")
		}
		for _, edge := range links {
			_, relevant, err := transition(edge)
			if err != nil {
				return nil, failure("invalid_definition", err.Error())
			}
			if !relevant || edge.Source != id {
				continue
			}
			if _, ok := s.Steps[edge.Target]; !ok {
				return nil, failure("invalid_definition", "transition target outside member steps")
			}
			pair := [2]int64{min(edge.Source, edge.Target), max(edge.Source, edge.Target)}
			if pairs[pair] {
				return nil, failure("invalid_definition", "parallel or opposing transitions")
			}
			pairs[pair] = true
			edges = append(edges, edge)
			s.Edges[id] = append(s.Edges[id], edge)
			total += len(edge.Context)
			if len(edges) > MaxTransitions || total > MaxDefinitionBytes {
				return nil, failure("invalid_definition", "transition bounds exceeded")
			}
		}
	}
	if TransitionHash(edges) != d.TransitionsSHA256 {
		return nil, failure("definition_changed", "transition digest differs")
	}
	for id, step := range s.Steps {
		defaults := 0
		for _, edge := range s.Edges[id] {
			t, _, _ := transition(edge)
			if t.Default {
				defaults++
			}
		}
		if defaults > 1 || step.Kind == "return" && len(s.Edges[id]) > 0 || step.Kind != "return" && len(s.Edges[id]) == 0 {
			return nil, failure("invalid_definition", "invalid outgoing transitions")
		}
	}
	seen := map[int64]bool{}
	var walk func(int64)
	walk = func(id int64) {
		if seen[id] {
			return
		}
		seen[id] = true
		for _, edge := range s.Edges[id] {
			walk(edge.Target)
		}
	}
	walk(d.Entry)
	for id := range s.Steps {
		if !seen[id] {
			return nil, failure("invalid_definition", "unreachable member step")
		}
	}
	return s, nil
}

func validateStep(step Step, s *Snapshot, keys map[string]bool) error {
	if s.Definition.Version == 2 {
		return validateStepV2(step, s)
	}
	if step.Exports != nil {
		return fmt.Errorf("v1 does not support exports")
	}
	for _, v := range []any{step.Input, step.Output} {
		if err := checkBinding(v, s.Definition.Inputs, keys); err != nil {
			return err
		}
	}
	contextKeys := map[string]bool{"input": true}
	for _, c := range step.Context {
		if contextKeys[c.Key] {
			return fmt.Errorf("duplicate or reserved context key")
		}
		contextKeys[c.Key] = true
		if !pathKey.MatchString(c.Key) || c.Node > 0 && c.Value != nil {
			return fmt.Errorf("invalid context binding")
		}
		if c.ToolResult && c.Node > 0 {
			return fmt.Errorf("tool round must bind observed state")
		}
		if c.Node > 0 {
			if _, ok := s.Nodes[c.Node]; !ok {
				return fmt.Errorf("undeclared context node")
			}
		} else if err := checkBinding(c.Value, s.Definition.Inputs, keys); err != nil {
			return err
		}
	}
	switch step.Kind {
	case "action":
		if step.Tool <= 0 || step.Prompt != "" || step.Output != nil || len(step.Tools) > 0 || len(step.Context) > 0 {
			return fmt.Errorf("invalid action step")
		}
		if _, ok := s.Tools[step.Tool]; !ok {
			return fmt.Errorf("missing action tool")
		}
	case "judgement":
		if strings.TrimSpace(step.Prompt) == "" || step.Tool != 0 || step.Output != nil {
			return fmt.Errorf("invalid judgement step")
		}
		names := map[string]bool{}
		for _, id := range step.Tools {
			tool, ok := s.Tools[id]
			if !ok || tool.Builtin == "execute_selected_tool" || names[tool.Name] {
				return fmt.Errorf("invalid judgement allowlist")
			}
			names[tool.Name] = true
		}
	case "return":
		if step.Output == nil || step.Tool != 0 || step.Prompt != "" || len(step.Tools) > 0 || step.Input != nil || len(step.Context) > 0 {
			return fmt.Errorf("invalid return step")
		}
	default:
		return fmt.Errorf("unsupported step kind")
	}
	return nil
}
