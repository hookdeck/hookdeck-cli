package mcpcore

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Input is a thin wrapper around the raw JSON arguments from an MCP tool call.
// It provides typed accessors that return zero values when a key is missing.
type Input map[string]interface{}

// ParseInput unmarshals the raw JSON arguments into an Input map.
func ParseInput(raw json.RawMessage) (Input, error) {
	if len(raw) == 0 {
		return Input{}, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return Input(m), nil
}

// String returns the string value for a key, or "" if missing/wrong type.
func (in Input) String(key string) string {
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
//
// A numeric string is accepted as well as a JSON number, for the same reason
// NumberOrString accepts a number as well as a string: models routinely emit
// numbers as JSON strings, and returning the default for limit: "5" silently
// substituted the API's page size for the one that was asked for.
func (in Input) Int(key string, def int) int {
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
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			return def
		}
		return i
	default:
		return def
	}
}

// NumberOrString returns the value for a key as a string, accepting either a
// JSON string or a JSON number.
//
// Several filters take "an integer, or the API's operator syntax" — attempts,
// response_status, events_count. Their schema type is string, because the
// operator form is a string, and String would silently return "" for a model
// that sent the integer as a JSON number. The filter was then dropped and the
// caller got an unfiltered result with nothing to indicate its filter had been
// ignored: a wrong answer that reads as a right one. Sending events_count: 0 to
// find requests that delivered nothing is exactly the case that would break.
//
// Integral values are rendered without a decimal point, so 0 becomes "0" rather
// than "0.000000" — the API parses the query string, and "0.000000" is not zero
// to it.
func (in Input) NumberOrString(key string) string {
	v, ok := in[key]
	if !ok {
		return ""
	}
	switch n := v.(type) {
	case string:
		return n
	case float64:
		if n == math.Trunc(n) {
			return strconv.FormatInt(int64(n), 10)
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	case json.Number:
		return n.String()
	default:
		return ""
	}
}

// Bool returns the boolean value for a key, or false if missing. A quoted
// boolean counts, on the same grounds as BoolOrString.
func (in Input) Bool(key string) bool {
	b := in.BoolOrString(key)
	return b != nil && *b
}

// BoolOrString returns a *bool for a key, accepting either a JSON bool or a
// string strconv.ParseBool understands ("true", "false", and also 1/0/T/F).
// Anything else, including an absent key, is nil.
//
// This is NumberOrString's failure in the other direction. Boolean filters —
// verified, disabled, eligible_for_retry — were read with a plain type
// assertion, so a model that sent verified: "false" had the filter dropped
// rather than rejected: gateway_requests then listed every request, verified
// ones included, and nothing in the response said a filter had been ignored.
// The caller reports unverified requests that were never unverified. Models
// emit booleans as JSON strings often enough that this is the common path, and
// a wrong answer that reads as a right one is worse than an error.
//
// nil still means "no filter", so a genuinely unparseable value (verified:
// "yes") behaves as before rather than guessing at an intent.
func (in Input) BoolOrString(key string) *bool {
	v, ok := in[key]
	if !ok {
		return nil
	}
	switch b := v.(type) {
	case bool:
		return &b
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(b))
		if err != nil {
			return nil
		}
		return &parsed
	default:
		return nil
	}
}

// StringSlice returns the string slice for a key, or nil if missing.
func (in Input) StringSlice(key string) []string {
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

// SetIfNonEmpty adds the value to the map if it is not empty.
func SetIfNonEmpty(params map[string]string, key, value string) {
	if value != "" {
		params[key] = value
	}
}

// SetInt adds the int value to the map if it is > 0.
func SetInt(params map[string]string, key string, value int) {
	if value > 0 {
		params[key] = strconv.Itoa(value)
	}
}

// JSONFilterParam returns a JSON filter value for API query params (body, headers, etc.).
// Accepts a JSON string or object from MCP tool arguments.
func (in Input) JSONFilterParam(key string) (string, error) {
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

// SetJSONFilter adds a JSON filter param when present and valid.
func SetJSONFilter(params map[string]string, key string, in Input) error {
	value, err := in.JSONFilterParam(key)
	if err != nil {
		return err
	}
	SetIfNonEmpty(params, key, value)
	return nil
}

// SetPayloadSearchFilters forwards body, headers, parsed_query, and path list filters.
func SetPayloadSearchFilters(params map[string]string, in Input) error {
	for _, key := range []string{"body", "headers", "parsed_query", "path"} {
		if err := SetJSONFilter(params, key, in); err != nil {
			return err
		}
	}
	return nil
}

// StringList reads a value that may be given either as an array of strings or,
// mirroring the CLI's comma-separated flags, as a single string.
func StringList(in Input, key string) []string {
	if values := in.StringSlice(key); len(values) > 0 {
		return values
	}
	raw := in.String(key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Object reads a JSON object argument. A missing key yields nil, not an error.
func Object(in Input, key string) (map[string]interface{}, error) {
	v, ok := in[key]
	if !ok || v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON object", key)
	}
	return m, nil
}

// StringMap reads a JSON object whose values must all be strings, such as
// resource metadata or transformation environment variables.
func StringMap(in Input, key string) (map[string]string, error) {
	raw, err := Object(in, key)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s.%s must be a string", key, k)
		}
		out[k] = s
	}
	return out, nil
}

// RequireString returns the value for key, or an error naming the action that
// needs it.
func RequireString(in Input, key, action string) (string, error) {
	value := in.String(key)
	if value == "" {
		return "", fmt.Errorf("%s is required for the %s action", key, action)
	}
	return value, nil
}

// OptionalStringPtr returns a pointer to the value for key, or nil when the key
// is absent. Update requests use this to distinguish "not supplied" from
// "set to empty".
func OptionalStringPtr(in Input, key string) *string {
	v, ok := in[key]
	if !ok || v == nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return nil
	}
	return &s
}
