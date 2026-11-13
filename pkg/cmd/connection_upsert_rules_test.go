package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func storedConnectionWithRule() *hookdeck.Connection {
	stored := storedUpsertConnection()
	stored.Rules = []hookdeck.Rule{{"type": "retry", "strategy": "linear", "count": 2}}
	return stored
}

// TestConnectionUpsertSendsAnEmptyRulesArray covers #472: an explicit empty
// array was dropped by a len(rules) > 0 check, so the stored rules stayed.
// Each form of the flag is its own call site.
func TestConnectionUpsertSendsAnEmptyRulesArray(t *testing.T) {
	emptyFile := filepath.Join(t.TempDir(), "rules.json")
	require.NoError(t, os.WriteFile(emptyFile, []byte("[]"), 0o644))

	for name, flags := range map[string][]string{
		"--rules":      {"--rules", "[]"},
		"--rules-file": {"--rules-file", emptyFile},
	} {
		flags := flags
		t.Run(name, func(t *testing.T) {
			body := captureConnectionUpsertBody(t, storedConnectionWithRule(),
				append([]string{"gateway", "connection", "upsert", "my-conn"}, flags...)...)

			rules, present := body["rules"]
			require.True(t, present, "an empty array must be sent, not left out")
			assert.Equal(t, []interface{}{}, rules)
		})
	}
}

// TestConnectionUpsertWithoutRulesFlagOmitsRules is the negative control: only
// an explicit rules flag may touch the stored ruleset.
func TestConnectionUpsertWithoutRulesFlagOmitsRules(t *testing.T) {
	body := captureConnectionUpsertBody(t, storedConnectionWithRule(),
		"gateway", "connection", "upsert", "my-conn", "--description", "d")

	assert.NotContains(t, body, "rules")
}

// TestConnectionUpsertDryRunReportsClearedRules runs the preview directly,
// because it prints to stdout.
func TestConnectionUpsertDryRunReportsClearedRules(t *testing.T) {
	empty := []hookdeck.Rule{}

	preview := func(t *testing.T, req *hookdeck.ConnectionCreateRequest) string {
		t.Helper()
		cu := newUpsertCmdForFlags()
		cu.name = "my-conn"

		r, w, err := os.Pipe()
		require.NoError(t, err)
		old := os.Stdout
		os.Stdout = w
		previewErr := cu.previewUpsertChanges(storedConnectionWithRule(), req, true)
		os.Stdout = old
		require.NoError(t, w.Close())
		out, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, previewErr)
		return string(out)
	}

	t.Run("an empty array says the rules will be cleared", func(t *testing.T) {
		out := preview(t, &hookdeck.ConnectionCreateRequest{Rules: &empty})

		assert.Contains(t, out, "will be cleared")
		assert.NotContains(t, out, "rules (unchanged)")
	})

	t.Run("no rules flag says the rules are unchanged", func(t *testing.T) {
		out := preview(t, &hookdeck.ConnectionCreateRequest{})

		assert.Contains(t, out, "Rules: 1 rules (unchanged)")
		assert.NotContains(t, out, "will be cleared")
	})
}
