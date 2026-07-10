package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"apple-music-downloader/utils/task"

	"github.com/olekukonko/tablewriter"
)

func loadURLsFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var jsonItems []string
	if err := json.Unmarshal(data, &jsonItems); err == nil {
		return dedupeStrings(jsonItems), nil
	}
	var objectItems []DBItem
	if err := json.Unmarshal(data, &objectItems); err == nil {
		urls := make([]string, 0, len(objectItems))
		for _, item := range objectItems {
			urls = append(urls, urlForDBItem(item))
		}
		return dedupeStrings(urls), nil
	}
	return dedupeStrings(strings.Fields(string(data))), nil
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func urlFromID(kind, id, storefront string) (string, error) {
	if storefront == "" {
		storefront = Config.Storefront
	}
	if len(storefront) != 2 {
		return "", fmt.Errorf("invalid storefront %q", storefront)
	}
	item := DBItem{Storefront: storefront, MediaType: kind, AppleID: id}
	u := urlForDBItem(item)
	if u == "" {
		return "", fmt.Errorf("invalid id-type %q", kind)
	}
	return u, nil
}

func trackDBItem(state *sessionState, track *task.Track, path string) DBItem {
	mediaType := track.Type
	if mediaType == "" {
		mediaType = "track"
	}
	if mediaType == "songs" {
		mediaType = "song"
	}
	if mediaType == "music-videos" {
		mediaType = "music-video"
	}
	var sourceURL string
	if state != nil {
		sourceURL = state.SourceURL
	}
	if sourceURL == "" {
		sourceURL = urlForDBItem(DBItem{Storefront: track.Storefront, MediaType: mediaType, AppleID: track.ID})
	}
	return DBItem{
		Storefront: track.Storefront,
		MediaType:  mediaType,
		AppleID:    track.ID,
		Codec:      track.Codec,
		Quality:    track.Quality,
		SourceURL:  sourceURL,
		SavePath:   path,
	}
}

func recordDownload(item DBItem) {
	if appDB != nil && !no_database {
		if err := appDB.RecordDownload(item); err != nil {
			fmt.Println("Failed to record download:", err)
		}
	}
	runSummary.RecordDownloaded(item)
}

func recordSkip(item DBItem, reason string) {
	runSummary.RecordSkipped(item, reason)
}

func recordFailure(item DBItem, err error) {
	err = redactError(err)
	if appDB != nil && !no_database && Config.FailedDownloadsEnabled {
		if dbErr := appDB.RecordFailure(item, err); dbErr != nil {
			fmt.Println("Failed to record failure:", dbErr)
		}
	}
	runSummary.RecordFailed(item, err)
}

func existingDownload(item DBItem) (bool, DBItem, error) {
	ok, saved, err := appDB.Downloaded(item)
	if err != nil || !ok {
		return ok, saved, err
	}
	if saved.SavePath == "" {
		return false, saved, nil
	}
	info, err := os.Stat(saved.SavePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, saved, nil
		}
		return false, saved, err
	}
	if !info.Mode().IsRegular() {
		return false, saved, nil
	}
	return true, saved, nil
}

func dbItemFromURL(rawURL string) DBItem {
	if storefront, id := checkUrl(rawURL); storefront != "" {
		return DBItem{Storefront: storefront, MediaType: "album", AppleID: id}
	}
	if storefront, id := checkUrlSong(rawURL); storefront != "" {
		return DBItem{Storefront: storefront, MediaType: "song", AppleID: id}
	}
	if storefront, id := checkUrlPlaylist(rawURL); storefront != "" {
		return DBItem{Storefront: storefront, MediaType: "playlist", AppleID: id}
	}
	if storefront, id := checkUrlStation(rawURL); storefront != "" {
		return DBItem{Storefront: storefront, MediaType: "station", AppleID: id}
	}
	if storefront, id := checkUrlMv(rawURL); storefront != "" {
		return DBItem{Storefront: storefront, MediaType: "music-video", AppleID: id}
	}
	return DBItem{Storefront: Config.Storefront, MediaType: "unknown", AppleID: rawURL}
}

func openConfiguredDB() error {
	if no_database || !Config.DatabaseEnabled {
		return nil
	}
	db, err := OpenDownloadDB(Config.DatabasePath)
	if err != nil {
		return err
	}
	appDB = db
	return nil
}

func printDBList(table string) error {
	if appDB == nil {
		return errors.New("database is disabled")
	}
	switch strings.ToLower(table) {
	case "downloads":
		items, err := appDB.ListDownloads()
		if err != nil {
			return err
		}
		printDBItems("Downloads", items, false)
	case "failed":
		items, err := appDB.ListFailed()
		if err != nil {
			return err
		}
		printDBItems("Failed Downloads", items, true)
	default:
		return fmt.Errorf("invalid db-list %q: expected downloads or failed", table)
	}
	return nil
}

func printDBItems(title string, items []DBItem, includeError bool) {
	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"Storefront", "Type", "ID", "Codec", "Quality", "Path/URL"}
	if includeError {
		headers = append(headers, "Attempts", "Error")
	}
	table.SetHeader(headers)
	for _, item := range items {
		pathOrURL := item.SavePath
		if pathOrURL == "" {
			pathOrURL = item.SourceURL
		}
		row := []string{item.Storefront, item.MediaType, item.AppleID, item.Codec, item.Quality, pathOrURL}
		if includeError {
			row = append(row, fmt.Sprintf("%d", item.Attempts), item.Error)
		}
		table.Append(row)
	}
	fmt.Println(title)
	table.Render()
}

func runDoctor() error {
	checks := []struct {
		name string
		ok   bool
		msg  string
	}{
		{"config", true, "loaded"},
		{"ffmpeg", toolAvailable(Config.FFmpegPath), Config.FFmpegPath},
		{"MP4Box", toolAvailable(Config.MP4BoxPath), Config.MP4BoxPath},
		{"mp4decrypt", toolAvailable(Config.MP4DecryptPath), Config.MP4DecryptPath},
		{"decrypt wrapper port", isLoopbackConfig(Config.DecryptM3u8Port), Config.DecryptM3u8Port},
		{"get-m3u8 wrapper port", isLoopbackConfig(Config.GetM3u8Port), Config.GetM3u8Port},
	}
	for _, check := range checks {
		status := "OK"
		if !check.ok {
			status = "WARN"
		}
		fmt.Printf("[%s] %s: %s\n", status, check.name, check.msg)
	}
	if appDB != nil {
		fmt.Printf("[OK] database: %s\n", Config.DatabasePath)
	}
	if missing, err := missingManagedConfigKeys(config_path); err == nil && len(missing) > 0 {
		fmt.Printf("[WARN] config keys: %d managed key(s) missing; run --config-upgrade\n", len(missing))
	} else if err != nil {
		fmt.Printf("[WARN] config keys: %s\n", err)
	} else {
		fmt.Println("[OK] config keys: complete")
	}
	return nil
}

func isLoopbackConfig(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func plannedTrackPath(track *task.Track, path string) string {
	if path != "" {
		return path
	}
	if track.SavePath != "" {
		return track.SavePath
	}
	if track.SaveDir != "" && track.SaveName != "" {
		return filepath.Join(track.SaveDir, track.SaveName)
	}
	return ""
}
