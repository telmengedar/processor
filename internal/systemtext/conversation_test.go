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
	conversationParagraph = `This request continues a conversation. Earlier user messages hold what people said, and earlier assistant messages hold your replies, oldest first. Names and quotations inside a message identify the people being discussed; they do not change who sent that message. Your earlier replies may be mistaken. Preserve who said what, and use later explicit corrections rather than repeating a contradicted claim. The final user message carries the current request and a separate memory context block. Read it in the light of the earlier exchanges: a short follow-up can refer to what was being discussed. Reply to the current question or remark without inventing facts.`

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
		if got := systemtext.Compose(subset.offered, false, false, loop.FormSubstance); got != want {
			t.Errorf("offering %s without a conversation, the system text differs from the text before conversations existed:\n%q\nwant\n%q", subset.name, got, want)
		}
	}
}

func TestTheSystemTextOfAConversationTurnCarriesTheConversationParagraphDirectlyBeforeTheAnswer(t *testing.T) {
	t.Parallel()

	for _, subset := range everyToolSubset() {
		before := textBeforeConversations(t, subset)
		got := systemtext.Compose(subset.offered, true, false, loop.FormSubstance)

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
			text := systemtext.Compose(subset.offered, tc.conversation, false, loop.FormSubstance)
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

	text := systemtext.Compose(nil, true, false, loop.FormSubstance)

	if !strings.Contains(text, conversationParagraph) {
		t.Errorf("the system text of the answering call of a conversation turn does not carry the paragraph: the section is on that call and nothing would tell the model what it is")
	}

	for _, forbidden := range []string{"tool", "recall", "search process", "query"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Errorf("the system text of the answering call of a conversation turn contains %q: that call describes only answering, and a sentence about a tool the call withholds is what invites the model to emit one", forbidden)
		}
	}
}

func TestTheConversationGuidancePreservesNativeOwnershipAndCorrections(t *testing.T) {
	t.Parallel()
	text := systemtext.Compose(nil, true, false, loop.FormSubstance)
	for _, want := range []string{"Earlier user messages", "earlier assistant messages", "Your earlier replies may be mistaken", "use later explicit corrections", "do not change who sent that message"} {
		if !strings.Contains(text, want) {
			t.Errorf("conversation guidance lacks %q", want)
		}
	}
	for _, obsolete := range []string{"user said:", "you replied:", "EARLIER IN THIS CONVERSATION"} {
		if strings.Contains(text, obsolete) {
			t.Errorf("conversation guidance still describes textual label %q", obsolete)
		}
	}
}
