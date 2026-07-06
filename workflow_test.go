package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadURLsFromFileTextDedupes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "urls.txt")
	content := "https://music.apple.com/us/album/a/1\nhttps://music.apple.com/us/album/a/1\nhttps://music.apple.com/us/song/b/2"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	urls, err := loadURLsFromFile(path)
	if err != nil {
		t.Fatalf("loadURLsFromFile: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("len(urls) = %d, want 2: %#v", len(urls), urls)
	}
}

func TestLoadURLsFromFileJSONItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.json")
	content := `[{"storefront":"us","media_type":"song","apple_id":"123"}]`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	urls, err := loadURLsFromFile(path)
	if err != nil {
		t.Fatalf("loadURLsFromFile: %v", err)
	}
	want := "https://music.apple.com/us/song/1/123"
	if len(urls) != 1 || urls[0] != want {
		t.Fatalf("urls = %#v, want %q", urls, want)
	}
}

func TestURLFromID(t *testing.T) {
	Config.Storefront = "us"
	got, err := urlFromID("playlist", "pl.abc", "")
	if err != nil {
		t.Fatalf("urlFromID: %v", err)
	}
	want := "https://music.apple.com/us/playlist/1/pl.abc"
	if got != want {
		t.Fatalf("urlFromID = %q, want %q", got, want)
	}
}
