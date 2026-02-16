package cmd

import (
	"encoding/json"
	"fmt"
	"os"
)

// buildSourceConfigFromFlags parses source config from either --config (JSON string)
// or --config-file (path to JSON file). Used by source create, upsert, and update
// to avoid duplicating the same parsing logic.
// Returns (nil, nil) when both are empty.
func buildSourceConfigFromFlags(configStr, configFile string) (map[string]interface{}, error) {
	if configStr != "" {
		var out map[string]interface{}
		if err := json.Unmarshal([]byte(configStr), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON in --config: %w", err)
		}
		return out, nil
	}
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read --config-file: %w", err)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("invalid JSON in config file: %w", err)
		}
		return out, nil
	}
	return nil, nil
}
