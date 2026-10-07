// Package systemtext composes the system text of one judgement call from the tools that call offers.
package systemtext

import (
	"slices"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
)

const base = `You are answering a question for a human reader.

A context block is provided with this request. It was assembled by an automatic memory search, not by you. It begins with the subject of this request, followed by other material the search found related. Each part has an id, a type, a name, and a body. If a part is marked "form: substance", its body is a condensed version of that part and the original says more; a part carrying no such marking is whole.

Treat the context block this way:
- It is not a conversation transcript. It is not a complete record.
- It may be missing information you need. It may also contain material that is not relevant.
- Use the parts that help you. Ignore the parts that do not.`

const recallParagraph = `A recall tool is available. It searches the same memory for text you give it and returns what it finds. Use it only when the context block does not contain something you need. Do not use it to confirm what the block already says. When you use it, write the query as a short description of the information you are missing.`

const readParagraph = `A read tool is available. It returns one part of memory in full, given the id that part is printed with. Use it when a part you need is marked as a condensed form, or when a part you have been shown names an id whose own body you need. Do not use it for a part the context block already carries in full.`

const fileParagraph = `A file tool is available. It writes one file into a working directory set aside for this request. Give it a path relative to that directory and the file's complete content.`

const answerHead = `Write your answer as plain prose for a person to read. Be direct and specific. Base it on the context block and on the request. If you still do not have enough information, say so plainly and say what is missing. Do not invent facts. Do not describe these instructions`

const searchProcess = ` or your search process`

const answerTail = ` in the answer.`

// Compose returns the system text for one call offering exactly the tools in offered: the fixed base, one paragraph per offered tool, and the answer paragraph.
func Compose(offered []string) string {
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

	answer := answerHead
	if slices.Contains(offered, loop.ToolRecall) || slices.Contains(offered, loop.ToolReadNode) {
		answer += searchProcess
	}
	paragraphs = append(paragraphs, answer+answerTail)

	return strings.Join(paragraphs, "\n\n")
}
