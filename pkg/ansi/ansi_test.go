package ansi

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetColorState restores the package globals a test flipped, so the next test
// sees the default configuration.
func resetColorState(t *testing.T) {
	t.Helper()
	force, disable := ForceColors, DisableColors
	t.Cleanup(func() {
		ForceColors, DisableColors = force, disable
	})
}

// TestCanHyperlinkFollowsColor pins #403. OSC 8 hyperlinks were written
// unconditionally by the listen printer, which meant a redirected run (a log
// file, a CI job) received the escape bytes while a real terminal — the only
// thing that can render them — received a plain URL. A hyperlink is terminal
// decoration and must be gated exactly like colour.
func TestCanHyperlinkFollowsColor(t *testing.T) {
	t.Run("a plain writer is not a terminal, so no hyperlink", func(t *testing.T) {
		resetColorState(t)

		assert.False(t, CanHyperlink(&bytes.Buffer{}))
	})

	t.Run("a colour-capable writer can be hyperlinked", func(t *testing.T) {
		resetColorState(t)
		ForceColors = true

		assert.True(t, CanHyperlink(&bytes.Buffer{}))
	})

	t.Run("--color off suppresses hyperlinks too", func(t *testing.T) {
		resetColorState(t)
		ForceColors = true
		DisableColors = true

		assert.False(t, CanHyperlink(&bytes.Buffer{}),
			"--color off must strip the OSC 8 bytes, not just the SGR ones")
	})

	t.Run("CanHyperlink and ShouldUseColors agree", func(t *testing.T) {
		resetColorState(t)
		w := &bytes.Buffer{}

		for _, force := range []bool{false, true} {
			for _, disable := range []bool{false, true} {
				ForceColors, DisableColors = force, disable
				assert.Equal(t, ShouldUseColors(w), CanHyperlink(w),
					"hyperlinks and colour must be gated by one decision")
			}
		}
	})
}

// TestLinkifyEmitsOSC8OnlyWhenItCanBeRendered checks the bytes themselves.
func TestLinkifyEmitsOSC8OnlyWhenItCanBeRendered(t *testing.T) {
	const url = "https://dashboard.hookdeck.com/events/cli?team_id=tm_1"

	t.Run("no terminal, no escape", func(t *testing.T) {
		resetColorState(t)

		out := Linkify("label", url, &bytes.Buffer{})

		assert.Equal(t, "label", out)
		assert.NotContains(t, out, "\x1b]8;;")
	})

	t.Run("terminal gets the escape", func(t *testing.T) {
		resetColorState(t)
		ForceColors = true

		out := Linkify("label", url, &bytes.Buffer{})

		assert.Equal(t, 2, strings.Count(out, "\x1b]8;;"),
			"an OSC 8 hyperlink opens and closes")
		assert.Contains(t, out, url)
	})
}

// TestNoColorDisablesDecoration covers the NO_COLOR half of #403. The CLI
// honoured --color off and CLICOLOR but had never implemented https://no-color.org,
// so NO_COLOR only appeared to work in the places where output was not a
// terminal anyway.
func TestNoColorDisablesDecoration(t *testing.T) {
	t.Run("NO_COLOR turns decoration off", func(t *testing.T) {
		resetColorState(t)
		t.Setenv("NO_COLOR", "1")
		t.Setenv("CLICOLOR_FORCE", "1") // would otherwise force colour on

		assert.False(t, ShouldUseColors(&bytes.Buffer{}))
		assert.False(t, CanHyperlink(&bytes.Buffer{}))
	})

	t.Run("an empty NO_COLOR is not set", func(t *testing.T) {
		resetColorState(t)
		t.Setenv("NO_COLOR", "")
		ForceColors = true

		assert.True(t, ShouldUseColors(&bytes.Buffer{}),
			"the spec treats only a non-empty value as set")
	})

	t.Run("--color on still wins", func(t *testing.T) {
		resetColorState(t)
		t.Setenv("NO_COLOR", "1")
		ForceColors = true

		assert.True(t, ShouldUseColors(&bytes.Buffer{}),
			"an explicit flag beats an environment default")
	})
}

// TestCanSpinStillRequiresATerminal guards the #376 fix: readiness reporting
// branches on CanSpin, and it must stay false for a non-terminal so the plain
// status line is printed instead of a spinner nobody can see.
func TestCanSpinStillRequiresATerminal(t *testing.T) {
	resetColorState(t)
	ForceColors = true

	assert.False(t, CanSpin(&bytes.Buffer{}), "a buffer is not a terminal")

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = devNull.Close() })
	assert.False(t, CanSpin(devNull), "/dev/null is a file, not a terminal")
}
