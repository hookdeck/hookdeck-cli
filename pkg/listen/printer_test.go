package listen

import (
	"io"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// capturePrinterOutput runs fn with os.Stdout redirected and returns what it
// wrote. printSourcesWithConnections prints with fmt.Printf and decides on
// hyperlinks from os.Stdout, so the pipe here is also the "not a terminal" case.
func capturePrinterOutput(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		out, _ := io.ReadAll(r)
		done <- string(out)
	}()

	fn()

	require.NoError(t, w.Close())
	os.Stdout = original

	return <-done
}

func printerFixture(t *testing.T, numConnections int) (*config.Config, []*hookdeck.Source, []*hookdeck.Connection, *url.URL) {
	t.Helper()

	targetURL, err := url.Parse("http://localhost:3030")
	require.NoError(t, err)

	source := &hookdeck.Source{ID: "src_1", Name: "my-source", URL: "https://hkdk.events/src_1"}

	connections := make([]*hookdeck.Connection, 0, numConnections)
	for i := 0; i < numConnections; i++ {
		fullName := "my-source -> dest"
		connections = append(connections, &hookdeck.Connection{
			ID:          "web_1",
			FullName:    &fullName,
			Source:      source,
			Destination: &hookdeck.Destination{ID: "des_1", Type: "CLI", Config: map[string]interface{}{"path": "/"}},
		})
	}

	cfg := &config.Config{
		DashboardBaseURL: "https://dashboard.hookdeck.com",
		ConsoleBaseURL:   "https://console.hookdeck.com",
	}

	return cfg, []*hookdeck.Source{source}, connections, targetURL
}

// resetColorState restores the ansi package globals a subtest flipped.
func resetColorState(t *testing.T) {
	t.Helper()
	force, disable := ansi.ForceColors, ansi.DisableColors
	t.Cleanup(func() {
		ansi.ForceColors, ansi.DisableColors = force, disable
	})
}

// TestCompactBannerCarriesTheSummary is the regression test for #402. Compact
// output line 2 was the bare preposition "Listening on" followed by a blank
// line: the renderer had the sources and connections but printed neither.
// Compact is the automatic no-TTY fallback, so this is the line CI logs keep.
func TestCompactBannerCarriesTheSummary(t *testing.T) {
	t.Run("one source, one connection", func(t *testing.T) {
		resetColorState(t)
		cfg, sources, connections, targetURL := printerFixture(t, 1)

		out := capturePrinterOutput(t, func() {
			printSourcesWithConnections(cfg, "tm_1", sources, connections, targetURL, "")
		})

		assert.Contains(t, out, "Listening on 1 source • 1 connection")
		assert.NotRegexp(t, `(?m)^Listening on\s*$`, out,
			"a dangling preposition is worse than nothing (#402)")
	})

	t.Run("counts are pluralised", func(t *testing.T) {
		resetColorState(t)
		cfg, sources, connections, targetURL := printerFixture(t, 2)

		out := capturePrinterOutput(t, func() {
			printSourcesWithConnections(cfg, "tm_1", sources, connections, targetURL, "")
		})

		assert.Contains(t, out, "Listening on 1 source • 2 connections")
	})
}

// TestDashboardLinkHyperlinksOnlyOnATerminal is the regression test for #403.
// The printer hand-rolled the OSC 8 escape unconditionally, so the measurement
// came out exactly backwards: redirected output carried two escape sequences and
// a real terminal carried none. Worse, the link label omits team_id, so the
// redirected output was strictly *less* informative than the terminal output.
func TestDashboardLinkHyperlinksOnlyOnATerminal(t *testing.T) {
	const fullURL = "https://dashboard.hookdeck.com/events/cli?team_id=tm_1"

	t.Run("not a terminal: no escape, and the full URL including team_id", func(t *testing.T) {
		resetColorState(t)
		cfg, sources, connections, targetURL := printerFixture(t, 1)

		out := capturePrinterOutput(t, func() {
			printSourcesWithConnections(cfg, "tm_1", sources, connections, targetURL, "")
		})

		assert.Equal(t, 0, strings.Count(out, "\x1b]8;;"),
			"a log file cannot render OSC 8")
		assert.Contains(t, out, fullURL,
			"without the hyperlink the query parameters must be visible, not hidden in an escape")
	})

	t.Run("a terminal gets the hyperlink", func(t *testing.T) {
		resetColorState(t)
		ansi.ForceColors = true
		cfg, sources, connections, targetURL := printerFixture(t, 1)

		out := capturePrinterOutput(t, func() {
			printSourcesWithConnections(cfg, "tm_1", sources, connections, targetURL, "")
		})

		assert.Equal(t, 2, strings.Count(out, "\x1b]8;;"),
			"a terminal is the one thing that can render OSC 8")
		assert.Contains(t, out, fullURL, "the escape still targets the full URL")
	})

	t.Run("--color off strips the hyperlink and keeps the full URL", func(t *testing.T) {
		resetColorState(t)
		ansi.ForceColors = true
		ansi.DisableColors = true
		cfg, sources, connections, targetURL := printerFixture(t, 1)

		out := capturePrinterOutput(t, func() {
			printSourcesWithConnections(cfg, "tm_1", sources, connections, targetURL, "")
		})

		assert.Equal(t, 0, strings.Count(out, "\x1b]8;;"),
			"--color off left the OSC 8 bytes behind (#403)")
		assert.Contains(t, out, fullURL)
	})

	t.Run("NO_COLOR strips the hyperlink too", func(t *testing.T) {
		resetColorState(t)
		ansi.ForceColors = false
		t.Setenv("NO_COLOR", "1")
		t.Setenv("CLICOLOR_FORCE", "1")
		cfg, sources, connections, targetURL := printerFixture(t, 1)

		out := capturePrinterOutput(t, func() {
			printSourcesWithConnections(cfg, "tm_1", sources, connections, targetURL, "")
		})

		assert.Equal(t, 0, strings.Count(out, "\x1b]8;;"))
		assert.Contains(t, out, fullURL)
	})

	t.Run("a guest session prints its sign-up link plainly", func(t *testing.T) {
		resetColorState(t)
		cfg, sources, connections, targetURL := printerFixture(t, 1)

		out := capturePrinterOutput(t, func() {
			printSourcesWithConnections(cfg, "tm_1", sources, connections, targetURL, "https://hookdeck.com/signup?x=1")
		})

		assert.Contains(t, out, "https://hookdeck.com/signup?x=1")
		assert.Equal(t, 0, strings.Count(out, "\x1b]8;;"))
	})
}
