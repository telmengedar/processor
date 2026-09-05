package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

// ArmRawInput is the arm name a sweep carries when no derivation sidecar was supplied.
const ArmRawInput = "raw-input"

// Derivation is one corpus row's pinned query set.
type Derivation struct {
	Row     string   `json:"row"`
	Queries []string `json:"queries"`
}

// Derivations is a validated sidecar: the pinned queries per row id, the path
// they were read from, and the hash of those bytes. The zero value is the
// raw-input arm and yields each row its own input alone.
type Derivations struct {
	Path    string
	Hash    string
	Queries map[string][]string
}

// Arm names the retrieval arm a sweep taken with these derivations ran under.
func (d Derivations) Arm() string {
	if d.Path == "" {
		return ArmRawInput
	}
	return d.Path
}

// Rows reports how many corpus rows carry a pinned query set.
func (d Derivations) Rows() int {
	return len(d.Queries)
}

// Unpinned returns the ids of corpus rows this sidecar does not pin, in
// corpus order. A sidecar that validated against an earlier, smaller corpus
// can still be missing rows a later corpus added -- LoadDerivations only
// checks that every sidecar row resolves to a corpus row, not the reverse.
// The zero sidecar (the raw-input arm) pins nothing by definition, which is
// not itself a coverage gap, so it reports none.
func (d Derivations) Unpinned(corpus Corpus) []string {
	if d.Path == "" {
		return nil
	}

	var unpinned []string
	for _, row := range corpus.Rows {
		if _, ok := d.Queries[row.ID]; !ok {
			unpinned = append(unpinned, row.ID)
		}
	}
	return unpinned
}

// QueriesFor is the row's own input followed by whatever the sidecar pinned for it, without repeats.
func (d Derivations) QueriesFor(row Row) []string {
	queries := []string{row.Input}
	for _, query := range d.Queries[row.ID] {
		if slices.Contains(queries, query) {
			continue
		}
		queries = append(queries, query)
	}
	return queries
}

// LoadDerivations reads, decodes and validates the sidecar at path against corpus.
func LoadDerivations(path string, corpus Corpus) (Derivations, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Derivations{}, fmt.Errorf("eval: read derivations: %w", err)
	}

	var entries []Derivation
	if err := json.Unmarshal(raw, &entries); err != nil {
		return Derivations{}, fmt.Errorf("eval: decode derivations: %w", err)
	}

	queries, err := validateDerivations(entries, corpus)
	if err != nil {
		return Derivations{}, err
	}

	sum := sha256.Sum256(raw)
	return Derivations{Path: path, Hash: hex.EncodeToString(sum[:]), Queries: queries}, nil
}

func validateDerivations(entries []Derivation, corpus Corpus) (map[string][]string, error) {
	if len(entries) == 0 {
		return nil, errors.New("eval: the derivation sidecar holds no rows, and a sweep would then report a derived arm's name over the raw-input arm's ranking (design 4.5)")
	}

	known := make(map[string]bool, len(corpus.Rows))
	for _, row := range corpus.Rows {
		known[row.ID] = true
	}

	queries := make(map[string][]string, len(entries))
	for _, entry := range entries {
		if err := validateDerivation(entry, known, queries); err != nil {
			return nil, err
		}
		queries[entry.Row] = entry.Queries
	}
	return queries, nil
}

func validateDerivation(entry Derivation, known map[string]bool, seen map[string][]string) error {
	if !known[entry.Row] {
		return fmt.Errorf("eval: derivation %q: no corpus row carries that id, so its queries would be pinned against nothing and the row would sweep on its raw input while the result named a derived arm (design 4.5)", entry.Row)
	}
	if _, duplicate := seen[entry.Row]; duplicate {
		return fmt.Errorf("eval: derivation %q: the row id is not unique, so which query set the sweep used is decided by file order (design 4.5)", entry.Row)
	}
	if len(entry.Queries) == 0 {
		return fmt.Errorf("eval: derivation %q: no queries, which is the raw-input arm carrying a derived arm's hash (design 4.5)", entry.Row)
	}
	for i, query := range entry.Queries {
		if strings.TrimSpace(query) == "" {
			return fmt.Errorf("eval: derivation %q: query %d is blank, and a blank query ranks the whole graph by nothing (design 4.5)", entry.Row, i)
		}
		if slices.Contains(entry.Queries[:i], query) {
			return fmt.Errorf("eval: derivation %q: query %d repeats an earlier one, which doubles that list's weight in the reciprocal-rank sum (design 4.2)", entry.Row, i)
		}
	}
	return nil
}
