package process

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

func templatePaths(text string) ([]string, error) {
	paths := []string{}
	_, err := scanTemplate(text, func(path string) (string, error) { paths = append(paths, path); return "", nil })
	return paths, err
}

func scanTemplate(text string, lookup func(string) (string, error)) (string, error) {
	if !utf8.ValidString(text) {
		return "", fmt.Errorf("invalid template Unicode")
	}
	var out strings.Builder
	for len(text) > 0 {
		switch {
		case strings.HasPrefix(text, "{{{{"):
			out.WriteString("{{")
			text = text[4:]
		case strings.HasPrefix(text, "}}}}"):
			out.WriteString("}}")
			text = text[4:]
		case strings.HasPrefix(text, "{{"):
			end := strings.Index(text[2:], "}}")
			if end < 0 {
				return "", fmt.Errorf("unclosed placeholder")
			}
			path := text[2 : 2+end]
			if !strings.HasPrefix(path, "context.") {
				return "", fmt.Errorf("unsupported placeholder")
			}
			path = strings.TrimPrefix(path, "context.")
			for _, key := range strings.Split(path, ".") {
				if !pathKey.MatchString(key) {
					return "", fmt.Errorf("invalid placeholder path")
				}
			}
			value, err := lookup(path)
			if err != nil {
				return "", err
			}
			if !utf8.ValidString(value) {
				return "", fmt.Errorf("invalid substituted Unicode")
			}
			out.WriteString(value)
			text = text[2+end+2:]
		case strings.HasPrefix(text, "}}"):
			return "", fmt.Errorf("unmatched placeholder closing")
		default:
			out.WriteByte(text[0])
			text = text[1:]
		}
	}
	return out.String(), nil
}

func renderTemplate(text string, data map[string]any) (string, error) {
	return scanTemplate(text, func(path string) (string, error) {
		value, err := v2Path(data, path)
		if err != nil {
			return "", err
		}
		if !validContextText(value) {
			return "", fmt.Errorf("invalid substituted Unicode")
		}
		if s, ok := value.(string); ok {
			return s, nil
		}
		encoded, err := json.Marshal(value)
		return string(encoded), err
	})
}

func validContextText(value any) bool {
	switch x := value.(type) {
	case string:
		return utf8.ValidString(x)
	case map[string]any:
		for k, v := range x {
			if !utf8.ValidString(k) || !validContextText(v) {
				return false
			}
		}
	case []any:
		for _, v := range x {
			if !validContextText(v) {
				return false
			}
		}
	}
	return true
}
