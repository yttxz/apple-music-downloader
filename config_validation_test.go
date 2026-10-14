package main

import (
	"strings"
	"testing"

	"amdl/utils/structs"
)

func validConfigForTest() structs.ConfigSet {
	return structs.ConfigSet{
		CoverFormat:           "jpg",
		LrcType:               "lyrics",
		LrcFormat:             "lrc",
		GetM3u8Mode:           "hires",
		AacType:               "aac-lc",
		MVAudioType:           "atmos",
		ConvertFormat:         "flac",
		MaxMemoryLimit:        256,
		LimitMax:              200,
		AlacMax:               192000,
		AtmosMax:              2768,
		MVMax:                 2160,
		CoverSize:             "5000x5000",
		AlacSaveFolder:        "AM-DL downloads",
		AtmosSaveFolder:       "AM-DL-Atmos downloads",
		AacSaveFolder:         "AM-DL-AAC downloads",
		MVSaveFolder:          "AM-DL-MV downloads",
		DecryptM3u8Port:       "127.0.0.1:10020",
		GetM3u8Port:           "127.0.0.1:20020",
		DownloadConcurrency:   1,
		MVSegmentConcurrency:  6,
		RequestsPerMinute:     60,
		RetryCount:            3,
		RetryBackoffMS:        500,
		RequestTimeoutSeconds: 30,
		ArtworkTimeoutSeconds: 180,
		QualityFallback:       "skip",
	}
}

func TestValidateConfigAcceptsValidConfig(t *testing.T) {
	config := validConfigForTest()
	if err := validateConfig(&config); err != nil {
		t.Fatalf("validateConfig returned error for valid config: %v", err)
	}
}

func TestDefaultConfigUsesBestAvailableFallback(t *testing.T) {
	if got := defaultConfig().QualityFallback; got != "best-available" {
		t.Fatalf("default quality fallback = %q, want best-available", got)
	}
	if got := managedConfigDefaults["quality-fallback"]; got != "best-available" {
		t.Fatalf("managed quality fallback = %#v, want best-available", got)
	}
}

func TestValidateConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*structs.ConfigSet)
		wantErr string
	}{
		{
			name:    "cover format",
			mutate:  func(c *structs.ConfigSet) { c.CoverFormat = "webp" },
			wantErr: "cover-format",
		},
		{
			name:    "aac type",
			mutate:  func(c *structs.ConfigSet) { c.AacType = "aac-surround" },
			wantErr: "aac-type",
		},
		{
			name:    "convert format",
			mutate:  func(c *structs.ConfigSet) { c.ConvertFormat = "alac" },
			wantErr: "convert-format",
		},
		{
			name:    "memory limit",
			mutate:  func(c *structs.ConfigSet) { c.MaxMemoryLimit = 0 },
			wantErr: "max-memory-limit",
		},
		{
			name:    "cover size",
			mutate:  func(c *structs.ConfigSet) { c.CoverSize = " " },
			wantErr: "cover-size",
		},
		{
			name:    "wrapper port",
			mutate:  func(c *structs.ConfigSet) { c.DecryptM3u8Port = "0.0.0.0:10020" },
			wantErr: "decrypt-m3u8-port",
		},
		{
			name:    "convert extra args",
			mutate:  func(c *structs.ConfigSet) { c.ConvertExtraArgs = "-i evil.m4a" },
			wantErr: "convert-extra-args",
		},
		{
			name:    "quality fallback",
			mutate:  func(c *structs.ConfigSet) { c.QualityFallback = "always" },
			wantErr: "quality-fallback",
		},
		{
			name:    "artwork timeout",
			mutate:  func(c *structs.ConfigSet) { c.ArtworkTimeoutSeconds = 0 },
			wantErr: "artwork-timeout-seconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validConfigForTest()
			tt.mutate(&config)
			err := validateConfig(&config)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateConfig error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
