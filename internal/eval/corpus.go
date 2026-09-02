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

// StratumLabelled marks a row whose required nodes were labelled by hand.
const StratumLabelled = "labelled"

// StratumControl marks a constructed row that must read 1.00 on every sweep.
const StratumControl = "control"

// Required is one node a row demands, with the hash it carried when it was labelled.
type Required struct {
	Node int64  `json:"node"`
	Hash string `json:"hash"`
	Why  string `json:"why"`
}

// Row is one corpus entry: an input, its subject, and the nodes retrieval must surface.
type Row struct {
	ID       string     `json:"id"`
	Input    string     `json:"input"`
	Subject  int64      `json:"subject"`
	Stratum  string     `json:"stratum"`
	Required []Required `json:"required"`
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
	return nil
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
	if len(row.Required) == 0 || len(row.Required) > maxRequiredPerRow {
		return fmt.Errorf("eval: row %q: %d required nodes, want between 1 and %d (design 6.2)", row.ID, len(row.Required), maxRequiredPerRow)
	}
	for _, req := range row.Required {
		if err := validateRequired(row, req); err != nil {
			return err
		}
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
	if req.Why == "" {
		return fmt.Errorf("eval: row %q: required node %d carries no reason (design 6.2)", row.ID, req.Node)
	}
	return nil
}
