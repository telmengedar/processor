package loop

import (
	"strings"
	"testing"
	"time"
)

const (
	userContentTestBlock = "===== ANCHOR =====\nid: 1\ntype: t\nname: solo\n\njust the anchor\n"
	userContentTestInput = "Generate a new barebones webpage and a repo for it."
)

var userContentTestInstant = time.Date(2026, 3, 4, 5, 6, 7, 0, time.FixedZone("test+02", 2*60*60))

const userContentTestInstantRFC3339 = "2026-03-04T05:06:07+02:00"

func TestRenderUserContentOpensWithTheRequestAndKeepsTheTailCopy(t *testing.T) {
	t.Parallel()

	got := RenderUserContent(userContentTestBlock, userContentTestInput, userContentTestInstant, "", "", "")

	want := "===== INPUT =====\n" + userContentTestInput +
		"\n\n===== NOW =====\n" + userContentTestInstantRFC3339 + "\n\n\n" +
		userContentTestBlock +
		"\n===== INPUT =====\n" + userContentTestInput

	if got != want {
		t.Fatalf("user content =\n%q\nwant\n%q", got, want)
	}
}

func TestTheAssembledPromptStatesTheInstantInTheZoneItWasRead(t *testing.T) {
	t.Parallel()

	got := RenderUserContent(userContentTestBlock, userContentTestInput, userContentTestInstant, "", "", "")

	if !strings.Contains(got, userContentTestInstantRFC3339) {
		t.Fatalf("user content does not state %q, so a task saying \"today\" has nothing to resolve it against; content=%q",
			userContentTestInstantRFC3339, got)
	}
	if strings.Contains(got, "03:06:07") {
		t.Fatalf("user content normalises the instant to UTC rather than stating the zone it was read in; content=%q", got)
	}
}

func TestRenderUserContentGivenNoInstantIsByteIdenticalToTheDatelessLayout(t *testing.T) {
	t.Parallel()

	got := RenderUserContent(userContentTestBlock, userContentTestInput, time.Time{}, "", "", "")

	want := "===== INPUT =====\n" + userContentTestInput + "\n\n\n" +
		userContentTestBlock +
		"\n===== INPUT =====\n" + userContentTestInput

	if got != want {
		t.Fatalf("with no instant the user content =\n%q\nwant the dateless layout\n%q", got, want)
	}
}

func TestRenderUserContentStatesTheInstantAloneInTheNowSpan(t *testing.T) {
	t.Parallel()

	got := RenderUserContent(userContentTestBlock, userContentTestInput, userContentTestInstant, "", "", "")

	want := "===== INPUT =====\n" + userContentTestInput +
		"\n\n===== NOW =====\n" + userContentTestInstantRFC3339 + "\n\n\n" +
		userContentTestBlock +
		"\n===== INPUT =====\n" + userContentTestInput

	if got != want {
		t.Fatalf("the NOW span holds more than the instant: got=\n%q\nwant\n%q", got, want)
	}
}

func TestTheUserMessageCarriesNoWindowLine(t *testing.T) {
	t.Parallel()

	got := RenderUserContent(userContentTestBlock, userContentTestInput, userContentTestInstant, "", "", "")

	if strings.Contains(got, "retrieval is limited") {
		t.Fatalf("the user message states a retrieval window nothing derives any more: %q", got)
	}
}

func TestRenderUserContentPlacesExactlyTwoVerbatimRequestCopiesTheFirstAtTheHead(t *testing.T) {
	t.Parallel()

	const (
		marker = "===== INPUT =====\n"
		input  = "Ship it.\r\n\tsecond line — 100% \"done\" <&> %s %%\n\tlast"
	)

	got := RenderUserContent(userContentTestBlock, input, userContentTestInstant, "", "", "")

	sections := strings.Split(got, marker)
	if len(sections) != 3 {
		t.Fatalf("user content carries %d request markers, want exactly 2; content=%q", len(sections)-1, got)
	}
	if sections[0] != "" {
		t.Fatalf("user content does not open with the request; it opens with %q", sections[0])
	}

	blockLayout := "\n\n===== NOW =====\n" + userContentTestInstantRFC3339 + "\n\n\n" + userContentTestBlock + "\n"

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
