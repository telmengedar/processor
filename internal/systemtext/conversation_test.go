package systemtext_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telmengedar/processor/internal/loop"
	"github.com/telmengedar/processor/internal/systemtext"
)

const (
	conversationParagraph = `This request continues a conversation between you and the user. The section headed "EARLIER IN THIS CONVERSATION" holds what was said before it, oldest first: a "user:" line is what the user said, and a "you:" line is what you replied. You took part in that conversation, so you know what was said in it. It is not part of the context block, and the block not mentioning something said there does not make it unknown to you. Read the request in the light of those exchanges: when it refers to someone or something without naming it, it means what was being talked about there. When the request is a remark rather than a question, reply to the remark; do not answer it with a statement that information is missing.`

	basisWithoutConversation = "Base it on the context block and on the request."
	basisWithConversation    = "Base it on the earlier conversation, the context block and the request."
)

type toolSubset struct {
	name    string
	offered []string
}

func everyToolSubset() []toolSubset {
	every := []string{loop.ToolRecall, loop.ToolReadNode, loop.ToolWriteFile}

	var subsets []toolSubset
	for mask := 0; mask < 1<<len(every); mask++ {
		var offered []string
		for i, tool := range every {
			if mask&(1<<i) != 0 {
				offered = append(offered, tool)
			}
		}
		name := "none"
		if len(offered) > 0 {
			name = strings.Join(offered, "+")
		}
		subsets = append(subsets, toolSubset{name: name, offered: offered})
	}
	return subsets
}

func textBeforeConversations(t *testing.T, subset toolSubset) string {
	t.Helper()

	golden, err := os.ReadFile(filepath.Join("testdata", "without_conversation_"+subset.name+".txt"))
	if err != nil {
		t.Fatalf("reading the fixture for the tool set %s: %v", subset.name, err)
	}
	return string(golden)
}

func TestTheSystemTextWithoutAConversationIsByteIdenticalToTheTextBeforeConversations(t *testing.T) {
	t.Parallel()

	for _, subset := range everyToolSubset() {
		want := textBeforeConversations(t, subset)
		if strings.Contains(want, "\r") {
			t.Fatalf("the fixture for the tool set %s holds a carriage return, so it was not stored with LF line endings", subset.name)
		}
		if got := systemtext.Compose(subset.offered, false, false); got != want {
			t.Errorf("offering %s without a conversation, the system text differs from the text before conversations existed:\n%q\nwant\n%q", subset.name, got, want)
		}
	}
}

func TestTheSystemTextOfAConversationTurnCarriesTheConversationParagraphDirectlyBeforeTheAnswer(t *testing.T) {
	t.Parallel()

	if got := len(conversationParagraph); got != 713 {
		t.Fatalf("the paragraph this guard holds the system text to is %d bytes, want 713", got)
	}

	for _, subset := range everyToolSubset() {
		before := textBeforeConversations(t, subset)
		got := systemtext.Compose(subset.offered, true, false)

		if count := strings.Count(got, conversationParagraph); count != 1 {
			t.Errorf("offering %s, the paragraph occurs %d times, want exactly once", subset.name, count)
			continue
		}

		position := strings.Index(before, basisWithoutConversation)
		answerStart := strings.LastIndex(before[:position], "\n\n") + len("\n\n")
		wantPrefix := before[:answerStart] + conversationParagraph + "\n\n"
		if !strings.HasPrefix(got, wantPrefix) {
			t.Errorf("offering %s, the text does not carry the paragraph directly before the answer paragraph, after every tool paragraph:\n%q\nwant it to begin\n%q", subset.name, got, wantPrefix)
			continue
		}

		reverted := strings.Replace(strings.Replace(got, conversationParagraph+"\n\n", "", 1), basisWithConversation, basisWithoutConversation, 1)
		if reverted != before {
			t.Errorf("offering %s, taking out the paragraph and reverting the answer sentence gives\n%q\nwant the text before conversations\n%q: something else differs", subset.name, reverted, before)
		}
	}
}

func TestTheAnswerParagraphNamesTheEarlierConversationOnlyWhenOneIsPresent(t *testing.T) {
	t.Parallel()

	for _, subset := range everyToolSubset() {
		for _, tc := range []struct {
			conversation bool
			want         string
			unwanted     string
		}{
			{false, basisWithoutConversation, basisWithConversation},
			{true, basisWithConversation, basisWithoutConversation},
		} {
			text := systemtext.Compose(subset.offered, tc.conversation, false)
			if strings.Count(text, tc.want) != 1 {
				t.Errorf("offering %s with conversation %v, the system text does not carry %q exactly once", subset.name, tc.conversation, tc.want)
			}
			if strings.Contains(text, tc.unwanted) {
				t.Errorf("offering %s with conversation %v, the system text carries %q", subset.name, tc.conversation, tc.unwanted)
			}
		}
	}
}

func TestTheAnsweringCallOfAConversationTurnNamesNoToolAndCarriesTheConversationParagraph(t *testing.T) {
	t.Parallel()

	text := systemtext.Compose(nil, true, false)

	if !strings.Contains(text, conversationParagraph) {
		t.Errorf("the system text of the answering call of a conversation turn does not carry the paragraph: the section is on that call and nothing would tell the model what it is")
	}

	for _, forbidden := range []string{"tool", "recall", "search process", "query"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Errorf("the system text of the answering call of a conversation turn contains %q: that call describes only answering, and a sentence about a tool the call withholds is what invites the model to emit one", forbidden)
		}
	}
}
