package config

import "github.com/hookdeck/hookdeck-cli/pkg/validators"

type Profile struct {
	Name     string // profile name
	APIKey   string
	TeamID   string
	TeamMode string

	Config *Config
}

// getConfigField returns the configuration field for the specific profile
func (p *Profile) getConfigField(field string) string {
	return p.Name + "." + field
}

func (p *Profile) SaveProfile() error {
	p.Config.viper.Set(p.getConfigField("api_key"), p.APIKey)
	p.Config.viper.Set(p.getConfigField("workspace_id"), p.TeamID)
	p.Config.viper.Set(p.getConfigField("workspace_mode"), p.TeamMode)
	return p.Config.writeConfig()
}

func (p *Profile) RemoveProfile() error {
	var err error
	runtimeViper := p.Config.viper

	runtimeViper, err = removeKey(runtimeViper, "profile")
	if err != nil {
		return err
	}
	runtimeViper, err = removeKey(runtimeViper, p.Name)
	if err != nil {
		return err
	}

	runtimeViper.SetConfigType("toml")
	runtimeViper.SetConfigFile(p.Config.viper.ConfigFileUsed())
	p.Config.viper = runtimeViper
	return p.Config.writeConfig()
}

func (p *Profile) UseProfile() error {
	p.Config.viper.Set("profile", p.Name)
	return p.Config.writeConfig()
}

func (p *Profile) ValidateAPIKey() error {
	if p.APIKey == "" {
		return validators.ErrAPIKeyNotConfigured
	}
	return nil
}
