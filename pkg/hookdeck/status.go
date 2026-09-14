package hookdeck

import "strings"

// Status vocabularies of the log collections, as value lists.
//
// These are the `status` enums the OpenAPI document declares for the routes the
// CLI and MCP query, and they are not interchangeable: GET /requests describes
// what happened to a request at the edge, while GET /events and
// GET /requests/{id}/events describe where a delivery is in its lifecycle. The
// two sit one argument apart in the MCP schema - hookdeck_requests action
// "list" takes the first and action "events" the second - so both layers need
// to be able to name which vocabulary they mean.
//
// Spelled as the API spells them: the request log enum is lower case and the
// event enum upper case. Callers match case-insensitively and send the
// canonical spelling, so nobody has to know that.
var (
	// EventStatusValueList is the status enum of GET /events and of
	// GET /requests/{id}/events, which shares the /events filter set.
	EventStatusValueList = []string{"SCHEDULED", "QUEUED", "HOLD", "SUCCESSFUL", "FAILED", "CANCELLED"}

	// RequestLogStatusValueList is the status enum of GET /requests. The
	// metrics route spells the same two values upper case; see
	// RequestStatusValues.
	RequestLogStatusValueList = []string{"accepted", "rejected"}

	// RequestLogStatusValues renders the request log vocabulary for --help and
	// for a tool schema.
	RequestLogStatusValues = ValueList(RequestLogStatusValueList)
)

// CanonicalStatusValue returns the vocabulary's own spelling of value, matched
// without regard to case, and false when the vocabulary does not carry it.
//
// The API is strict about both the vocabulary and the case. Checked live on
// 2026-09-14: GET /requests answers 422 "status must be one of [accepted,
// rejected]" for ACCEPTED, and GET /requests/{id}/events answers 422 "status
// must be one of [SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED]" for
// successful. Canonicalising makes either spelling work; the false return lets
// a caller turn a 422 that only ever names one route's enum into a message that
// names the route which does take the value.
func CanonicalStatusValue(vocabulary []string, value string) (string, bool) {
	for _, v := range vocabulary {
		if strings.EqualFold(v, value) {
			return v, true
		}
	}
	return "", false
}
