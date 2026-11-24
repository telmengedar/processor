package process

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/telmengedar/processor/internal/completion"
)

func v2Messages(step Step, input any, args map[string]any, s *Snapshot, r *transportRuntime) ([]completion.Message, []Admission, []int64, error) {
	data := map[string]any{"input": input}
	admissions := make([]Admission, len(step.Context))
	values := make([]any, len(step.Context))
	required := make([]bool, len(step.Context))
	caps := make([]*capability, len(step.Context))
	paths, _ := templatePaths(step.Prompt)
	needed := map[string]bool{}
	for _, path := range paths {
		needed[strings.Split(path, ".")[0]] = true
	}
	var round []completion.Message
	for i, item := range step.Context {
		var err error
		if item.Node > 0 {
			values[i] = s.Nodes[item.Node].Content
		} else {
			values[i], err = r.resolve(item.Value, args)
		}
		required[i] = item.Required || needed[item.Key]
		admissions[i] = Admission{Key: item.Key, Node: item.Node, Hash: hashJSON(values[i])}
		if item.ToolResult {
			cap, ok := values[i].(*capability)
			if !ok || cap.used || cap.phase != "round" {
				err = failure("tool_not_allowed", "tool context requires an exported unconsumed round")
			} else {
				caps[i] = cap
				admissions[i].Hash = hashJSON(toolRoundMessages(cap.call))
			}
		}
		if err != nil {
			if required[i] {
				return nil, admissions, nil, failure("invalid_binding", "required context unavailable")
			}
			continue
		}
		if required[i] {
			admissions[i].Included = true
			if item.ToolResult {
				if len(round) > 0 {
					return nil, admissions, nil, failure("invalid_definition", "multiple native rounds in one context")
				}
				round = toolRoundMessages(caps[i].call)
				data[item.Key] = jsonValue(caps[i].call.Result)
			} else {
				data[item.Key] = values[i]
			}
		}
	}
	build := func() ([]completion.Message, int, error) {
		prompt, err := renderTemplate(step.Prompt, data)
		if err != nil {
			return nil, 0, failure("invalid_template", err.Error())
		}
		if !validContextText(data) {
			return nil, 0, failure("invalid_template", "invalid context Unicode")
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return nil, 0, failure("invalid_binding", "context cannot be encoded")
		}
		if !utf8.Valid(encoded) {
			return nil, 0, failure("invalid_template", "invalid context Unicode")
		}
		messages := append([]completion.Message{{Role: "system", Content: prompt}, {Role: "user", Content: string(encoded)}}, round...)
		wire, _ := json.Marshal(messages)
		return messages, len(wire) + transportBytes(r.data), nil
	}
	messages, size, err := build()
	if err != nil {
		return nil, admissions, nil, err
	}
	if size > MaxContextBytes {
		return nil, admissions, nil, failure("context_bound", "required rendered context and transport exceed bound")
	}
	for i, item := range step.Context {
		if required[i] {
			continue
		}
		if item.ToolResult {
			if caps[i] == nil || len(round) > 0 {
				continue
			}
			round = toolRoundMessages(caps[i].call)
			data[item.Key] = jsonValue(caps[i].call.Result)
		} else {
			if values[i] == nil {
				continue
			}
			data[item.Key] = values[i]
		}
		candidate, n, err := build()
		if err != nil {
			return nil, admissions, nil, err
		}
		if n > MaxContextBytes {
			if item.ToolResult {
				round = nil
				delete(data, item.Key)
			} else {
				delete(data, item.Key)
			}
			continue
		}
		admissions[i].Included = true
		messages = candidate
	}
	for i, cap := range caps {
		if cap != nil && admissions[i].Included {
			cap.used = true
		}
	}
	return messages, admissions, exposedNodeIDs(data), nil
}
