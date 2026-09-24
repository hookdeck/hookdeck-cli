package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateBlock matches the marker pair tools/generate-reference writes between.
var (
	generateStart = regexp.MustCompile(`^<!-- GENERATE[A-Z0-9_]*(?::[^:]*)?:START -->`)
	generateEnd   = regexp.MustCompile(`^<!-- GENERATE_END -->`)

	// A `hookdeck ...` invocation written in prose. Stops at anything that
	// cannot be a command name, so flags and placeholders end the path.
	invocation = regexp.MustCompile("hookdeck((?: [a-z][a-z0-9-]*)+)")
)

// handwrittenReference returns the parts of REFERENCE.md no generator owns.
//
// 96% of the file sits inside GENERATE markers and is rewritten from the cobra
// tree, so it cannot drift. The remaining few per cent is curated prose —
// task-to-command tables whose descriptions are not derivable from command
// metadata — and that is exactly where it does drift.
func handwrittenReference(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "REFERENCE.md"))
	require.NoError(t, err)
	body, err := os.ReadFile(path)
	require.NoError(t, err)

	var out []string
	inBlock := false
	for _, line := range strings.Split(string(body), "\n") {
		switch {
		case generateStart.MatchString(line):
			inBlock = true
		case generateEnd.MatchString(line):
			inBlock = false
		case !inBlock:
			out = append(out, line)
		}
	}
	require.NotEmpty(t, out, "no hand-maintained prose found; the marker regexes are probably wrong")
	return out
}

// resolve walks the command tree as far as the words go, and reports the first
// word that should have been a subcommand and is not.
//
// The rule is about the PARENT, not the word: if the command reached so far has
// subcommands, it is a group, and the next word has to be one of them —
// `metrics` has attempts/events/requests/transformations, so `queue-depth` is
// wrong. If it has none, it is a leaf and the remaining words are positional
// arguments — `project use prj_x` is fine.
//
// An earlier version of this accepted anything once the current command was
// Runnable, which made the whole test a no-op: it passed with the three known
// bad examples reinstated. A guard that cannot fail is worse than no guard,
// because it reads like coverage.
func resolve(root *cobra.Command, words []string) (bad string, ok bool) {
	cmd := root
	for _, word := range words {
		subs := cmd.Commands()
		if len(subs) == 0 {
			return "", true // leaf: the rest are arguments
		}
		var next *cobra.Command
		for _, sub := range subs {
			if sub.Name() == word {
				next = sub
				break
			}
		}
		if next == nil {
			return word, false
		}
		cmd = next
	}
	return "", true
}

// TestReferenceExamplesNameRealCommands generalises the metrics-only guard.
//
// REFERENCE.md documented `hookdeck gateway metrics queue-depth`, `pending` and
// `events-by-issue` long after those were consolidated into `metrics events`;
// anyone following the table got "unknown command". That was caught by a check
// scoped to `hookdeck gateway metrics <word>`, which leaves every other
// hand-written example in the file unguarded. This checks them all.
func TestReferenceExamplesNameRealCommands(t *testing.T) {
	root := rootCmd

	seen := map[string]bool{}
	var bad []string
	for _, line := range handwrittenReference(t) {
		for _, m := range invocation.FindAllStringSubmatch(line, -1) {
			words := strings.Fields(m[1])
			if len(words) == 0 {
				continue
			}
			if word, ok := resolve(root, words); !ok {
				full := "hookdeck " + strings.Join(words, " ")
				if !seen[full] {
					seen[full] = true
					bad = append(bad, full+"  (unknown: "+word+")")
				}
			}
		}
	}
	sort.Strings(bad)
	assert.Empty(t, bad,
		"REFERENCE.md prose outside the GENERATE markers names commands that do not exist:\n  %s",
		strings.Join(bad, "\n  "))
}
