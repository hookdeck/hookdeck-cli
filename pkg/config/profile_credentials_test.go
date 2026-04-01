package config

import (
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/require"
)

func TestProfile_ApplyValidateAPIKeyResponse(t *testing.T) {
	t.Run("nil response is no-op", func(t *testing.T) {
		p := &Profile{ProjectId: "keep", GuestURL: "https://guest"}
		p.ApplyValidateAPIKeyResponse(nil, true)
		require.Equal(t, "keep", p.ProjectId)
		require.Equal(t, "https://guest", p.GuestURL)
	})

	t.Run("sets project fields and clears guest when requested", func(t *testing.T) {
		p := &Profile{GuestURL: "https://guest"}
		p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
			ProjectID:   "team_1",
			ProjectMode: "inbound",
		}, true)
		require.Equal(t, "team_1", p.ProjectId)
		require.Equal(t, "inbound", p.ProjectMode)
		require.Equal(t, ProjectTypeGateway, p.ProjectType)
		require.Empty(t, p.GuestURL)
	})

	t.Run("preserves guest URL when clearGuestURL is false", func(t *testing.T) {
		p := &Profile{GuestURL: "https://guest.example/x"}
		p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
			ProjectID:   "team_2",
			ProjectMode: "console",
		}, false)
		require.Equal(t, "team_2", p.ProjectId)
		require.Equal(t, ProjectTypeConsole, p.ProjectType)
		require.Equal(t, "https://guest.example/x", p.GuestURL)
	})
}

func TestProfile_ApplyPollAPIKeyResponse(t *testing.T) {
	t.Run("nil response is no-op", func(t *testing.T) {
		p := &Profile{APIKey: "k", ProjectId: "p"}
		p.ApplyPollAPIKeyResponse(nil, "")
		require.Equal(t, "k", p.APIKey)
		require.Equal(t, "p", p.ProjectId)
	})

	t.Run("sets credentials and guest URL", func(t *testing.T) {
		p := &Profile{}
		p.ApplyPollAPIKeyResponse(&hookdeck.PollAPIKeyResponse{
			APIKey:      "key_from_poll",
			ProjectID:   "team_p",
			ProjectMode: "inbound",
		}, "https://guest")
		require.Equal(t, "key_from_poll", p.APIKey)
		require.Equal(t, "team_p", p.ProjectId)
		require.Equal(t, ProjectTypeGateway, p.ProjectType)
		require.Equal(t, "https://guest", p.GuestURL)
	})

	t.Run("clears-style guest with empty string", func(t *testing.T) {
		p := &Profile{GuestURL: "old"}
		p.ApplyPollAPIKeyResponse(&hookdeck.PollAPIKeyResponse{
			APIKey:      "k123456789012",
			ProjectID:   "t",
			ProjectMode: "inbound",
		}, "")
		require.Empty(t, p.GuestURL)
	})
}

func TestProfile_ApplyCIClient(t *testing.T) {
	p := &Profile{}
	p.ApplyCIClient(hookdeck.CIClient{
		APIKey:      "ci_key_123456",
		ProjectID:   "team_ci",
		ProjectMode: "inbound",
	})
	require.Equal(t, "ci_key_123456", p.APIKey)
	require.Equal(t, "team_ci", p.ProjectId)
	require.Equal(t, ProjectTypeGateway, p.ProjectType)
	require.Empty(t, p.GuestURL)
}
