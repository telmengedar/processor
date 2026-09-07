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

func TestRenderUserContentHeadCopyCostsExactlySeventyTwoCharactersOnTheYardstickRequest(t *testing.T) {
	t.Parallel()

	tailOnly := userContentTestBlock + "\n===== INPUT =====\n" + userContentTestInput

	if got := len(RenderUserContent(userContentTestBlock, userContentTestInput)) - len(tailOnly); got != 72 {
		t.Fatalf("the head copy costs %d characters, want the 72 the measured arm added", got)
	}
}

func TestRenderUserContentRendersTheHeadAndTailRequestFromOneSourceSoTheyCannotDrift(t *testing.T) {
	t.Parallel()

	const (
		marker = "===== INPUT =====\n"
		input  = "Ship it.\r\n\tsecond line — 100% \"done\" <&> %s %%\n\tlast\n"
	)

	got := RenderUserContent(userContentTestBlock, input)

	sections := strings.Split(got, marker)
	if len(sections) != 3 {
		t.Fatalf("user content carries %d request markers, want exactly 2; content=%q", len(sections)-1, got)
	}
	if sections[0] != "" {
		t.Fatalf("user content does not open with the request; it opens with %q", sections[0])
	}

	head := strings.TrimSuffix(sections[1], "\n\n\n"+userContentTestBlock+"\n")
	tail := sections[2]

	if head != tail {
		t.Fatalf("head request %q and tail request %q differ; both must render from one source", head, tail)
	}
	if head != input {
		t.Fatalf("both request copies read %q, want the input verbatim %q", head, input)
	}
}
