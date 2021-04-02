package useragent

import (
	"encoding/json"
	"runtime"

	"github.com/hookdeck/hookdeck-cli/pkg/version"
)

//
// Public functions
//

// GetEncodedHookdeckUserAgent returns the string to be used as the value for
// the `X-Hookdeck-Client-User-Agent` HTTP header.
func GetEncodedHookdeckUserAgent() string {
	return encodedHookdeckUserAgent
}

// GetEncodedUserAgent returns the string to be used as the value for
// the `User-Agent` HTTP header.
func GetEncodedUserAgent() string {
	return encodedUserAgent
}

//
// Private types
//

// hookdeckClientUserAgent contains information about the current runtime which
// is serialized and sent in the `X-Hookdeck-Client-User-Agent` as additional
// debugging information.
type hookdeckClientUserAgent struct {
	Name      string `json:"name"`
	OS        string `json:"os"`
	Publisher string `json:"publisher"`
	Uname     string `json:"uname"`
	Version   string `json:"version"`
}

//
// Private variables
//

var encodedHookdeckUserAgent string
var encodedUserAgent string

//
// Private functions
//

func init() {
	initUserAgent()
}

func initUserAgent() {
	encodedUserAgent = "Hookdeck/v1 hookdeck-cli/" + version.Version

	hookdeckUserAgent := &hookdeckClientUserAgent{
		Name:      "hookdeck-cli",
		Version:   version.Version,
		Publisher: "hookdeck",
		OS:        runtime.GOOS,
		Uname:     getUname(),
	}
	marshaled, err := json.Marshal(hookdeckUserAgent)
	// Encoding this struct should never be a problem, so we're okay to panic
	// in case it is for some reason.
	if err != nil {
		panic(err)
	}

	encodedHookdeckUserAgent = string(marshaled)
}
