// Package runconfig decodes the config node a run request names: a strict JSON object whose filter processor forwards to the graph unread.
package runconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/telmengedar/processor/internal/loop"
)

// NodeType is the graph node type a config must have; a node of any other type is no config.
const NodeType = "processor-config"

const (
	filterKey  = "filter"
	silenceKey = "silence"
)

// Config is what a config node's content carries: the filter forwarded to the graph and whether the run offers the model a chosen silence.
type Config struct {
	Filter  loop.Filter
	Silence bool
}

var ownedKeys = []string{"query", "queries", "count", "continue", "sort", "descending", "fields", "id"}

var forwardedNames = []string{
	"type", "name", "status", "nostatus", "linkedto", "path", "bounds", "minSimilarity",
	"excludeId", "excludeName", "updatedFrom", "updatedTo", "createdFrom", "createdTo",
	"severity", "severityMin", "severityMax", "noSeverity", "refinement", "norefinement",
	"rootNodeId", "noRootNodeId",
}

// ReadFilter is the filter a config node is read under, so the graph answers with nothing for a node of any other type.
func ReadFilter() loop.Filter {
	return loop.Filter{"type": {NodeType}}
}

// Decode returns what the config content carries, the filter nil when it carries none; the first violation is named in the error.
func Decode(content string) (Config, error) {
	top, err := objectOf([]byte(content))
	if err != nil {
		return Config{}, fmt.Errorf("the config is not a JSON object: %w", err)
	}

	for _, name := range slices.Sorted(maps.Keys(top)) {
		if name != filterKey && name != silenceKey {
			return Config{}, fmt.Errorf("the config has the key %q, which processor does not know", name)
		}
	}

	var config Config
	if raw, present := top[silenceKey]; present {
		switch string(bytes.TrimSpace(raw)) {
		case "true":
			config.Silence = true
		case "false":
		default:
			return Config{}, fmt.Errorf("the config's %q is not a JSON boolean", silenceKey)
		}
	}

	filter, err := filterOf(top)
	if err != nil {
		return Config{}, err
	}
	config.Filter = filter
	return config, nil
}

func filterOf(top map[string]json.RawMessage) (loop.Filter, error) {
	raw, present := top[filterKey]
	if !present {
		return nil, nil
	}

	entries, err := objectOf(raw)
	if err != nil {
		return nil, fmt.Errorf("the config's %q is not a JSON object: %w", filterKey, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("the config's %q is empty", filterKey)
	}

	filter := make(loop.Filter, len(entries))
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		if slices.ContainsFunc(ownedKeys, func(owned string) bool { return strings.EqualFold(owned, name) }) {
			return nil, fmt.Errorf("the filter key %q is one processor sets itself on every read", name)
		}
		if !slices.Contains(forwardedNames, name) {
			return nil, fmt.Errorf("the filter key %q is not a graph filter name processor forwards", name)
		}

		values, err := valuesOf(entries[name])
		if err != nil {
			return nil, fmt.Errorf("the filter key %q: %w", name, err)
		}
		filter[name] = values
	}
	return filter, nil
}

func objectOf(raw []byte) (map[string]json.RawMessage, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return nil, errors.New("not an object")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	var object map[string]json.RawMessage
	if err := decoder.Decode(&object); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("text follows the object")
	}
	return object, nil
}

func valuesOf(raw json.RawMessage) ([]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	if list, isList := value.([]any); isList {
		if len(list) == 0 {
			return nil, errors.New("an empty array would forward no filter")
		}
		values := make([]string, len(list))
		for i, element := range list {
			text, err := scalarText(element)
			if err != nil {
				return nil, err
			}
			values[i] = text
		}
		return values, nil
	}

	text, err := scalarText(value)
	if err != nil {
		return nil, err
	}
	return []string{text}, nil
}

func scalarText(value any) (string, error) {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "", errors.New("a blank value would forward no filter")
		}
		return v, nil
	case json.Number:
		return v.String(), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return "", errors.New("a value must be a string, a number or a boolean, or a non-empty array of them")
	}
}
