package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// metricsCommandInReference matches a `hookdeck gateway metrics <word>`
// invocation written in REFERENCE.md prose.
var metricsCommandInReference = regexp.MustCompile(`hookdeck gateway metrics ([a-z][a-z-]*)`)

// TestReferenceMetricsExamplesNameRealSubcommands guards the one part of the
// Metrics section that no generator maintains.
//
// It sits outside the <!-- GENERATE --> markers, so it drifted: it documented
// `metrics queue-depth`, `metrics pending` and `metrics events-by-issue` long
// after those were consolidated into `metrics events`, and contradicted the
// paragraph directly below it describing that consolidation. Anyone following
// the table got "unknown command".
func TestReferenceMetricsExamplesNameRealSubcommands(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("..", "..", "REFERENCE.md"))
	require.NoError(t, err)
	body, err := os.ReadFile(path)
	require.NoError(t, err)

	real := map[string]bool{}
	var names []string
	for _, sub := range newMetricsCmd().cmd.Commands() {
		real[sub.Name()] = true
		names = append(names, sub.Name())
	}
	sort.Strings(names)
	require.NotEmpty(t, names)

	var documented []string
	for _, m := range metricsCommandInReference.FindAllStringSubmatch(string(body), -1) {
		documented = append(documented, m[1])
		assert.True(t, real[m[1]],
			"REFERENCE.md documents `hookdeck gateway metrics %s`, which is not a subcommand; the real ones are %v",
			m[1], names)
	}
	assert.NotEmpty(t, documented, "the examples should still be there to check")
}
