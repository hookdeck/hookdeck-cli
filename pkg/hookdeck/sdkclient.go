package hookdeck

import (
	"encoding/base64"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/hookdeck/hookdeck-cli/pkg/useragent"
	hookdeckclient "github.com/hookdeck/hookdeck-go-sdk/client"
	hookdeckoption "github.com/hookdeck/hookdeck-go-sdk/option"
)

const apiVersion = "/2024-03-01"

type SDKClientInit struct {
	APIBaseURL string
	APIKey     string
	TeamID     string
}

func CreateSDKClient(init SDKClientInit) *hookdeckclient.Client {
	parsedBaseURL, err := url.Parse(init.APIBaseURL + apiVersion)
	if err != nil {
		log.Fatal("Invalid API base URL")
	}

	header := http.Header{}
	header.Set("User-Agent", useragent.GetEncodedUserAgent())
	header.Set("X-Hookdeck-Client-User-Agent", useragent.GetEncodedHookdeckUserAgent())
	if init.TeamID != "" {
		header.Set("X-Team-ID", init.TeamID)
	}
	if init.APIKey != "" {
		header.Set("Authorization", "Basic "+basicAuth(init.APIKey, ""))
	}

	if !telemetryOptedOut(os.Getenv("HOOKDECK_CLI_TELEMETRY_OPTOUT")) {
		telemetryHeader, err := getTelemetryHeader()
		if err == nil {
			header.Set("Hookdeck-CLI-Telemetry", telemetryHeader)
		}
	}

	return hookdeckclient.NewClient(
		hookdeckoption.WithBaseURL(parsedBaseURL.String()),
		hookdeckoption.WithHTTPHeader(header),
	)
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
