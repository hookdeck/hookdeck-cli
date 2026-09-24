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
//
// delivery_group is declared the same way as id in the API -- a single value or
// an array, with no comma splitting -- and was sent as one value in the same
// way, so --delivery-group a,b returned nothing too.
var listValuedParams = map[string]bool{
	"id":             true,
	"delivery_group": true,
}

// IsListValuedParam reports whether a list-endpoint filter is sent as a list
// when given a comma-separated value. The CLI's guard over flags documented as
// comma-separated uses it, so a flag cannot promise a list the query does not
// send.
func IsListValuedParam(name string) bool { return listValuedParams[name] }

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
