package mcp

import (
	"fmt"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// stringList reads a value that may be given either as an array of strings or,
// mirroring the CLI's comma-separated flags, as a single string.
func stringList(in mcpcore.Input, key string) []string {
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

// object reads a JSON object argument. A missing key yields nil, not an error.
func object(in mcpcore.Input, key string) (map[string]interface{}, error) {
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

// stringMap reads a JSON object whose values must all be strings, such as
// resource metadata.
func stringMap(in mcpcore.Input, key string) (map[string]string, error) {
	raw, err := object(in, key)
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

// requireString returns the value for key, or an error naming the action that
// needs it.
func requireString(in mcpcore.Input, key, action string) (string, error) {
	value := in.String(key)
	if value == "" {
		return "", fmt.Errorf("%s is required for the %s action", key, action)
	}
	return value, nil
}
