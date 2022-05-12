package hookdeck

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPerformRequest_ParamsEncoding_Delete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/delete", r.URL.Path)
		require.Equal(t, "key_a=value_a&key_b=value_b", r.URL.RawQuery)

		body, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "", string(body))
	}))
	defer ts.Close()

	baseURL, _ := url.Parse(ts.URL)
	client := Client{
		BaseURL: baseURL,
	}

	params := url.Values{}
	params.Add("key_a", "value_a")
	params.Add("key_b", "value_b")

	req := &http.Request{
		Method: http.MethodDelete,
		URL: &url.URL{
			Scheme:   baseURL.Scheme,
			Host:     baseURL.Host,
			Path:     "/delete",
			RawQuery: params.Encode(),
		},
	}
	resp, err := client.PerformRequest(context.TODO(), req)
	require.NoError(t, err)

	defer resp.Body.Close()
}

func TestPerformRequest_ParamsEncoding_Get(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/get", r.URL.Path)
		require.Equal(t, "key_a=value_a&key_b=value_b", r.URL.RawQuery)

		body, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "", string(body))
	}))
	defer ts.Close()

	baseURL, _ := url.Parse(ts.URL)
	client := Client{
		BaseURL: baseURL,
	}

	params := url.Values{}
	params.Add("key_a", "value_a")
	params.Add("key_b", "value_b")

	req := &http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Scheme:   baseURL.Scheme,
			Host:     baseURL.Host,
			Path:     "/get",
			RawQuery: params.Encode(),
		},
	}

	resp, err := client.PerformRequest(context.TODO(), req)
	require.NoError(t, err)

	defer resp.Body.Close()
}

func TestPerformRequest_ParamsEncoding_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/post", r.URL.Path)
		require.Equal(t, "", r.URL.RawQuery)

		body, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "key_a=value_a&key_b=value_b", string(body))
	}))
	defer ts.Close()

	baseURL, _ := url.Parse(ts.URL)
	client := Client{
		BaseURL: baseURL,
	}

	params := url.Values{}
	params.Add("key_a", "value_a")
	params.Add("key_b", "value_b")

	req := &http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Scheme: baseURL.Scheme,
			Host:   baseURL.Host,
			Path:   "/post",
		},
		Body: io.NopCloser(strings.NewReader(params.Encode())),
	}

	resp, err := client.PerformRequest(context.TODO(), req)
	require.NoError(t, err)

	defer resp.Body.Close()
}

func TestPerformRequest_ApiKey_Provided(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Basic c2tfdGVzdF8xMjM0Og==", r.Header.Get("Authorization"))
	}))
	defer ts.Close()

	baseURL, _ := url.Parse(ts.URL)
	client := Client{
		BaseURL: baseURL,
		APIKey:  "sk_test_1234",
	}

	req := &http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Scheme: baseURL.Scheme,
			Host:   baseURL.Host,
			Path:   "/get",
		},
	}

	resp, err := client.PerformRequest(context.TODO(), req)
	require.NoError(t, err)

	defer resp.Body.Close()
}

func TestPerformRequest_ApiKey_Omitted(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "", r.Header.Get("Authorization"))
	}))
	defer ts.Close()

	baseURL, _ := url.Parse(ts.URL)
	client := Client{
		BaseURL: baseURL,
	}

	req := &http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Scheme: baseURL.Scheme,
			Host:   baseURL.Host,
			Path:   "/get",
		},
	}

	resp, err := client.PerformRequest(context.TODO(), req)
	require.NoError(t, err)

	defer resp.Body.Close()
}

func TestPerformRequest_ConfigureFunc(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "2019-07-10", r.Header.Get("Hookdeck-Version"))
	}))
	defer ts.Close()

	baseURL, _ := url.Parse(ts.URL)
	client := Client{
		BaseURL: baseURL,
	}

	req := &http.Request{
		Method: http.MethodGet,
		Header: http.Header{
			"Hookdeck-Version": []string{"2019-07-10"},
		},
		URL: &url.URL{
			Scheme: baseURL.Scheme,
			Host:   baseURL.Host,
			Path:   "/get",
		},
	}

	resp, err := client.PerformRequest(context.TODO(), req)
	require.NoError(t, err)

	defer resp.Body.Close()
}
