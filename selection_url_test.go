package main

import (
	"reflect"
	"testing"
)

func TestParseSelectionIndexes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		total   int
		want    []int
		wantErr bool
	}{
		{name: "all", input: "all", total: 4, want: []int{1, 2, 3, 4}},
		{name: "empty", input: "", total: 4, want: nil},
		{name: "list and range", input: "1,3-5", total: 6, want: []int{1, 3, 4, 5}},
		{name: "deduplicates", input: "1,2,2,1-3", total: 4, want: []int{1, 2, 3}},
		{name: "whitespace separators", input: "1 2\t4", total: 4, want: []int{1, 2, 4}},
		{name: "range out of bounds", input: "2-5", total: 4, wantErr: true},
		{name: "reversed range", input: "3-2", total: 4, wantErr: true},
		{name: "non numeric", input: "x", total: 4, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSelectionIndexes(tt.input, tt.total)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseSelectionIndexes(%q, %d) = %#v, want %#v", tt.input, tt.total, got, tt.want)
			}
		})
	}
}

func TestAppleMusicURLParsing(t *testing.T) {
	tests := []struct {
		name       string
		parse      func(string) (string, string)
		url        string
		storefront string
		id         string
	}{
		{
			name:       "album",
			parse:      checkUrl,
			url:        "https://music.apple.com/us/album/example/123456789",
			storefront: "us",
			id:         "123456789",
		},
		{
			name:       "song",
			parse:      checkUrlSong,
			url:        "https://music.apple.com/jp/song/example/987654321?i=987654321",
			storefront: "jp",
			id:         "987654321",
		},
		{
			name:       "playlist",
			parse:      checkUrlPlaylist,
			url:        "https://music.apple.com/gb/playlist/example/pl.abc-123",
			storefront: "gb",
			id:         "pl.abc-123",
		},
		{
			name:       "station",
			parse:      checkUrlStation,
			url:        "https://music.apple.com/ca/station/example/ra.456-abc",
			storefront: "ca",
			id:         "ra.456-abc",
		},
		{
			name:       "music video",
			parse:      checkUrlMv,
			url:        "https://music.apple.com/us/music-video/example/123456789",
			storefront: "us",
			id:         "123456789",
		},
		{
			name:       "artist",
			parse:      checkUrlArtist,
			url:        "https://music.apple.com/de/artist/example/123456789",
			storefront: "de",
			id:         "123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storefront, id := tt.parse(tt.url)
			if storefront != tt.storefront || id != tt.id {
				t.Fatalf("parse(%q) = (%q, %q), want (%q, %q)", tt.url, storefront, id, tt.storefront, tt.id)
			}
		})
	}
}
