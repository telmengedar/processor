package loop

import (
	"strings"
	"testing"
)

const (
	userContentTestBlock = "===== ANCHOR =====\nid: 1\ntype: t\nname: solo\n\njust the anchor\n"
	userContentTestInput = "Generate a new barebones webpage and a repo for it."
)

func TestRenderUserContentOpensWithTheRequestAndKeepsTheTailCopy(t *testing.T) {
	t.Parallel()

	got := RenderUserContent(userContentTestBlock, userContentTestInput)

	want := "===== INPUT =====\n" + userContentTestInput + "\n\n\n" +
		userContentTestBlock +
		"\n===== INPUT =====\n" + userContentTestInput

	if got != want {
		t.Fatalf("user content =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderUserContentPlacesExactlyTwoVerbatimRequestCopiesTheFirstAtTheHead(t *testing.T) {
	t.Parallel()

	const (
		marker = "===== INPUT =====\n"
		input  = "Ship it.\r\n\tsecond line — 100% \"done\" <&> %s %%\n\tlast"
	)

	got := RenderUserContent(userContentTestBlock, input)

	sections := strings.Split(got, marker)
	if len(sections) != 3 {
		t.Fatalf("user content carries %d request markers, want exactly 2; content=%q", len(sections)-1, got)
	}
	if sections[0] != "" {
		t.Fatalf("user content does not open with the request; it opens with %q", sections[0])
	}

	blockLayout := "\n\n\n" + userContentTestBlock + "\n"

	head, ok := strings.CutSuffix(sections[1], blockLayout)
	if !ok {
		t.Fatalf("the span between the two request markers is %q, want it to end with the block layout %q", sections[1], blockLayout)
	}

	tail := sections[2]

	if head != tail {
		t.Fatalf("head request %q and tail request %q differ", head, tail)
	}
	if head != input {
		t.Fatalf("both request copies read %q, want the input verbatim %q", head, input)
	}
}
