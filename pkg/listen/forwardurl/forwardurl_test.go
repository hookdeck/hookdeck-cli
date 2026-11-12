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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := url.Parse(tt.target)
			require.NoError(t, err)
			assert.Equal(t, tt.want, Build(target, tt.eventPath))
		})
	}
}
