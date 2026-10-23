package loop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// NoteSource is the caller's attributed original utterance retained with a selected private note.
type NoteSource struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

// ErrInvalidNoteSource marks invalid or incompatible request evidence.
var ErrInvalidNoteSource = errors.New("invalid note source")

// UnmarshalJSON rejects source encodings that would lose Unicode information during decoding.
func (s *NoteSource) UnmarshalJSON(data []byte) error {
	if !validSourceJSON(data) || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return ErrInvalidNoteSource
	}
	type source NoteSource
	var decoded source
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return ErrInvalidNoteSource
	}
	*s = NoteSource(decoded)
	return nil
}

func validSourceJSON(data []byte) bool {
	if !utf8.Valid(data) || !json.Valid(data) {
		return false
	}
	for i := 0; i < len(data); i++ {
		if data[i] != '\\' {
			continue
		}
		i++
		if data[i] != 'u' {
			continue
		}
		code, _ := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		i += 4
		if code >= 0xdc00 && code <= 0xdfff {
			return false
		}
		if code < 0xd800 || code > 0xdbff {
			continue
		}
		if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
			return false
		}
		low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return true
}

func (s *NoteSource) validate(view *View) error {
	if s == nil {
		return nil
	}
	if !noteDecisionMode(view) || !utf8.ValidString(s.Speaker) || !utf8.ValidString(s.Text) || strings.TrimSpace(s.Speaker) == "" || strings.TrimSpace(s.Text) == "" || len(s.Speaker) > 256 || len(s.Text) > 8*1024 {
		return fmt.Errorf("%w: requires decision mode, nonblank Unicode speaker and text within 256 and 8192 bytes", ErrInvalidNoteSource)
	}
	return nil
}

func (s *NoteSource) compose(text string) (string, bool) {
	if s == nil {
		return text, true
	}
	encoded, _ := json.Marshal(s)
	final := text + "\n\nReported source evidence:\n" + string(encoded)
	return final, len(final) <= 20*1024
}
