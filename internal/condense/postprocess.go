package condense

import (
	"regexp"
	"strings"
)

const fenceMarker = "```"

var (
	preamblePattern = regexp.MustCompile(`(?i)^(here|below|sure|okay|this is|i have)`)
	emphasisPattern = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	spacePattern    = regexp.MustCompile(`[ \t]+`)
)

const passageLines = 3

// Postprocess strips an outermost code fence and markdown emphasis from a raw condensation, and rewrites nothing else.
func Postprocess(raw string) string {
	return strings.TrimSpace(emphasisPattern.ReplaceAllString(stripFence(raw), "$1"))
}

// Defect names the storage rule a postprocessed condensation of content fails, or the empty string when it may be written.
func Defect(content, substance string) string {
	switch {
	case substance == "":
		return skipEmptyCondensation
	case preamblePattern.MatchString(firstLine(substance)):
		return skipPreamble
	}
	if count, _ := repeatedPassages(content, substance); count > 0 {
		return skipRepeatedPassage
	}
	return ""
}

func repeatedPassages(content, substance string) (int, int) {
	bodyCounts := passageCounts(normalisedLines(content))
	lines := normalisedLines(substance)
	substanceCounts := passageCounts(lines)

	count, first := 0, 0
	seen := make(map[string]bool)
	for i := 0; i+passageLines <= len(lines); i++ {
		passage := strings.Join(lines[i:i+passageLines], "\n")
		if seen[passage] {
			continue
		}
		seen[passage] = true
		if substanceCounts[passage] >= 2 && substanceCounts[passage] > bodyCounts[passage] {
			count++
			if first == 0 {
				first = i + 1
			}
		}
	}
	return count, first
}

func passageCounts(lines []string) map[string]int {
	counts := make(map[string]int)
	for i := 0; i+passageLines <= len(lines); i++ {
		counts[strings.Join(lines[i:i+passageLines], "\n")]++
	}
	return counts
}

func normalisedLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		line = spacePattern.ReplaceAllString(strings.TrimSpace(emphasisPattern.ReplaceAllString(line, "$1")), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
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
