package login

import (
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
			// Rejected key: continue into browser login below (must clear key first
			// or we would re-enter this branch only).
			fmt.Fprintln(os.Stdout, "Your saved API key is no longer valid. Starting browser sign-in...")
			config.Profile.APIKey = ""
		} else if response.UserID != "" {
			if config.Profile.GuestURL == "" || !response.UserIsGuest {
				message := SuccessMessage(response.UserName, response.UserEmail, response.OrganizationName, response.ProjectName, response.ProjectProduct == configpkg.ProjectProductConsole)
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
		fmt.Printf("Press Enter to open the browser (^C to quit)")
		fmt.Fscanln(input)

		s = ansi.StartNewSpinner("Waiting for confirmation...", os.Stdout)

		err := openBrowser(session.BrowserURL)
		if err != nil {
			msg := fmt.Sprintf("Failed to open browser, please go to %s manually.", session.BrowserURL)
			ansi.StopSpinner(s, msg, os.Stdout)
			s = ansi.StartNewSpinner("Waiting for confirmation...", os.Stdout)
		}
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

	message := SuccessMessage(response.UserName, response.UserEmail, response.OrganizationName, response.ProjectName, response.ProjectProduct == configpkg.ProjectProductConsole)
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
	guestURL := RefreshGuestSigninLink(config)
	if guestURL == "" {
		return fmt.Errorf("unable to create guest sign-up link")
	}

	var s *spinner.Spinner
	if isSSH() || !canOpenBrowser() {
		fmt.Printf("To create a permanent Hookdeck account, please go to: %s\n", guestURL)
		s = ansi.StartNewSpinner("Waiting for account creation...", os.Stdout)
	} else {
		fmt.Printf("Press Enter to open the browser (^C to quit)")
		fmt.Fscanln(input)

		s = ansi.StartNewSpinner("Waiting for account creation...", os.Stdout)

		err := openBrowser(guestURL)
		if err != nil {
			msg := fmt.Sprintf("Failed to open browser, please go to %s manually.", guestURL)
			ansi.StopSpinner(s, msg, os.Stdout)
			s = ansi.StartNewSpinner("Waiting for account creation...", os.Stdout)
		}
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

	message := SuccessMessage(response.UserName, response.UserEmail, response.OrganizationName, response.ProjectName, response.ProjectProduct == configpkg.ProjectProductConsole)
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
