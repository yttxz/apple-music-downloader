package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type downloadJob struct {
	Index int
	Total int
	URL   string
}

type downloadJobResult struct {
	Job     downloadJob
	Session *sessionState
}

func effectiveDownloadConcurrency(queueSize int) int {
	concurrency := Config.DownloadConcurrency
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > queueSize && queueSize > 0 {
		concurrency = queueSize
	}
	if dl_select && concurrency > 1 {
		fmt.Println("Selective mode is interactive; using download-concurrency 1.")
		return 1
	}
	return concurrency
}

func runDownloadQueue(urls []string, token string) *sessionState {
	aggregate := newSessionState("")
	pending := append([]string(nil), urls...)
	for {
		results := runDownloadPass(pending, token, effectiveDownloadConcurrency(len(pending)))
		passState := newSessionState("")
		failedURLs := make([]string, 0)
		for _, result := range results {
			result.Session.FlushLog()
			passState.merge(result.Session)
			aggregate.merge(result.Session)
			if result.Session != nil && result.Session.Counter.Error > 0 {
				failedURLs = append(failedURLs, result.Job.URL)
			}
		}
		fmt.Printf("=======  [\u2714 ] Completed: %d/%d  |  [\u26A0 ] Warnings: %d  |  [\u2716 ] Errors: %d  =======\n", passState.Counter.Success, passState.Counter.Total, passState.Counter.Unavailable+passState.Counter.NotSong, passState.Counter.Error)
		if passState.Counter.Error == 0 {
			break
		}
		if Config.ExitOnError {
			fmt.Println("Error detected, exiting...")
			os.Exit(1)
		}
		if len(failedURLs) == 0 {
			break
		}
		fmt.Println("Error detected, press Enter to try failed items again...")
		fmt.Scanln()
		fmt.Println("Start trying failed items again...")
		pending = failedURLs
	}
	return aggregate
}

func runDownloadPass(urls []string, token string, concurrency int) []downloadJobResult {
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > len(urls) && len(urls) > 0 {
		concurrency = len(urls)
	}
	jobs := make(chan downloadJob)
	results := make(chan downloadJobResult, len(urls))
	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				state := newSessionState(job.URL)
				processDownloadURL(state, job, token)
				results <- downloadJobResult{Job: job, Session: state}
			}
		}()
	}
	for i, rawURL := range urls {
		jobs <- downloadJob{Index: i, Total: len(urls), URL: rawURL}
	}
	close(jobs)
	wg.Wait()
	close(results)

	collected := make([]downloadJobResult, 0, len(urls))
	for result := range results {
		collected = append(collected, result)
	}
	sort.Slice(collected, func(i, j int) bool {
		return collected[i].Job.Index < collected[j].Job.Index
	})
	return collected
}

func recordJobURLFailure(state *sessionState, rawURL string, err error) {
	item := dbItemFromURL(rawURL)
	item.Codec = state.currentCodec()
	item.SourceURL = rawURL
	recordFailure(item, err)
	state.Counter.Error++
}

func processDownloadURL(state *sessionState, job downloadJob, token string) {
	urlRaw := job.URL
	state.Logf("Queue %d of %d: ", job.Index+1, job.Total)
	var storefront, albumId string

	if strings.Contains(urlRaw, "/music-video/") {
		state.Logln("Music Video")
		if debug_mode {
			return
		}
		state.Counter.Total++

		mvSaveDir := strings.NewReplacer(
			"{ArtistName}", "",
			"{UrlArtistName}", "",
			"{ArtistId}", "",
		).Replace(Config.ArtistFolderFormat)
		if mvSaveDir != "" {
			mvSaveDir = filepath.Join(Config.MVSaveFolder, forbiddenNames.ReplaceAllString(mvSaveDir, "_"))
		} else {
			mvSaveDir = Config.MVSaveFolder
		}
		storefront, albumId = checkUrlMv(urlRaw)
		mvDBItem := DBItem{Storefront: storefront, MediaType: "music-video", AppleID: albumId, Codec: "MV", Quality: fmt.Sprintf("%dp", Config.MVMax), SourceURL: urlRaw}
		if ok, saved, err := existingDownload(mvDBItem); err == nil && ok {
			state.Logln("MV already recorded in database.")
			recordSkip(saved, "already-downloaded")
			state.Counter.Success++
			return
		}
		unlockDownload, locked := downloadInFlight.TryLock(mvDBItem)
		if !locked {
			state.Logln("MV already in progress in this run.")
			recordSkip(mvDBItem, "already-in-progress")
			state.Counter.Success++
			return
		}
		defer unlockDownload()
		if ok, saved, err := existingDownload(mvDBItem); err == nil && ok {
			state.Logln("MV already recorded in database.")
			recordSkip(saved, "already-downloaded")
			state.Counter.Success++
			return
		}
		if dry_run {
			state.Logln("Dry run: MV download skipped.")
			recordSkip(mvDBItem, "dry-run")
			state.Counter.Success++
			return
		}
		startIdx := len(state.AddedTracks)
		err := mvDownloader(state, albumId, mvSaveDir, token, storefront, Config.MediaUserToken, nil, dl_select)
		if err != nil {
			state.Logln("\u26A0 Failed to dl MV:", err)
			recordFailure(mvDBItem, err)
			state.Counter.Error++
			return
		}
		if len(state.AddedTracks) > startIdx {
			mvDBItem.SavePath = state.AddedTracks[len(state.AddedTracks)-1].Path
			recordDownload(mvDBItem)
		}
		state.Counter.Success++
		return
	}
	if strings.Contains(urlRaw, "/song/") {
		state.Logf("Song->")
		storefront, songId := checkUrlSong(urlRaw)
		if storefront == "" || songId == "" {
			state.Logln("Invalid song URL format.")
			recordJobURLFailure(state, urlRaw, errors.New("invalid song URL format"))
			return
		}
		if dl_select {
			err := selectSingleSongAudioFormat(state, songId, token, storefront)
			if err != nil {
				state.Logln("Failed to select audio format:", err)
				recordJobURLFailure(state, urlRaw, err)
				return
			}
		}
		err := ripSong(state, songId, token, storefront, Config.MediaUserToken)
		if err != nil {
			state.Logln("Failed to rip song:", err)
			recordJobURLFailure(state, urlRaw, err)
		}
		return
	}
	parse, err := url.Parse(urlRaw)
	if err != nil {
		state.Logf("Invalid URL: %v\n", err)
		recordJobURLFailure(state, urlRaw, err)
		return
	}
	urlArg_i := parse.Query().Get("i")

	if strings.Contains(urlRaw, "/album/") {
		state.Logln("Album")
		storefront, albumId = checkUrl(urlRaw)
		err := ripAlbum(state, albumId, token, storefront, Config.MediaUserToken, urlArg_i)
		if err != nil {
			state.Logln("Failed to rip album:", err)
			recordJobURLFailure(state, urlRaw, err)
		}
	} else if strings.Contains(urlRaw, "/playlist/") {
		state.Logln("Playlist")
		storefront, albumId = checkUrlPlaylist(urlRaw)
		err := ripPlaylist(state, albumId, token, storefront, Config.MediaUserToken)
		if err != nil {
			state.Logln("Failed to rip playlist:", err)
			recordJobURLFailure(state, urlRaw, err)
		}
	} else if strings.Contains(urlRaw, "/station/") {
		state.Logf("Station")
		storefront, albumId = checkUrlStation(urlRaw)
		if len(Config.MediaUserToken) <= 50 {
			state.Logln(": meida-user-token is not set, skip station dl")
			recordSkip(dbItemFromURL(urlRaw), "missing-media-user-token")
			return
		}
		err := ripStation(state, albumId, token, storefront, Config.MediaUserToken)
		if err != nil {
			state.Logln("Failed to rip station:", err)
			recordJobURLFailure(state, urlRaw, err)
		}
	} else {
		state.Logln("Invalid type")
		recordJobURLFailure(state, urlRaw, errors.New("invalid URL type"))
	}
}
