package hookdeck

import (
	"encoding/json"
	"net/http"
)

func redactHeadersForLog(headers http.Header) http.Header {
	if headers == nil {
		return nil
	}

	redacted := headers.Clone()
	if redacted.Get("Authorization") != "" {
		redacted.Set("Authorization", "[redacted]")
	}
	return redacted
}

func redactRequestBodyForLog(body string) string {
	if body == "" {
		return body
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return body
	}

	if _, ok := parsed["guest_api_key"]; !ok {
		return body
	}

	parsed["guest_api_key"] = json.RawMessage(`"[redacted]"`)
	redacted, err := json.Marshal(parsed)
	if err != nil {
		return body
	}
	return string(redacted)
}
