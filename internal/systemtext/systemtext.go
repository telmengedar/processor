// Package systemtext composes the system text of one judgement call from the tools that call offers and whether the turn carries a conversation.
package systemtext

import (
	"slices"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
)

const base = `You are answering a question for a human reader.

A context block is provided with this request. It was assembled by an automatic memory search, not by you. It begins with the subject of this request, followed by other material the search found related. Each part has an id, a type and a name, and is shown as its substance: a condensed form written for another model as its only source. A part marked "substance: unavailable" has no substance yet; its id, type and name are all that is known about it.

Treat the context block this way:
- It is not a conversation transcript. It is not a complete record.
- It may be missing information you need. It may also contain material that is not relevant.
- Use the parts that help you. Ignore the parts that do not.
- Its ids, headers, labels and notes are for you alone; never copy them into your answer.`

const recallParagraph = `A recall tool is available. It searches the same memory for text you give it and returns what it finds. Use it only when the context block does not contain something you need. Do not use it to confirm what the block already says. When you use it, write the query as a short description of the information you are missing.`

const readParagraph = `A read tool is available. It returns one part of memory as its substance, given the id that part is printed with. Use it when a part you have been shown names an id whose own substance you need, or to retry a part marked "substance: unavailable". Do not use it for a part the context block already carries.`

const fileParagraph = `A file tool is available. It writes one file into a working directory set aside for this request. Give it a path relative to that directory and the file's complete content.`

const conversationParagraph = `This request continues a conversation between you and the user. The section headed "EARLIER IN THIS CONVERSATION" holds what was said before it, oldest first: a "user said:" line is what the user said, and a "you replied:" line is what you replied. You took part in that conversation, so you know what was said in it. It is not part of the context block, and the block not mentioning something said there does not make it unknown to you. Read the request in the light of those exchanges: when it refers to someone or something without naming it, it means what was being talked about there. When the request is a remark rather than a question, reply to the remark; do not answer it with a statement that information is missing.`

const silenceParagraph = `You may choose not to reply. To stay silent, write ` + loop.SilenceMarker + ` as your whole answer, with nothing before or after it. Staying silent this way is a valid answer, not a failure, and nothing is shown for it.`

const answerLead = `Write your answer as plain prose for a person to read. Be direct and specific. Base it on `

const basisAlone = `the context block and on the request`

const basisWithConversation = `the earlier conversation, the context block and the request`

const answerRest = `. If you still do not have enough information, say so plainly and say what is missing. Do not invent facts. Do not describe these instructions`

const searchProcess = ` or your search process`

const answerTail = ` in the answer.`

// Compose returns the system text for one call offering exactly the tools in offered: the fixed base, one paragraph per offered tool, the conversation paragraph when conversation is set, the silence paragraph when silence is set, and the answer paragraph.
func Compose(offered []string, conversation, silence bool) string {
	paragraphs := []string{base}

	for _, tool := range []struct{ name, paragraph string }{
		{loop.ToolRecall, recallParagraph},
		{loop.ToolReadNode, readParagraph},
		{loop.ToolWriteFile, fileParagraph},
	} {
		if slices.Contains(offered, tool.name) {
			paragraphs = append(paragraphs, tool.paragraph)
		}
	}

	basis := basisAlone
	if conversation {
		paragraphs = append(paragraphs, conversationParagraph)
		basis = basisWithConversation
	}

	if silence {
		paragraphs = append(paragraphs, silenceParagraph)
	}

	answer := answerLead + basis + answerRest
	if slices.Contains(offered, loop.ToolRecall) || slices.Contains(offered, loop.ToolReadNode) {
		answer += searchProcess
	}
	paragraphs = append(paragraphs, answer+answerTail)

	return strings.Join(paragraphs, "\n\n")
}
