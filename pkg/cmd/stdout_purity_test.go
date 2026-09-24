package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Diagnostics must not be written to stdout.
//
// `--output json` promises parseable output, and stdout is where that output
// goes. A warning printed with fmt.Printf lands ahead of it, so the caller gets
//
//	invalid character 'W' looking for beginning of value
//
// This shipped: source validation warned to stdout when the OpenAPI spec fetch
// failed, which happens intermittently, so JSON consumers broke at random
// rather than consistently. A reviewer will not catch the next one by eye — the
// offending line looks exactly like every other fmt.Printf in the package.
func TestDiagnosticsDoNotGoToStdout(t *testing.T) {
	// fmt.Print/Printf/Println whose first argument starts with a diagnostic
	// prefix. Anything routed through os.Stderr is fine and not matched.
	offender := regexp.MustCompile(`fmt\.Print(f|ln)?\(\s*"(Warning|WARNING|Error|ERROR|Deprecated|Note):`)

	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	require.NotEmpty(t, files)

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		require.NoError(t, err)

		for i, line := range strings.Split(string(src), "\n") {
			assert.False(t, offender.MatchString(line),
				"%s:%d writes a diagnostic to stdout, which corrupts --output json for "+
					"every command that prints structured output. Use "+
					"fmt.Fprintf(os.Stderr, …) instead:\n  %s",
				file, i+1, strings.TrimSpace(line))
		}
	}
}
