package process

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"
)

type Schema struct {
	Type                 string            `json:"type"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	Required             []string          `json:"required,omitempty"`
	AdditionalProperties bool              `json:"additionalProperties"`
	Items                *Schema           `json:"items,omitempty"`
}

func decode(data string, v any) error {
	if !utf8.ValidString(data) {
		return fmt.Errorf("JSON is not Unicode")
	}
	if err := uniqueKeys(json.NewDecoder(strings.NewReader(data)), 0); err != nil {
		return err
	}
	d := json.NewDecoder(strings.NewReader(data))
	d.DisallowUnknownFields()
	d.UseNumber()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}

func uniqueKeys(d *json.Decoder, depth int) error {
	if depth > 64 {
		return fmt.Errorf("JSON nesting bound exceeded")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	opening, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch opening {
	case '{':
		keys := map[string]bool{}
		for d.More() {
			token, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok || keys[key] {
				return fmt.Errorf("duplicate JSON key")
			}
			keys[key] = true
			if err := uniqueKeys(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueKeys(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}

func (s Schema) check() error {
	switch s.Type {
	case "object":
		if s.Items != nil {
			return fmt.Errorf("object has items")
		}
		seen := map[string]bool{}
		for _, key := range s.Required {
			if _, ok := s.Properties[key]; !ok || seen[key] {
				return fmt.Errorf("invalid required property")
			}
			seen[key] = true
		}
		for _, p := range s.Properties {
			if err := p.check(); err != nil {
				return err
			}
		}
	case "array":
		if s.Items == nil || len(s.Properties) > 0 || len(s.Required) > 0 || s.AdditionalProperties {
			return fmt.Errorf("invalid array schema")
		}
		return s.Items.check()
	case "string", "integer", "number", "boolean":
		if s.Items != nil || len(s.Properties) > 0 || len(s.Required) > 0 || s.AdditionalProperties {
			return fmt.Errorf("invalid scalar schema")
		}
	default:
		return fmt.Errorf("unsupported schema type")
	}
	return nil
}

func (s Schema) validate(v any) error {
	if s.Type == "object" {
		m, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("expected object")
		}
		for _, key := range s.Required {
			if _, ok := m[key]; !ok {
				return fmt.Errorf("required property missing: %s", key)
			}
		}
		for k, x := range m {
			p, ok := s.Properties[k]
			if !ok {
				if !s.AdditionalProperties {
					return fmt.Errorf("unknown property: %s", k)
				}
				continue
			}
			if err := p.validate(x); err != nil {
				return fmt.Errorf("property %s: %w", k, err)
			}
		}
		return nil
	}
	if s.Type == "array" {
		a, ok := v.([]any)
		if !ok {
			return fmt.Errorf("expected array")
		}
		for _, x := range a {
			if err := s.Items.validate(x); err != nil {
				return err
			}
		}
		return nil
	}
	switch s.Type {
	case "string":
		if _, ok := v.(string); !ok {
			return fmt.Errorf("expected string")
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("expected boolean")
		}
	case "number", "integer":
		n, ok := number(v)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) || s.Type == "integer" && n != math.Trunc(n) {
			return fmt.Errorf("expected %s", s.Type)
		}
	default:
		return fmt.Errorf("unsupported schema")
	}
	return nil
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, e := n.Float64()
		return f, e == nil
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func nodeID(v any) (int64, bool) {
	n, ok := number(v)
	if !ok || n <= 0 || n != math.Trunc(n) || n >= float64(math.MaxInt64) {
		return 0, false
	}
	return int64(n), true
}

func jsonValue(v any) any {
	b, _ := json.Marshal(v)
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var x any
	_ = d.Decode(&x)
	return x
}
