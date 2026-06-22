package login

import (
	"testing"

	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/require"
)

func TestResolveLoginIntent_guestProfileDefaultsClaimGuest(t *testing.T) {
	cfg := &configpkg.Config{
		Profile: configpkg.Profile{
			GuestURL:    "https://console.test/signin/guest?token=abc",
			GuestUserID: "usr_guest",
			APIKey:      "hk_guest_key",
		},
	}

	require.Equal(t, hookdeck.CLIAuthIntentClaimGuest, resolveLoginIntent(cfg))
}

func TestResolveLoginIntent_noGuestProfileDefaultsLogin(t *testing.T) {
	cfg := &configpkg.Config{
		Profile: configpkg.Profile{},
	}

	require.Equal(t, hookdeck.CLIAuthIntentLogin, resolveLoginIntent(cfg))
}

func TestBuildStartLoginInput_guestCredentialsOnlyForClaim(t *testing.T) {
	cfg := &configpkg.Config{
		DeviceName: "dev",
		Profile: configpkg.Profile{
			GuestURL:    "https://console.test/signin/guest?token=abc",
			GuestUserID: "usr_guest",
			APIKey:      "hk_guest_key",
		},
	}

	login_input := buildStartLoginInput(cfg, hookdeck.CLIAuthIntentLogin, "")
	require.Equal(t, hookdeck.CLIAuthIntentLogin, login_input.AuthIntent)
	require.Empty(t, login_input.GuestUserID)
	require.Empty(t, login_input.GuestAPIKey)

	claim_input := buildStartLoginInput(cfg, hookdeck.CLIAuthIntentClaimGuest, "")
	require.Equal(t, "usr_guest", claim_input.GuestUserID)
	require.Equal(t, "hk_guest_key", claim_input.GuestAPIKey)
}
