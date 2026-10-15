package loop

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	echoUnfinishedPhrase = "E1"
	echoInputFragment    = "E2"
	echoDerivedQuery     = "E3"

	minUnfinishedPhraseWords = 3
	minInputFragmentWords    = 2
)

type echoReader struct {
	lines   [][]string
	queries [][]string
}

func newEchoReader(input string, queries []string) echoReader {
	reader := echoReader{}
	for _, line := range strings.Split(input, "\n") {
		if words := wordsOf(line); len(words) > 0 {
			reader.lines = append(reader.lines, words)
		}
	}
	for _, query := range queries {
		if query == input {
			continue
		}
		if words := wordsOf(query); len(words) > 0 {
			reader.queries = append(reader.queries, words)
		}
	}
	return reader
}

func wordsOf(text string) []string {
	var words []string
	var current []rune
	runes := []rune(text)
	flush := func() {
		if len(current) > 0 {
			words = append(words, strings.ToLower(string(current)))
			current = current[:0]
		}
	}
	for i, r := range runes {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			current = append(current, r)
		case r == '\'' && len(current) > 0 && i+1 < len(runes) && (unicode.IsLetter(runes[i+1]) || unicode.IsNumber(runes[i+1])):
			current = append(current, r)
		default:
			flush()
		}
	}
	flush()
	return words
}

func endsInLetterOrDigit(text string) bool {
	last, size := utf8.DecodeLastRuneInString(strings.TrimRightFunc(text, unicode.IsSpace))
	return size > 0 && (unicode.IsLetter(last) || unicode.IsNumber(last))
}

func containsRun(words, run []string) bool {
	for start := 0; start+len(run) <= len(words); start++ {
		if equalWords(words[start:start+len(run)], run) {
			return true
		}
	}
	return false
}

func equalWords(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (e echoReader) shapeOf(answer string, silence bool) string {
	text := strings.TrimSpace(answer)
	if !IsPhrase(text) || strings.Contains(text, "\n") {
		return ""
	}
	if _, reference := partReferenceOf(text); reference {
		return ""
	}
	if silence && choseSilence(text) {
		return ""
	}
	if opensAsToolCall(text) || containsProcessorSyntax(text) {
		return ""
	}

	words := wordsOf(text)
	switch {
	case endsInLetterOrDigit(text) && len(words) >= minUnfinishedPhraseWords:
		return echoUnfinishedPhrase
	case len(words) >= minInputFragmentWords && e.isInputFragment(words):
		return echoInputFragment
	case len(words) > 0 && e.isDerivedQuery(words):
		return echoDerivedQuery
	default:
		return ""
	}
}

func (e echoReader) isInputFragment(words []string) bool {
	for _, line := range e.lines {
		if containsRun(line, words) {
			return true
		}
	}
	return false
}

func (e echoReader) isDerivedQuery(words []string) bool {
	for _, query := range e.queries {
		if equalWords(query, words) {
			return true
		}
	}
	return false
}

func (e echoReader) recovered(result JudgeResult, kind TrailerKind, silence bool) (JudgeResult, string) {
	if kind != TrailerClause || result.Reason != Answered || result.UnhonouredCall != "" {
		return result, ""
	}
	shape := e.shapeOf(result.deliveredText(), silence)
	if shape == "" {
		return result, ""
	}
	result.Reason = WantsRecall
	result.RecallQuery = strings.TrimSpace(result.deliveredText())
	result.ToolSource = ToolSourceEcho
	return result, shape
}

var callNames = []string{"recall", "read_node", "write_file", "write_note", "call"}

func opensAsToolCall(text string) bool {
	opening := strings.TrimLeftFunc(text, unicode.IsSpace)
	for _, name := range callNames {
		if len(opening) > len(name) && strings.EqualFold(opening[:len(name)], name) && strings.ContainsRune(":{(", rune(opening[len(name)])) {
			return true
		}
	}
	return carriesCallMarkup(text)
}

func containsProcessorSyntax(text string) bool {
	return opensWithProcessorSyntax(text) || strings.Contains(text, "=====")
}
