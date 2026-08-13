package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFlagTestCmd builds a throwaway command carrying the flags under test, so
// these cases exercise rejectEmptyFlags through real pflag parsing (which is what
// sets Changed) rather than a hand-built flag set.
func newFlagTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}

	for _, name := range []string{
		"name", "type", "url",
		"webhook-secret", "api-key", "hmac-secret", "basic-auth-user", "basic-auth-pass",
		"source-webhook-secret", "source-name", "source-type",
		"destination-url", "destination-bearer-token",
		"config", "description",
	} {
		cmd.Flags().String(name, "", "")
	}

	return cmd
}

// TestRejectEmptyFlags covers #335. `--source-webhook-secret ""` was accepted and
// silently dropped, producing a source that looked configured but verified
// nothing. The trigger is an unexported shell variable expanding to "".
func TestRejectEmptyFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		errContains []string
	}{
		{
			name:    "no flags at all is fine",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "omitted flags are not treated as empty",
			args:    []string{"--name", "my-source"},
			wantErr: false,
		},
		{
			name:        "empty --webhook-secret is rejected",
			args:        []string{"--webhook-secret", ""},
			wantErr:     true,
			errContains: []string{"--webhook-secret", "exported"},
		},
		{
			name:        "empty --source-webhook-secret is rejected",
			args:        []string{"--source-webhook-secret", ""},
			wantErr:     true,
			errContains: []string{"--source-webhook-secret", "exported"},
		},
		{
			name:        "whitespace-only secret is rejected",
			args:        []string{"--webhook-secret", "   "},
			wantErr:     true,
			errContains: []string{"--webhook-secret"},
		},
		{
			name:        "empty --name is rejected even though Cobra's required check passes it",
			args:        []string{"--name", ""},
			wantErr:     true,
			errContains: []string{"--name"},
		},
		{
			name:        "empty --type is rejected",
			args:        []string{"--type", ""},
			wantErr:     true,
			errContains: []string{"--type"},
		},
		{
			name:        "empty destination auth secret is rejected",
			args:        []string{"--destination-bearer-token", ""},
			wantErr:     true,
			errContains: []string{"--destination-bearer-token"},
		},
		{
			name:        "empty --config is rejected",
			args:        []string{"--config", ""},
			wantErr:     true,
			errContains: []string{"--config"},
		},
		{
			name:        "several empty flags are reported together, sorted",
			args:        []string{"--api-key", "", "--webhook-secret", ""},
			wantErr:     true,
			errContains: []string{"--api-key", "--webhook-secret", "exported"},
		},
		{
			name:    "a flag not on the list may be empty",
			args:    []string{"--description", ""},
			wantErr: false,
		},
		{
			name:    "valid values pass",
			args:    []string{"--name", "src", "--type", "STRIPE", "--webhook-secret", "whsec_abc"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newFlagTestCmd()
			require.NoError(t, cmd.ParseFlags(tt.args))

			err := rejectEmptyFlags(cmd)

			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			for _, want := range tt.errContains {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

// TestRejectEmptyFlagsWiredIntoResourceCommands guards against the helper being
// added but never called: every create/upsert/update path that accepts a secret
// must run it. A regression here is invisible — the command keeps working, it
// just stops catching the bug.
func TestRejectEmptyFlagsWiredIntoResourceCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"source create", []string{"gateway", "source", "create", "--name", "s", "--type", "STRIPE", "--webhook-secret", ""}},
		{"source upsert", []string{"gateway", "source", "upsert", "s", "--type", "STRIPE", "--webhook-secret", ""}},
		{"source update", []string{"gateway", "source", "update", "src_1", "--webhook-secret", ""}},
		{"connection create", []string{"gateway", "connection", "create", "--name", "c", "--source-webhook-secret", ""}},
		{"connection upsert", []string{"gateway", "connection", "upsert", "c", "--source-webhook-secret", ""}},
		{"destination create", []string{"gateway", "destination", "create", "--name", "d", "--type", "HTTP", "--bearer-token", ""}},
		{"destination upsert", []string{"gateway", "destination", "upsert", "d", "--bearer-token", ""}},
		{"destination update", []string{"gateway", "destination", "update", "des_1", "--bearer-token", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := &cobra.Command{Use: "hookdeck", SilenceUsage: true, SilenceErrors: true}
			root.AddCommand(newGatewayCmd().cmd)

			cmd, remaining, err := root.Find(tt.args)
			require.NoError(t, err, "command should resolve")
			require.NoError(t, cmd.ParseFlags(remaining))

			require.NotNil(t, cmd.PreRunE, "%s should have a PreRunE validator", tt.name)

			// Go through PreRunE, not rejectEmptyFlags directly: that is what
			// proves the helper is actually wired into this command.
			err = cmd.PreRunE(cmd, cmd.Flags().Args())
			require.Error(t, err, "%s must reject an explicitly empty secret", tt.name)
			assert.Contains(t, err.Error(), "exported",
				"%s should surface the empty-flag message", tt.name)
		})
	}
}
