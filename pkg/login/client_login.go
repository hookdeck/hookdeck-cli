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

const hookdeckCLIAuthPath = "/cli/auth"

// Links provides the URLs for the CLI to continue the login flow
type Links struct {
	BrowserURL string `json:"browser_url"`
	PollURL    string `json:"poll_url"`
}

// Login function is used to obtain credentials via hookdeck dashboard.
func Login(config *config.Config, input io.Reader) error {
	links, err := getLinks(config.APIBaseURL, config.Profile.DeviceName)
	if err != nil {
		return err
	}

	var s *spinner.Spinner

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
	config.Profile.DisplayName = response.UserName
	config.Profile.TeamName = response.TeamName

	profileErr := config.Profile.CreateProfile()
	if profileErr != nil {
		return profileErr
	}

	message := SuccessMessage(response.UserName, response.TeamName)
	ansi.StopSpinner(s, message, os.Stdout)

	return nil
}

func getLinks(baseURL string, deviceName string) (*Links, error) {
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	client := &hookdeck.Client{
		BaseURL: parsedBaseURL,
	}

	// TODO: Figure out device name
	data := url.Values{}
	data.Set("device_name", deviceName)

	res, err := client.Post(context.TODO(), hookdeckCLIAuthPath, nil, nil)
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
