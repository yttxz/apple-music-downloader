package main

import (
	"io"
	"net/http"
	"strings"
	"testing"

	task "amdl/internal/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestSetTrackQualityReadsManifest(t *testing.T) {
	oldClient := httpClient
	oldConfig := Config
	oldAtmos := dl_atmos
	oldAAC := dl_aac
	oldDebug := debug_mode
	t.Cleanup(func() {
		httpClient = oldClient
		Config = oldConfig
		dl_atmos = oldAtmos
		dl_aac = oldAAC
		debug_mode = oldDebug
	})

	Config = defaultConfig()
	dl_atmos = false
	dl_aac = false
	debug_mode = false

	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `#EXTM3U
#EXT-X-VERSION:6
#EXT-X-STREAM-INF:BANDWIDTH=3200000,AVERAGE-BANDWIDTH=2400000,CODECS="alac",AUDIO="audio-alac-stereo-96000-24"
alac/prog.m3u8
`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}

	state := newSessionState("https://music.apple.com/us/song/example/1")
	track := &task.Track{M3u8: "https://example.invalid/master.m3u8"}
	if err := setTrackQuality(state, track, false); err != nil {
		t.Fatalf("setTrackQuality: %v", err)
	}
	if track.Quality != "24B-96.0kHz" {
		t.Fatalf("track.Quality = %q, want 24B-96.0kHz", track.Quality)
	}
}

func TestAACFallbackCoversLosslessAndAtmos(t *testing.T) {
	oldConfig := Config
	oldAtmos := dl_atmos
	t.Cleanup(func() {
		Config = oldConfig
		dl_atmos = oldAtmos
	})

	Config = defaultConfig()
	dl_atmos = false
	if message, ok := aacFallbackMessage(newSessionState("")); !ok || !strings.Contains(message, "Lossless") {
		t.Fatalf("lossless fallback = (%q, %v), want enabled", message, ok)
	}

	dl_atmos = true
	if message, ok := aacFallbackMessage(newSessionState("")); !ok || !strings.Contains(message, "Atmos") {
		t.Fatalf("Atmos fallback = (%q, %v), want enabled", message, ok)
	}

	Config.QualityFallback = "skip"
	if message, ok := aacFallbackMessage(newSessionState("")); ok || message != "" {
		t.Fatalf("strict fallback = (%q, %v), want disabled", message, ok)
	}
}
