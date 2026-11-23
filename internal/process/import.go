package process

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/telmengedar/processor/internal/loop"
)

type DraftNode struct {
	Symbol string `json:"symbol"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Body   any    `json:"body"`
}

type DraftEdge struct {
	From    string     `json:"from"`
	To      string     `json:"to"`
	When    *Predicate `json:"when,omitempty"`
	Default bool       `json:"default,omitempty"`
	Role    string     `json:"role,omitempty"`
}

type Blueprint struct {
	Version int                    `json:"version,omitempty"`
	Name    string                 `json:"name"`
	Entry   string                 `json:"entry"`
	Inputs  map[string]Declaration `json:"inputs"`
	Outputs map[string]Declaration `json:"outputs"`
	Nodes   []DraftNode            `json:"nodes"`
	Edges   []DraftEdge            `json:"edges"`
}

type Publisher interface {
	CreateProcessNode(context.Context, string, string) (int64, error)
	SetProcessContent(context.Context, int64, string) error
	LinkProcessNodes(context.Context, Edge) error
}

type TextPublisher interface {
	SetProcessTextContent(context.Context, int64, string) error
}

type Publication struct {
	ProcessID int64            `json:"processId"`
	Members   map[string]int64 `json:"members"`
}

func Import(ctx context.Context, p Publisher, b Blueprint) (Publication, error) {
	staged := &draftGraph{nodes: map[int64]loop.Anchor{}}
	_, err := publish(ctx, staged, b)
	if err != nil {
		return Publication{}, err
	}
	return publish(ctx, p, b)
}

type draftGraph struct {
	nodes map[int64]loop.Anchor
	edges []Edge
}

func (g *draftGraph) CreateProcessNode(_ context.Context, t, n string) (int64, error) {
	id := int64(len(g.nodes) + 1)
	g.nodes[id] = loop.Anchor{ID: id, Type: t, Name: n}
	return id, nil
}
func (g *draftGraph) SetProcessContent(_ context.Context, id int64, body string) error {
	n := g.nodes[id]
	n.Content = body
	g.nodes[id] = n
	return nil
}
func (g *draftGraph) SetProcessTextContent(ctx context.Context, id int64, body string) error {
	return g.SetProcessContent(ctx, id, body)
}
func (g *draftGraph) LinkProcessNodes(_ context.Context, e Edge) error {
	g.edges = append(g.edges, e)
	return nil
}
func (g *draftGraph) Node(_ context.Context, id int64, _ loop.Filter) (loop.Anchor, bool, error) {
	n, ok := g.nodes[id]
	return n, ok, nil
}
func (g *draftGraph) FindNodes(_ context.Context, name, t string) ([]loop.Anchor, error) {
	var r []loop.Anchor
	for _, n := range g.nodes {
		if n.Name == name && n.Type == t {
			r = append(r, n)
		}
	}
	return r, nil
}
func (g *draftGraph) Links(_ context.Context, id int64) ([]Edge, error) {
	var r []Edge
	for _, e := range g.edges {
		if e.Source == id || e.Target == id {
			r = append(r, e)
		}
	}
	return r, nil
}
func (g *draftGraph) Recall(context.Context, loop.RecallRequest) ([]loop.Candidate, error) {
	return nil, fmt.Errorf("draft graph cannot recall")
}

func publish(ctx context.Context, p Publisher, b Blueprint) (Publication, error) {
	r := Publication{Members: map[string]int64{}}
	if b.Name == "" || len(b.Nodes) == 0 || len(b.Nodes) > MaxMembers || len(b.Edges) > MaxTransitions {
		return r, failure("invalid_definition", "invalid import bounds")
	}
	symbols := map[string]bool{}
	for _, node := range b.Nodes {
		if !pathKey.MatchString(node.Symbol) || symbols[node.Symbol] || (b.Version != 2 && node.Type != "process-step" && node.Type != "process-tool") || node.Type == "" || node.Type == "process" {
			return r, failure("invalid_definition", "invalid import node")
		}
		symbols[node.Symbol] = true
	}
	if !symbols[b.Entry] {
		return r, failure("invalid_definition", "import entry missing")
	}
	pairs := map[[2]string]bool{}
	for _, edge := range b.Edges {
		a, z := edge.From, edge.To
		if a > z {
			a, z = z, a
		}
		pair := [2]string{a, z}
		if !symbols[a] || !symbols[z] || a == z || pairs[pair] {
			return r, failure("invalid_definition", "import has invalid or repeated edge pair")
		}
		pairs[pair] = true
	}
	var err error
	r.ProcessID, err = p.CreateProcessNode(ctx, "process", b.Name)
	if err != nil {
		return r, err
	}
	for _, node := range b.Nodes {
		id, err := p.CreateProcessNode(ctx, node.Type, node.Name)
		if err != nil {
			return r, err
		}
		r.Members[node.Symbol] = id
	}
	version := b.Version
	if version == 0 {
		version = 1
	}
	d := Definition{Version: version, Entry: r.Members[b.Entry], Inputs: b.Inputs, Outputs: b.Outputs, Members: []Member{}}
	resolved := &draftGraph{nodes: map[int64]loop.Anchor{r.ProcessID: {ID: r.ProcessID, Type: "process", Name: b.Name}}}
	for _, node := range b.Nodes {
		var body any
		var err error
		if version == 2 && node.Type != "process-step" && node.Type != "process-tool" {
			body = node.Body
		} else {
			body, err = resolveSymbols(jsonValue(node.Body), r.Members)
		}
		if err != nil {
			return r, err
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			return r, err
		}
		if version == 2 && node.Type != "process-step" && node.Type != "process-tool" {
			raw, ok := body.(string)
			if !ok || !utf8.ValidString(raw) {
				return r, failure("invalid_definition", "data member body must be raw text")
			}
			encoded = []byte(raw)
		}
		id := r.Members[node.Symbol]
		resolved.nodes[id] = loop.Anchor{ID: id, Type: node.Type, Name: node.Name, Content: string(encoded)}
		d.Members = append(d.Members, Member{ID: r.Members[node.Symbol], SHA256: Hash(string(encoded))})
	}
	edges := make([]Edge, 0, len(b.Edges))
	for _, draft := range b.Edges {
		var metadata any = Transition{Kind: "process-transition", When: draft.When, Default: draft.Default}
		if draft.Role != "" {
			if version != 2 || draft.When != nil || draft.Default {
				return r, failure("invalid_definition", "invalid reference import")
			}
			metadata = Reference{Kind: "process-reference", Role: draft.Role}
		}
		context, err := json.Marshal(metadata)
		if err != nil {
			return r, err
		}
		edge := Edge{Source: r.Members[draft.From], Target: r.Members[draft.To], LinkType: "Unidirectional", Context: string(context)}
		if draft.Role != "" {
			if _, _, err := referenceEdge(edge); err != nil {
				return r, err
			}
		} else if _, _, err := transition(edge); err != nil {
			return r, err
		}
		edges = append(edges, edge)
	}
	d.TransitionsSHA256 = TransitionHash(edges)
	root, err := json.Marshal(d)
	if err != nil {
		return r, err
	}
	node := resolved.nodes[r.ProcessID]
	node.Content = string(root)
	resolved.nodes[r.ProcessID] = node
	resolved.edges = edges
	if _, err = Load(ctx, resolved, Request{ProcessID: r.ProcessID}); err != nil {
		return r, err
	}
	for _, node := range b.Nodes {
		id := r.Members[node.Symbol]
		if version == 2 && node.Type != "process-step" && node.Type != "process-tool" {
			writer, ok := p.(TextPublisher)
			if !ok {
				return r, failure("invalid_definition", "publisher does not support raw Markdown members")
			}
			err = writer.SetProcessTextContent(ctx, id, resolved.nodes[id].Content)
		} else {
			err = p.SetProcessContent(ctx, id, resolved.nodes[id].Content)
		}
		if err != nil {
			return r, err
		}
	}
	for _, edge := range edges {
		if err = p.LinkProcessNodes(ctx, edge); err != nil {
			return r, err
		}
	}
	if err = p.SetProcessContent(ctx, r.ProcessID, string(root)); err != nil {
		return r, err
	}
	return r, nil
}

func resolveSymbols(value any, ids map[string]int64) (any, error) {
	switch x := value.(type) {
	case map[string]any:
		if symbol, ok := x["nodeRef"].(string); ok && len(x) == 1 {
			id, ok := ids[symbol]
			if !ok {
				return nil, fmt.Errorf("unknown member reference")
			}
			return id, nil
		}
		r := map[string]any{}
		for k, v := range x {
			value, err := resolveSymbols(v, ids)
			if err != nil {
				return nil, err
			}
			r[k] = value
		}
		return r, nil
	case []any:
		r := make([]any, len(x))
		for i, v := range x {
			value, err := resolveSymbols(v, ids)
			if err != nil {
				return nil, err
			}
			r[i] = value
		}
		return r, nil
	default:
		return value, nil
	}
}
