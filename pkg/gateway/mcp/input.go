package mcp

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// input is a thin wrapper around the raw JSON arguments from an MCP tool call.
// It provides typed accessors that return zero values when a key is missing.
type input map[string]interface{}

// parseInput unmarshals the raw JSON arguments into an input map.
func parseInput(raw json.RawMessage) (input, error) {
	if len(raw) == 0 {
		return input{}, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return input(m), nil
}

// String returns the string value for a key, or "" if missing/wrong type.
func (in input) String(key string) string {
	v, ok := in[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// Int returns the integer value for a key, or the given default if missing.
func (in input) Int(key string, def int) int {
	v, ok := in[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return def
		}
		return int(i)
	default:
		return def
	}
}

// Bool returns the boolean value for a key, or false if missing.
func (in input) Bool(key string) bool {
	v, ok := in[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}

// BoolPtr returns a *bool for a key, or nil if missing.
func (in input) BoolPtr(key string) *bool {
	v, ok := in[key]
	if !ok {
		return nil
	}
	b, ok := v.(bool)
	if !ok {
		return nil
	}
	return &b
}

// StringSlice returns the string slice for a key, or nil if missing.
func (in input) StringSlice(key string) []string {
	v, ok := in[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// setIfNonEmpty adds the value to the map if it is not empty.
func setIfNonEmpty(params map[string]string, key, value string) {
	if value != "" {
		params[key] = value
	}
}

// setInt adds the int value to the map if it is > 0.
func setInt(params map[string]string, key string, value int) {
	if value > 0 {
		params[key] = strconv.Itoa(value)
	}
}

// JSONFilterParam returns a JSON filter value for API query params (body, headers, etc.).
// Accepts a JSON string or object from MCP tool arguments.
func (in input) JSONFilterParam(key string) (string, error) {
	v, ok := in[key]
	if !ok {
		return "", nil
	}
	switch val := v.(type) {
	case string:
		return val, nil
	case map[string]interface{}:
		b, err := json.Marshal(val)
		if err != nil {
			return "", fmt.Errorf("%s: invalid JSON object: %w", key, err)
		}
		return string(b), nil
	default:
		return "", fmt.Errorf("%s must be a JSON string or object", key)
	}
}

// setJSONFilter adds a JSON filter param when present and valid.
func setJSONFilter(params map[string]string, key string, in input) error {
	value, err := in.JSONFilterParam(key)
	if err != nil {
		return err
	}
	setIfNonEmpty(params, key, value)
	return nil
}

// setPayloadSearchFilters forwards body, headers, parsed_query, and path list filters.
func setPayloadSearchFilters(params map[string]string, in input) error {
	for _, key := range []string{"body", "headers", "parsed_query", "path"} {
		if err := setJSONFilter(params, key, in); err != nil {
			return err
		}
	}
	return nil
}
