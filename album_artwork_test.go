package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAlbumAndArtistCoversRunsConcurrently(t *testing.T) {
	oldConfig := Config
	oldArtworkHTTPClient := artworkHTTPClient
	oldDryRun := dry_run
	t.Cleanup(func() {
		Config = oldConfig
		artworkHTTPClient = oldArtworkHTTPClient
		dry_run = oldDryRun
	})

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-release
		_, _ = w.Write([]byte("image-data"))
	}))
	defer server.Close()

	Config = defaultConfig()
	Config.CoverFormat = "jpg"
	Config.CoverSize = "100x100"
	dry_run = false
	artworkHTTPClient = server.Client()

	root := t.TempDir()
	albumFolder := filepath.Join(root, "album")
	if err := os.MkdirAll(albumFolder, 0755); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	var albumPath string
	var albumErr, artistErr error
	go func() {
		albumPath, albumErr, artistErr = writeAlbumAndArtistCovers(
			root,
			albumFolder,
			server.URL+"/artist/{w}x{h}.jpg",
			server.URL+"/album/{w}x{h}.jpg",
		)
		close(done)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("album and artist artwork requests did not overlap")
		}
	}
	close(release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("artwork downloads did not finish")
	}
	if albumErr != nil || artistErr != nil {
		t.Fatalf("album error = %v, artist error = %v", albumErr, artistErr)
	}
	if albumPath != filepath.Join(albumFolder, "cover.jpg") {
		t.Fatalf("album path = %q", albumPath)
	}
	for _, path := range []string{albumPath, filepath.Join(root, "folder.jpg")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artwork %s: %v", path, err)
		}
	}
}
