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
//
// When the forwarding URL has no path, the event path is appended straight
// after the host, so an event path that does not start with "/", "?" or "#"
// would be read as part of the host. It gets a "/" in front instead.
func JoinPath(base, eventPath string) string {
	if base == "" && eventPath != "" && !strings.ContainsRune("/?#", rune(eventPath[0])) {
		eventPath = "/" + eventPath
	}
	if strings.HasPrefix(eventPath, "/") {
		base = strings.TrimSuffix(base, "/")
	}
	return base + eventPath
}

// Build returns the full URL for an event: the forwarding URL's scheme, host
// and path, then the event's path.
func Build(target *url.URL, eventPath string) string {
	return target.Scheme + "://" + target.Host + JoinPath(target.Path, eventPath)
}
