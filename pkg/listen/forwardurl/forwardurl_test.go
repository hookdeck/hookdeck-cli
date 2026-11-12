package forwardurl

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		eventPath string
		want      string
	}{
		{"forwarding URL with a trailing slash", "http://localhost:3000/base/", "/webhook", "http://localhost:3000/base/webhook"},
		{"forwarding URL without a trailing slash", "http://localhost:3000/base", "/webhook", "http://localhost:3000/base/webhook"},
		{"forwarding URL is the root", "http://localhost:3000/", "/webhook", "http://localhost:3000/webhook"},
		{"forwarding URL has no path", "http://localhost:3000", "/webhook", "http://localhost:3000/webhook"},
		{"event path keeps its own trailing slash", "http://localhost:3000/base/", "/webhook/", "http://localhost:3000/base/webhook/"},
		{"event path is empty", "http://localhost:3000/base/", "", "http://localhost:3000/base/"},
		{"no forwarding path and an event path without a leading slash", "http://localhost:3000", "webhook", "http://localhost:3000/webhook"},
		{"no forwarding path and an event path that is only a query", "http://localhost:3000", "?a=b", "http://localhost:3000?a=b"},
		{"no forwarding path and an event path that is only a fragment", "http://localhost:3000", "#frag", "http://localhost:3000#frag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := url.Parse(tt.target)
			require.NoError(t, err)
			assert.Equal(t, tt.want, Build(target, tt.eventPath))
		})
	}
}

// With no path on the forwarding URL the event path follows the host directly,
// so an event path that does not begin with "/" must not be able to turn into
// part of the host or into user information. The URL has to parse back to the
// host the user asked to forward to.
func TestBuildKeepsTheForwardingHost(t *testing.T) {
	for _, eventPath := range []string{
		"@other.example",
		"@other.example/x",
		":8080/x",
		".other.example",
		"other.example",
		"/@other.example",
		"//other.example/x",
		"/../x",
		"%2f%2fother.example",
	} {
		for _, forwardTo := range []string{"http://localhost:3000", "http://localhost:3000/", "http://localhost:3000/base"} {
			t.Run(forwardTo+" "+eventPath, func(t *testing.T) {
				target, err := url.Parse(forwardTo)
				require.NoError(t, err)

				built, err := url.Parse(Build(target, eventPath))
				require.NoError(t, err)

				assert.Equal(t, "localhost:3000", built.Host)
				assert.Nil(t, built.User)
			})
		}
	}
}
