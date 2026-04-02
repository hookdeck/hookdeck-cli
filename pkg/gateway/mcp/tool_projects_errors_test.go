package mcp

import (
	"fmt"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
)

func TestShouldSuggestReauthAfterListProjectsFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "APIError 403",
			err:  &hookdeck.APIError{StatusCode: 403, Message: "not allowed"},
			want: true,
		},
		{
			name: "APIError 401",
			err:  &hookdeck.APIError{StatusCode: 401, Message: "unauthorized"},
			want: true,
		},
		{
			name: "APIError 500 no match",
			err:  &hookdeck.APIError{StatusCode: 500, Message: "internal error"},
			want: false,
		},
		{
			name: "APIError with fatal message",
			err:  &hookdeck.APIError{StatusCode: 500, Message: "fatal: cannot proceed"},
			want: true,
		},
		{
			name: "plain error with status code 403",
			err:  fmt.Errorf("unexpected http status code: 403 <nil>"),
			want: true,
		},
		{
			name: "plain error with status code 401",
			err:  fmt.Errorf("unexpected http status code: 401 <nil>"),
			want: true,
		},
		{
			name: "plain error without status code",
			err:  fmt.Errorf("network timeout"),
			want: false,
		},
		{
			name: "plain error with 403 in ID should not match",
			err:  fmt.Errorf("project proj_403_abc not found"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldSuggestReauthAfterListProjectsFailure(tt.err))
		})
	}
}
