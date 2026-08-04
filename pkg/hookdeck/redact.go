package hookdeck

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// redactedPlaceholder replaces any credential value before it reaches a log.
const redactedPlaceholder = "[REDACTED]"

// sensitiveHeaders carry credentials and must never be logged verbatim. The
// CLI asks users to re-run with `--log-level debug` and share the output, so
// anything logged here should be assumed to end up in a public issue.
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
}

// sensitiveQueryParams are query-string names that carry credentials. The
// login poll endpoint (/cli-auth/poll?key=...) puts the API key in the URL,
// and URLs are logged at error level — visible without --log-level debug.
var sensitiveQueryParams = map[string]bool{
	"key":           true,
	"api_key":       true,
	"apikey":        true,
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"secret":        true,
}

// sensitiveJSONKeys are request/response body fields that carry credentials —
// the API key returned by the login poll, and the source/destination auth
// secrets the user supplies on create/update.
var sensitiveJSONKeys = map[string]bool{
	"api_key":            true,
	"key":                true,
	"secret":             true,
	"webhook_secret":     true,
	"hmac_secret":        true,
	"signing_secret":     true,
	"client_secret":      true,
	"password":           true,
	"token":              true,
	"bearer_token":       true,
	"access_token":       true,
	"refresh_token":      true,
	"api_key_value":      true,
	"webhook_secret_key": true,
}

// redactCredential hides a credential while keeping its auth scheme visible,
// so logs still show whether the CLI sent Basic or Bearer.
func redactCredential(value string) string {
	if value == "" {
		return value
	}

	if scheme, _, found := strings.Cut(value, " "); found {
		switch strings.ToLower(scheme) {
		case "basic", "bearer", "digest":
			return scheme + " " + redactedPlaceholder
		}
	}

	return redactedPlaceholder
}

// redactHeaders returns a copy of header with credential values replaced. The
// input is never modified — it is still used to make the request.
func redactHeaders(header http.Header) http.Header {
	if header == nil {
		return nil
	}

	redacted := make(http.Header, len(header))
	for name, values := range header {
		if !sensitiveHeaders[strings.ToLower(name)] {
			redacted[name] = values
			continue
		}

		safe := make([]string, len(values))
		for i, value := range values {
			safe[i] = redactCredential(value)
		}
		redacted[name] = safe
	}

	return redacted
}

// redactURL renders a URL with credential-bearing query parameters replaced.
// The path and every other parameter are preserved so the log still identifies
// which endpoint was called.
func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}

	query := u.Query()
	redactedAny := false
	for name := range query {
		if sensitiveQueryParams[strings.ToLower(name)] {
			query.Set(name, redactedPlaceholder)
			redactedAny = true
		}
	}

	if !redactedAny {
		return u.String()
	}

	safe := *u
	safe.RawQuery = query.Encode()
	return safe.String()
}

// redactBody renders a JSON request/response body with credential fields
// replaced, preserving the rest of the payload for diagnosis. Non-JSON bodies
// are returned unchanged — this client only sends and receives JSON, so there
// is no known credential-bearing non-JSON payload to guard against.
func redactBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var parsed interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return string(body)
	}

	safe, err := json.Marshal(redactJSONValue(parsed))
	if err != nil {
		// Should not happen: the value came from json.Unmarshal. Fail closed
		// rather than logging the unredacted body.
		return redactedPlaceholder
	}

	return string(safe)
}

// redactJSONValue walks a decoded JSON value and replaces the values of any
// key named like a credential, at any depth.
func redactJSONValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		safe := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			if sensitiveJSONKeys[strings.ToLower(key)] {
				safe[key] = redactedPlaceholder
				continue
			}
			safe[key] = redactJSONValue(nested)
		}
		return safe
	case []interface{}:
		safe := make([]interface{}, len(typed))
		for i, nested := range typed {
			safe[i] = redactJSONValue(nested)
		}
		return safe
	default:
		return value
	}
}
