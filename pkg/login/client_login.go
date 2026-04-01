package login

import (
	"fmt"
	"io"
	"net/url"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/briandowns/spinner"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/open"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

var openBrowser = open.Browser
var canOpenBrowser = open.CanOpenBrowser

// Login function is used to obtain credentials via hookdeck dashboard.
func Login(config *configpkg.Config, input io.Reader) error {
	var s *spinner.Spinner

	if config.Profile.APIKey != "" {
		log.WithFields(log.Fields{
			"prefix": "login.Login",
			"APIKey": config.Profile.APIKey,
		}).Debug("Logging in with API key")

		s = ansi.StartNewSpinner("Verifying credentials...", os.Stdout)
		response, err := config.GetAPIClient().ValidateAPIKey()
		if err != nil {
			return err
		}

		message := SuccessMessage(response.UserName, response.UserEmail, response.OrganizationName, response.ProjectName, response.ProjectMode == "console")
		ansi.StopSpinner(s, message, os.Stdout)

		config.Profile.ApplyValidateAPIKeyResponse(response, true)

		if err = config.Profile.SaveProfile(); err != nil {
			return err
		}
		if err = config.Profile.UseProfile(); err != nil {
			return err
		}

		return nil
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

	if isSSH() || !canOpenBrowser() {
		fmt.Printf("To authenticate with Hookdeck, please go to: %s\n", session.BrowserURL)

		s = ansi.StartNewSpinner("Waiting for confirmation...", os.Stdout)
	} else {
		fmt.Printf("Press Enter to open the browser (^C to quit)")
		fmt.Fscanln(input)

		s = ansi.StartNewSpinner("Waiting for confirmation...", os.Stdout)

		err = openBrowser(session.BrowserURL)
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

	message := SuccessMessage(response.UserName, response.UserEmail, response.OrganizationName, response.ProjectName, response.ProjectMode == "console")
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
