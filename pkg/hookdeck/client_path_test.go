package hookdeck

import (
	"errors"
	"net/url"
	"testing"
)

// TestResolveRequestURL_Backstop covers the request layer's last check, which
// exists for call sites that build a path without apiPath. apiPath rejects all
// of these at the source; this proves they cannot be sent even if it is bypassed.
func TestResolveRequestURL_Backstop(t *testing.T) {
	base, err := url.Parse("https://api.hookdeck.com")
	if err != nil {
		t.Fatalf("parse base: %v", err)
	}
	client := &Client{BaseURL: base}

	rejected := []struct {
		name string
		path string
	}{
		// The authority cases matter most: the path that follows is correct and
		// passes every path-shaped check, so without a host comparison the
		// request is sent to an attacker's server with the API key attached.
		{"protocol-relative authority", "//evil.example.com" + APIPathPrefix + "/sources"},
		{"absolute url to another host", "https://evil.example.com" + APIPathPrefix + "/sources"},
		{"empty path segment", APIPathPrefix + "//sources"},
		{"traversal out of the api prefix", APIPathPrefix + "/sources/../../../etc"},
		{"traversal to another resource", APIPathPrefix + "/sources/src_1/../../destinations/des_2"},
		{"relative traversal", "sources/../destinations/des_2"},
		{"outside the api prefix", "/some-other-api/sources"},
	}

	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.resolveRequestURL(tt.path)
			if err == nil {
				t.Fatalf("expected rejection, got %s", got)
			}
			if !errors.Is(err, ErrRequestPathRejected) {
				t.Errorf("expected ErrRequestPathRejected, got %v", err)
			}
		})
	}

	accepted := []struct {
		name string
		path string
		want string
	}{
		{"the prefix itself", APIPathPrefix, "https://api.hookdeck.com" + APIPathPrefix},
		{"a collection", APIPathPrefix + "/sources", "https://api.hookdeck.com" + APIPathPrefix + "/sources"},
		{"an escaped id", APIPathPrefix + "/tenants/a%2Fb", "https://api.hookdeck.com" + APIPathPrefix + "/tenants/a%2Fb"},
		{"a query string", APIPathPrefix + "/sources?limit=10", "https://api.hookdeck.com" + APIPathPrefix + "/sources?limit=10"},
	}

	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.resolveRequestURL(tt.path)
			if err != nil {
				t.Fatalf("expected acceptance, got %v", err)
			}
			if got.String() != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}
