package process

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/completion"
	"github.com/telmengedar/processor/internal/loop"
)

func (e *Engine) Run(ctx context.Context, request Request) Response {
	response := Response{Status: "failed", Trace: []Trace{}}
	s, err := Load(ctx, e.Graph, request)
	if err != nil {
		response.Error = asError(err, "invalid_definition")
		return response
	}
	response.Process = Identity{ID: s.ID, Name: s.Name, Hash: s.Hash}
	if err = e.arguments(ctx, s.Definition.Inputs, request.Arguments, false); err != nil {
		response.Error = asError(err, "invalid_arguments")
		return response
	}
	state := map[string]any{}
	var transport *transportRuntime
	if s.Definition.Version == 2 {
		transport = &transportRuntime{data: map[string]any{}}
	}
	current := s.Definition.Entry
	calls, dispatches := 0, 0
	var pending *selectedCall
	var lastRound *selectedCall
	var roundValue any
	var previous int64
	for visit := 0; visit < MaxVisits; visit++ {
		if ctx.Err() != nil {
			response.Error = failure("process_cancelled", "process deadline or cancellation reached")
			return response
		}
		step := s.Steps[current]
		if transport != nil {
			transport.step = current
			transport.visit = visit + 1
		}
		started := time.Now()
		trace := Trace{Ordinal: visit + 1, StepID: current, Key: step.Key, Kind: step.Kind, Hash: s.Hashes[current], Context: []Admission{}, Offered: append([]int64{}, step.Tools...)}
		if step.Kind == "action" {
			trace.Offered = []int64{step.Tool}
		}
		resolve := func(value any) (any, error) { return bind(value, request.Arguments, state) }
		if transport != nil {
			resolve = func(value any) (any, error) { return transport.resolve(value, request.Arguments) }
			trace.TemplateHash = s.Hashes[step.templateID]
			trace.TransportHash = hashJSON(transport.data)
		}
		input, bindErr := resolve(step.Input)
		trace.InputHash = hashJSON(input)
		var value any
		if bindErr != nil {
			err = failure("invalid_binding", bindErr.Error())
		} else {
			switch step.Kind {
			case "action":
				args, ok := input.(map[string]any)
				if !ok {
					err = failure("tool_input", "action input must be an object")
					break
				}
				tool := s.Tools[step.Tool]
				checked := any(args)
				if transport != nil {
					checked = jsonValue(args)
				}
				if tool.Schema.validate(checked) != nil {
					err = failure("tool_input", "action input does not satisfy tool schema")
					break
				}
				if dispatches >= MaxDispatches {
					err = failure("dispatch_limit", "tool dispatch bound reached")
					break
				}
				dispatches++
				if tool.Builtin == "execute_selected_tool" {
					if transport != nil {
						cap, ok := args["capability"].(*capability)
						if !ok || cap.used || cap.phase != "selection" || cap.call.StepID != previous || !reflect.DeepEqual(jsonValue(args["selection"]), jsonValue(selectedValue(cap.call))) {
							err = failure("tool_not_allowed", "selected tool requires its exported originating capability")
							break
						}
						if dispatches >= MaxDispatches {
							err = failure("dispatch_limit", "tool dispatch bound reached")
							break
						}
						dispatches++
						cap.used = true
						observed := e.toolV2(ctx, s.Tools[cap.call.ToolID], cap.call.Arguments, transport, cap.call.Exposed)
						cap.call.Result, _ = json.Marshal(observed)
						value = map[string]any{"observation": observed, "capability": newCapability(cap.call, "round")}
						break
					}
					if pending == nil || pending.StepID != previous || !reflect.DeepEqual(jsonValue(args), jsonValue(selectedValue(pending))) || !slices.Contains(pending.Allowed, pending.ToolID) {
						err = failure("tool_not_allowed", "selected tool has no valid originating judgement")
						break
					}
					if dispatches >= MaxDispatches {
						err = failure("dispatch_limit", "tool dispatch bound reached")
						break
					}
					dispatches++
					value = e.tool(ctx, s.Tools[pending.ToolID], pending.Arguments)
					pending.Result, _ = json.Marshal(value)
					lastRound = pending
					roundValue = value
					pending = nil
				} else {
					if transport != nil {
						value = e.toolV2(ctx, tool, args, transport, exposedNodeIDs(transport.data))
					} else {
						value = e.tool(ctx, tool, args)
					}
				}
			case "judgement":
				if calls >= MaxJudgements {
					err = failure("judgement_limit", "judgement call bound reached")
					break
				}
				var messages []completion.Message
				var exposed []int64
				if transport != nil {
					messages, trace.Context, exposed, err = v2Messages(step, input, request.Arguments, s, transport)
				} else {
					messages, trace.Context, err = stepMessages(step, input, request.Arguments, state, s, lastRound, roundValue)
				}
				if err != nil {
					break
				}
				tools := make([]completion.Tool, 0, len(step.Tools))
				for _, id := range step.Tools {
					t := s.Tools[id]
					schema, _ := json.Marshal(t.Schema)
					tools = append(tools, completion.Tool{Name: t.Name, Description: t.Description, Parameters: schema})
				}
				if e.Model == nil {
					err = failure("model_unavailable", "process model is unavailable")
					break
				}
				calls++
				var result completion.Result
				result, err = e.Model.Complete(ctx, completion.Request{Messages: messages, Tools: tools, Thinking: false})
				if err != nil {
					err = failure("model_unavailable", "step completion failed")
					break
				}
				trace.Usage = result.Usage
				trace.Provider = &result.Provider
				value, pending, err = decision(s, step, current, result)
				if transport != nil && pending != nil && err == nil {
					pending.Exposed = exposed
					value.(map[string]any)["capability"] = newCapability(pending, "selection")
					pending = nil
				}
			case "return":
				value, err = resolve(step.Output)
				if err != nil {
					err = failure("invalid_binding", err.Error())
					break
				}
				output, ok := value.(map[string]any)
				if !ok {
					err = failure("invalid_output", "process output must be an object")
					break
				}
				if err = e.arguments(ctx, s.Definition.Outputs, output, s.Definition.Version == 2); err != nil {
					err = failure("invalid_output", "output does not satisfy process declaration")
					break
				}
				response.Output = output
				response.Status = "completed"
			}
		}
		if err == nil && step.Kind != "return" {
			if transport == nil {
				state[step.Key] = jsonValue(value)
			}
			var edge Edge
			edge, err = nextEdge(s.Edges[current], jsonValue(value))
			if err == nil && transport != nil {
				err = transport.advance(step.Exports, request.Arguments, value)
			}
			if err == nil {
				trace.Edge = &edge
				previous = current
				current = edge.Target
			}
		}
		trace.ElapsedMS = time.Since(started).Milliseconds()
		if transport != nil {
			for _, receipt := range transport.fresh {
				trace.Receipts = append(trace.Receipts, receipt.MemoryReceipt)
			}
			if err == nil {
				for _, receipt := range transport.receipts {
					if receipt.StepID == trace.StepID && receipt.Visit == visit+1 {
						trace.Receipts = append(trace.Receipts, receipt.MemoryReceipt)
					}
				}
			}
		}
		trace.Outcome = "completed"
		observedValue := value
		if wrapped, ok := value.(map[string]any); ok && wrapped["observation"] != nil {
			observedValue = wrapped["observation"]
		}
		if result, ok := observedValue.(map[string]any); ok && result["ok"] == false {
			trace.Outcome = "tool_error"
			if toolError, ok := result["error"].(*Error); ok {
				trace.Error = toolError
			}
			if repair, ok := result["repair"].(MemoryResult); ok && repair.NodeID > 0 {
				trace.Repair = &repair
			}
		}
		if err != nil {
			trace.Outcome = "failed"
			trace.Error = asError(err, "process_failed")
			response.Error = trace.Error
		}
		response.Trace = append(response.Trace, trace)
		if err != nil || step.Kind == "return" {
			return response
		}
	}
	response.Error = failure("visit_limit", "process visit bound reached")
	return response
}

func asError(err error, code string) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return failure(code, "process operation failed")
}

func (e *Engine) arguments(ctx context.Context, decls map[string]Declaration, args map[string]any, allowBlankStrings bool) error {
	if len(args) != len(decls) {
		return failure("invalid_arguments", "argument set does not match declaration")
	}
	for key, decl := range decls {
		v, ok := args[key]
		if !ok {
			return failure("invalid_arguments", "required argument missing")
		}
		if decl.Type == "nodeRef" {
			id, valid := nodeID(v)
			if !valid {
				return failure("invalid_arguments", "node reference must be a positive integer")
			}
			node, found, err := e.Graph.Node(ctx, id, nil)
			if err != nil || !found || node.Type != decl.NodeType {
				return failure("invalid_arguments", "node reference missing or wrong type")
			}
			continue
		}
		if decl.Type == "string" {
			text, ok := v.(string)
			if !ok || !allowBlankStrings && strings.TrimSpace(text) == "" {
				return failure("invalid_arguments", "string argument must be nonblank")
			}
			continue
		}
		if decl.Type == "object" {
			if _, ok := v.(map[string]any); !ok {
				return failure("invalid_arguments", "expected object")
			}
			continue
		}
		if decl.Type == "array" {
			if _, ok := v.([]any); !ok {
				return failure("invalid_arguments", "expected array")
			}
			continue
		}
		if (Schema{Type: decl.Type}).validate(v) != nil {
			return failure("invalid_arguments", "argument type mismatch")
		}
	}
	return nil
}

func selectedValue(call *selectedCall) map[string]any {
	return map[string]any{"kind": "tool", "toolId": call.ToolID, "args": call.Arguments}
}

func decision(s *Snapshot, step Step, id int64, result completion.Result) (any, *selectedCall, error) {
	if result.FinishReason == "length" || len(result.Calls) > 1 {
		return nil, nil, failure("invalid_decision", "truncated or multiple tool decisions")
	}
	if len(result.Calls) == 0 {
		if strings.TrimSpace(result.Text) == "" {
			return nil, nil, failure("invalid_decision", "empty reply")
		}
		return map[string]any{"kind": "reply", "reply": result.Text}, nil, nil
	}
	call := result.Calls[0]
	for _, toolID := range step.Tools {
		tool := s.Tools[toolID]
		if tool.Name != call.Name {
			continue
		}
		var args map[string]any
		if decode(string(call.Arguments), &args) != nil || args == nil || tool.Schema.validate(args) != nil {
			return nil, nil, failure("tool_input", "model tool arguments do not satisfy schema")
		}
		if call.ID == "" {
			return nil, nil, failure("invalid_decision", "tool call has no identity")
		}
		p := &selectedCall{StepID: id, ToolID: toolID, Arguments: args, Call: call, Text: result.Text, Allowed: append([]int64{}, step.Tools...)}
		return selectedValue(p), p, nil
	}
	return nil, nil, failure("tool_not_allowed", "step did not offer requested tool")
}

func nextEdge(edges []Edge, result any) (Edge, error) {
	var matches []Edge
	var fallback *Edge
	for _, edge := range edges {
		t, _, _ := transition(edge)
		if t.Default {
			copy := edge
			fallback = &copy
			continue
		}
		v, err := pathValue(result, t.When.Path)
		if err == nil && reflect.DeepEqual(jsonValue(v), jsonValue(t.When.Equals)) {
			matches = append(matches, edge)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 && fallback != nil {
		return *fallback, nil
	}
	return Edge{}, failure("invalid_transition", "expected exactly one matching transition")
}

func stepMessages(step Step, input any, args, state map[string]any, s *Snapshot, round *selectedCall, roundValue any) ([]completion.Message, []Admission, error) {
	data := map[string]any{"input": input}
	admissions := []Admission{}
	budget := MaxContextBytes - len(step.Prompt)
	encoded, _ := json.Marshal(data)
	budget -= len(encoded)
	if budget < 0 {
		return nil, admissions, failure("context_bound", "required prompt and input exceed context bound")
	}
	var roundMessages []completion.Message
	values := make([]any, len(step.Context))
	costs := make([]int, len(step.Context))
	valid := make([]bool, len(step.Context))
	requiredCost := 0
	for i, item := range step.Context {
		var err error
		if item.Node > 0 {
			values[i] = s.Nodes[item.Node].Content
		} else {
			values[i], err = bind(item.Value, args, state)
		}
		encoded, _ := json.Marshal(values[i])
		costs[i] = len(encoded) + len(item.Key) + 8
		if item.ToolResult && round != nil && reflect.DeepEqual(jsonValue(values[i]), jsonValue(roundValue)) {
			messages, _ := json.Marshal(toolRoundMessages(round))
			costs[i] = len(messages)
		}
		valid[i] = err == nil
		if item.ToolResult {
			valid[i] = valid[i] && round != nil && reflect.DeepEqual(jsonValue(values[i]), jsonValue(roundValue))
		}
		if item.Required {
			if err != nil {
				return nil, admissions, failure("context_bound", "required context missing")
			}
			requiredCost += costs[i]
		}
	}
	if requiredCost > budget {
		return nil, admissions, failure("context_bound", "required context exceeds bound")
	}
	budget -= requiredCost
	for i, item := range step.Context {
		value := values[i]
		encoded, _ := json.Marshal(value)
		included := valid[i] && (item.Required || costs[i] <= budget)
		admissions = append(admissions, Admission{Key: item.Key, Node: item.Node, Hash: Hash(string(encoded)), Included: included})
		if !included {
			if item.Required {
				return nil, admissions, failure("context_bound", "required context missing or exceeds bound")
			}
			continue
		}
		if !item.Required {
			budget -= costs[i]
		}
		if item.ToolResult {
			roundMessages = toolRoundMessages(round)
		} else {
			data[item.Key] = value
		}
	}
	encoded, _ = json.Marshal(data)
	messages := []completion.Message{{Role: "system", Content: step.Prompt}, {Role: "user", Content: string(encoded)}}
	return append(messages, roundMessages...), admissions, nil
}

func toolRoundMessages(round *selectedCall) []completion.Message {
	return []completion.Message{{Role: "assistant", Content: round.Text, Calls: []completion.Call{round.Call}}, {Role: "tool", Content: string(round.Result), CallID: round.Call.ID, ToolName: round.Call.Name}}
}

func (e *Engine) tool(ctx context.Context, tool Tool, args map[string]any) any {
	var value any
	var err error
	switch tool.Builtin {
	case "select":
		return args
	case "read_node":
		id, ok := nodeID(args["id"])
		if !ok {
			err = failure("tool_input", "read_node requires a positive id")
			break
		}
		node, found, readErr := e.Graph.Node(ctx, id, nil)
		if readErr != nil || !found {
			err = failure("node_not_found", "requested node unavailable")
		} else {
			value = map[string]any{"id": node.ID, "type": node.Type, "name": node.Name, "content": node.Content}
		}
	case "recall":
		query, ok := args["query"].(string)
		if !ok || !loop.IsPhrase(query) {
			err = failure("tool_input", "recall query must be a nonblank phrase of at most 256 bytes")
			break
		}
		var filter loop.Filter
		if v, ok := args["filter"]; ok {
			filter, err = recallFilter(v)
		}
		if err == nil {
			value, err = e.Graph.Recall(ctx, loop.RecallRequest{Queries: []string{query}, Limit: 6, Filter: filter})
		}
	case "":
		value, err = e.Python.Run(ctx, tool.Python, args)
	default:
		err = failure("tool_not_allowed", "tool implementation unavailable")
	}
	if err != nil {
		return map[string]any{"ok": false, "error": asError(err, "tool_failed")}
	}
	return map[string]any{"ok": true, "value": value}
}

func recallFilter(value any) (loop.Filter, error) {
	allowed := strings.Fields("type name status nostatus linkedto path bounds minSimilarity excludeId excludeName updatedFrom updatedTo createdFrom createdTo severity severityMin severityMax noSeverity refinement norefinement rootNodeId noRootNodeId")
	entries, ok := value.(map[string]any)
	if !ok || len(entries) == 0 {
		return nil, failure("tool_input", "filter must be a nonempty object")
	}
	result := loop.Filter{}
	for key, value := range entries {
		if !slices.Contains(allowed, key) {
			return nil, failure("tool_input", "unsupported filter key")
		}
		values, ok := value.([]any)
		if !ok {
			values = []any{value}
		}
		if len(values) == 0 {
			return nil, failure("tool_input", "empty filter values")
		}
		for _, value := range values {
			var text string
			switch v := value.(type) {
			case string:
				text = v
			case json.Number:
				text = v.String()
			case bool:
				if v {
					text = "true"
				} else {
					text = "false"
				}
			default:
				return nil, failure("tool_input", "unsupported filter value")
			}
			if strings.TrimSpace(text) == "" {
				return nil, failure("tool_input", "blank filter value")
			}
			result[key] = append(result[key], text)
		}
	}
	return result, nil
}
