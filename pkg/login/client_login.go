package login

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/briandowns/spinner"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/open"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

var openBrowser = open.Browser
var canOpenBrowser = open.CanOpenBrowser
var stdinIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// ErrRejectedKeyNoTerminal is returned when the key was rejected and there is no
// terminal to complete browser sign-in with. It names the likely cause because
// "invalid or expired" is often untrue: a project API key is valid but not
// accepted by the CLI auth endpoints, and an org key is not accepted at all.
var ErrRejectedKeyNoTerminal = errors.New(
	"the API key was rejected, and browser sign-in needs an interactive terminal; " +
		"check the key is a CLI key from hookdeck login rather than a project or organization API key, " +
		"or use hookdeck ci --api-key with a project API key",
)

// ErrNoCredentialsNoTerminal is returned when nothing is saved to sign in with
// and there is no terminal to complete browser sign-in with. It names the ways
// in that need no terminal, because the only other advice - "run it in a
// terminal" - is no help to the CI job, container or agent that hit this.
var ErrNoCredentialsNoTerminal = errors.New(
	"no saved credentials, and browser sign-in needs an interactive terminal; " +
		"use hookdeck ci --api-key with a project API key, " +
		"hookdeck login --cli-key with a CLI key, " +
		"or set HOOKDECK_API_KEY to a project API key",
)

// ErrGuestUpgradeNoTerminal is returned when a guest profile's browser sign-up
// would have to read stdin and there is no terminal. Unlike the other two this
// one has no headless equivalent - a permanent account is created in the
// browser - so it names signing in with an account that already exists.
var ErrGuestUpgradeNoTerminal = errors.New(
	"creating a permanent account needs browser sign-up, and browser sign-up needs an interactive terminal; " +
		"run hookdeck login in a terminal to keep this sandbox's data, " +
		"or sign in to an account you already have with hookdeck ci --api-key or hookdeck login --cli-key",
)

// browserSignInNeedsStdin reports whether waitForLoginSession would take the
// branch that prompts for Enter and opens a browser. Its other branch prints
// the URL and polls without reading stdin, which works headlessly and must not
// be blocked.
func browserSignInNeedsStdin() bool {
	return !isSSH() && canOpenBrowser()
}

const guestUpgradePollInterval = 2 * time.Second
const guestUpgradeMaxAttempts = 2 * 60

// Login function is used to obtain credentials via hookdeck dashboard.
func Login(config *configpkg.Config, input io.Reader) error {
	var s *spinner.Spinner

	if config.Profile.APIKey != "" {
		log.WithFields(log.Fields{
			"prefix": "login.Login",
		}).Debug("Logging in with saved API key")

		s = ansi.StartNewSpinner("Verifying credentials...", os.Stdout)
		response, err := config.GetAPIClient().ValidateAPIKey()
		if err != nil {
			ansi.StopSpinner(s, "", os.Stdout)
			if !hookdeck.IsUnauthorizedError(err) {
				return err
			}
			// Refuse only where the flow would have to read stdin, mirroring the
			// branch in waitForLoginSession.
			if !stdinIsTerminal() && browserSignInNeedsStdin() {
				return ErrRejectedKeyNoTerminal
			}
			// Must clear the key first or we would re-enter this branch only.
			fmt.Fprintln(os.Stdout, "Your saved API key is no longer valid. Starting browser sign-in...")
			config.Profile.APIKey = ""
		} else if response.UserID != "" {
			if config.Profile.GuestURL == "" || !response.UserIsGuest {
				message := SuccessMessage(response.UserName, response.UserEmail, response.OrganizationName, response.ProjectName, configpkg.IsConsoleProject(response.ProjectType, response.ProjectMode))
				ansi.StopSpinner(s, message, os.Stdout)

				config.Profile.ApplyValidateAPIKeyResponse(response, true)

				if err = config.Profile.SaveProfile(); err != nil {
					return err
				}
				if err = config.Profile.UseProfile(); err != nil {
					return err
				}

				config.RefreshCachedAPIClient()

				return nil
			}
			ansi.StopSpinner(s, "", os.Stdout)
			return waitForGuestUpgrade(config, input)
		} else {
			ansi.StopSpinner(s, "", os.Stdout)
			if !stdinIsTerminal() {
				return project.ErrProjectScopedCredentials
			}
			fmt.Fprintln(os.Stdout, "Your saved key is scoped to a single project (CI). Starting browser sign-in...")
			config.Profile.APIKey = ""
		}
	}

	// Same guard, for the path that never had a key to reject. An empty config
	// skipped the block above entirely and arrived here, where waitForLoginSession
	// prompted for Enter, read EOF from /dev/null instantly, opened a browser
	// window on somebody's desktop and then polled forever. Refuse before
	// StartLogin so no session is created for a sign-in nobody can complete.
	if !stdinIsTerminal() && browserSignInNeedsStdin() {
		return ErrNoCredentialsNoTerminal
	}

	parsedBaseURL, err := url.Parse(config.APIBaseURL)
	if err != nil {
		return err
	}

	client := &hookdeck.Client{
		BaseURL:           parsedBaseURL,
		TelemetryDisabled: config.TelemetryDisabled,
	}

	session, err := client.StartLogin(config.DeviceName)
	if err != nil {
		return err
	}

	return waitForLoginSession(config, input, session)
}

func waitForLoginSession(config *configpkg.Config, input io.Reader, session *hookdeck.LoginSession) error {
	var s *spinner.Spinner

	if isSSH() || !canOpenBrowser() {
		fmt.Printf("To authenticate with Hookdeck, please go to: %s\n", session.BrowserURL)

		s = ansi.StartNewSpinner("Waiting for confirmation...", os.Stdout)
	} else {
		// Reached only with a terminal on stdin (Login refuses otherwise), so
		// there is someone to press Enter and a terminal to deliver ^C to.
		fmt.Println("Press Enter to open the browser (^C to quit)")
		fmt.Fscanln(input)

		// Print the URL whether or not the browser opens (#373). open.Browser
		// is exec.Command(...).Start(), which returns nil the moment the child
		// is spawned, so a browser that dies straight after - WSL, containers,
		// VS Code Remote - reports success and leaves the user with a spinner
		// and no link. The error branch below is the detectable half only.
		fmt.Printf("To authenticate with Hookdeck, please go to: %s\n", session.BrowserURL)

		if err := openBrowser(session.BrowserURL); err != nil {
			fmt.Println("Could not open the browser for you; use the link above.")
		}

		s = ansi.StartNewSpinner("Waiting for confirmation...", os.Stdout)
	}

	response, err := session.WaitForAPIKey(0, 0)
	if err != nil {
		return err
	}

	err = validators.APIKey(response.APIKey)
	if err != nil {
		return err
	}

	config.Profile.ApplyPollAPIKeyResponse(response, "")

	if err = config.Profile.SaveProfile(); err != nil {
		return err
	}
	if err = config.Profile.UseProfile(); err != nil {
		return err
	}

	config.RefreshCachedAPIClient()

	message := SuccessMessage(response.UserName, response.UserEmail, response.OrganizationName, response.ProjectName, configpkg.IsConsoleProject(response.ProjectType, response.ProjectMode))
	ansi.StopSpinner(s, message, os.Stdout)

	return nil
}

func GuestLogin(config *configpkg.Config) (string, error) {
	parsedBaseURL, err := url.Parse(config.APIBaseURL)
	if err != nil {
		return "", err
	}

	client := &hookdeck.Client{
		BaseURL:           parsedBaseURL,
		TelemetryDisabled: config.TelemetryDisabled,
	}

	fmt.Println("\n🚩 You are using the CLI for the first time without a permanent account. Creating a guest account...")

	session, err := client.StartGuestLogin(config.DeviceName)
	if err != nil {
		return "", err
	}

	response, err := session.WaitForAPIKey(0, 0)
	if err != nil {
		return "", err
	}

	if err = validators.APIKey(response.APIKey); err != nil {
		return "", err
	}

	config.Profile.ApplyPollAPIKeyResponse(response, session.GuestURL)

	if err = config.Profile.SaveProfile(); err != nil {
		return "", err
	}
	if err = config.Profile.UseProfile(); err != nil {
		return "", err
	}

	return session.GuestURL, nil
}

func CILogin(config *configpkg.Config, apiKey string, name string) error {
	parsedBaseURL, err := url.Parse(config.APIBaseURL)
	if err != nil {
		return err
	}

	client := &hookdeck.Client{
		BaseURL:           parsedBaseURL,
		APIKey:            apiKey,
		TelemetryDisabled: config.TelemetryDisabled,
	}

	deviceName := name
	if deviceName == "" {
		deviceName = config.DeviceName
	}
	response, err := client.CreateCIClient(hookdeck.CreateCIClientInput{
		DeviceName: deviceName,
	})
	if err != nil {
		return err
	}

	if err := validators.APIKey(response.APIKey); err != nil {
		return err
	}

	config.Profile.ApplyCIClient(response)

	if err = config.Profile.SaveProfile(); err != nil {
		return err
	}
	if err = config.Profile.UseProfile(); err != nil {
		return err
	}

	color := ansi.Color(os.Stdout)

	log.Println(fmt.Sprintf(
		"The Hookdeck CLI is configured on project %s in organization %s\n",
		color.Bold(response.ProjectName),
		color.Bold(response.OrganizationName),
	))

	return nil
}

func isSSH() bool {
	if os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" {
		return true
	}

	return false
}

func waitForGuestUpgrade(config *configpkg.Config, input io.Reader) error {
	// The third copy of the same branch (#408). Refuse before
	// RefreshGuestSigninLink mints a link for a sign-up nobody can complete:
	// without this, a guest profile with a still-valid key opened a browser
	// window unasked and then polled for four minutes.
	if !stdinIsTerminal() && browserSignInNeedsStdin() {
		return ErrGuestUpgradeNoTerminal
	}

	guestURL := RefreshGuestSigninLink(config)
	if guestURL == "" {
		return fmt.Errorf("unable to create guest sign-up link")
	}

	var s *spinner.Spinner
	if isSSH() || !canOpenBrowser() {
		fmt.Printf("To create a permanent Hookdeck account, please go to: %s\n", guestURL)
		s = ansi.StartNewSpinner("Waiting for account creation...", os.Stdout)
	} else {
		// Reached only with a terminal on stdin (guarded above), so there is
		// someone to press Enter and a terminal to deliver ^C to.
		fmt.Println("Press Enter to open the browser (^C to quit)")
		fmt.Fscanln(input)

		// Printed whether or not the browser opens, for the reason given in
		// waitForLoginSession (#373).
		fmt.Printf("To create a permanent Hookdeck account, please go to: %s\n", guestURL)

		if err := openBrowser(guestURL); err != nil {
			fmt.Println("Could not open the browser for you; use the link above.")
		}

		s = ansi.StartNewSpinner("Waiting for account creation...", os.Stdout)
	}

	response, err := waitForGuestUpgradeCompletion(config)
	if err != nil {
		return err
	}

	config.Profile.ApplyValidateAPIKeyResponse(response, true)

	if err = config.Profile.SaveProfile(); err != nil {
		return err
	}
	if err = config.Profile.UseProfile(); err != nil {
		return err
	}

	config.RefreshCachedAPIClient()

	message := SuccessMessage(response.UserName, response.UserEmail, response.OrganizationName, response.ProjectName, configpkg.IsConsoleProject(response.ProjectType, response.ProjectMode))
	ansi.StopSpinner(s, message, os.Stdout)

	return nil
}

func waitForGuestUpgradeCompletion(config *configpkg.Config) (*hookdeck.ValidateAPIKeyResponse, error) {
	for attempt := 0; attempt < guestUpgradeMaxAttempts; attempt++ {
		response, err := config.GetAPIClient().ValidateAPIKey()
		if err != nil {
			return nil, err
		}
		if !response.UserIsGuest {
			return response, nil
		}
		time.Sleep(guestUpgradePollInterval)
	}

	return nil, fmt.Errorf("exceeded max attempts waiting for guest account creation")
}
