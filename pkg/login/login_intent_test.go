package login

import (
	"testing"

	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestBuildStartLoginInput_guestProfileIncludesCredentials(t *testing.T) {
	cfg := &configpkg.Config{
		DeviceName: "dev",
		Profile: configpkg.Profile{
			GuestURL:    "https://console.test/signin/guest?token=abc",
			GuestUserID: "usr_guest",
			APIKey:      "hk_guest_key",
		},
	}

	input := buildStartLoginInput(cfg, "")
	require.Equal(t, "usr_guest", input.GuestUserID)
	require.Equal(t, "hk_guest_key", input.GuestAPIKey)
}

func TestBuildStartLoginInput_noGuestProfileOmitsCredentials(t *testing.T) {
	cfg := &configpkg.Config{
		DeviceName: "dev",
		Profile:    configpkg.Profile{},
	}

	input := buildStartLoginInput(cfg, "")
	require.Empty(t, input.GuestUserID)
	require.Empty(t, input.GuestAPIKey)
}
