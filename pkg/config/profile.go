package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// Profile handles all things related to managing the project specific configurations
type Profile struct {
	Config      *Config
	DeviceName  string
	ProfileName string
	TeamName    string
	TeamID      string
	TeamMode    string
	APIKey      string
	ClientID    string
	DisplayName string
}

type partialPollResponse struct {
	TeamName string `json:"team_name"`
}

// CreateProfile creates a profile when logging in
func (p *Profile) CreateProfile() error {
	writeErr := p.writeProfile(p.Config.GlobalConfig)
	if writeErr != nil {
		return writeErr
	}

	return nil
}

// GetColor gets the color setting for the user based on the flag or the
// persisted color stored in the config file
func (p *Profile) GetColor() (string, error) {
	color := p.Config.GlobalConfig.GetString("color")
	if color != "" {
		return color, nil
	}

	color = p.Config.GlobalConfig.GetString(p.GetConfigField("color"))
	switch color {
	case "", ColorAuto:
		return ColorAuto, nil
	case ColorOn:
		return ColorOn, nil
	case ColorOff:
		return ColorOff, nil
	default:
		return "", fmt.Errorf("color value not supported: %s", color)
	}
}

// GetDeviceName returns the configured device name
func (p *Profile) GetDeviceName() (string, error) {
	if os.Getenv("HOOKDECK_DEVICE_NAME") != "" {
		return os.Getenv("HOOKDECK_DEVICE_NAME"), nil
	}

	if p.DeviceName != "" {
		return p.DeviceName, nil
	}

	if err := p.Config.GlobalConfig.ReadInConfig(); err == nil {
		return p.Config.GlobalConfig.GetString(p.GetConfigField("device_name")), nil
	}

	return "", validators.ErrDeviceNameNotConfigured
}

// GetAPIKey will return the existing key for the given profile
func (p *Profile) GetAPIKey() (string, error) {
	envKey := os.Getenv("HOOKDECK_CLI_KEY")
	if envKey != "" {
		err := validators.APIKey(envKey)
		if err != nil {
			return "", err
		}

		return envKey, nil
	}

	if p.APIKey != "" {
		err := validators.APIKey(p.APIKey)
		if err != nil {
			return "", err
		}

		return p.APIKey, nil
	}

	// Try to fetch the API key from the configuration file
	if err := p.Config.GlobalConfig.ReadInConfig(); err == nil {
		key := p.Config.GlobalConfig.GetString(p.GetConfigField("api_key"))

		err := validators.APIKey(key)
		if err != nil {
			return "", err
		}

		return key, nil
	}

	return "", validators.ErrAPIKeyNotConfigured
}

// GetDisplayName returns the account display name of the user
func (p *Profile) GetDisplayName() string {
	if err := p.Config.GlobalConfig.ReadInConfig(); err == nil {
		return p.Config.GlobalConfig.GetString(p.GetConfigField("display_name"))
	}

	return ""
}

func (p *Profile) GetTeamMode() string {
	if p.TeamMode != "" {
		return p.TeamMode
	}
	if err := p.Config.GlobalConfig.ReadInConfig(); err == nil {
		return p.Config.GlobalConfig.GetString(p.GetConfigField("team_mode"))
	}

	return ""
}

func (p *Profile) GetTeamId() string {
	if p.TeamID != "" {
		return p.TeamID
	}
	if err := p.Config.GlobalConfig.ReadInConfig(); err == nil {
		return p.Config.GlobalConfig.GetString(p.GetConfigField("team_id"))
	}

	return ""
}

func (p *Profile) refreshTeamName() string {
	apiKey, err := p.GetAPIKey()
	if err != nil {
		panic(err)
	}

	parsedURL, err := url.Parse(p.Config.APIBaseURL + "/cli-auth/poll?key=" + apiKey)
	if err != nil {
		panic(err)
	}

	client := &hookdeck.Client{
		BaseURL: parsedURL,
	}

	res, err := client.Get(context.TODO(), parsedURL.Path, parsedURL.Query().Encode(), nil)
	if err != nil {
		panic(err)
	}

	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	var response partialPollResponse

	err = json.Unmarshal(body, &response)
	if err != nil {
		panic(err)
	}

	p.TeamName = response.TeamName
	p.CreateProfile()

	return response.TeamName
}

// GetDisplayName returns the account display name of the team
func (p *Profile) GetTeamName() string {
	if err := p.Config.GlobalConfig.ReadInConfig(); err != nil {
		return ""
	}

	teamName := p.Config.GlobalConfig.GetString(p.GetConfigField("team_name"))

	if teamName == p.GetDisplayName() {
		// Could be a bug where we used to store the display name in the
		// team name field. But it could also be a legit team name that
		// just happens to be the same as the user display name.
		//
		// Call the CLI poll endpoint to make sure.
		teamName = p.refreshTeamName()
	}

	return teamName
}

// GetTerminalPOSDeviceID returns the device id from the config for Terminal quickstart to use
func (p *Profile) GetTerminalPOSDeviceID() string {
	if err := p.Config.GlobalConfig.ReadInConfig(); err == nil {
		return p.Config.GlobalConfig.GetString(p.GetConfigField("terminal_pos_device_id"))
	}

	return ""
}

// GetConfigField returns the configuration field for the specific profile
func (p *Profile) GetConfigField(field string) string {
	return p.ProfileName + "." + field
}

// RegisterAlias registers an alias for a given key.
func (p *Profile) RegisterAlias(alias, key string) {
	p.Config.GlobalConfig.RegisterAlias(p.GetConfigField(alias), p.GetConfigField(key))
}

// WriteConfigField updates a configuration field and writes the updated
// configuration to disk.
func (p *Profile) WriteConfigField(field, value string) error {
	p.Config.GlobalConfig.Set(p.GetConfigField(field), value)
	return p.Config.GlobalConfig.WriteConfig()
}

// DeleteConfigField deletes a configuration field.
func (p *Profile) DeleteConfigField(field string) error {
	v, err := removeKey(p.Config.GlobalConfig, p.GetConfigField(field))
	if err != nil {
		return err
	}

	return p.writeProfile(v)
}

func (p *Profile) writeProfile(runtimeViper *viper.Viper) error {
	profilesFile := p.Config.GlobalConfig.ConfigFileUsed()

	err := makePath(profilesFile)
	if err != nil {
		return err
	}

	if p.DeviceName != "" {
		runtimeViper.Set(p.GetConfigField("device_name"), strings.TrimSpace(p.DeviceName))
	}

	if p.APIKey != "" {
		runtimeViper.Set(p.GetConfigField("api_key"), strings.TrimSpace(p.APIKey))
	}

	if p.ClientID != "" {
		runtimeViper.Set(p.GetConfigField("client_id"), strings.TrimSpace(p.ClientID))
	}

	if p.DisplayName != "" {
		runtimeViper.Set(p.GetConfigField("display_name"), strings.TrimSpace(p.DisplayName))
	}

	if p.TeamName != "" {
		runtimeViper.Set(p.GetConfigField("team_name"), strings.TrimSpace(p.TeamName))
	}

	if p.TeamID != "" {
		runtimeViper.Set(p.GetConfigField("team_id"), strings.TrimSpace(p.TeamID))
	}

	if p.TeamMode != "" {
		runtimeViper.Set(p.GetConfigField("team_mode"), strings.TrimSpace(p.TeamMode))
	}

	runtimeViper.MergeInConfig()

	runtimeViper.SetConfigFile(profilesFile)

	// Ensure we preserve the config file type
	runtimeViper.SetConfigType(filepath.Ext(profilesFile))

	err = runtimeViper.WriteConfig()
	if err != nil {
		return err
	}

	return nil
}
