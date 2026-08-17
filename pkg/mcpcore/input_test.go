package mcpcore

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInput_JSONFilterParam_Missing(t *testing.T) {
	in := Input{}
	value, err := in.JSONFilterParam("body")
	require.NoError(t, err)
	assert.Empty(t, value)
}

func TestInput_JSONFilterParam_String(t *testing.T) {
	in := Input{"body": `{"type":"payment"}`}
	value, err := in.JSONFilterParam("body")
	require.NoError(t, err)
	assert.Equal(t, `{"type":"payment"}`, value)
}

func TestInput_JSONFilterParam_Object(t *testing.T) {
	in := Input{"body": map[string]interface{}{"type": "payment", "amount": float64(100)}}
	value, err := in.JSONFilterParam("body")
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"payment","amount":100}`, value)
}

func TestInput_JSONFilterParam_InvalidType(t *testing.T) {
	in := Input{"body": 42}
	_, err := in.JSONFilterParam("body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "body must be a JSON string or object")
}

func TestSetPayloadSearchFilters(t *testing.T) {
	params := make(map[string]string)
	in := Input{
		"body":         map[string]interface{}{"a": "b"},
		"headers":      `{"x-test":"1"}`,
		"parsed_query": map[string]interface{}{"q": "x"},
		"path":         "/webhooks",
	}
	require.NoError(t, SetPayloadSearchFilters(params, in))
	assert.JSONEq(t, `{"a":"b"}`, params["body"])
	assert.Equal(t, `{"x-test":"1"}`, params["headers"])
	assert.JSONEq(t, `{"q":"x"}`, params["parsed_query"])
	assert.Equal(t, "/webhooks", params["path"])
}

func TestInput_Accessors(t *testing.T) {
	raw := json.RawMessage(`{
		"name": "test",
		"count": 42,
		"active": true,
		"tags": ["a", "b"],
		"missing_bool": null
	}`)

	in, err := ParseInput(raw)
	require.NoError(t, err)

	assert.Equal(t, "test", in.String("name"))
	assert.Equal(t, "", in.String("nonexistent"))
	assert.Equal(t, 42, in.Int("count", 0))
	assert.Equal(t, 99, in.Int("nonexistent", 99))
	assert.Equal(t, true, in.Bool("active"))
	assert.Equal(t, false, in.Bool("nonexistent"))
	assert.Equal(t, []string{"a", "b"}, in.StringSlice("tags"))
	assert.Nil(t, in.StringSlice("nonexistent"))

	bp := in.BoolPtr("active")
	require.NotNil(t, bp)
	assert.True(t, *bp)
	assert.Nil(t, in.BoolPtr("nonexistent"))
}

func TestInput_EmptyArgs(t *testing.T) {
	in, err := ParseInput(nil)
	require.NoError(t, err)
	assert.Equal(t, "", in.String("anything"))
}

func TestInput_InvalidJSON(t *testing.T) {
	_, err := ParseInput(json.RawMessage(`{invalid`))
	assert.Error(t, err)
}
