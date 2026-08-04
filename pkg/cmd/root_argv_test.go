package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgvContainsGatewayMCP(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want bool
	}{
		{"minimal", []string{"hookdeck", "gateway", "mcp"}, true},
		{"with profile long", []string{"hookdeck", "--profile", "p1", "gateway", "mcp"}, true},
		{"profile equals", []string{"hookdeck", "--profile=p1", "gateway", "mcp"}, true},
		{"short p", []string{"hookdeck", "-p", "p1", "gateway", "mcp"}, true},
		{"double dash positional", []string{"hookdeck", "--", "gateway", "mcp"}, true},
		{"not mcp", []string{"hookdeck", "gateway", "connection", "list"}, false},
		{"wrong order", []string{"hookdeck", "mcp", "gateway"}, false},
		{"too short", []string{"hookdeck", "gateway"}, false},
		{"api key flag before", []string{"hookdeck", "--api-key", "k", "gateway", "mcp"}, true},
		// Boolean flags (--insecure, --version) are not in flagNeedsNextArg, so
		// globalPositionalArgs treats them as single-token flags and skips them.
		{"bool flag before gateway", []string{"hookdeck", "--insecure", "gateway", "mcp"}, true},
		{"bool flag between gateway and mcp", []string{"hookdeck", "gateway", "--insecure", "mcp"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, argvContainsGatewayMCP(tt.argv))
		})
	}
}
