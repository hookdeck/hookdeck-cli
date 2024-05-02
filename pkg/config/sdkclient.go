package config

import (
	"sync"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	hookdeckclient "github.com/hookdeck/hookdeck-go-sdk/client"
)

var client *hookdeckclient.Client
var once sync.Once

func (c *Config) GetClient() *hookdeckclient.Client {
	once.Do(func() {
		client = hookdeck.CreateSDKClient(hookdeck.SDKClientInit{
			APIBaseURL: c.APIBaseURL,
			APIKey:     c.Profile.APIKey,
			TeamID:     c.Profile.TeamID,
		})
	})

	return client
}
