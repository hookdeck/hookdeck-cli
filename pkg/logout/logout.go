package logout

import (
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
)

// Logout function is used to clear the credentials set for the current Profile
func Logout(config *config.Config) error {
	if config.Profile.APIKey == "" {
		fmt.Println("You are already logged out.")
		return nil
	}

	fmt.Println("Logging out...")

	if err := config.Profile.RemoveProfile(); err != nil {
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
