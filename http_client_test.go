package main

import (
	"testing"
	"time"
)

func TestConfigureHTTPClientsUsesSeparateArtworkTimeout(t *testing.T) {
	oldConfig := Config
	oldHTTPClient := httpClient
	oldArtworkHTTPClient := artworkHTTPClient
	t.Cleanup(func() {
		Config = oldConfig
		httpClient = oldHTTPClient
		artworkHTTPClient = oldArtworkHTTPClient
	})

	Config = defaultConfig()
	Config.RequestTimeoutSeconds = 7
	Config.ArtworkTimeoutSeconds = 180
	configureHTTPClients()

	if httpClient.Timeout != 7*time.Second {
		t.Fatalf("general timeout = %s, want 7s", httpClient.Timeout)
	}
	if artworkHTTPClient.Timeout != 180*time.Second {
		t.Fatalf("artwork timeout = %s, want 180s", artworkHTTPClient.Timeout)
	}
}
