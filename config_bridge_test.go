package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"amdl/internal/config"
	"amdl/internal/download"
	"amdl/internal/wrapper"
	legacystructs "amdl/utils/structs"
	"github.com/knadh/koanf/providers/structs"
	"gopkg.in/yaml.v2"
)

func TestUpstreamConfigRoundTrip(t *testing.T) {
	want := config.Default()
	want.General.MediaUserToken = "test-token"
	want.General.Proxy = "socks5://localhost:1080"
	want.Metadata.Lyrics.Extra = "translation"
	want.Media.AlacMax = 0
	// Provider.Read preserves koanf names, unlike YAML's default Go field names.
	raw, err := structs.Provider(want, "koanf").Read()
	if err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	data, err = normalizeConfigData(data)
	if err != nil {
		t.Fatal(err)
	}
	var legacy legacystructs.ConfigSet
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	if got := upstreamConfig(legacy); !reflect.DeepEqual(got, want) {
		t.Fatalf("config did not round-trip:\n got %#v\nwant %#v", got, want)
	}
}

func TestNestedConfigWinsAndKeepsForkOptions(t *testing.T) {
	data, err := normalizeConfigData([]byte("alac-max: 48000\nembed-cover: true\nmedia:\n  alac-max: 0\nmetadata:\n  artwork:\n    embed: false\ndownload-concurrency: 4\n"))
	if err != nil {
		t.Fatal(err)
	}
	legacy := defaultConfig()
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.AlacMax != 0 || legacy.EmbedCover || legacy.DownloadConcurrency != 4 {
		t.Fatalf("config precedence lost: %#v", legacy)
	}
}

func TestConfigExampleLoadsIntoFork(t *testing.T) {
	oldConfig, oldHTTP, oldArtwork, oldDownload, oldWrapper := Config, httpClient, artworkHTTPClient, download.Client, wrapper.ControlClient
	t.Cleanup(func() {
		Config = oldConfig
		httpClient = oldHTTP
		artworkHTTPClient = oldArtwork
		download.Client = oldDownload
		wrapper.ControlClient = oldWrapper
	})
	if err := loadConfig("config.yaml.example"); err != nil {
		t.Fatal(err)
	}
	if Config.LiteServer == "" || Config.AlacSaveFolder != "AM-Lossless" || !Config.DatabaseEnabled {
		t.Fatal("upstream and fork defaults were not loaded")
	}
	if httpClient.Timeout == 0 || artworkHTTPClient.Timeout == 0 || download.Client.Timeout != 0 {
		t.Fatal("catalog, artwork and streaming timeouts were not separated")
	}
}

func TestMigrationPreservesFlatValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("media-user-token: test-secret\nalac-save-folder: /custom/music\nembed-cover: false\nalac-max: 0\ndownload-concurrency: 4\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	template, err := migrationTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(template), &raw); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]interface{}{"general.media-user-token": "test-secret", "paths.alac": "/custom/music", "metadata.artwork.embed": false, "media.alac-max": 0} {
		if got, ok := configValue(raw, key); !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("migration changed %s: %v", key, got)
		}
	}
	saved, err := os.ReadFile(path)
	if err != nil || string(saved) != string(original) {
		t.Fatal("preparing migration changed the user's file")
	}
}

func TestPreviousNestedConfigAliases(t *testing.T) {
	data, err := normalizeConfigData([]byte("metadata:\n  format:\n    song-file: '{ArtistName} - {SongName}'\n    limit-max: 128\n  tags:\n    explicit: EXPLICIT\n"))
	if err != nil {
		t.Fatal(err)
	}
	var legacy legacystructs.ConfigSet
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.LimitMax != 128 || legacy.ExplicitChoice != "EXPLICIT" || legacy.SongFileFormat != "{ArtistName} - {SongName}" {
		t.Fatal("previous upstream schema was not preserved")
	}
}

func TestMalformedNestedConfigFails(t *testing.T) {
	if _, err := normalizeConfigData([]byte("general: not-a-mapping\n")); err == nil {
		t.Fatal("malformed config was silently ignored")
	}
}
