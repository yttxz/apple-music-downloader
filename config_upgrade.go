package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

var managedConfigDefaults = map[string]interface{}{
	"database-enabled":               true,
	"database-path":                  "amdl.db",
	"failed-downloads-enabled":       true,
	"download-concurrency":           1,
	"mv-segment-concurrency":         6,
	"requests-per-minute":            60,
	"retry-count":                    3,
	"retry-backoff-ms":               500,
	"request-timeout-seconds":        30,
	"artwork-timeout-seconds":        180,
	"allow-nonlocal-wrapper":         false,
	"quality-fallback":               "best-available",
	"artist-filter-singles-eps":      false,
	"artist-filter-live":             false,
	"artist-filter-compilations":     false,
	"artist-filter-duplicates":       false,
	"ffmpeg-path":                    "ffmpeg",
	"mp4box-path":                    "MP4Box",
	"mp4decrypt-path":                "mp4decrypt",
	"convert-extra-args":             "",
	"convert-with-metadata":          true,
	"convert-warn-lossy-to-lossless": true,
	"convert-skip-lossy-to-lossless": true,
}

var managedConfigOrder = []string{
	"database-enabled",
	"database-path",
	"failed-downloads-enabled",
	"download-concurrency",
	"mv-segment-concurrency",
	"requests-per-minute",
	"retry-count",
	"retry-backoff-ms",
	"request-timeout-seconds",
	"artwork-timeout-seconds",
	"allow-nonlocal-wrapper",
	"quality-fallback",
	"artist-filter-singles-eps",
	"artist-filter-live",
	"artist-filter-compilations",
	"artist-filter-duplicates",
	"ffmpeg-path",
	"mp4box-path",
	"mp4decrypt-path",
	"convert-extra-args",
	"convert-with-metadata",
	"convert-warn-lossy-to-lossless",
	"convert-skip-lossy-to-lossless",
}

func missingManagedConfigKeys(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[interface{}]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	missing := make([]string, 0)
	for _, key := range managedConfigOrder {
		if _, ok := raw[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing, nil
}

func printConfigUpgrade(path string) error {
	missing, err := missingManagedConfigKeys(path)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		fmt.Printf("Config has all managed keys: %s\n", path)
		return nil
	}
	fmt.Printf("Config is missing %d managed key(s): %s\n\n", len(missing), strings.Join(missing, ", "))
	fmt.Println("Add this snippet to your config if you want the current defaults written explicitly:")
	fmt.Println()
	for _, key := range missing {
		data, err := yaml.Marshal(map[string]interface{}{key: managedConfigDefaults[key]})
		if err != nil {
			return err
		}
		fmt.Print(string(data))
	}
	return nil
}
