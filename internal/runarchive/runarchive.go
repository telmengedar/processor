// Package runarchive decodes archived run-record nodes into the records every mechanical instrument reads.
package runarchive

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/telmengedar/processor/internal/loop"
)

const (
	// SkipContentAbsent names a node that carries no content to decode.
	SkipContentAbsent = "content absent"
	// SkipUnrecognisedShape names content that is neither bare record JSON nor a recognisable fenced record.
	SkipUnrecognisedShape = "content is neither bare JSON nor a recognisable fenced record"
	// SkipNameUnparseable names a node whose name carries no parseable run instant.
	SkipNameUnparseable = "node name does not carry a parseable timestamp"
)

const (
	fenceOpenNeedle  = "\n```json\n"
	fenceCloseNeedle = "\n```"
)

var runNamePattern = regexp.MustCompile(`^processor-run (\S+) — `)

// Node is one graph row handed to the reader, carrying only what decoding needs.
type Node struct {
	ID      int64
	Name    string
	Content string
}

// Entry is one decoded archive node: the record, the bytes it decoded from, and where those bytes came from.
type Entry struct {
	Node int64     `json:"node"`
	Name string    `json:"name"`
	At   time.Time `json:"at"`

	// Dated is true when the node's name stated the run instant; when it is false At is unknown rather than early, and no instant question may be answered from it.
	Dated bool `json:"dated"`

	// Raw is the record's own bytes as the node carries them, never a re-marshal, so a field absent from the record stays absent here.
	Raw []byte `json:"-"`

	// Fenced is true when the bytes came from a fenced block below a prose account rather than from the whole content.
	Fenced bool `json:"fenced"`

	Record loop.Record `json:"-"`
}

// Skip is one node the reader did not decode, and the named rule that stopped it.
type Skip struct {
	Node   int64  `json:"node"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

// Archive is one read over a set of nodes: what decoded, and every node that did not.
type Archive struct {
	// Selector names the subset these entries are, and no figure computed over them is quotable without it.
	Selector string `json:"selector"`

	Entries []Entry `json:"entries"`
	Skipped []Skip  `json:"skipped"`
}

// Read decodes every node, isolating each node's failure from the rest of the read so no node is dropped silently.
func Read(selector string, nodes []Node) Archive {
	archive := Archive{
		Selector: selector,
		Entries:  make([]Entry, 0, len(nodes)),
		Skipped:  make([]Skip, 0, len(nodes)),
	}

	for _, node := range nodes {
		entry, skip, ok := Decode(node)
		if !ok {
			archive.Skipped = append(archive.Skipped, skip)
			continue
		}
		archive.Entries = append(archive.Entries, entry)
	}

	return archive
}

// Decode decodes one node; a node carrying no name decodes undated, and ok is false only where the skip names the rule that stopped it.
func Decode(node Node) (entry Entry, skip Skip, ok bool) {
	if strings.TrimSpace(node.Content) == "" {
		return Entry{}, Skip{Node: node.ID, Name: node.Name, Reason: SkipContentAbsent}, false
	}

	raw, record, fenced, err := classify(node.Content)
	if err != nil {
		return Entry{}, Skip{Node: node.ID, Name: node.Name, Reason: SkipUnrecognisedShape, Detail: err.Error()}, false
	}

	if node.Name == "" {
		return Entry{Node: node.ID, Raw: raw, Fenced: fenced, Record: record}, Skip{}, true
	}

	at, err := ParseRunInstant(node.Name)
	if err != nil {
		return Entry{}, Skip{Node: node.ID, Name: node.Name, Reason: SkipNameUnparseable, Detail: err.Error()}, false
	}

	return Entry{Node: node.ID, Name: node.Name, At: at.UTC(), Dated: true, Raw: raw, Fenced: fenced, Record: record}, Skip{}, true
}

// ParseRunInstant reads the run instant out of an archived record node's name.
func ParseRunInstant(name string) (time.Time, error) {
	m := runNamePattern.FindStringSubmatch(name)
	if m == nil {
		return time.Time{}, fmt.Errorf("name %q does not match \"processor-run <RFC3339> — ...\"", name)
	}
	at, err := time.Parse(time.RFC3339, m[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("name %q carries %q, which is not RFC3339: %w", name, m[1], err)
	}
	return at, nil
}

// Classify decodes content into the record's own bytes and the record they carry, reporting whether those bytes came from a fenced block.
func Classify(content string) (raw []byte, record loop.Record, fenced bool, err error) {
	return classify(content)
}

func classify(content string) (raw []byte, record loop.Record, fenced bool, err error) {
	var bare loop.Record
	if err := json.Unmarshal([]byte(content), &bare); err == nil {
		return []byte(content), bare, false, nil
	}

	block, ok := extractLastFencedJSON(content)
	if !ok {
		return nil, loop.Record{}, false, fmt.Errorf("content has no closing json fence and does not parse whole as a Record")
	}

	var fencedRecord loop.Record
	if err := json.Unmarshal(block, &fencedRecord); err != nil {
		return nil, loop.Record{}, false, fmt.Errorf("last fenced json block does not parse as a Record: %w", err)
	}
	return block, fencedRecord, true, nil
}

func extractLastFencedJSON(content string) ([]byte, bool) {
	openAt := strings.LastIndex(content, fenceOpenNeedle)
	if openAt == -1 {
		return nil, false
	}
	start := openAt + len(fenceOpenNeedle)

	closeAt := strings.Index(content[start:], fenceCloseNeedle)
	if closeAt == -1 {
		return nil, false
	}
	return []byte(content[start : start+closeAt]), true
}
