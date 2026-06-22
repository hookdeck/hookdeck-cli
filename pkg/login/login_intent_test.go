package login

import (
	"strings"
	"testing"

	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/require"
)

func TestResolveLoginIntent_explicitFlags(t *testing.T) {
	cfg := &configpkg.Config{
		Profile: configpkg.Profile{
			GuestURL: "https://console.test/signin/guest?token=abc",
		},
	}

	intent, err := resolveLoginIntent(cfg, strings.NewReader("\n"), Options{LoginExisting: true})
	require.NoError(t, err)
	require.Equal(t, hookdeck.CLIAuthIntentLogin, intent)

	intent, err = resolveLoginIntent(cfg, strings.NewReader("\n"), Options{CreateAccount: true})
	require.NoError(t, err)
	require.Equal(t, hookdeck.CLIAuthIntentCreateNew, intent)

	intent, err = resolveLoginIntent(cfg, strings.NewReader("\n"), Options{ClaimGuest: true})
	require.NoError(t, err)
	require.Equal(t, hookdeck.CLIAuthIntentClaimGuest, intent)

	_, err = resolveLoginIntent(cfg, strings.NewReader("\n"), Options{ClaimGuest: true, LoginExisting: true})
	require.Error(t, err)
}

func TestResolveLoginIntent_nonTTYGuestDefaultsClaimGuest(t *testing.T) {
	cfg := &configpkg.Config{
		Profile: configpkg.Profile{
			GuestURL: "https://console.test/signin/guest?token=abc",
			GuestUserID: "usr_guest",
			APIKey: "hk_guest_key",
		},
	}

	intent, err := resolveLoginIntent(cfg, strings.NewReader("\n"), Options{})
	require.NoError(t, err)
	require.Equal(t, hookdeck.CLIAuthIntentClaimGuest, intent)
}

func TestBuildStartLoginInput_guestCredentialsOnlyForClaimAndCreate(t *testing.T) {
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
