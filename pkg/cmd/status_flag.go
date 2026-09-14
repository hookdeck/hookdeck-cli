package cmd

import (
	"errors"
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// statusFlagVocabulary is the status enum one command's --status filters by,
// paired with the sibling command that takes the other one.
//
// The two log collections disagree on both vocabulary and case: GET /requests
// describes what happened to a request at the edge and spells its enum lower
// case, while GET /events and GET /requests/{id}/events describe where a
// delivery is in its lifecycle and spell theirs upper case. The API rejects
// either mistake with a 422 that names only the enum of the route it was sent
// to.
//
// MCP canonicalises through hookdeck.CanonicalStatusValue and the CLI did not,
// so in one release `hookdeck_requests {action:"list", status:"ACCEPTED"}`
// succeeded and `hookdeck gateway request list --status ACCEPTED` was a 422:
// same contract, two surfaces, two answers.
type statusFlagVocabulary struct {
	values []string
	// other is the sibling vocabulary, named in the error so a caller who
	// reached for the wrong command is told which one takes the value.
	other        []string
	otherCommand string
}

var (
	// eventStatusFlag is the vocabulary of the commands that read the event
	// collection: `gateway event list` and `gateway request events`.
	eventStatusFlag = statusFlagVocabulary{
		values:       hookdeck.EventStatusValueList,
		other:        hookdeck.RequestLogStatusValueList,
		otherCommand: "gateway request list",
	}

	// requestStatusFlag is the vocabulary of `gateway request list`.
	requestStatusFlag = statusFlagVocabulary{
		values:       hookdeck.RequestLogStatusValueList,
		other:        hookdeck.EventStatusValueList,
		otherCommand: "gateway event list",
	}
)

// usage renders the --status help for this command, so what is advertised and
// what is accepted are the same list.
func (v statusFlagVocabulary) usage() string {
	return fmt.Sprintf("Filter by status (%s)", hookdeck.ValueList(v.values))
}

// canonical returns the value to send, in the API's own spelling, or an error
// naming the vocabulary this command does filter by. An empty value means the
// flag was not given.
func (v statusFlagVocabulary) canonical(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if canonical, ok := hookdeck.CanonicalStatusValue(v.values, value); ok {
		return canonical, nil
	}
	msg := fmt.Sprintf("--status %q is not supported by this command; it filters by %s",
		value, hookdeck.ValueList(v.values))
	if _, ok := hookdeck.CanonicalStatusValue(v.other, value); ok {
		msg += fmt.Sprintf(". It belongs to `hookdeck %s`, which filters by %s",
			v.otherCommand, hookdeck.ValueList(v.other))
	}
	return "", errors.New(msg)
}
