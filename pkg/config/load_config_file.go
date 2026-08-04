package config

import (
	"github.com/spf13/viper"
)

// LoadConfigFromFile loads an existing Hookdeck CLI TOML config into Config with viper and
// filesystem helpers wired so SaveProfile / writeConfig work. Intended for integration tests;
// production startup should use InitConfig.
func LoadConfigFromFile(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	c := &Config{viper: v, fs: newConfigFS()}
	c.Profile.Config = c
	c.constructConfig()
	return c, nil
}
