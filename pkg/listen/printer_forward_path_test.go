package listen

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// The startup banner lists "Forwards to → <URL>" for each connection. It has to
// show the URL the events will be forwarded to, including the forwarding URL's
// path, joined to the connection's path with a single "/".
func TestCompactBannerForwardsToJoinsForwardingURLAndConnectionPath(t *testing.T) {
	for _, tc := range []struct {
		name      string
		forwardTo string
		cliPath   string
		want      string
	}{
		{"trailing slash on the forwarding URL", "http://localhost:3030/base/", "/webhook", "Forwards to → http://localhost:3030/base/webhook"},
		{"no trailing slash on the forwarding URL", "http://localhost:3030/base", "/webhook", "Forwards to → http://localhost:3030/base/webhook"},
		{"forwarding URL has no path", "http://localhost:3030", "/webhook", "Forwards to → http://localhost:3030/webhook"},
		{"connection path is the default root", "http://localhost:3030/base/", "/", "Forwards to → http://localhost:3030/base/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetColorState(t)
			cfg, sources, connections, _ := printerFixture(t, 1)
			targetURL, err := url.Parse(tc.forwardTo)
			require.NoError(t, err)
			connections[0].Destination = &hookdeck.Destination{
				ID: "des_1", Type: "CLI", Config: map[string]interface{}{"path": tc.cliPath},
			}

			out := capturePrinterOutput(t, func() {
				printSourcesWithConnections(cfg, "tm_1", sources, connections, targetURL, "")
			})

			assert.Contains(t, out, tc.want)
		})
	}
}
