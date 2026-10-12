package loop

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SegmentTitleRunes bounds the title an outline line carries for one segment.
const SegmentTitleRunes = 80

const maxHeadingLevel = 3

// Segments splits content at markdown headings of level 1 to 3 into pieces of at most limit bytes that concatenate back to content.
func Segments(content string, limit int) []string {
	if content == "" {
		return nil
	}
	limit = max(limit, utf8.UTFMax)

	var segments []string
	for _, section := range splitAtHeadings(content) {
		segments = append(segments, fitting(section, limit, 0)...)
	}
	return segments
}

// Outline renders a node's table of contents: one line per segment with its index, title and size.
func Outline(id int64, name string, size int, segments []string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "outline of part %d, %q: %d bytes in %d segments. Read one by its index.\n", id, name, size, len(segments))
	for i, segment := range segments {
		fmt.Fprintf(&b, "%d. %s (%d bytes)\n", i+1, segmentTitle(segment), len(segment))
	}
	return b.String()
}

func splitAtHeadings(content string) []string {
	var sections []string
	start := 0
	fenced := false

	for offset, line := range lines(content) {
		if isFence(line) {
			fenced = !fenced
			continue
		}
		if !fenced && offset > start && isHeading(line) {
			sections = append(sections, content[start:offset])
			start = offset
		}
	}
	return append(sections, content[start:])
}

func lines(text string) func(yield func(int, string) bool) {
	return func(yield func(int, string) bool) {
		offset := 0
		for offset < len(text) {
			end := len(text)
			if newline := strings.IndexByte(text[offset:], '\n'); newline >= 0 {
				end = offset + newline + 1
			}
			if !yield(offset, text[offset:end]) {
				return
			}
			offset = end
		}
	}
}

func isFence(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

func isHeading(line string) bool {
	level := len(line) - len(strings.TrimLeft(line, "#"))
	if level < 1 || level > maxHeadingLevel || level == len(line) {
		return false
	}
	return line[level] == ' ' || line[level] == '\t'
}

var splitters = []func(string) []string{splitParagraphs, splitLines, splitCharacters}

func fitting(text string, limit, level int) []string {
	if len(text) <= limit {
		return []string{text}
	}

	var pieces []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			pieces = append(pieces, current.String())
			current.Reset()
		}
	}

	for _, unit := range splitters[level](text) {
		if len(unit) > limit {
			flush()
			pieces = append(pieces, fitting(unit, limit, level+1)...)
			continue
		}
		if current.Len()+len(unit) > limit {
			flush()
		}
		current.WriteString(unit)
	}
	flush()

	return pieces
}

func splitParagraphs(text string) []string {
	var units []string
	start := 0
	blank := false

	for offset, line := range lines(text) {
		isBlank := strings.TrimSpace(line) == ""
		if blank && !isBlank {
			units = append(units, text[start:offset])
			start = offset
		}
		blank = isBlank
	}
	return append(units, text[start:])
}

func splitLines(text string) []string {
	var units []string
	for _, line := range lines(text) {
		units = append(units, line)
	}
	return units
}

func splitCharacters(text string) []string {
	var units []string
	for len(text) > 0 {
		_, width := utf8.DecodeRuneInString(text)
		units = append(units, text[:width])
		text = text[width:]
	}
	return units
}

func segmentTitle(segment string) string {
	for _, line := range lines(segment) {
		title := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if title == "" {
			continue
		}
		if runes := []rune(title); len(runes) > SegmentTitleRunes {
			return string(runes[:SegmentTitleRunes-1]) + "…"
		}
		return title
	}
	return "(blank)"
}
