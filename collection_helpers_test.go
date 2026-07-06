package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"apple-music-downloader/utils/task"
)

func TestMarkTrackSkippedOnlyListsExistingOrDownloadedPaths(t *testing.T) {
	oldSummary := runSummary
	oldNoDatabase := no_database
	oldDB := appDB
	t.Cleanup(func() {
		runSummary = oldSummary
		no_database = oldNoDatabase
		appDB = oldDB
	})

	runSummary = newRunSummary()
	no_database = true
	appDB = nil

	state := newSessionState("https://music.apple.com/us/album/example/1")
	track := &task.Track{
		ID:         "123",
		Type:       "songs",
		Storefront: "us",
		Codec:      "ALAC",
		TaskNum:    1,
		PreID:      "album-1",
	}
	track.Resp.Attributes.ArtistName = "Artist"
	track.Resp.Attributes.AlbumName = "Album"
	track.Resp.Attributes.Name = "Song"

	markTrackSkipped(state, track, "", "dry-run")
	if len(state.AddedTracks) != 0 {
		t.Fatalf("dry-run skip added tracks = %#v, want none", state.AddedTracks)
	}

	markTrackSkipped(state, track, "/music/Song.m4a", "file-exists")
	if len(state.AddedTracks) != 1 {
		t.Fatalf("file-exists skip added tracks = %#v, want one", state.AddedTracks)
	}
	if state.AddedTracks[0].Path != "/music/Song.m4a" {
		t.Fatalf("added path = %q, want /music/Song.m4a", state.AddedTracks[0].Path)
	}
	if runSummary.Counts.Skipped != 2 {
		t.Fatalf("summary skipped count = %d, want 2", runSummary.Counts.Skipped)
	}
}

func TestWriteM3UPlaylistSkipsEmptyPaths(t *testing.T) {
	oldSave := save_m3u8_playlist
	oldDryRun := dry_run
	t.Cleanup(func() {
		save_m3u8_playlist = oldSave
		dry_run = oldDryRun
	})

	save_m3u8_playlist = true
	dry_run = false
	dir := t.TempDir()
	err := writeM3UPlaylist(dir, "playlist", []AddedTrack{
		{Path: "", Artist: "Missing", Song: "Skipped"},
		{Path: filepath.Join(dir, "Song.m4a"), Artist: "Artist", Song: "Song"},
	})
	if err != nil {
		t.Fatalf("writeM3UPlaylist: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "playlist.m3u8"))
	if err != nil {
		t.Fatalf("ReadFile playlist: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "Missing - Skipped") {
		t.Fatalf("playlist contains skipped track with empty path:\n%s", text)
	}
	if !strings.Contains(text, "Artist - Song") || !strings.Contains(text, "Song.m4a") {
		t.Fatalf("playlist missing real track:\n%s", text)
	}
}
