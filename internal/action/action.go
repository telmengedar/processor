// Package action defines the harness's own action protocol: how a model declares work in its response text and how that text is read back.
package action

import (
	"encoding/json"
	"errors"
	"strings"
)

const (
	// Open begins one action block.
	Open = "<processor-action>"
	// Close ends one action block.
	Close = "</processor-action>"
)

const (
	// Recall is the name of the supplementary-memory action.
	Recall = "recall"
	// WriteFile is the name of the file-writing action.
	WriteFile = "write_file"
)

// Protocol is the section of system text that states this protocol to the model.
const Protocol = `When you need something done, write an action block: the line ` + Open + `, then one JSON object, then the line ` + Close + `.

The JSON object has two members: name, the name of the action, and arguments, an object holding that action's arguments. Write nothing else between the two lines.

Two actions exist.

` + Recall + ` — searches the same memory the context block came from and returns what it finds. arguments: query, a short description of the information you are missing. Use it only when the block does not already contain what you need.

` + WriteFile + ` — writes one file into the working directory set aside for this request. arguments: path, a relative path naming the file, and content, the file's complete text.

Write as many action blocks as the request needs. They are carried out in the order you write them and every result comes back to you together. Text outside the blocks is read as your answer, so you may explain and act in the same response.

An action block looks like this:

` + Open + `
{"name": "` + WriteFile + `", "arguments": {"path": "index.html", "content": "<!DOCTYPE html>\n<html></html>\n"}}
` + Close

const (
	reasonUnreadable   = "an action block's JSON could not be read"
	reasonUnterminated = "an action block was opened and never closed"
	reasonNotObject    = "an action block held something other than a JSON object"
	reasonBadMembers   = "an action block's members could not be read"
	reasonNoName       = "an action block carried no action name"
)

// Call is one action block that was read: the action's name and its arguments, still encoded.
type Call struct {
	Name      string
	Arguments json.RawMessage
}

// Problem is one action block that could not be read, carrying the text that could not be read.
type Problem struct {
	Reason string
	Text   string
}

// Result is one response decomposed into the text around its action blocks, the calls it declared, and the blocks that could not be read.
type Result struct {
	Prose    string
	Calls    []Call
	Problems []Problem
}

type envelope struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Parse decomposes content into prose, calls and unreadable blocks, letting the JSON grammar alone decide where a block's object ends.
func Parse(content string) Result {
	var result Result
	var prose strings.Builder

	rest := content
	for {
		start := strings.Index(rest, Open)
		if start < 0 {
			prose.WriteString(rest)
			break
		}
		prose.WriteString(rest[:start])
		block := rest[start+len(Open):]

		raw, after, err := readObject(block)
		if err != nil {
			result.Problems = append(result.Problems, Problem{Reason: err.Error(), Text: Open + block})
			break
		}

		result.take(raw, Open+block[:len(block)-len(after)])
		rest = after
	}

	result.Prose = strings.TrimSpace(prose.String())
	return result
}

func (r *Result) take(raw json.RawMessage, text string) {
	if len(raw) == 0 || raw[0] != '{' {
		r.Problems = append(r.Problems, Problem{Reason: reasonNotObject, Text: text})
		return
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		r.Problems = append(r.Problems, Problem{Reason: reasonBadMembers, Text: text})
		return
	}

	name := strings.TrimSpace(env.Name)
	if name == "" {
		r.Problems = append(r.Problems, Problem{Reason: reasonNoName, Text: text})
		return
	}

	r.Calls = append(r.Calls, Call{Name: name, Arguments: env.Arguments})
}

func readObject(block string) (json.RawMessage, string, error) {
	decoder := json.NewDecoder(strings.NewReader(block))

	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, "", errors.New(reasonUnreadable)
	}

	after := strings.TrimLeft(block[decoder.InputOffset():], " \t\r\n")
	if !strings.HasPrefix(after, Close) {
		return nil, "", errors.New(reasonUnterminated)
	}

	return raw, after[len(Close):], nil
}
