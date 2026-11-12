// Package forwardurl builds the URL that `hookdeck listen` forwards an event
// to. The request and every place that prints the URL share it, so what the
// CLI shows is what the CLI sent.
package forwardurl

import (
	"net/url"
	"strings"
)

// JoinPath appends the event's path to the forwarding URL's path with a single
// "/" between them. A trailing "/" on the forwarding URL is dropped only when
// the event path starts with one, and the event path itself is left as is: a
// receiver may route on its own trailing slash, which is why path.Join is not
// used.
func JoinPath(base, eventPath string) string {
	if strings.HasPrefix(eventPath, "/") {
		base = strings.TrimSuffix(base, "/")
	}
	return base + eventPath
}

// Build returns the full URL for an event: the forwarding URL's scheme, host
// and path, then the event's path. The forwarding URL's query string is not
// carried over.
func Build(target *url.URL, eventPath string) string {
	return target.Scheme + "://" + target.Host + JoinPath(target.Path, eventPath)
}
