// Package eval scores what the loop's retrieval delivered against a hand-labelled corpus.
package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const maxRequiredPerRow = 3

const pairSize = 2

// StratumLabelled marks a row whose required nodes were labelled by hand.
const StratumLabelled = "labelled"

// StratumControl marks a constructed row whose required nodes retrieval must surface on every sweep.
const StratumControl = "control"

// AnchorPlace marks a row whose anchor's title names a durable location or subject.
const AnchorPlace = "place"

// AnchorWork marks a row whose anchor's title names a unit of work.
const AnchorWork = "work"

// Required is one node a row demands, with the hash it carried when it was labelled.
type Required struct {
	Node int64  `json:"node"`
	Hash string `json:"hash"`
	Why  string `json:"why"`
}

// Row is one corpus entry: an input, its subject, and the nodes retrieval must surface.
type Row struct {
	ID          string     `json:"id"`
	Pair        string     `json:"pair,omitempty"`
	Input       string     `json:"input"`
	Subject     int64      `json:"subject"`
	Anchor      string     `json:"anchor,omitempty"`
	AnchorTitle string     `json:"anchorTitle,omitempty"`
	Stratum     string     `json:"stratum"`
	Required    []Required `json:"required"`
}

// Corpus is a validated set of rows and the hash of the bytes they were read from.
type Corpus struct {
	Rows []Row
	Hash string
}

// Load reads, decodes and validates the corpus file at path.
func Load(path string) (Corpus, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Corpus{}, fmt.Errorf("eval: read corpus: %w", err)
	}

	var rows []Row
	if err := json.Unmarshal(raw, &rows); err != nil {
		return Corpus{}, fmt.Errorf("eval: decode corpus: %w", err)
	}

	if err := validate(rows); err != nil {
		return Corpus{}, err
	}

	sum := sha256.Sum256(raw)
	return Corpus{Rows: rows, Hash: hex.EncodeToString(sum[:])}, nil
}

func validate(rows []Row) error {
	if len(rows) == 0 {
		return errors.New("eval: the corpus is empty, and a sweep over no rows would report 0/0 as a success (design 6.2)")
	}

	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if err := validateRow(row, seen); err != nil {
			return err
		}
		seen[row.ID] = true
	}

	if err := validateAnchorClassification(rows); err != nil {
		return err
	}
	return validatePairs(rows)
}

func validateAnchorClassification(rows []Row) error {
	classified := 0
	for _, row := range rows {
		if row.Anchor != "" {
			classified++
		}
	}

	if classified != 0 && classified != len(rows) {
		return fmt.Errorf("eval: %d of %d rows carry an anchor class, so a sweep would report a stratified rate over a population that is only partly stratified (anchor corpus design 5)", classified, len(rows))
	}
	return nil
}

func validatePairs(rows []Row) error {
	paired := 0
	for _, row := range rows {
		if row.Pair != "" {
			paired++
		}
	}

	if paired == 0 {
		return nil
	}
	if paired != len(rows) {
		return fmt.Errorf("eval: %d of %d rows carry a pair, so the rows outside one would be counted into a comparison they are not half of (anchor corpus design 5)", paired, len(rows))
	}

	order, members := groupByPair(rows)
	for _, pair := range order {
		if err := validatePair(pair, members[pair]); err != nil {
			return err
		}
	}
	return nil
}

func groupByPair(rows []Row) ([]string, map[string][]Row) {
	var order []string
	members := make(map[string][]Row, len(rows))
	for _, row := range rows {
		if _, known := members[row.Pair]; !known {
			order = append(order, row.Pair)
		}
		members[row.Pair] = append(members[row.Pair], row)
	}
	return order, members
}

func validatePair(pair string, rows []Row) error {
	if len(rows) != pairSize {
		return fmt.Errorf("eval: pair %q holds %d rows, want %d, so the two arms it exists to compare are not both present (anchor corpus design 5)", pair, len(rows), pairSize)
	}

	first, second := rows[0], rows[1]
	if first.Anchor == "" || second.Anchor == "" {
		return fmt.Errorf("eval: pair %q holds a row with no anchor class, so which arm that row stands for is unknown (anchor corpus design 5)", pair)
	}
	if first.Anchor == second.Anchor {
		return fmt.Errorf("eval: pair %q classifies both of its rows %q, so the comparison holds the anchor fixed and varies nothing (anchor corpus design 5)", pair, first.Anchor)
	}
	if first.Input != second.Input {
		return fmt.Errorf("eval: pair %q holds two different inputs, so a difference between its arms is not attributable to the anchor (anchor corpus design 5)", pair)
	}
	if !sameRequiredNodes(first, second) {
		return fmt.Errorf("eval: pair %q holds two different required sets, so its arms are scored against different answer keys (anchor corpus design 5)", pair)
	}
	return nil
}

func sameRequiredNodes(first, second Row) bool {
	if len(first.Required) != len(second.Required) {
		return false
	}

	nodes := make(map[int64]bool, len(first.Required))
	for _, req := range first.Required {
		nodes[req.Node] = true
	}
	for _, req := range second.Required {
		if !nodes[req.Node] {
			return false
		}
	}
	return true
}

func validateRow(row Row, seen map[string]bool) error {
	if row.ID == "" {
		return errors.New("eval: a row carries no id, so results cannot be diffed across sweeps (design 6.2)")
	}
	if seen[row.ID] {
		return fmt.Errorf("eval: row %q: the id is not unique (design 6.2)", row.ID)
	}
	if row.Input == "" {
		return fmt.Errorf("eval: row %q: the input is empty (design 6.2)", row.ID)
	}
	if row.Subject <= 0 {
		return fmt.Errorf("eval: row %q: subject %d is not a node id (design 6.2)", row.ID, row.Subject)
	}
	if row.Stratum != StratumLabelled && row.Stratum != StratumControl {
		return fmt.Errorf("eval: row %q: stratum %q is outside the closed set (design 6.2)", row.ID, row.Stratum)
	}
	if row.Anchor != "" && row.Anchor != AnchorPlace && row.Anchor != AnchorWork {
		return fmt.Errorf("eval: row %q: anchor class %q is outside the closed set, so the stratum it is counted into is decided by a typo (anchor corpus design 5)", row.ID, row.Anchor)
	}
	if row.Anchor != "" && row.AnchorTitle == "" {
		return fmt.Errorf("eval: row %q: the anchor is classified %q and no title is pinned, so the judgement cannot be audited against the words it was made from (anchor corpus design 5)", row.ID, row.Anchor)
	}
	if row.Anchor == "" && row.AnchorTitle != "" {
		return fmt.Errorf("eval: row %q: an anchor title is pinned and no class is recorded, so the row carries the evidence for a judgement nobody made (anchor corpus design 5)", row.ID)
	}
	if len(row.Required) == 0 || len(row.Required) > maxRequiredPerRow {
		return fmt.Errorf("eval: row %q: %d required nodes, want between 1 and %d (design 6.2)", row.ID, len(row.Required), maxRequiredPerRow)
	}
	required := make(map[int64]bool, len(row.Required))
	for _, req := range row.Required {
		if err := validateRequired(row, req); err != nil {
			return err
		}
		if required[req.Node] {
			return fmt.Errorf("eval: row %q: required node %d is listed twice, so one label would be counted twice in both rates (design 6.2)", row.ID, req.Node)
		}
		required[req.Node] = true
	}
	return nil
}

func validateRequired(row Row, req Required) error {
	if req.Node <= 0 {
		return fmt.Errorf("eval: row %q: required node %d is not a node id (design 6.2)", row.ID, req.Node)
	}
	if req.Node == row.Subject {
		return fmt.Errorf("eval: row %q: required node %d is the row's own subject, which is fetched by id and can never be a recall miss (design 6.2)", row.ID, req.Node)
	}
	if req.Hash == "" {
		return fmt.Errorf("eval: row %q: required node %d carries no hash (design 6.2)", row.ID, req.Node)
	}
	if !isContentHash(req.Hash) {
		return fmt.Errorf("eval: row %q: required node %d carries hash %q, which is not lowercase sha256 hex and so can never equal a live content hash, leaving the row stale on every sweep (design 6.2)", row.ID, req.Node, req.Hash)
	}
	if req.Why == "" {
		return fmt.Errorf("eval: row %q: required node %d carries no reason (design 6.2)", row.ID, req.Node)
	}
	return nil
}

func isContentHash(hash string) bool {
	if len(hash) != hex.EncodedLen(sha256.Size) {
		return false
	}
	for _, c := range []byte(hash) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
