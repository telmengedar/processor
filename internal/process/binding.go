package process

import (
	"fmt"
	"regexp"
	"strings"
)

var pathKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func pathValue(v any, path string) (any, error) {
	if path == "" {
		return v, nil
	}
	for _, key := range strings.Split(path, ".") {
		if !pathKey.MatchString(key) {
			return nil, fmt.Errorf("invalid binding path")
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("binding is not an object")
		}
		v, ok = m[key]
		if !ok {
			return nil, fmt.Errorf("binding path missing: %s", path)
		}
	}
	return v, nil
}

func bind(v any, input, state map[string]any) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		if source, ok := x["source"]; ok {
			path, pok := x["path"].(string)
			if len(x) != 2 || !pok {
				return nil, fmt.Errorf("invalid source binding")
			}
			switch source {
			case "input":
				return pathValue(input, path)
			case "state":
				return pathValue(state, path)
			default:
				return nil, fmt.Errorf("unknown binding source")
			}
		}
		r := map[string]any{}
		for k, value := range x {
			b, err := bind(value, input, state)
			if err != nil {
				return nil, err
			}
			r[k] = b
		}
		return r, nil
	case []any:
		r := make([]any, len(x))
		for i, value := range x {
			b, err := bind(value, input, state)
			if err != nil {
				return nil, err
			}
			r[i] = b
		}
		return r, nil
	default:
		return v, nil
	}
}

func checkBinding(v any, inputs map[string]Declaration, keys map[string]bool) error {
	switch x := v.(type) {
	case map[string]any:
		if source, ok := x["source"]; ok {
			path, pok := x["path"].(string)
			if len(x) != 2 || !pok {
				return fmt.Errorf("invalid binding")
			}
			parts := strings.Split(path, ".")
			if path != "" {
				for _, p := range parts {
					if !pathKey.MatchString(p) {
						return fmt.Errorf("invalid binding path")
					}
				}
			}
			switch source {
			case "input":
				if path != "" {
					if _, ok := inputs[parts[0]]; !ok {
						return fmt.Errorf("undeclared input binding")
					}
				}
			case "state":
				if path != "" && !keys[parts[0]] {
					return fmt.Errorf("unknown state binding")
				}
			default:
				return fmt.Errorf("invalid binding source")
			}
			return nil
		}
		for _, value := range x {
			if err := checkBinding(value, inputs, keys); err != nil {
				return err
			}
		}
	case []any:
		for _, value := range x {
			if err := checkBinding(value, inputs, keys); err != nil {
				return err
			}
		}
	}
	return nil
}
