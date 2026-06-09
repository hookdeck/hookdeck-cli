package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInput_JSONFilterParam_Missing(t *testing.T) {
	in := input{}
	value, err := in.JSONFilterParam("body")
	require.NoError(t, err)
	assert.Empty(t, value)
}

func TestInput_JSONFilterParam_String(t *testing.T) {
	in := input{"body": `{"type":"payment"}`}
	value, err := in.JSONFilterParam("body")
	require.NoError(t, err)
	assert.Equal(t, `{"type":"payment"}`, value)
}

func TestInput_JSONFilterParam_Object(t *testing.T) {
	in := input{"body": map[string]interface{}{"type": "payment", "amount": float64(100)}}
	value, err := in.JSONFilterParam("body")
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"payment","amount":100}`, value)
}

func TestInput_JSONFilterParam_InvalidType(t *testing.T) {
	in := input{"body": 42}
	_, err := in.JSONFilterParam("body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "body must be a JSON string or object")
}

func TestSetPayloadSearchFilters(t *testing.T) {
	params := make(map[string]string)
	in := input{
		"body":          map[string]interface{}{"a": "b"},
		"headers":       `{"x-test":"1"}`,
		"parsed_query":  map[string]interface{}{"q": "x"},
		"path":          "/webhooks",
	}
	require.NoError(t, setPayloadSearchFilters(params, in))
	assert.JSONEq(t, `{"a":"b"}`, params["body"])
	assert.Equal(t, `{"x-test":"1"}`, params["headers"])
	assert.JSONEq(t, `{"q":"x"}`, params["parsed_query"])
	assert.Equal(t, "/webhooks", params["path"])
}
