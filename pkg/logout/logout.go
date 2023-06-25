package logout

import (
	"fmt"
	"os"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
)

// Logout function is used to clear the credentials set for the current Profile
func Logout(config *config.Config) error {
	if config.Profile.APIKey == "" {
		fmt.Println("You are already logged out.")
		return nil
	}

	fmt.Println("Logging out...")

	profileName := config.Profile.Name
	if err := config.Profile.RemoveProfile(); err != nil {
		return err
	}

	color := ansi.Color(os.Stdout)
	if profileName == "default" {
		fmt.Printf("Credentials have been cleared for the %s project.\n", color.Green(profileName))
	} else {
		fmt.Printf("Credentials have been cleared for %s.\n", color.Green(profileName))
	}

	return nil
}

// All function is used to clear the credentials on all profiles
func All(cfg *config.Config) error {
	fmt.Println("Logging out...")

	err := cfg.RemoveAllProfiles()
	if err != nil {
		return err
	}

	fmt.Println("Credentials have been cleared for all projects.")

	return nil
}
