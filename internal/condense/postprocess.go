package condense

import (
	"regexp"
	"strings"
)

const substanceFloorFraction = 0.05

const fenceMarker = "```"

var (
	preamblePattern = regexp.MustCompile(`(?i)^(here|below|sure|okay|this is|i have)`)
	emphasisPattern = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
)

// Postprocess strips an outermost code fence and markdown emphasis from a raw condensation, and rewrites nothing else.
func Postprocess(raw string) string {
	return strings.TrimSpace(emphasisPattern.ReplaceAllString(stripFence(raw), "$1"))
}

// Defect names the storage rule a postprocessed condensation fails, or the empty string when it may be written.
func Defect(content, substance string) string {
	switch {
	case substance == "":
		return skipEmptyCondensation
	case len(substance) >= len(content):
		return skipNotShorter
	case float64(len(substance)) < substanceFloorFraction*float64(len(content)):
		return skipBelowFloor
	case preamblePattern.MatchString(firstLine(substance)):
		return skipPreamble
	default:
		return ""
	}
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
