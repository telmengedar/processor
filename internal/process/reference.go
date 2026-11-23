package process

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Reference struct {
	Kind string `json:"kind"`
	Role string `json:"role"`
}

func referenceEdge(edge Edge) (Reference, bool, error) {
	var marker map[string]json.RawMessage
	if json.Unmarshal([]byte(edge.Context), &marker) != nil {
		if strings.Contains(edge.Context, "process-reference") {
			return Reference{}, true, fmt.Errorf("malformed reference")
		}
		return Reference{}, false, nil
	}
	var kind string
	_ = json.Unmarshal(marker["kind"], &kind)
	if kind != "process-reference" {
		return Reference{}, false, nil
	}
	var ref Reference
	if decode(edge.Context, &ref) != nil || edge.LinkType != "Unidirectional" || edge.Source == edge.Target || ref.Role != "prompt" && ref.Role != "tool" {
		return ref, true, fmt.Errorf("invalid directed reference")
	}
	return ref, true, nil
}

func resolveReferences(ctx context.Context, g Graph, s *Snapshot) ([]Edge, error) {
	refs := []Edge{}
	pairs := map[[2]int64]bool{}
	for id, step := range s.Steps {
		if step.Prompt != "" || step.Tool != 0 || len(step.Tools) > 0 {
			return nil, failure("invalid_definition", "v2 capabilities and prompts require inbound references")
		}
		links, err := g.Links(ctx, id)
		if err != nil {
			return nil, failure("graph_unavailable", "reference read failed")
		}
		prompts := 0
		for _, edge := range links {
			ref, relevant, err := referenceEdge(edge)
			if err != nil {
				return nil, failure("invalid_definition", err.Error())
			}
			if !relevant {
				continue
			}
			if edge.Target != id {
				return nil, failure("invalid_definition", "reference must point into step")
			}
			node, ok := s.Nodes[edge.Source]
			if !ok {
				return nil, failure("invalid_definition", "reference outside members")
			}
			pair := [2]int64{min(edge.Source, edge.Target), max(edge.Source, edge.Target)}
			if pairs[pair] {
				return nil, failure("invalid_definition", "repeated reference pair")
			}
			pairs[pair] = true
			switch ref.Role {
			case "prompt":
				if node.Type != "process-prompt-template" || step.Kind != "judgement" {
					return nil, failure("invalid_definition", "wrong prompt reference type")
				}
				prompts++
				step.Prompt = node.Content
				step.templateID = edge.Source
			case "tool":
				if node.Type != "process-tool" || step.Kind == "return" {
					return nil, failure("invalid_definition", "wrong tool reference type")
				}
				if step.Kind == "action" {
					if step.Tool != 0 {
						return nil, failure("invalid_definition", "action requires exactly one tool")
					}
					step.Tool = edge.Source
				} else {
					step.Tools = append(step.Tools, edge.Source)
				}
			}
			refs = append(refs, edge)
		}
		if step.Kind == "judgement" && prompts != 1 {
			return nil, failure("invalid_definition", "judgement requires exactly one prompt")
		}
		sort.Slice(step.Tools, func(i, j int) bool { return step.Tools[i] < step.Tools[j] })
		s.Steps[id] = step
	}
	for _, tool := range s.Tools {
		if tool.Builtin == "memorize" {
			m := tool.Memory
			if m == nil || m.Root <= 0 || m.Anchor <= 0 || strings.TrimSpace(m.Type) == "" {
				return nil, failure("invalid_definition", "invalid memory target")
			}
			if _, ok := s.Nodes[m.Root]; !ok {
				return nil, failure("invalid_definition", "memory root not declared")
			}
			if _, ok := s.Nodes[m.Anchor]; !ok {
				return nil, failure("invalid_definition", "memory anchor not declared")
			}
		} else if tool.Memory != nil {
			return nil, failure("invalid_definition", "memory placement on unrelated tool")
		}
	}
	return refs, nil
}

func validateStepV2(step Step, s *Snapshot) error {
	if step.Version != 2 {
		return fmt.Errorf("v2 requires v2 step")
	}
	for _, v := range []any{step.Input, step.Output} {
		if err := checkV2Binding(v, s.Definition.Inputs, false); err != nil {
			return err
		}
	}
	if step.Exports != nil {
		exports, ok := step.Exports.(map[string]any)
		if !ok {
			return fmt.Errorf("exports must be an object")
		}
		if _, ok := exports["receipts"]; ok {
			return fmt.Errorf("receipts is runtime reserved")
		}
		if err := checkV2Binding(exports, s.Definition.Inputs, true); err != nil {
			return err
		}
	}
	contextKeys := map[string]bool{"input": true}
	for _, item := range step.Context {
		if contextKeys[item.Key] || !pathKey.MatchString(item.Key) {
			return fmt.Errorf("invalid context key")
		}
		contextKeys[item.Key] = true
		if item.Node > 0 {
			if item.Value != nil || item.ToolResult {
				return fmt.Errorf("invalid context node")
			}
			if _, ok := s.Nodes[item.Node]; !ok {
				return fmt.Errorf("undeclared context node")
			}
		} else if err := checkV2Binding(item.Value, s.Definition.Inputs, false); err != nil {
			return err
		}
	}
	switch step.Kind {
	case "action":
		if step.Tool <= 0 || step.Prompt != "" || step.Output != nil || len(step.Tools) > 0 || len(step.Context) > 0 {
			return fmt.Errorf("invalid action")
		}
	case "judgement":
		if step.Prompt == "" || step.Tool != 0 || step.Output != nil {
			return fmt.Errorf("invalid judgement")
		}
		paths, err := templatePaths(step.Prompt)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if !contextKeys[strings.Split(path, ".")[0]] {
				return fmt.Errorf("unknown template context")
			}
		}
		names := map[string]bool{}
		for _, id := range step.Tools {
			tool := s.Tools[id]
			if tool.Builtin == "execute_selected_tool" || names[tool.Name] {
				return fmt.Errorf("invalid judgement tools")
			}
			names[tool.Name] = true
		}
	case "return":
		if step.Output == nil || step.Tool != 0 || step.Prompt != "" || len(step.Tools) > 0 || step.Input != nil || len(step.Context) > 0 || step.Exports != nil {
			return fmt.Errorf("invalid return")
		}
	default:
		return fmt.Errorf("unsupported step")
	}
	return nil
}
