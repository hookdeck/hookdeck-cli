package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
)

// nextPageCommand runs a gateway list command against a server that always has
// another page, and returns the command suggested for fetching it.
func nextPageCommand(t *testing.T, args ...string) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"id":"id_1"}],"pagination":{"limit":1,"next":"cur_2"}}`))
	}))
	t.Cleanup(server.Close)

	config.ResetAPIClientForTesting()
	t.Cleanup(config.ResetAPIClientForTesting)

	old := Config
	t.Cleanup(func() { Config = old })
	Config = config.Config{APIBaseURL: server.URL, LogLevel: "info"}
	Config.Profile.APIKey = "sk_test_123456789012"
	Config.Profile.ProjectId = "proj_1"
	Config.Profile.ProjectType = config.ProjectTypeEventGateway

	root := &cobra.Command{Use: "hookdeck", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringP("profile", "p", "", "")
	root.PersistentFlags().String("api-key", "", "")
	root.AddCommand(newGatewayCmd().cmd)
	root.SetArgs(args)

	r, w, err := os.Pipe()
	require.NoError(t, err)
	stdout := os.Stdout
	os.Stdout = w
	execErr := root.Execute()
	os.Stdout = stdout
	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, execErr)

	_, command, found := strings.Cut(string(out), "To get the next page:\n")
	require.True(t, found, "no next page hint in %q", out)
	return strings.TrimSpace(command)
}

func TestNextPageCommandRepeatsTheCommandThatWasRun(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "keeps a required flag",
			args: []string{"gateway", "attempt", "list", "--event-id", "evt_123"},
			want: "hookdeck gateway attempt list --event-id evt_123 --next cur_2",
		},
		{
			name: "keeps filters and replaces the previous cursor",
			args: []string{"gateway", "event", "list", "--status", "FAILED", "--limit", "1", "--next", "cur_1"},
			want: "hookdeck gateway event list --limit 1 --status FAILED --next cur_2",
		},
		{
			name: "keeps a boolean flag",
			args: []string{"gateway", "source", "list", "--disabled"},
			want: "hookdeck gateway source list --disabled --next cur_2",
		},
		{
			name: "quotes values for the shell",
			args: []string{"gateway", "event", "list", "--body", `{"name":"it's"}`},
			want: `hookdeck gateway event list --body '{"name":"it'\''s"}' --next cur_2`,
		},
		{
			name: "keeps the profile but does not echo credentials",
			args: []string{"gateway", "event", "list", "-p", "prod", "--api-key", "sk_test_other_123456"},
			want: "hookdeck gateway event list --profile prod --next cur_2",
		},
		{
			name: "keeps a positional argument",
			args: []string{"gateway", "transformation", "executions", "list", "trs_123"},
			want: "hookdeck gateway transformation executions list trs_123 --next cur_2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, nextPageCommand(t, tt.args...))
		})
	}
}
