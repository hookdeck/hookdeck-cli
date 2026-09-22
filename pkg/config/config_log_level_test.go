package config

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// An unset log level must not be treated as an invalid one.
//
// InitConfig runs its level switch before constructConfig coalesces LogLevel to
// "info", and the --log-level flag default only covers a Config built through
// the root command. Everything else — a programmatically constructed Config, a
// test helper resetting the global — reached the switch with "" and hit
// log.Fatalf, which calls os.Exit and takes the whole process down. In a test
// binary that kills every remaining test; in an MCP stdio session it would kill
// the server mid-request.
func TestUnsetLogLevelDefaultsRatherThanExiting(t *testing.T) {
	original := log.GetLevel()
	t.Cleanup(func() { log.SetLevel(original) })

	c := &Config{}
	c.InitConfig()

	assert.Equal(t, "info", c.LogLevel, "an unset level should resolve to the documented default")
	assert.Equal(t, log.InfoLevel, log.GetLevel())
}
