package hookdeck

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

//
// Constants
//

// TelemetryHeaderName is the HTTP header used to send CLI telemetry data.
const TelemetryHeaderName = "X-Hookdeck-CLI-Telemetry"

//
// Public types
//

// CLITelemetry is the structure that holds telemetry data sent to Hookdeck in
// API requests.
type CLITelemetry struct {
	Source            string `json:"source"`
	Environment       string `json:"environment"`
	CommandPath       string `json:"command_path"`
	InvocationID      string `json:"invocation_id"`
	DeviceName        string `json:"device_name"`
	GeneratedResource bool   `json:"generated_resource,omitempty"`
	MCPClient         string `json:"mcp_client,omitempty"`
	// CommandFlags lists flag names (not values) explicitly set on the CLI for this invocation.
	CommandFlags []string `json:"command_flags,omitempty"`

	// Disabled is set from the user's config (telemetry_disabled).
	// It is checked by PerformRequest as a fallback so that clients
	// which don't set Client.TelemetryDisabled still respect opt-out.
	Disabled bool `json:"-"`
}

// SetCommandContext sets the telemetry values for the command being executed.
func (t *CLITelemetry) SetCommandContext(cmd *cobra.Command) {
	t.CommandPath = cmd.CommandPath()
	t.GeneratedResource = false

	for _, value := range cmd.Annotations {
		// Generated commands have an annotation called "operation", we can
		// search for that to let us know it's generated
		if value == "operation" {
			t.GeneratedResource = true
		}
	}
}

// SetDeviceName puts the device name into telemetry
func (t *CLITelemetry) SetDeviceName(deviceName string) {
	t.DeviceName = deviceName
}

// SetCommandFlagsFromCobra records which flags the user explicitly set (names only).
func (t *CLITelemetry) SetCommandFlagsFromCobra(cmd *cobra.Command) {
	t.CommandFlags = CollectChangedFlagNames(cmd)
}

// SetSource sets the telemetry source (e.g. "cli" or "mcp").
func (t *CLITelemetry) SetSource(source string) {
	t.Source = source
}

// SetEnvironment sets the runtime environment (e.g. "interactive" or "ci").
func (t *CLITelemetry) SetEnvironment(env string) {
	t.Environment = env
}

// SetInvocationID sets the unique invocation identifier.
func (t *CLITelemetry) SetInvocationID(id string) {
	t.InvocationID = id
}

// SetDisabled records the config-level telemetry opt-out in the singleton.
func (t *CLITelemetry) SetDisabled(disabled bool) {
	t.Disabled = disabled
}

//
// Public functions
//

// GetTelemetryInstance returns the CLITelemetry instance (initializing it
// first if necessary).
func GetTelemetryInstance() *CLITelemetry {
	once.Do(func() {
		instance = &CLITelemetry{}
	})

	return instance
}

// NewInvocationID generates a unique invocation ID with the prefix "inv_"
// followed by 16 hex characters (8 random bytes).
func NewInvocationID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "inv_" + hex.EncodeToString(b)
}

// CollectChangedFlagNames returns sorted unique names of flags the user explicitly
// set on the command line (pflag Changed), walking from cmd up to the root. Values
// are not included.
func CollectChangedFlagNames(cmd *cobra.Command) []string {
	if cmd == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var names []string
	add := func(fs *pflag.FlagSet) {
		if fs == nil {
			return
		}
		fs.VisitAll(func(f *pflag.Flag) {
			if !f.Changed {
				return
			}
			if _, ok := seen[f.Name]; ok {
				return
			}
			seen[f.Name] = struct{}{}
			names = append(names, f.Name)
		})
	}
	for c := cmd; c != nil; c = c.Parent() {
		add(c.Flags())
		add(c.PersistentFlags())
	}
	sort.Strings(names)
	return names
}

// DetectEnvironment returns "ci" if a CI environment is detected,
// "interactive" otherwise.
func DetectEnvironment() string {
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" ||
		os.Getenv("GITLAB_CI") == "true" || os.Getenv("BUILDKITE") == "true" ||
		os.Getenv("TF_BUILD") == "true" || os.Getenv("JENKINS_URL") != "" ||
		os.Getenv("CODEBUILD_BUILD_ID") != "" {
		return "ci"
	}
	return "interactive"
}

//
// Private variables
//

var instance *CLITelemetry
var once sync.Once

// ResetTelemetryInstanceForTesting resets the global telemetry singleton so
// that tests can start with a fresh instance. Must only be called from tests.
func ResetTelemetryInstanceForTesting() {
	instance = nil
	once = sync.Once{}
}

//
// Private functions
//

func getTelemetryHeader() (string, error) {
	telemetry := GetTelemetryInstance()
	b, err := json.Marshal(telemetry)

	if err != nil {
		return "", err
	}

	return string(b), nil
}

// telemetryOptedOut returns true if the user has opted out of telemetry,
// false otherwise. It checks both the environment variable and the
// config-based flag.
func telemetryOptedOut(envVar string, configDisabled bool) bool {
	if configDisabled {
		return true
	}
	envVar = strings.ToLower(envVar)
	return envVar == "1" || envVar == "true"
}
