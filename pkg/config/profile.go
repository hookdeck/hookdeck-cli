package config

import (
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type Profile struct {
	Name        string // profile name
	APIKey      string
	ProjectId   string
	ProjectMode string
	ProjectType string // display type: Gateway, Outpost, Console
	GuestURL    string // URL to create permanent account for guest users

	Config *Config
}

// getConfigField returns the configuration field for the specific profile
func (p *Profile) getConfigField(field string) string {
	return p.Name + "." + field
}

// ResolveProjectType returns the API project type for this profile: the stored
// type if there is one, otherwise derived from the legacy mode. Values written
// by older CLIs held a display label, so everything goes through
// NormalizeProjectType rather than being trusted as-is.
func (p *Profile) ResolveProjectType() string {
	if t := NormalizeProjectType(p.ProjectType); t != "" {
		return t
	}
	return ModeToType(p.ProjectMode)
}

func (p *Profile) SaveProfile() error {
	p.Config.viper.Set(p.getConfigField("api_key"), p.APIKey)
	p.Config.viper.Set(p.getConfigField("project_id"), p.ProjectId)
	p.Config.viper.Set(p.getConfigField("project_mode"), p.ProjectMode)
	p.Config.viper.Set(p.getConfigField("project_type"), p.ResolveProjectType())
	p.Config.viper.Set(p.getConfigField("guest_url"), p.GuestURL)

	if err := p.removeLegacyConfigKeys(); err != nil {
		return err
	}

	return p.Config.writeConfig()
}

func (p *Profile) removeLegacyConfigKeys() error {
	legacyKeys := []string{"workspace_id", "workspace_mode", "team_id", "team_mode"}
	configFile := p.Config.viper.ConfigFileUsed()
	var err error
	for _, key := range legacyKeys {
		p.Config.viper, err = removeKey(p.Config.viper, p.getConfigField(key))
		if err != nil {
			return err
		}
		p.Config.viper, err = removeKey(p.Config.viper, key)
		if err != nil {
			return err
		}
	}
	if configFile != "" {
		p.Config.viper.SetConfigFile(configFile)
	}
	return nil
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
