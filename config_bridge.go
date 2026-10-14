package main

import (
	"amdl/internal/config"
	"amdl/utils/structs"
	"fmt"
	"github.com/knadh/koanf/v2"
	"gopkg.in/yaml.v2"
	"strings"
)

// legacyConfigKeys maps the fork's flat YAML keys to current upstream domains.
// Nested values win when both forms are present; fork-only controls stay flat.
var legacyConfigKeys = map[string]string{
	"proxy":                          "general.proxy",
	"lite-server":                    "general.lite-server",
	"storefront":                     "general.storefront",
	"media-user-token":               "general.media-user-token",
	"authorization-token":            "general.authorization-token",
	"language":                       "general.language",
	"max-memory-limit":               "general.max-memory-limit",
	"exit-on-error":                  "general.exit-on-error",
	"get-m3u8-mode":                  "media.get-m3u8-mode",
	"alac-max":                       "media.alac-max",
	"atmos-max":                      "media.atmos-max",
	"aac-type":                       "media.aac-type",
	"alac-fix":                       "media.alac-fix",
	"mv-audio-type":                  "media.mv.audio-type",
	"mv-max":                         "media.mv.max",
	"alac-save-folder":               "paths.alac",
	"atmos-save-folder":              "paths.atmos",
	"aac-save-folder":                "paths.aac",
	"mv-save-folder":                 "paths.mv",
	"album-folder-format":            "paths.album-folder",
	"playlist-folder-format":         "paths.playlist-folder",
	"artist-folder-format":           "paths.artist-folder",
	"song-file-format":               "paths.song-file",
	"limit-max":                      "paths.limit-max",
	"explicit-choice":                "paths.explicit",
	"clean-choice":                   "paths.clean",
	"apple-master-choice":            "paths.apple-master",
	"save-lrc-file":                  "metadata.lyrics.save-file",
	"embed-lrc":                      "metadata.lyrics.embed",
	"lrc-type":                       "metadata.lyrics.type",
	"lrc-format":                     "metadata.lyrics.format",
	"lrc-extra":                      "metadata.lyrics.extra",
	"embed-cover":                    "metadata.artwork.embed",
	"cover-size":                     "metadata.artwork.size",
	"cover-format":                   "metadata.artwork.format",
	"save-artist-cover":              "metadata.artwork.save-artist",
	"save-animated-artwork":          "metadata.artwork.save-animated",
	"emby-animated-artwork":          "metadata.artwork.emby-animated",
	"dl-albumcover-for-playlist":     "metadata.artwork.dl-for-playlist",
	"tag-sort-order":                 "metadata.tags.sort-order",
	"tag-itunes-id":                  "metadata.tags.itunes-id",
	"use-songinfo-for-playlist":      "metadata.tags.use-songinfo-for-playlist",
	"convert-after-download":         "convert.after-download",
	"convert-format":                 "convert.format",
	"convert-keep-original":          "convert.keep-original",
	"convert-skip-if-source-matches": "convert.skip-if-source-matches",
	"ffmpeg-path":                    "convert.ffmpeg-path",
	"convert-extra-args":             "convert.extra-args",
	"convert-with-metadata":          "convert.with-metadata",
	"convert-warn-lossy-to-lossless": "convert.warn-lossy-to-lossless",
	"convert-skip-lossy-to-lossless": "convert.skip-lossy-to-lossless",
	"convert-check-bad-alac":         "convert.check-bad-alac",
	"convert-delete-bad-alac":        "convert.delete-bad-alac",
}

var previousNestedConfigKeys = map[string]string{
	"album-folder-format":       "metadata.format.album-folder",
	"playlist-folder-format":    "metadata.format.playlist-folder",
	"artist-folder-format":      "metadata.format.artist-folder",
	"song-file-format":          "metadata.format.song-file",
	"limit-max":                 "metadata.format.limit-max",
	"explicit-choice":           "metadata.tags.explicit",
	"clean-choice":              "metadata.tags.clean",
	"apple-master-choice":       "metadata.tags.apple-master",
	"use-songinfo-for-playlist": "metadata.format.use-songinfo-for-playlist",
}

func configValue(raw map[interface{}]interface{}, path string) (interface{}, bool) {
	var current interface{} = raw
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[interface{}]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func normalizeConfigData(data []byte) ([]byte, error) {
	var raw map[interface{}]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		raw = map[interface{}]interface{}{}
	}
	for _, domain := range []string{"general", "media", "paths", "metadata", "convert"} {
		if value, ok := raw[domain]; ok && value != nil {
			if _, ok := value.(map[interface{}]interface{}); !ok {
				return nil, fmt.Errorf("config section %s must be a YAML mapping", domain)
			}
		}
	}
	for flat, nested := range legacyConfigKeys {
		if value, ok := configValue(raw, nested); ok {
			raw[flat] = value
		}
	}
	// Previous upstream releases stored naming templates under metadata.format.
	for flat, nested := range previousNestedConfigKeys {
		if _, present := raw[flat]; !present {
			if value, ok := configValue(raw, nested); ok {
				raw[flat] = value
			}
		}
	}

	return yaml.Marshal(raw)
}

func upstreamConfig(legacy structs.ConfigSet) config.ConfigSet {
	data, _ := yaml.Marshal(legacy)
	var raw map[string]interface{}
	_ = yaml.Unmarshal(data, &raw)
	k := koanf.New(".")
	for flat, nested := range legacyConfigKeys {
		_ = k.Set(nested, raw[flat])
	}
	var cfg config.ConfigSet
	_ = k.Unmarshal("", &cfg)
	return cfg
}
