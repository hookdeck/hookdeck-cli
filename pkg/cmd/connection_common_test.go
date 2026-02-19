package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldShowConnectionDeprecation(t *testing.T) {
	saveArgs := os.Args
	defer func() { os.Args = saveArgs }()

	tests := []struct {
		name     string
		args     []string
		showWant bool
	}{
		{"root connection list", []string{"hookdeck", "connection", "list"}, true},
		{"root connections list", []string{"hookdeck", "connections", "list"}, true},
		{"gateway path - no notice", []string{"hookdeck", "gateway", "connection", "list"}, false},
		{"gateway path connections - no notice", []string{"hookdeck", "gateway", "connections", "list"}, false},
		{"output json - no notice", []string{"hookdeck", "connection", "list", "--output", "json"}, false},
		{"output=json - no notice", []string{"hookdeck", "connection", "list", "--output=json"}, false},
		{"single arg - no notice", []string{"hookdeck"}, false},
		{"connection only - show", []string{"hookdeck", "connection"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			got := shouldShowConnectionDeprecation()
			assert.Equal(t, tt.showWant, got, "args=%v", tt.args)
		})
	}
}
