package hookdeck

import (
	"encoding/json"
	"net/http"
	"strings"
)

// redactedPlaceholder is what a credential looks like in a debug log.
const redactedPlaceholder = "[redacted]"

// secretHeaders are request and response headers whose value is a credential.
var secretHeaders = []string{
	"Authorization",
	"X-Api-Key",
	"Set-Cookie",
	"Cookie",
	"Proxy-Authorization",
}

// secretFieldNames are JSON keys whose value is a credential, wherever they
// appear in a body.
//
// Matching is by name at any depth, not by a path, because the same secret
// arrives at different depths on different routes: an Outpost destination
// carries "credentials" nested under the destination, a tenant token comes back
// at the top level, and an API key create returns the secret as "key".
//
// Over-redacting a debug log costs legibility. Under-redacting writes a
// reusable credential to disk for as long as the log survives, so where the
// two conflict this errs towards redaction.
var secretFieldNames = map[string]bool{
	"access_token":   true,
	"api_key":        true,
	"authorization":  true,
	"client_secret":  true,
	"credentials":    true,
	"guest_api_key":  true,
	"key":            true,
	"password":       true,
	"refresh_token":  true,
	"secret":         true,
	"signing_secret": true,
	"token":          true,
	"webhook_secret": true,
}

func redactHeadersForLog(headers http.Header) http.Header {
	if headers == nil {
		return nil
	}

	redacted := headers.Clone()
	for _, name := range secretHeaders {
		if redacted.Get(name) != "" {
			redacted.Set(name, redactedPlaceholder)
		}
	}
	return redacted
}

// redactBodyForLog redacts every secret-bearing field in a JSON body, at any
// depth, and is used for requests and responses alike.
//
// A body that is not JSON is returned unchanged: it may be a raw webhook
// payload, and there is no structure to search. Callers logging one should
// consider whether it belongs in a log at all.
func redactBodyForLog(body string) string {
	if strings.TrimSpace(body) == "" {
		return body
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return body
	}

	redacted, changed := redactValue(parsed)
	if !changed {
		return body
	}

	out, err := json.Marshal(redacted)
	if err != nil {
		// Re-marshalling failed, so the redacted form cannot be produced.
		// Returning the original would log the secret this function exists to
		// remove, so return nothing useful instead.
		return redactedPlaceholder
	}
	return string(out)
}

// redactValue walks a decoded JSON value, replacing the value of any key named
// in secretFieldNames. It reports whether anything was replaced, so a body with
// no secrets is logged exactly as it was sent rather than re-serialised.
func redactValue(value interface{}) (interface{}, bool) {
	switch typed := value.(type) {
	case map[string]interface{}:
		changed := false
		out := make(map[string]interface{}, len(typed))
		for key, inner := range typed {
			if secretFieldNames[strings.ToLower(key)] {
				out[key] = redactedPlaceholder
				changed = true
				continue
			}
			redactedInner, innerChanged := redactValue(inner)
			out[key] = redactedInner
			changed = changed || innerChanged
		}
		return out, changed
	case []interface{}:
		changed := false
		out := make([]interface{}, len(typed))
		for i, inner := range typed {
			redactedInner, innerChanged := redactValue(inner)
			out[i] = redactedInner
			changed = changed || innerChanged
		}
		return out, changed
	default:
		return value, false
	}
}
