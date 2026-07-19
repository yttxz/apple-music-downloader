package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCoverUsesArtworkHTTPClient(t *testing.T) {
	oldConfig := Config
	oldHTTPClient := httpClient
	oldArtworkHTTPClient := artworkHTTPClient
	oldDryRun := dry_run
	t.Cleanup(func() {
		Config = oldConfig
		httpClient = oldHTTPClient
		artworkHTTPClient = oldArtworkHTTPClient
		dry_run = oldDryRun
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/100x100.jpg" {
			t.Errorf("request path = %q, want /100x100.jpg", r.URL.Path)
		}
		_, _ = w.Write([]byte("cover-data"))
	}))
	defer server.Close()

	Config = defaultConfig()
	Config.CoverFormat = "jpg"
	Config.CoverSize = "100x100"
	dry_run = false
	httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("general client must not be used for artwork")
	})}
	artworkHTTPClient = server.Client()

	dir := t.TempDir()
	path, err := writeCover(dir, "cover", server.URL+"/{w}x{h}.jpg")
	if err != nil {
		t.Fatalf("writeCover: %v", err)
	}
	if path != filepath.Join(dir, "cover.jpg") {
		t.Fatalf("cover path = %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "cover-data" {
		t.Fatalf("cover data = %q", data)
	}
}

func TestWriteCoverReturnsHTTPStatus(t *testing.T) {
	oldConfig := Config
	oldArtworkHTTPClient := artworkHTTPClient
	oldDryRun := dry_run
	t.Cleanup(func() {
		Config = oldConfig
		artworkHTTPClient = oldArtworkHTTPClient
		dry_run = oldDryRun
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	Config = defaultConfig()
	Config.CoverFormat = "jpg"
	Config.CoverSize = "100x100"
	dry_run = false
	artworkHTTPClient = server.Client()

	_, err := writeCover(t.TempDir(), "cover", server.URL+"/{w}x{h}.jpg")
	if err == nil {
		t.Fatal("expected HTTP error")
	}
	if !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Fatalf("error = %q, want HTTP status", err)
	}
}
