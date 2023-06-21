package config

type Profile struct {
	Name     string // profile name
	APIKey 	 string
	TeamID 	 string
	TeamMode string

	Config *Config
}

// GetConfigField returns the configuration field for the specific profile
func (p *Profile) GetConfigField(field string) string {
	return p.Name + "." + field
}

func (p *Profile) SaveProfile() error {
	p.Config.GlobalConfig.Set(p.GetConfigField("api_key"), p.APIKey)
	p.Config.GlobalConfig.Set(p.GetConfigField("team_id"), p.TeamID)
	p.Config.GlobalConfig.Set(p.GetConfigField("team_mode"), p.TeamMode)
	return p.Config.GlobalConfig.WriteConfig()
}

func (p *Profile) RemoveProfile() error {
	var err error
	runtimeViper := p.Config.GlobalConfig

	// TODO: see if we can switch to another profile
	runtimeViper, err = removeKey(runtimeViper, "profile");
	if err != nil {
		return err
	}
	runtimeViper, err = removeKey(runtimeViper, p.Name);
	if err != nil {
		return err
	}

	runtimeViper.SetConfigType("toml")
	runtimeViper.SetConfigFile(p.Config.GlobalConfig.ConfigFileUsed())
	p.Config.GlobalConfig = runtimeViper
	return p.Config.GlobalConfig.WriteConfig()
}
