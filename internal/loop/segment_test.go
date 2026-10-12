package loop

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func markdownDocument(sections, bodyBytes int) string {
	var b strings.Builder
	for i := 1; i <= sections; i++ {
		fmt.Fprintf(&b, "## Section %d\n\n%s\n\n", i, strings.Repeat("w", bodyBytes))
	}
	return b.String()
}

func TestSegmentsConcatenateToTheContentByteForByte(t *testing.T) {
	t.Parallel()

	fenced := "# Title\n\n```sh\n# a comment, not a heading\n## nor this\n```\n\n## Real\n\ntext\n"

	for _, tc := range []struct {
		name    string
		content string
		limit   int
	}{
		{"headings of every level", "# One\na\n## Two\nb\n### Three\nc\n#### Four\nd\n", 20_000},
		{"text before the first heading", "preamble\n\n# One\na\n", 20_000},
		{"a fenced block holding heading-looking lines", fenced, 20_000},
		{"CRLF line endings", "# One\r\na\r\n\r\n## Two\r\nb\r\n", 20_000},
		{"no trailing newline", "# One\na\n## Two\nb", 20_000},
		{"no headings and blank lines", strings.Repeat("paragraph of text\n\n", 50), 100},
		{"no headings and no blank lines", strings.Repeat("a line of text\n", 50), 100},
		{"one line longer than the limit", strings.Repeat("x", 1_000), 100},
		{"multi-byte characters cut at the limit", strings.Repeat("äöü€", 300), 101},
		{"a limit below one character", "ä€", 1},
		{"a hash that is not a heading", "#tag\n#\n####### seven\n", 20_000},
		{"a large markdown document", markdownDocument(40, 1_900), 20_000},
		{"a short paragraph before an oversized one", "## H\n\nshort paragraph\n\n" + strings.Repeat("y", 25_000), 20_000},
	} {
		segments := Segments(tc.content, tc.limit)
		if got := strings.Join(segments, ""); got != tc.content {
			t.Errorf("%s: the %d segments concatenate to %d bytes that differ from the content's %d: a segment boundary that drops or repeats a byte hands the model a document that never existed", tc.name, len(segments), len(got), len(tc.content))
		}
	}
}

func TestEverySegmentFitsOneRead(t *testing.T) {
	t.Parallel()

	const limit = SupplementaryByteBudget

	for _, tc := range []struct {
		name         string
		content      string
		wantSegments int
	}{
		{"a single 30 KB section with no inner headings", "## Only\n\n" + strings.Repeat(strings.Repeat("p", 99)+"\n\n", 300), 2},
		{"a single 30 KB line", strings.Repeat("l", 30_000), 2},
		{"a single 30 KB line of multi-byte characters", strings.Repeat("€", 10_000), 2},
		{"a large markdown document, one segment per heading", markdownDocument(40, 1_900), 40},
	} {
		segments := Segments(tc.content, limit)

		if len(segments) != tc.wantSegments {
			t.Errorf("%s: %d segments, want %d", tc.name, len(segments), tc.wantSegments)
		}
		for i, segment := range segments {
			if len(segment) > limit {
				t.Errorf("%s: segment %d is %d bytes, over the %d one read carries: an outline that lists it lists something no read can return", tc.name, i+1, len(segment), limit)
			}
			if !utf8.ValidString(segment) {
				t.Errorf("%s: segment %d is not valid UTF-8: a cut inside a character", tc.name, i+1)
			}
		}
	}
}

func TestASegmentBeginsAtEveryHeadingOfLevelOneToThreeOutsideAFence(t *testing.T) {
	t.Parallel()

	content := "preamble\n# One\na\n## Two\n```\n# not a heading\n```\n### Three\n#### Four stays inside Three\n"

	segments := Segments(content, 20_000)

	want := []string{"preamble\n", "# One\na\n", "## Two\n```\n# not a heading\n```\n", "### Three\n#### Four stays inside Three\n"}
	if strings.Join(segments, "|") != strings.Join(want, "|") {
		t.Errorf("segments = %q, want %q", segments, want)
	}
}

func TestAnEmptyContentHasNoSegments(t *testing.T) {
	t.Parallel()

	if segments := Segments("", 20_000); len(segments) != 0 {
		t.Errorf("an empty content has %d segments, want none", len(segments))
	}
}

func TestTheOutlineNamesTheNodeAndListsEverySegmentByIndexTitleAndSize(t *testing.T) {
	t.Parallel()

	segments := []string{"preamble\n", "## Alpha beta\nbody\n", "\n\n   \n", strings.Repeat("t", 200) + "\n"}

	outline := Outline(71, "A name", 1_000, segments)

	for _, want := range []string{
		`outline of part 71, "A name": 1000 bytes in 4 segments`,
		"1. preamble (9 bytes)\n",
		"2. Alpha beta (19 bytes)\n",
		"3. (blank) (6 bytes)\n",
		"4. " + strings.Repeat("t", SegmentTitleRunes-1) + "… (201 bytes)\n",
	} {
		if !strings.Contains(outline, want) {
			t.Errorf("the outline lacks %q:\n%s", want, outline)
		}
	}
}
