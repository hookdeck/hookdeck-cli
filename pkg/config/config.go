package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// ColorOn represnets the on-state for colors
const ColorOn = "on"

// ColorOff represents the off-state for colors
const ColorOff = "off"

// ColorAuto represents the auto-state for colors
const ColorAuto = "auto"

// Config handles all overall configuration for the CLI
type Config struct {
	Profile    Profile
	Color      string
	LogLevel   string
	DeviceName string

	// Helpers
	APIBaseURL       string
	DashboardBaseURL string
	ConsoleBaseURL   string
	WSBaseURL        string
	Insecure         bool

	// Config
	ConfigFileFlag string // flag -- should NOT use this directly
	configFile     string // resolved path of config file
	viper          *viper.Viper

	// Internal
	fs ConfigFS
}

// InitConfig reads in profiles file and ENV variables if set.
func (c *Config) InitConfig() {
	if c.fs == nil {
		c.fs = newConfigFS()
	}

	c.Profile.Config = c

	// Set log level
	switch c.LogLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.Fatalf("Unrecognized log level value: %s. Expected one of debug, info, warn, error.", c.LogLevel)
	}

	logFormatter := &prefixed.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC1123,
	}

	c.viper = viper.New()

	configPath, isGlobalConfig := c.getConfigPath(c.ConfigFileFlag)
	c.configFile = configPath
	c.viper.SetConfigType("toml")
	c.viper.SetConfigFile(c.configFile)

	if isGlobalConfig {
		// Try to change permissions manually, because we used to create files
		// with default permissions (0644)
		c.viper.SetConfigPermissions(os.FileMode(0600))
		err := os.Chmod(c.configFile, os.FileMode(0600))
		if err != nil && !os.IsNotExist(err) {
			log.Fatalf("%s", err)
		}
	}

	// Check if config file exists, create if not
	var exists bool
	var checkErr error
	exists, checkErr = c.fs.fileExists(c.configFile)
	if checkErr != nil {
		log.Fatalf("Error checking existence of config file %s: %v", c.configFile, checkErr)
	}

	if !exists {
		log.WithFields(log.Fields{"prefix": "config.Config.InitConfig", "path": c.configFile}).Debug("Configuration file not found. Creating a new one.")
		createErr := c.fs.makePath(c.configFile)
		if createErr != nil {
			log.Fatalf("Error creating directory for config file %s: %v", c.configFile, createErr)
		}

		file, createErr := os.Create(c.configFile)
		if createErr != nil {
			log.Fatalf("Error creating new config file %s: %v", c.configFile, createErr)
		}
		file.Close() // Immediately close the newly created file

		if isGlobalConfig {
			permErr := os.Chmod(c.configFile, os.FileMode(0600))
			if permErr != nil {
				log.Fatalf("Error setting permissions for new config file %s: %v", c.configFile, permErr)
			}
		}
	}

	// Read config file
	log.WithFields(log.Fields{
		"prefix": "config.Config.InitConfig",
		"path":   c.viper.ConfigFileUsed(),
	}).Debug("Reading config file")
	if readErr := c.viper.ReadInConfig(); readErr != nil {
		log.Fatalf("Error reading config file %s: %v", c.viper.ConfigFileUsed(), readErr)
	} else {
		log.WithFields(log.Fields{"prefix": "config.Config.InitConfig", "path": c.viper.ConfigFileUsed()}).Debug("Successfully read config file")
	}

	// Construct the config struct
	c.constructConfig()

	if c.DeviceName == "" {
		deviceName, err := os.Hostname()
		if err != nil {
			deviceName = "unknown"
		}
		c.DeviceName = deviceName
	}

	switch c.Color {
	case ColorOn:
		ansi.ForceColors = true
		logFormatter.ForceColors = true
	case ColorOff:
		ansi.DisableColors = true
		logFormatter.DisableColors = true
	case ColorAuto:
		// Nothing to do
	default:
		log.Fatalf("Unrecognized color value: %s. Expected one of on, off, auto.", c.Color)
	}

	log.SetFormatter(logFormatter)
}

// UseProject selects the active project to be used
func (c *Config) UseProject(projectId string, projectMode string) error {
	c.Profile.ProjectId = projectId
	c.Profile.ProjectMode = projectMode
	return c.Profile.SaveProfile()
}

// UseProjectLocal selects the active project to be used in local config
// Returns true if a new file was created, false if existing file was updated
func (c *Config) UseProjectLocal(projectId string, projectMode string) (bool, error) {
	// Get current working directory
	workingDir, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create .hookdeck directory
	hookdeckDir := filepath.Join(workingDir, ".hookdeck")
	if err := os.MkdirAll(hookdeckDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create .hookdeck directory: %w", err)
	}

	// Define local config path
	localConfigPath := filepath.Join(hookdeckDir, "config.toml")

	// Check if local config file exists
	fileExists, err := c.fs.fileExists(localConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check if local config exists: %w", err)
	}

	// Update in-memory state
	c.Profile.ProjectId = projectId
	c.Profile.ProjectMode = projectMode

	// Write to local config file using shared helper
	if err := c.writeProjectConfig(localConfigPath, !fileExists); err != nil {
		return false, err
	}

	return !fileExists, nil
}

// writeProjectConfig writes the current profile's project configuration to the specified config file
func (c *Config) writeProjectConfig(configPath string, isNewFile bool) error {
	// Create a new viper instance for the config
	v := viper.New()
	v.SetConfigType("toml")

	// If file exists, read it first to preserve any other settings
	if !isNewFile {
		v.SetConfigFile(configPath)
		_ = v.ReadInConfig() // Ignore error - we'll overwrite anyway
	}

	// Set all profile fields
	c.setProfileFieldsInViper(v)

	// Write config file using WriteConfigAs which explicitly takes a path
	// This avoids the viper internal "configPath" issue
	writeErr := v.WriteConfigAs(configPath)
	if writeErr != nil {
		return fmt.Errorf("failed to write config to %s: %w", configPath, writeErr)
	}

	return nil
}

// setProfileFieldsInViper sets the current profile's fields in the given viper instance
func (c *Config) setProfileFieldsInViper(v *viper.Viper) {
	if c.Profile.APIKey != "" {
		v.Set(c.Profile.getConfigField("api_key"), c.Profile.APIKey)
	}
	v.Set("profile", c.Profile.Name)
	v.Set(c.Profile.getConfigField("project_id"), c.Profile.ProjectId)
	v.Set(c.Profile.getConfigField("project_mode"), c.Profile.ProjectMode)
	if c.Profile.GuestURL != "" {
		v.Set(c.Profile.getConfigField("guest_url"), c.Profile.GuestURL)
	}
}

// GetConfigFile returns the path of the currently loaded config file
func (c *Config) GetConfigFile() string {
	return c.configFile
}

// FileExists checks if a file exists at the given path
func (c *Config) FileExists(path string) (bool, error) {
	return c.fs.fileExists(path)
}

func (c *Config) ListProfiles() []string {
	var profiles []string

	for field, value := range c.viper.AllSettings() {
		if isProfile(value) {
			profiles = append(profiles, field)
		}
	}

	return profiles
}

// RemoveAllProfiles removes all the profiles from the config file.
// TODO: consider adding log to clarify which config file is being used
func (c *Config) RemoveAllProfiles() error {
	runtimeViper := c.viper
	var err error

	for field, value := range runtimeViper.AllSettings() {
		if isProfile(value) {
			runtimeViper, err = removeKey(runtimeViper, field)
			if err != nil {
				return err
			}
		}
	}

	runtimeViper, err = removeKey(runtimeViper, "profile")
	if err != nil {
		return err
	}

	runtimeViper.SetConfigType("toml")
	runtimeViper.SetConfigFile(c.viper.ConfigFileUsed())
	c.viper = runtimeViper
	return c.writeConfig()
}

func (c *Config) writeConfig() error {
	if err := c.fs.makePath(c.viper.ConfigFileUsed()); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"prefix": "config.Config.writeConfig",
		"path":   c.viper.ConfigFileUsed(),
	}).Debug("Writing config")

	return c.viper.WriteConfig()
}

// Construct the config struct from flags > local config > global config
func (c *Config) constructConfig() {
	c.Color = stringCoalesce(c.Color, c.viper.GetString(("color")), "auto")
	c.LogLevel = stringCoalesce(c.LogLevel, c.viper.GetString(("log")), "info")
	c.APIBaseURL = stringCoalesce(c.APIBaseURL, c.viper.GetString(("api_base")), hookdeck.DefaultAPIBaseURL)
	c.DashboardBaseURL = stringCoalesce(c.DashboardBaseURL, c.viper.GetString(("dashboard_base")), hookdeck.DefaultDashboardBaseURL)
	c.ConsoleBaseURL = stringCoalesce(c.ConsoleBaseURL, c.viper.GetString(("console_base")), hookdeck.DefaultConsoleBaseURL)
	c.WSBaseURL = stringCoalesce(c.WSBaseURL, c.viper.GetString(("ws_base")), hookdeck.DefaultWebsocektURL)
	c.Profile.Name = stringCoalesce(c.Profile.Name, c.viper.GetString(("profile")), hookdeck.DefaultProfileName)
	// Needs to support both profile-based config
	// and top-level config for backward compat. For example:
	// ````
	// [default]
	//   api_key = "key"
	// ````
	// vs
	// ````
	// api_key = "key"
	// ```
	// Also support a few deprecated terminology
	// "workspace" > "team"
	// TODO: use "project" instead of "workspace"
	// TODO: use "cli_key" instead of "api_key"
	c.Profile.APIKey = stringCoalesce(c.Profile.APIKey, c.viper.GetString(c.Profile.getConfigField("api_key")), c.viper.GetString("api_key"), "")

	c.Profile.ProjectId = stringCoalesce(c.Profile.ProjectId, c.viper.GetString(c.Profile.getConfigField("project_id")), c.viper.GetString("project_id"), c.viper.GetString(c.Profile.getConfigField("workspace_id")), c.viper.GetString(c.Profile.getConfigField("team_id")), c.viper.GetString("workspace_id"), "")

	c.Profile.ProjectMode = stringCoalesce(c.Profile.ProjectMode, c.viper.GetString(c.Profile.getConfigField("project_mode")), c.viper.GetString("project_mode"), c.viper.GetString(c.Profile.getConfigField("workspace_mode")), c.viper.GetString(c.Profile.getConfigField("team_mode")), c.viper.GetString("workspace_mode"), "")

	c.Profile.GuestURL = stringCoalesce(c.Profile.GuestURL, c.viper.GetString(c.Profile.getConfigField("guest_url")), c.viper.GetString("guest_url"), "")
}

// getConfigPath returns the path for the config file.
// Precedence:
// - path (if path is provided)
// - `${PWD}/.hookdeck/config.toml`
// - `${HOME}/.config/hookdeck/config.toml`
// Returns the path string and a boolean indicating whether it's the global default path.
func (c *Config) getConfigPath(path string) (string, bool) {
	workspaceFolder, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	if path != "" {
		if filepath.IsAbs(path) {
			return path, false
		}
		return filepath.Join(workspaceFolder, path), false
	}

	localConfigPath := filepath.Join(workspaceFolder, ".hookdeck/config.toml")
	localConfigExists, err := c.fs.fileExists(localConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	if localConfigExists {
		return localConfigPath, false
	}

	globalConfigFolder := getSystemConfigFolder(os.Getenv("XDG_CONFIG_HOME"))
	return filepath.Join(globalConfigFolder, "config.toml"), true
}
