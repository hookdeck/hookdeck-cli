package hookdeck

import (
	"net/url"
	"strings"
)

// listValuedParams are list-endpoint filters the CLI documents as accepting a
// comma-separated list ("Filter by event ID(s) (comma-separated)").
//
// The comma-joined string used to be sent as one scalar value, which the API
// matched against nothing: `--id evt_A,evt_B` returned zero rows with exit 0,
// while each id on its own returned its row. See #411.
var listValuedParams = map[string]bool{
	"id": true,
}

// listParamKey is how a repeated value is spelled on the wire.
var listParamKey = func(key string) string { return key + "[]" }

// listQuery builds the query for a list endpoint, expanding list-valued
// filters into one query value per item. Everything else passes through
// unchanged, so a single id is sent exactly as it always was.
func listQuery(params map[string]string) url.Values {
	q := url.Values{}
	for k, v := range params {
		if listValuedParams[k] && strings.Contains(v, ",") {
			for _, part := range strings.Split(v, ",") {
				if part = strings.TrimSpace(part); part != "" {
					q.Add(listParamKey(k), part)
				}
			}
			continue
		}
		q.Add(k, v)
	}
	return q
}
