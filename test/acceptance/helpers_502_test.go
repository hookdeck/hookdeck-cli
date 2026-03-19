//go:build basic

package acceptance

import "testing"

func TestCombinedOutputLooksLikeHTTP502(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		stdout  string
		stderr  string
		want502 bool
	}{
		{
			name:    "cli client error message",
			stderr:  "Error: unexpected http status code: 502 bad gateway",
			want502: true,
		},
		{
			name:    "logrus status field",
			stderr:  `level=error msg="request failed" status=502`,
			want502: true,
		},
		{
			name:    "error code 502",
			stderr:  `error code: 502`,
			want502: true,
		},
		{
			name:    "503 not retried",
			stderr:  "unexpected http status code: 503",
			want502: false,
		},
		{
			name:    "unrelated 502 substring",
			stdout:  `{"retry":{"response_status_codes":["500","502"]}}`,
			want502: false,
		},
		{
			name:    "empty",
			want502: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := combinedOutputLooksLikeHTTP502(tt.stdout, tt.stderr)
			if got != tt.want502 {
				t.Fatalf("combinedOutputLooksLikeHTTP502(%q, %q) = %v, want %v", tt.stdout, tt.stderr, got, tt.want502)
			}
		})
	}
}
