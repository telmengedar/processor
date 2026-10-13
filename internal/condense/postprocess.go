package condense

import (
	"regexp"
	"strings"
	"unicode"
)

const fenceMarker = "```"

var (
	preamblePattern = regexp.MustCompile(`(?i)^(here|below|sure|okay|this is|i have)`)
	emphasisPattern = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	spacePattern    = regexp.MustCompile(`[ \t]+`)
)

const passageLines = 2

// Postprocess strips an outermost code fence and markdown emphasis from a raw condensation, and rewrites nothing else.
func Postprocess(raw string) string {
	return strings.TrimSpace(emphasisPattern.ReplaceAllString(stripFence(raw), "$1"))
}

// Defect names the storage rule a postprocessed condensation fails, or the empty string when it may be written.
func Defect(substance string) string {
	switch {
	case substance == "":
		return skipEmptyCondensation
	case preamblePattern.MatchString(firstLine(substance)):
		return skipPreamble
	}
	if count, _ := repeatedPassages(substance); count > 0 {
		return skipRepeatedPassage
	}
	return ""
}

func repeatedPassages(substance string) (int, int) {
	lines := contentLines(substance)
	counts := make(map[string]int)
	for i := 0; i+passageLines <= len(lines); i++ {
		counts[passageKey(lines[i:i+passageLines])]++
	}

	count, first := 0, 0
	seen := make(map[string]bool)
	for i := 0; i+passageLines <= len(lines); i++ {
		key := passageKey(lines[i : i+passageLines])
		if seen[key] {
			continue
		}
		seen[key] = true
		if counts[key] >= 2 {
			count++
			if first == 0 {
				first = lines[i].number
			}
		}
	}
	return count, first
}

type numberedLine struct {
	number int
	text   string
}

func passageKey(window []numberedLine) string {
	texts := make([]string, len(window))
	for i, line := range window {
		texts[i] = line.text
	}
	return strings.Join(texts, "\n")
}

func contentLines(text string) []numberedLine {
	var lines []numberedLine
	for i, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		line = spacePattern.ReplaceAllString(strings.TrimSpace(line), " ")
		if strings.HasPrefix(line, fenceMarker) || !strings.ContainsFunc(line, isLetterOrDigit) {
			continue
		}
		lines = append(lines, numberedLine{number: i + 1, text: line})
	}
	return lines
}

func isLetterOrDigit(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func stripFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 2 {
		return trimmed
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), fenceMarker) {
		return trimmed
	}
	if strings.TrimSpace(lines[len(lines)-1]) != fenceMarker {
		return trimmed
	}
	return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
