package main

import (
	"strings"
	"testing"

	"apple-music-downloader/utils/structs"
)

func validConfigForTest() structs.ConfigSet {
	return structs.ConfigSet{
		CoverFormat:    "jpg",
		LrcType:        "lyrics",
		LrcFormat:      "lrc",
		GetM3u8Mode:    "hires",
		AacType:        "aac-lc",
		MVAudioType:    "atmos",
		ConvertFormat:  "flac",
		MaxMemoryLimit: 256,
		LimitMax:       200,
		AlacMax:        192000,
		AtmosMax:       2768,
		MVMax:          2160,
		CoverSize:      "5000x5000",
	}
}

func TestValidateConfigAcceptsValidConfig(t *testing.T) {
	config := validConfigForTest()
	if err := validateConfig(&config); err != nil {
		t.Fatalf("validateConfig returned error for valid config: %v", err)
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
