package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
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

// getConfigFolder retrieves the folder where the profiles file is stored
// It searches for the xdg environment path first and will secondarily
// place it in the home directory
func getConfigFolder(xdgPath string) string {
	configPath := xdgPath

	if configPath == "" {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		configPath = filepath.Join(home, ".config")
	}

	log.WithFields(log.Fields{
		"prefix": "config.Config.GetProfilesFolder",
		"path":   configPath,
	}).Debug("Using profiles folder")

	return filepath.Join(configPath, "hookdeck")
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

	// Read config file
	if err := c.viper.ReadInConfig(); err == nil {
		log.WithFields(log.Fields{
			"prefix": "config.Config.InitConfig",
			"path":   c.viper.ConfigFileUsed(),
		}).Debug("Reading config file")
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

// EditConfig opens the configuration file in the default editor.
func (c *Config) EditConfig() error {
	var err error

	fmt.Println("Opening config file:", c.configFile)

	switch runtime.GOOS {
	case "darwin", "linux":
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		cmd := exec.Command(editor, c.configFile)
		// Some editors detect whether they have control of stdin/out and will
		// fail if they do not.
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout

		return cmd.Run()
	case "windows":
		// As far as I can tell, Windows doesn't have an easily accesible or
		// comparable option to $EDITOR, so default to notepad for now
		err = exec.Command("notepad", c.configFile).Run()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	return err
}

// UseProject selects the active project to be used
func (c *Config) UseProject(teamId string, teamMode string) error {
	c.Profile.TeamID = teamId
	c.Profile.TeamMode = teamMode
	return c.Profile.SaveProfile()
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
	return c.WriteConfig()
}

func (c *Config) WriteConfig() error {
	if err := c.fs.makePath(c.viper.ConfigFileUsed()); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"prefix": "config.Config.WriteConfig",
		"path":   c.viper.WriteConfig(),
	}).Debug("Writing config")

	return c.viper.WriteConfig()
}

// Construct the config struct from flags > local config > global config
func (c *Config) constructConfig() {
	c.Color = getStringConfig([]string{c.Color, c.viper.GetString(("color")), "auto"})
	c.LogLevel = getStringConfig([]string{c.LogLevel, c.viper.GetString(("log")), "info"})
	c.APIBaseURL = getStringConfig([]string{c.APIBaseURL, c.viper.GetString(("api_base")), hookdeck.DefaultAPIBaseURL})
	c.DashboardBaseURL = getStringConfig([]string{c.DashboardBaseURL, c.viper.GetString(("dashboard_base")), hookdeck.DefaultDashboardBaseURL})
	c.ConsoleBaseURL = getStringConfig([]string{c.ConsoleBaseURL, c.viper.GetString(("console_base")), hookdeck.DefaultConsoleBaseURL})
	c.WSBaseURL = getStringConfig([]string{c.WSBaseURL, c.viper.GetString(("ws_base")), hookdeck.DefaultWebsocektURL})
	c.Profile.Name = getStringConfig([]string{c.Profile.Name, c.viper.GetString(("profile")), hookdeck.DefaultProfileName})
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
	c.Profile.APIKey = getStringConfig([]string{c.Profile.APIKey, c.viper.GetString(c.Profile.GetConfigField("api_key")), c.viper.GetString("api_key"), ""})
	c.Profile.TeamID = getStringConfig([]string{c.Profile.TeamID, c.viper.GetString(c.Profile.GetConfigField("workspace_id")), c.viper.GetString(c.Profile.GetConfigField("team_id")), c.viper.GetString("workspace_id"), ""})
	c.Profile.TeamMode = getStringConfig([]string{c.Profile.TeamMode, c.viper.GetString(c.Profile.GetConfigField("workspace_mode")), c.viper.GetString(c.Profile.GetConfigField("team_mode")), c.viper.GetString("workspace_mode"), ""})
}

func getStringConfig(values []string) string {
	for _, str := range values {
		if str != "" {
			return str
		}
	}

	return values[len(values)-1]
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

	globalConfigFolder := getConfigFolder(os.Getenv("XDG_CONFIG_HOME"))
	return filepath.Join(globalConfigFolder, "config.toml"), true
}

// isProfile identifies whether a value in the config pertains to a profile.
func isProfile(value interface{}) bool {
	// TODO: ianjabour - ideally find a better way to identify projects in config
	_, ok := value.(map[string]interface{})
	return ok
}

// Temporary workaround until https://github.com/spf13/viper/pull/519 can remove a key from viper
func removeKey(v *viper.Viper, key string) (*viper.Viper, error) {
	configMap := v.AllSettings()
	path := strings.Split(key, ".")
	lastKey := strings.ToLower(path[len(path)-1])
	deepestMap := deepSearch(configMap, path[0:len(path)-1])
	delete(deepestMap, lastKey)

	buf := new(bytes.Buffer)

	encodeErr := toml.NewEncoder(buf).Encode(configMap)
	if encodeErr != nil {
		return nil, encodeErr
	}

	nv := viper.New()
	nv.SetConfigType("toml") // hint to viper that we've encoded the data as toml

	err := nv.ReadConfig(buf)
	if err != nil {
		return nil, err
	}

	return nv, nil
}

// taken from https://github.com/spf13/viper/blob/master/util.go#L199,
// we need this to delete configs, remove when viper supprts unset natively
func deepSearch(m map[string]interface{}, path []string) map[string]interface{} {
	for _, k := range path {
		m2, ok := m[k]
		if !ok {
			// intermediate key does not exist
			// => create it and continue from there
			m3 := make(map[string]interface{})
			m[k] = m3
			m = m3

			continue
		}

		m3, ok := m2.(map[string]interface{})
		if !ok {
			// intermediate key is a value
			// => replace with a new map
			m3 = make(map[string]interface{})
			m[k] = m3
		}

		// continue search from here
		m = m3
	}

	return m
}
