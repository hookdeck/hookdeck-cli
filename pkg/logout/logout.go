package logout

import (
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
)

// Logout function is used to clear the credentials set for the current Profile
func Logout(config *config.Config) error {
	if config.APIKey == "" {
		fmt.Println("You are already logged out.")
		return nil
	}

	fmt.Println("Logging out...")

	if err := config.ClearWorkspace(); err != nil {
		return err
	}

	// TOOD: figure out success notice for logout?
	// username := config.Client.UserName
	// if err := config.Clear(); err != nil {
	// 	return err
	// }

	// fmt.Printf("Credentials have been cleared for %s.\n", username)

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
