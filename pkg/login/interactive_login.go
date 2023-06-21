package login

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// InteractiveLogin lets the user set configuration on the command line
func InteractiveLogin(config *config.Config) error {
	fmt.Println("$ hookdeck login -i")
	return nil
	// apiKey, err := getConfigureAPIKey(os.Stdin)
	// if err != nil {
	// 	return err
	// }

	// config.Profile.DeviceName = getConfigureDeviceName(os.Stdin)

	// s := ansi.StartNewSpinner("Waiting for confirmation...", os.Stdout)

	// // Call poll function
	// response, err := PollForKey(config.APIBaseURL+"/cli-auth/poll?key="+apiKey, 0, 0)
	// if err != nil {
	// 	return err
	// }

	// parsedBaseURL, err := url.Parse(config.APIBaseURL)
	// if err != nil {
	// 	return err
	// }

	// client := &hookdeck.Client{
	// 	BaseURL: parsedBaseURL,
	// 	APIKey:  response.APIKey,
	// }

	// data := struct {
	// 	DeviceName string `json:"device_name"`
	// }{}
	// data.DeviceName = config.Profile.DeviceName
	// json_data, err := json.Marshal(data)
	// if err != nil {
	// 	return err
	// }

	// _, err = client.Put(context.TODO(), "/cli/"+response.ClientID, json_data, nil)
	// if err != nil {
	// 	return err
	// }

	// config.Profile.APIKey = response.APIKey
	// config.Profile.ClientID = response.ClientID
	// config.Profile.DisplayName = response.UserName
	// config.Profile.TeamName = response.TeamName
	// config.Profile.TeamMode = response.TeamMode
	// config.Profile.TeamID = response.TeamID

	// profileErr := config.Profile.SaveProfile()
	// if profileErr != nil {
	// 	ansi.StopSpinner(s, "", os.Stdout)
	// 	return profileErr
	// }

	// message := SuccessMessage(response.UserName, response.TeamName, response.TeamMode == "console")

	// ansi.StopSpinner(s, message, os.Stdout)

	// return nil
}

func getConfigureAPIKey(input io.Reader) (string, error) {
	fmt.Print("Enter your CLI API key: ")

	apiKey, err := securePrompt(input)
	if err != nil {
		return "", err
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", errors.New("CLI API key is required, please provide your CLI API key")
	}

	err = validators.APIKey(apiKey)
	if err != nil {
		return "", err
	}

	fmt.Printf("Your API key is: %s\n", redactAPIKey(apiKey))

	return apiKey, nil
}

func getConfigureDeviceName(input io.Reader) string {
	hostName, _ := os.Hostname()
	reader := bufio.NewReader(input)

	color := ansi.Color(os.Stdout)
	fmt.Printf("How would you like to identify this device in the Hookdeck Dashboard? [default: %s] ", color.Bold(color.Cyan(hostName)))

	deviceName, _ := reader.ReadString('\n')
	if strings.TrimSpace(deviceName) == "" {
		deviceName = hostName
	}

	return deviceName
}

// redactAPIKey returns a redacted version of API keys. The first 8 and last 4
// characters are not redacted, everything else is replaced by "*" characters.
//
// It panics if the provided string has less than 12 characters.
func redactAPIKey(apiKey string) string {
	var b strings.Builder

	b.WriteString(apiKey[0:8])                         // #nosec G104 (gosec bug: https://github.com/securego/gosec/issues/267)
	b.WriteString(strings.Repeat("*", len(apiKey)-12)) // #nosec G104 (gosec bug: https://github.com/securego/gosec/issues/267)
	b.WriteString(apiKey[len(apiKey)-4:])              // #nosec G104 (gosec bug: https://github.com/securego/gosec/issues/267)

	return b.String()
}

func securePrompt(input io.Reader) (string, error) {
	if input == os.Stdin {
		// terminal.ReadPassword does not reset terminal state on ctrl-c interrupts,
		// this results in the terminal input staying hidden after program exit.
		// We need to manually catch the interrupt and restore terminal state before exiting.
		signalChan, err := protectTerminalState()
		if err != nil {
			return "", err
		}

		buf, err := term.ReadPassword(int(syscall.Stdin)) //nolint:unconvert
		if err != nil {
			return "", err
		}

		signal.Stop(signalChan)

		fmt.Print("\n")

		return string(buf), nil
	}

	reader := bufio.NewReader(input)

	return reader.ReadString('\n')
}

func protectTerminalState() (chan os.Signal, error) {
	originalTerminalState, err := term.GetState(int(syscall.Stdin)) //nolint:unconvert
	if err != nil {
		return nil, err
	}

	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, os.Interrupt)

	go func() {
		<-signalChan
		term.Restore(int(syscall.Stdin), originalTerminalState) //nolint:unconvert
		os.Exit(1)
	}()

	return signalChan, nil
}
