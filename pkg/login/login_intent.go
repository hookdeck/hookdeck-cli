package login

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// Options configures hookdeck login when guest credentials or intent selection applies.
type Options struct {
	ClaimGuest     bool
	LoginExisting  bool
	CreateAccount  bool
}

func (o Options) hasExplicitIntent() bool {
	return o.ClaimGuest || o.LoginExisting || o.CreateAccount
}

func resolveLoginIntent(config *configpkg.Config, input io.Reader, opts Options) (hookdeck.CLIAuthIntent, error) {
	if opts.hasExplicitIntent() {
		set := 0
		if opts.ClaimGuest {
			set++
		}
		if opts.LoginExisting {
			set++
		}
		if opts.CreateAccount {
			set++
		}
		if set > 1 {
			return "", fmt.Errorf("use only one of --claim-guest, --login, or --create-account")
		}
		if opts.ClaimGuest {
			return hookdeck.CLIAuthIntentClaimGuest, nil
		}
		if opts.LoginExisting {
			return hookdeck.CLIAuthIntentLogin, nil
		}
		return hookdeck.CLIAuthIntentCreateNew, nil
	}

	has_guest_profile := config != nil && config.Profile.GuestURL != ""
	if !has_guest_profile {
		return hookdeck.CLIAuthIntentLogin, nil
	}

	if !isInteractiveTTY(input) {
		return hookdeck.CLIAuthIntentClaimGuest, nil
	}

	return promptGuestLoginIntent(input)
}

func isInteractiveTTY(input io.Reader) bool {
	if input != os.Stdin {
		return false
	}
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func promptGuestLoginIntent(input io.Reader) (hookdeck.CLIAuthIntent, error) {
	fmt.Fprintln(os.Stdout, "Your guest session has data in Hookdeck Console. What would you like to do?")
	fmt.Fprintln(os.Stdout, "  1) Persist my data (claim this guest account) [default]")
	fmt.Fprintln(os.Stdout, "  2) Log in to an existing Hookdeck account")
	fmt.Fprintln(os.Stdout, "  3) Create a new Hookdeck account")
	fmt.Fprint(os.Stdout, "Choice [1]: ")

	reader := bufio.NewReader(input)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return hookdeck.CLIAuthIntentClaimGuest, nil
	}

	choice := strings.TrimSpace(line)
	switch choice {
	case "", "1":
		return hookdeck.CLIAuthIntentClaimGuest, nil
	case "2":
		return hookdeck.CLIAuthIntentLogin, nil
	case "3":
		return hookdeck.CLIAuthIntentCreateNew, nil
	default:
		return "", fmt.Errorf("invalid choice %q: enter 1, 2, or 3", choice)
	}
}

func ResolveLoginIntent(config *configpkg.Config, input io.Reader, opts Options) (hookdeck.CLIAuthIntent, error) {
	return resolveLoginIntent(config, input, opts)
}

func BuildStartLoginInput(
	config *configpkg.Config,
	intent hookdeck.CLIAuthIntent,
	saved_guest_api_key string,
) hookdeck.StartLoginInput {
	return buildStartLoginInput(config, intent, saved_guest_api_key)
}

func buildStartLoginInput(
	config *configpkg.Config,
	intent hookdeck.CLIAuthIntent,
	saved_guest_api_key string,
) hookdeck.StartLoginInput {
	input := hookdeck.StartLoginInput{
		DeviceName: config.DeviceName,
		AuthIntent: intent,
	}

	if intent == hookdeck.CLIAuthIntentClaimGuest || intent == hookdeck.CLIAuthIntentCreateNew {
		input.GuestUserID = guestCredentialsUserID(config)
		input.GuestAPIKey = guestCredentialsAPIKey(config, saved_guest_api_key)
	}

	return input
}
