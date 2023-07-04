package config

type Profile struct {
	Name     string // profile name
	APIKey   string
	TeamID   string
	TeamMode string

	Config *Config
}

// GetConfigField returns the configuration field for the specific profile
func (p *Profile) GetConfigField(field string) string {
	return p.Name + "." + field
}

func (p *Profile) SaveProfile(local bool) error {
	// in local, we're d setting mode because it should always be inbound
	// as a user can't have both inbound & console teams (i think)
	// and we don't need to expose it to the end user
	if local {
		p.Config.GlobalConfig.Set(p.GetConfigField("api_key"), p.APIKey)
		if err := p.Config.GlobalConfig.WriteConfig(); err != nil {
			return err
		}
		p.Config.LocalConfig.Set("workspace_id", p.TeamID)
		return p.Config.LocalConfig.WriteConfig()
	} else {
		p.Config.GlobalConfig.Set(p.GetConfigField("api_key"), p.APIKey)
		p.Config.GlobalConfig.Set(p.GetConfigField("workspace_id"), p.TeamID)
		p.Config.GlobalConfig.Set(p.GetConfigField("workspace_mode"), p.TeamMode)
		return p.Config.GlobalConfig.WriteConfig()
	}
}

func (p *Profile) RemoveProfile() error {
	var err error
	runtimeViper := p.Config.GlobalConfig

	runtimeViper, err = removeKey(runtimeViper, "profile")
	if err != nil {
		return err
	}
	runtimeViper, err = removeKey(runtimeViper, p.Name)
	if err != nil {
		return err
	}

	runtimeViper.SetConfigType("toml")
	runtimeViper.SetConfigFile(p.Config.GlobalConfig.ConfigFileUsed())
	p.Config.GlobalConfig = runtimeViper
	return p.Config.GlobalConfig.WriteConfig()
}

func (p *Profile) UseProfile() error {
	p.Config.GlobalConfig.Set("profile", p.Name)
	return p.Config.GlobalConfig.WriteConfig()
}
