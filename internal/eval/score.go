package eval

import "github.com/telmengedar/processor/internal/loop"

// Verdict is the closed set of outcomes for one required node.
type Verdict string

// Admitted means the node was retrieved and rendered into the block the model saw.
const Admitted Verdict = "admitted"

// Cut means the node was retrieved and the byte budget discarded it.
const Cut Verdict = "cut"

// NotRetrieved means the node appeared in no candidate row.
const NotRetrieved Verdict = "notRetrieved"

// Unresolved means the node id no longer resolves in the graph.
const Unresolved Verdict = "unresolved"

// NodeResult is one required node's verdict, the rank that produced it, and whether its label has rotted.
type NodeResult struct {
	Node    int64   `json:"node"`
	Verdict Verdict `json:"verdict"`
	Rank    int     `json:"rank,omitempty"`
	Stale   bool    `json:"stale,omitempty"`
}

// Score classifies every required node against the dispositions one row produced.
func Score(required []Required, dispositions []loop.Disposition) []NodeResult {
	results := make([]NodeResult, len(required))
	for i, req := range required {
		results[i] = scoreOne(req, dispositions)
	}
	return results
}

func scoreOne(req Required, dispositions []loop.Disposition) NodeResult {
	for _, d := range dispositions {
		if d.ID != req.Node {
			continue
		}
		verdict := Cut
		if d.Included {
			verdict = Admitted
		}
		return NodeResult{Node: req.Node, Verdict: verdict, Rank: d.Rank, Stale: d.ContentHash != req.Hash}
	}
	return NodeResult{Node: req.Node, Verdict: NotRetrieved}
}
