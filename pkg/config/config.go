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
	GlobalConfigFile string
	GlobalConfig     *viper.Viper
	LocalConfigFile  string
	LocalConfig      *viper.Viper
}

// GetConfigFolder retrieves the folder where the profiles file is stored
// It searches for the xdg environment path first and will secondarily
// place it in the home directory
func (c *Config) GetConfigFolder(xdgPath string) string {
	configPath := xdgPath

	log.WithFields(log.Fields{
		"prefix": "config.Config.GetProfilesFolder",
		"path":   configPath,
	}).Debug("Using profiles file")

	if configPath == "" {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		configPath = filepath.Join(home, ".config")
	}

	return filepath.Join(configPath, "hookdeck")
}

// InitConfig reads in profiles file and ENV variables if set.
func (c *Config) InitConfig() {
	c.Profile.Config = c

	logFormatter := &prefixed.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC1123,
	}

	c.GlobalConfig = viper.New()
	c.LocalConfig = viper.New()

	// Read global config
	GlobalConfigFolder := c.GetConfigFolder(os.Getenv("XDG_CONFIG_HOME"))
	c.GlobalConfigFile = filepath.Join(GlobalConfigFolder, "config.toml")
	c.GlobalConfig.SetConfigType("toml")
	c.GlobalConfig.SetConfigFile(c.GlobalConfigFile)
	c.GlobalConfig.SetConfigPermissions(os.FileMode(0600))
	// Try to change permissions manually, because we used to create files
	// with default permissions (0644)
	err := os.Chmod(c.GlobalConfigFile, os.FileMode(0600))
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("%s", err)
	}
	if err := c.GlobalConfig.ReadInConfig(); err == nil {
		log.WithFields(log.Fields{
			"prefix": "config.Config.InitConfig",
			"path":   c.GlobalConfig.ConfigFileUsed(),
		}).Debug("Using profiles file")
	}

	// Read local config
	workspaceFolder, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	LocalConfigFile := ""
	if c.LocalConfigFile == "" {
		LocalConfigFile = filepath.Join(workspaceFolder, ".hookdeck/config.toml")
	} else {
		if filepath.IsAbs(c.LocalConfigFile) {
			LocalConfigFile = c.LocalConfigFile
		} else {
			LocalConfigFile = filepath.Join(workspaceFolder, c.LocalConfigFile)
		}
	}
	c.LocalConfig.SetConfigType("toml")
	c.LocalConfig.SetConfigFile(LocalConfigFile)
	c.LocalConfigFile = LocalConfigFile
	if err := c.LocalConfig.ReadInConfig(); err == nil {
		log.WithFields(log.Fields{
			"prefix": "config.Config.InitConfig",
			"path":   c.LocalConfig.ConfigFileUsed(),
		}).Debug("Using profiles file")
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
}

// EditConfig opens the configuration file in the default editor.
func (c *Config) EditConfig() error {
	var err error

	fmt.Println("Opening config file:", c.LocalConfigFile)

	switch runtime.GOOS {
	case "darwin", "linux":
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		cmd := exec.Command(editor, c.LocalConfigFile)
		// Some editors detect whether they have control of stdin/out and will
		// fail if they do not.
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout

		return cmd.Run()
	case "windows":
		// As far as I can tell, Windows doesn't have an easily accesible or
		// comparable option to $EDITOR, so default to notepad for now
		err = exec.Command("notepad", c.LocalConfigFile).Run()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	return err
}

// UseWorkspace selects the active workspace to be used
func (c *Config) UseWorkspace(local bool, teamId string, teamMode string) error {
	c.Profile.TeamID = teamId
	c.Profile.TeamMode = teamMode
	return c.Profile.SaveProfile(local)
}

func (c *Config) ListProfiles() []string {
	var profiles []string

	for field, value := range c.GlobalConfig.AllSettings() {
		if isProfile(value) {
			profiles = append(profiles, field)
		}
	}

	return profiles
}

// RemoveAllProfiles removes all the profiles from the config file.
func (c *Config) RemoveAllProfiles() error {
	runtimeViper := c.GlobalConfig
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
	runtimeViper.SetConfigFile(c.GlobalConfig.ConfigFileUsed())
	c.GlobalConfig = runtimeViper
	return c.GlobalConfig.WriteConfig()
}

func (c *Config) SaveLocalConfig() error {
	if err := ensureDirectoy(filepath.Dir(c.LocalConfigFile)); err != nil {
		return err
	}
	return c.LocalConfig.WriteConfig()
}

func ensureDirectoy(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

// Construct the config struct from flags > local config > global config
func (c *Config) constructConfig() {
	c.Color = getStringConfig([]string{c.Color, c.LocalConfig.GetString("color"), c.GlobalConfig.GetString(("color")), "auto"})
	c.LogLevel = getStringConfig([]string{c.LogLevel, c.LocalConfig.GetString("log"), c.GlobalConfig.GetString(("log")), "info"})
	c.APIBaseURL = getStringConfig([]string{c.APIBaseURL, c.LocalConfig.GetString("api_base"), c.GlobalConfig.GetString(("api_base")), hookdeck.DefaultAPIBaseURL})
	c.DashboardBaseURL = getStringConfig([]string{c.DashboardBaseURL, c.LocalConfig.GetString("dashboard_base"), c.GlobalConfig.GetString(("dashboard_base")), hookdeck.DefaultDashboardBaseURL})
	c.ConsoleBaseURL = getStringConfig([]string{c.ConsoleBaseURL, c.LocalConfig.GetString("console_base"), c.GlobalConfig.GetString(("console_base")), hookdeck.DefaultConsoleBaseURL})
	c.WSBaseURL = getStringConfig([]string{c.WSBaseURL, c.LocalConfig.GetString("ws_base"), c.GlobalConfig.GetString(("ws_base")), hookdeck.DefaultWebsocektURL})
	c.Profile.Name = getStringConfig([]string{c.Profile.Name, c.LocalConfig.GetString("profile"), c.GlobalConfig.GetString(("profile")), hookdeck.DefaultProfileName})
	c.Profile.APIKey = getStringConfig([]string{c.Profile.APIKey, c.LocalConfig.GetString("api_key"), c.GlobalConfig.GetString((c.Profile.GetConfigField("api_key"))), ""})
	c.Profile.TeamID = getStringConfig([]string{c.Profile.TeamID, c.LocalConfig.GetString("workspace_id"), c.LocalConfig.GetString("team_id"), c.GlobalConfig.GetString((c.Profile.GetConfigField("workspace_id"))), c.GlobalConfig.GetString((c.Profile.GetConfigField("team_id"))), ""})
	c.Profile.TeamMode = getStringConfig([]string{c.Profile.TeamMode, c.LocalConfig.GetString("workspace_mode"), c.LocalConfig.GetString("team_mode"), c.GlobalConfig.GetString((c.Profile.GetConfigField("workspace_mode"))), c.GlobalConfig.GetString((c.Profile.GetConfigField("team_mode"))), ""})
}

func getStringConfig(values []string) string {
	for _, str := range values {
		if str != "" {
			return str
		}
	}

	return values[len(values)-1]
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
