package main

import "sync"

func writeAlbumAndArtistCovers(singerFolder, albumFolder, artistURL, albumURL string) (string, error, error) {
	var (
		albumPath string
		albumErr  error
		artistErr error
		wg        sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		albumPath, albumErr = writeCoverWithLabel(albumFolder, "cover", albumURL, "Album artwork")
	}()

	if artistURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, artistErr = writeCoverWithLabel(singerFolder, "folder", artistURL, "Artist artwork")
		}()
	}

	wg.Wait()
	return albumPath, albumErr, artistErr
}
