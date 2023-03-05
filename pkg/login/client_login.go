package login

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/briandowns/spinner"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/open"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

var openBrowser = open.Browser
var canOpenBrowser = open.CanOpenBrowser

const hookdeckCLIAuthPath = "/cli-auth"

// Links provides the URLs for the CLI to continue the login flow
type Links struct {
	BrowserURL string `json:"browser_url"`
	PollURL    string `json:"poll_url"`
}

// Login function is used to obtain credentials via hookdeck dashboard.
func Login(config *config.Config, input io.Reader) error {
	var s *spinner.Spinner

	if config.Profile.APIKey != "" {
		s = ansi.StartNewSpinner("Verifying CLI Key...", os.Stdout)
		response, err := ValidateKey(config.APIBaseURL, config.Profile.APIKey)
		if err != nil {
			return err
		}

		config.Profile.ClientID = response.ClientID
		config.Profile.DisplayName = response.UserName
		config.Profile.TeamName = response.TeamName
		config.Profile.TeamMode = response.TeamMode
		config.Profile.TeamID = response.TeamID

		profileErr := config.Profile.CreateProfile()
		if profileErr != nil {
			return profileErr
		}

		message := SuccessMessage(response.UserName, response.TeamName, config.Profile.TeamMode == "console")
		ansi.StopSpinner(s, message, os.Stdout)

		return nil
	}

	links, err := getLinks(config.APIBaseURL, config.Profile.DeviceName)
	if err != nil {
		return err
	}

	if isSSH() || !canOpenBrowser() {
		fmt.Printf("To authenticate with Hookdeck, please go to: %s\n", links.BrowserURL)

		s = ansi.StartNewSpinner("Waiting for confirmation...", os.Stdout)
	} else {
		fmt.Printf("Press Enter to open the browser (^C to quit)")
		fmt.Fscanln(input)

		s = ansi.StartNewSpinner("Waiting for confirmation...", os.Stdout)

		err = openBrowser(links.BrowserURL)
		if err != nil {
			msg := fmt.Sprintf("Failed to open browser, please go to %s manually.", links.BrowserURL)
			ansi.StopSpinner(s, msg, os.Stdout)
			s = ansi.StartNewSpinner("Waiting for confirmation...", os.Stdout)
		}
	}

	// Call poll function
	response, err := PollForKey(links.PollURL, 0, 0)
	if err != nil {
		return err
	}

	validateErr := validators.APIKey(response.APIKey)
	if validateErr != nil {
		return validateErr
	}

	config.Profile.APIKey = response.APIKey
	config.Profile.ClientID = response.ClientID
	config.Profile.DisplayName = response.UserName
	config.Profile.TeamName = response.TeamName
	config.Profile.TeamMode = response.TeamMode
	config.Profile.TeamID = response.TeamID

	profileErr := config.Profile.CreateProfile()
	if profileErr != nil {
		return profileErr
	}

	message := SuccessMessage(response.UserName, response.TeamName, response.TeamMode == "console")
	ansi.StopSpinner(s, message, os.Stdout)

	return nil
}

func GuestLogin(config *config.Config) (string, error) {
	parsedBaseURL, err := url.Parse(config.APIBaseURL)
	if err != nil {
		return "", err
	}

	client := &hookdeck.Client{
		BaseURL: parsedBaseURL,
	}

	fmt.Println("🚩 Not connected with any account. Creating a guest account...")

	guest_user, err := client.CreateGuestUser(hookdeck.CreateGuestUserInput{
		DeviceName: config.Profile.DeviceName,
	})
	if err != nil {
		return "", err
	}

	// Call poll function
	response, err := PollForKey(guest_user.PollURL, 0, 0)
	if err != nil {
		return "", err
	}

	validateErr := validators.APIKey(response.APIKey)
	if validateErr != nil {
		return "", validateErr
	}

	config.Profile.APIKey = response.APIKey
	config.Profile.ClientID = response.ClientID
	config.Profile.DisplayName = response.UserName
	config.Profile.TeamName = response.TeamName
	config.Profile.TeamMode = response.TeamMode
	config.Profile.TeamID = response.TeamID

	profileErr := config.Profile.CreateProfile()
	if profileErr != nil {
		return "", profileErr
	}

	return guest_user.Url, nil
}

func getLinks(baseURL string, deviceName string) (*Links, error) {
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	client := &hookdeck.Client{
		BaseURL: parsedBaseURL,
	}

	data := struct {
		DeviceName string `json:"device_name"`
	}{}
	data.DeviceName = deviceName
	json_data, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	res, err := client.Post(context.TODO(), hookdeckCLIAuthPath, json_data, nil)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status code: %d %s", res.StatusCode, string(bodyBytes))
	}

	var links Links

	err = json.Unmarshal(bodyBytes, &links)
	if err != nil {
		return nil, err
	}

	return &links, nil
}

func isSSH() bool {
	if os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" {
		return true
	}

	return false
}
