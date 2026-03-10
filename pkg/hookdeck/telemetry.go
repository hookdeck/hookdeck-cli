package hookdeck

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

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
