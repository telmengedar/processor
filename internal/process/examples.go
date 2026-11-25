package process

func reference(source, path string) map[string]any {
	return map[string]any{"source": source, "path": path}
}

func memberRef(symbol string) map[string]any { return map[string]any{"nodeRef": symbol} }

func GangolfChat(personaType string) Blueprint {
	nodes := []DraftNode{}
	add := func(symbol, nodeType string, body any) {
		nodes = append(nodes, DraftNode{Symbol: symbol, Type: nodeType, Name: symbol, Body: body})
	}
	object := Schema{Type: "object", AdditionalProperties: true}
	add("select", "process-tool", Tool{Version: 1, Name: "select", Description: "Select explicitly supplied data.", Schema: object, Builtin: "select"})
	add("read", "process-tool", Tool{Version: 1, Name: "read_node", Description: "Read one memory node by positive integer ID.", Schema: Schema{Type: "object", Properties: map[string]Schema{"id": {Type: "integer"}}, Required: []string{"id"}}, Builtin: "read_node"})
	add("recall", "process-tool", Tool{Version: 1, Name: "recall", Description: "Find relevant memory using a short query.", Schema: Schema{Type: "object", Properties: map[string]Schema{"query": {Type: "string"}, "filter": {Type: "object", AdditionalProperties: true}}, Required: []string{"query"}}, Builtin: "recall"})
	add("dispatch", "process-tool", Tool{Version: 1, Name: "execute_selected_tool", Description: "Execute the preceding judgement's validated tool selection.", Schema: object, Builtin: "execute_selected_tool"})
	action := func(key, tool string, input any) {
		add(key, "process-step", map[string]any{"version": 1, "key": key, "kind": "action", "tool": memberRef(tool), "input": input})
	}
	action("load_message", "select", map[string]any{"text": reference("input", "message")})
	action("load_persona", "read", map[string]any{"id": reference("input", "persona")})
	action("recall_context", "recall", map[string]any{"query": reference("state", "load_message.text")})
	add("assess_situation", "process-step", map[string]any{"version": 1, "key": "assess_situation", "kind": "judgement", "prompt": exampleText("artifacts/assessment-v1.md"), "input": reference("state", "load_message"), "context": []any{map[string]any{"key": "memory", "value": reference("state", "recall_context"), "required": false}, map[string]any{"key": "latest_result", "value": reference("state", "refresh_context"), "required": false, "toolResult": true}}, "tools": []any{memberRef("read"), memberRef("recall")}})
	action("execute_tool", "dispatch", reference("state", "assess_situation"))
	action("refresh_context", "select", reference("state", "execute_tool"))
	add("compose_response", "process-step", map[string]any{"version": 1, "key": "compose_response", "kind": "judgement", "prompt": exampleText("artifacts/response-v1.md"), "input": reference("state", "load_message"), "context": []any{map[string]any{"key": "persona", "value": reference("state", "load_persona.value.content"), "required": true}, map[string]any{"key": "assessment", "value": reference("state", "assess_situation.reply"), "required": true}}})
	add("reply", "process-step", Step{Version: 1, Key: "reply", Kind: "return", Output: map[string]any{"reply": reference("state", "compose_response.reply")}})
	edges := []DraftEdge{{From: "load_message", To: "load_persona", Default: true}, {From: "load_persona", To: "recall_context", Default: true}, {From: "recall_context", To: "assess_situation", Default: true}, {From: "assess_situation", To: "execute_tool", When: &Predicate{Path: "kind", Equals: "tool"}}, {From: "execute_tool", To: "refresh_context", Default: true}, {From: "refresh_context", To: "assess_situation", Default: true}, {From: "assess_situation", To: "compose_response", When: &Predicate{Path: "kind", Equals: "reply"}}, {From: "compose_response", To: "reply", When: &Predicate{Path: "kind", Equals: "reply"}}}
	return Blueprint{Name: "Gangolf Chat", Entry: "load_message", Inputs: map[string]Declaration{"message": {Type: "string"}, "persona": {Type: "nodeRef", NodeType: personaType}}, Outputs: map[string]Declaration{"reply": {Type: "string"}}, Nodes: nodes, Edges: edges}
}

func PythonExample() Blueprint {
	return Blueprint{Name: "Python transformation", Entry: "transform", Inputs: map[string]Declaration{"text": {Type: "string"}}, Outputs: map[string]Declaration{"reply": {Type: "string"}}, Nodes: []DraftNode{
		{Symbol: "uppercase", Type: "process-tool", Name: "Uppercase", Body: Tool{Version: 1, Name: "uppercase", Description: "Uppercase supplied text.", Schema: Schema{Type: "object", Properties: map[string]Schema{"text": {Type: "string"}}, Required: []string{"text"}}, Python: "import json,sys\nvalue=json.load(sys.stdin)\njson.dump({'reply':value['text'].upper()},sys.stdout)"}},
		{Symbol: "transform", Type: "process-step", Name: "Transform", Body: map[string]any{"version": 1, "key": "transform", "kind": "action", "tool": memberRef("uppercase"), "input": map[string]any{"text": reference("input", "text")}}},
		{Symbol: "return_reply", Type: "process-step", Name: "Return", Body: Step{Version: 1, Key: "return_reply", Kind: "return", Output: map[string]any{"reply": reference("state", "transform.value.reply")}}},
	}, Edges: []DraftEdge{{From: "transform", To: "return_reply", Default: true}}}
}
