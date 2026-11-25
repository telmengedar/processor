package process

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
)

type capability struct {
	token string
	phase string
	call  *selectedCall
	used  bool
}

func (c *capability) MarshalJSON() ([]byte, error) { return json.Marshal(c.token) }

type receiptHandle struct{ MemoryReceipt }

type transportRuntime struct {
	data     map[string]any
	result   any
	receipts []*receiptHandle
	fresh    []*receiptHandle
	step     int64
	visit    int
}

func checkV2Binding(value any, inputs map[string]Declaration, exports bool) error {
	switch x := value.(type) {
	case map[string]any:
		if source, ok := x["source"]; ok {
			path, ok := x["path"].(string)
			if !ok || len(x) != 2 {
				return fmt.Errorf("invalid v2 binding")
			}
			if path != "" {
				for _, key := range strings.Split(path, ".") {
					if !pathKey.MatchString(key) {
						return fmt.Errorf("invalid binding path")
					}
				}
			}
			switch source {
			case "argument":
				first := strings.Split(path, ".")[0]
				if _, ok := inputs[first]; !ok {
					return fmt.Errorf("argument must select a declared parameter")
				}
			case "transport", "receipt":
			case "result":
				if !exports {
					return fmt.Errorf("current result is available only in exports")
				}
			default:
				return fmt.Errorf("unsupported v2 binding source")
			}
			return nil
		}
		for _, v := range x {
			if err := checkV2Binding(v, inputs, exports); err != nil {
				return err
			}
		}
	case []any:
		for _, v := range x {
			if err := checkV2Binding(v, inputs, exports); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *transportRuntime) resolve(value any, args map[string]any) (any, error) {
	switch x := value.(type) {
	case map[string]any:
		if source, ok := x["source"]; ok {
			path, ok := x["path"].(string)
			if !ok || len(x) != 2 {
				return nil, fmt.Errorf("invalid v2 binding")
			}
			switch source {
			case "argument":
				return v2Path(args, path)
			case "transport":
				return v2Path(r.data, path)
			case "result":
				return v2Path(r.result, path)
			case "receipt":
				space := map[string]any{}
				if len(r.receipts) > 0 {
					space["latest"] = r.receipts[len(r.receipts)-1]
				}
				return v2Path(space, path)
			default:
				return nil, fmt.Errorf("unsupported v2 binding source")
			}
		}
		result := map[string]any{}
		for key, v := range x {
			resolved, err := r.resolve(v, args)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil
	case []any:
		result := make([]any, len(x))
		for i, v := range x {
			resolved, err := r.resolve(v, args)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil
	default:
		return value, nil
	}
}

func copyTransport(value any, receipts *[]*receiptHandle) (any, error) {
	switch x := value.(type) {
	case *capability:
		if x.used {
			return nil, failure("capability_revoked", "consumed tool capability cannot be forwarded")
		}
		return x, nil
	case *receiptHandle:
		found := false
		for _, r := range *receipts {
			if r == x {
				found = true
			}
		}
		if !found {
			*receipts = append(*receipts, x)
		}
		return x, nil
	case map[string]any:
		result := map[string]any{}
		for k, v := range x {
			copied, err := copyTransport(v, receipts)
			if err != nil {
				return nil, err
			}
			result[k] = copied
		}
		return result, nil
	case []any:
		result := make([]any, len(x))
		for i, v := range x {
			copied, err := copyTransport(v, receipts)
			if err != nil {
				return nil, err
			}
			result[i] = copied
		}
		return result, nil
	default:
		return value, nil
	}
}

func transportBytes(value any) int {
	b, _ := json.Marshal(value)
	n := len(b)
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case *capability:
			if x.phase == "round" {
				b, _ := json.Marshal(toolRoundMessages(x.call))
				n += len(b)
			} else {
				b, _ := json.Marshal(x.call.Call)
				n += len(b) + len(x.call.Text)
			}
		case map[string]any:
			for _, v := range x {
				walk(v)
			}
		case []any:
			for _, v := range x {
				walk(v)
			}
		}
	}
	walk(value)
	return n
}

func (r *transportRuntime) advance(exports any, args map[string]any, value any) error {
	r.result = value
	if exports == nil {
		exports = map[string]any{}
	}
	resolved, err := r.resolve(exports, args)
	if err != nil {
		return failure("invalid_binding", err.Error())
	}
	receipts := append([]*receiptHandle{}, r.fresh...)
	copied, err := copyTransport(resolved, &receipts)
	if err != nil {
		return err
	}
	data, ok := copied.(map[string]any)
	if !ok {
		return failure("invalid_binding", "exports must be an object")
	}
	if _, exists := data["receipts"]; exists {
		return failure("invalid_binding", "receipts is runtime reserved")
	}
	if len(receipts) > 0 {
		items := make([]any, 0, len(receipts))
		for _, receipt := range receipts {
			items = append(items, receipt)
		}
		data["receipts"] = items
	}
	if transportBytes(data) > MaxContextBytes {
		return failure("transport_bound", "outgoing transport exceeds byte bound")
	}
	r.data = data
	r.result = nil
	r.receipts = receipts
	r.fresh = nil
	return nil
}

func newCapability(call *selectedCall, phase string) *capability {
	return &capability{token: rand.Text(), phase: phase, call: call}
}

func v2Path(value any, path string) (any, error) {
	if path == "" {
		return value, nil
	}
	for _, key := range strings.Split(path, ".") {
		if !pathKey.MatchString(key) {
			return nil, fmt.Errorf("invalid binding path")
		}
		if receipt, ok := value.(*receiptHandle); ok {
			value = jsonValue(receipt)
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("binding is not an object")
		}
		value, ok = object[key]
		if !ok {
			return nil, fmt.Errorf("binding path missing")
		}
	}
	return value, nil
}
