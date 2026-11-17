package cmd

import (
	"io"
	"os"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintOutpostAttemptResponse(t *testing.T) {
	for _, tc := range []struct {
		name         string
		response     map[string]interface{}
		wantResponse bool
	}{
		{name: "included", response: map[string]interface{}{"body": "boom", "status": 500}, wantResponse: true},
		{name: "not included"},
		{name: "empty", response: map[string]interface{}{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)
			defer r.Close()
			defer func() { os.Stdout = original }()
			os.Stdout = w
			printOutpostAttempt(&hookdeck.OutpostAttempt{ID: "att_test", ResponseData: tc.response})
			require.NoError(t, w.Close())
			os.Stdout = original
			output, err := io.ReadAll(r)
			require.NoError(t, err)
			if tc.wantResponse {
				assert.Contains(t, string(output), "  Response: {\n")
				assert.Contains(t, string(output), `"body": "boom"`)
				assert.Contains(t, string(output), `"status": 500`)
			} else {
				assert.NotContains(t, string(output), "Response:")
			}
		})
	}
}

func TestOutpostAttemptGetHelpResponse(t *testing.T) {
	assert.Contains(t, newOutpostAttemptGetCmd().cmd.Long, "--include response_data")
}
