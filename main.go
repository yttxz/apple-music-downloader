package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"apple-music-downloader/utils/alacfix"
	"apple-music-downloader/utils/ampapi"
	"apple-music-downloader/utils/lyrics"
	"apple-music-downloader/utils/runv2"
	"apple-music-downloader/utils/runv3"
	"apple-music-downloader/utils/structs"
	"apple-music-downloader/utils/task"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/grafov/m3u8"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/pflag"

	"github.com/zhaarey/go-mp4tag"
	"gopkg.in/yaml.v2"
)

var (
	forbiddenNames        = regexp.MustCompile(`[/\\<>:"|?*]`)
	videoResolutionRegex  = regexp.MustCompile(`_(\d+)x(\d+)`)
	dl_atmos              bool
	dl_aac                bool
	dl_select             bool
	dl_song               bool
	artist_select         bool
	debug_mode            bool
	print_json            bool
	save_m3u8_playlist    bool
	alac_max              *int
	atmos_max             *int
	mv_max                *int
	mv_audio_type         *string
	aac_type              *string
	selected_audio_format *AudioFormat
	Config                structs.ConfigSet
	session               = newSessionState()
)

type AddedTrack struct {
	Path     string `json:"path"`
	Artist   string `json:"artist"`
	ArtistID string `json:"artist_id"`
	Album    string `json:"album"`
	Song     string `json:"song"`
}

type sessionState struct {
	Counter     structs.Counter
	OKDict      map[string][]int
	AddedTracks []AddedTrack
}

func newSessionState() *sessionState {
	return &sessionState{
		OKDict: make(map[string][]int),
	}
}

type ArtistSelectItem struct {
	Type string
	Name string
	Date string
	ID   string
	URL  string
}

type AudioFormat struct {
	Codecs           string
	Audio            string
	URI              string
	Bandwidth        uint32
	AverageBandwidth uint32
}

func loadConfig() error {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(data, &Config)
	if err != nil {
		return err
	}
	if len(Config.Storefront) != 2 {
		Config.Storefront = "us"
	}
	setDefaultToolPaths()
	return validateConfig(&Config)
}

func setDefaultToolPaths() {
	Config.FFmpegPath = resolveToolPath(defaultToolPath(Config.FFmpegPath, "ffmpeg"))
	Config.MP4BoxPath = resolveToolPath(defaultToolPath(Config.MP4BoxPath, "MP4Box"))
	Config.MP4DecryptPath = resolveToolPath(defaultToolPath(Config.MP4DecryptPath, "mp4decrypt"))
}

func defaultToolPath(configured string, fallback string) string {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return fallback
	}
	return configured
}

func resolveToolPath(tool string) string {
	if tool == "" {
		return tool
	}
	if path, err := exec.LookPath(tool); err == nil {
		return path
	}
	if filepath.IsAbs(tool) || strings.ContainsAny(tool, `/\`) {
		return tool
	}
	for _, dir := range []string{"/opt/homebrew/bin", "/usr/local/bin", "/opt/local/bin"} {
		candidate := filepath.Join(dir, tool)
		if fileExistsOnPath(candidate) {
			return candidate
		}
	}
	return tool
}

func fileExistsOnPath(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func toolAvailable(tool string) bool {
	if _, err := exec.LookPath(tool); err == nil {
		return true
	}
	if filepath.IsAbs(tool) || strings.ContainsAny(tool, `/\`) {
		return fileExistsOnPath(tool)
	}
	return false
}

func LimitString(s string) string {
	if len([]rune(s)) > Config.LimitMax {
		return string([]rune(s)[:Config.LimitMax])
	}
	return s
}

func releaseYear(releaseDate string) string {
	if len(releaseDate) < 4 {
		return ""
	}
	return releaseDate[:4]
}

func firstString(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func isInArray(arr []int, target int) bool {
	for _, num := range arr {
		if num == target {
			return true
		}
	}
	return false
}

func fileExists(path string) (bool, error) {
	f, err := os.Stat(path)
	if err == nil {
		return !f.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getUrlSong(songUrl string, token string) (string, error) {
	storefront, songId := checkUrlSong(songUrl)
	manifest, err := ampapi.GetSongResp(storefront, songId, Config.Language, token)
	if err != nil {
		fmt.Println("\u26A0 Failed to get manifest:", err)
		session.Counter.NotSong++
		return "", err
	}
	if len(manifest.Data[0].Relationships.Albums.Data) == 0 {
		err := errors.New("song response contains no album relationship")
		session.Counter.NotSong++
		return "", err
	}
	albumId := manifest.Data[0].Relationships.Albums.Data[0].ID
	songAlbumUrl := fmt.Sprintf("https://music.apple.com/%s/album/1/%s?i=%s", storefront, albumId, songId)
	return songAlbumUrl, nil
}
func getUrlArtistName(artistUrl string, token string) (string, string, error) {
	storefront, artistId := checkUrlArtist(artistUrl)
	req, err := http.NewRequest("GET", fmt.Sprintf("https://amp-api.music.apple.com/v1/catalog/%s/artists/%s", storefront, artistId), nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Origin", "https://music.apple.com")
	query := url.Values{}
	query.Set("l", Config.Language)
	req.URL.RawQuery = query.Encode()
	do, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return "", "", errors.New(do.Status)
	}
	obj := new(structs.AutoGeneratedArtist)
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return "", "", err
	}
	if len(obj.Data) == 0 {
		return "", "", errors.New("artist response contains no data")
	}
	return obj.Data[0].Attributes.Name, obj.Data[0].ID, nil
}

func checkArtist(artistUrl string, token string, relationship string) ([]string, error) {
	storefront, artistId := checkUrlArtist(artistUrl)
	Num := 0
	//id := 1
	var args []string
	var urls []string
	var options [][]string
	for {
		req, err := http.NewRequest("GET", fmt.Sprintf("https://amp-api.music.apple.com/v1/catalog/%s/artists/%s/%s?limit=100&offset=%d&l=%s", storefront, artistId, relationship, Num, Config.Language), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		req.Header.Set("Origin", "https://music.apple.com")
		do, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer do.Body.Close()
		if do.StatusCode != http.StatusOK {
			return nil, errors.New(do.Status)
		}
		obj := new(structs.AutoGeneratedArtist)
		err = json.NewDecoder(do.Body).Decode(&obj)
		if err != nil {
			return nil, err
		}
		for _, album := range obj.Data {
			options = append(options, []string{album.Attributes.Name, album.Attributes.ReleaseDate, album.ID, album.Attributes.URL})
		}
		Num = Num + 100
		if len(obj.Next) == 0 {
			break
		}
	}
	sort.Slice(options, func(i, j int) bool {
		// 将日期字符串解析为 time.Time 类型进行比较
		dateI, _ := time.Parse("2006-01-02", options[i][1])
		dateJ, _ := time.Parse("2006-01-02", options[j][1])
		return dateI.Before(dateJ) // 返回 true 表示 i 在 j 前面
	})
	if len(options) == 0 {
		fmt.Println("No " + relationship + " found.")
		return nil, nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	typeLabel := strings.ToUpper(strings.TrimSuffix(relationship, "s"))
	if relationship == "music-videos" {
		typeLabel = "MV"
	}
	if relationship == "albums" {
		table.SetHeader([]string{"ID", "TYPE", "TITLE", "DATE", "APPLE_ID"})
	} else if relationship == "music-videos" {
		table.SetHeader([]string{"ID", "TYPE", "TITLE", "DATE", "APPLE_ID"})
	}
	table.SetRowLine(false)
	table.SetHeaderColor(tablewriter.Colors{},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.FgRedColor, tablewriter.Bold},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor})

	table.SetColumnColor(tablewriter.Colors{tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor})
	for i, v := range options {
		urls = append(urls, v[3])
		table.Append([]string{fmt.Sprint(i + 1), typeLabel, v[0], v[1], v[2]})
	}
	table.Render()
	if artist_select {
		fmt.Println("You have selected all options:")
		return urls, nil
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Please select from the " + relationship + " options above (multiple options separated by commas, ranges supported, or type 'all' to select all)")
	cyanColor := color.New(color.FgCyan)
	cyanColor.Print("Enter your choice: ")
	input, _ := reader.ReadString('\n')

	input = strings.TrimSpace(input)
	if input == "all" {
		fmt.Println("You have selected all options:")
		return urls, nil
	}

	selectedOptions := [][]string{}
	parts := strings.Split(input, ",")
	for _, part := range parts {
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			selectedOptions = append(selectedOptions, rangeParts)
		} else {
			selectedOptions = append(selectedOptions, []string{part})
		}
	}

	fmt.Println("You have selected the following options:")
	for _, opt := range selectedOptions {
		if len(opt) == 1 {
			num, err := strconv.Atoi(opt[0])
			if err != nil {
				fmt.Println("Invalid option:", opt[0])
				continue
			}
			if num > 0 && num <= len(options) {
				fmt.Println(options[num-1])
				args = append(args, urls[num-1])
			} else {
				fmt.Println("Option out of range:", opt[0])
			}
		} else if len(opt) == 2 {
			start, err1 := strconv.Atoi(opt[0])
			end, err2 := strconv.Atoi(opt[1])
			if err1 != nil || err2 != nil {
				fmt.Println("Invalid range:", opt)
				continue
			}
			if start < 1 || end > len(options) || start > end {
				fmt.Println("Range out of range:", opt)
				continue
			}
			for i := start; i <= end; i++ {
				fmt.Println(options[i-1])
				args = append(args, urls[i-1])
			}
		} else {
			fmt.Println("Invalid option:", opt)
		}
	}
	return args, nil
}

func fetchArtistItems(artistUrl string, token string, relationship string) ([]ArtistSelectItem, error) {
	storefront, artistId := checkUrlArtist(artistUrl)
	offset := 0
	items := []ArtistSelectItem{}
	itemType := "ALBUM"
	if relationship == "music-videos" {
		itemType = "MV"
	}

	for {
		req, err := http.NewRequest("GET", fmt.Sprintf("https://amp-api.music.apple.com/v1/catalog/%s/artists/%s/%s?limit=100&offset=%d&l=%s", storefront, artistId, relationship, offset, Config.Language), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		req.Header.Set("Origin", "https://music.apple.com")
		do, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer do.Body.Close()
		if do.StatusCode != http.StatusOK {
			return nil, errors.New(do.Status)
		}
		obj := new(structs.AutoGeneratedArtist)
		err = json.NewDecoder(do.Body).Decode(&obj)
		if err != nil {
			return nil, err
		}
		for _, item := range obj.Data {
			items = append(items, ArtistSelectItem{
				Type: itemType,
				Name: item.Attributes.Name,
				Date: item.Attributes.ReleaseDate,
				ID:   item.ID,
				URL:  item.Attributes.URL,
			})
		}
		offset += 100
		if len(obj.Next) == 0 {
			break
		}
	}
	return items, nil
}

func checkArtistCombined(artistUrl string, token string) ([]string, error) {
	albums, err := fetchArtistItems(artistUrl, token, "albums")
	if err != nil {
		return nil, err
	}
	mvs, err := fetchArtistItems(artistUrl, token, "music-videos")
	if err != nil {
		return nil, err
	}

	items := append(albums, mvs...)
	if len(items) == 0 {
		fmt.Println("No artist items found.")
		return nil, nil
	}

	sort.SliceStable(items, func(i, j int) bool {
		dateI, errI := time.Parse("2006-01-02", items[i].Date)
		dateJ, errJ := time.Parse("2006-01-02", items[j].Date)
		if errI == nil && errJ == nil && !dateI.Equal(dateJ) {
			return dateI.After(dateJ)
		}
		if items[i].Type != items[j].Type {
			return items[i].Type == "ALBUM"
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "TYPE", "TITLE", "DATE", "APPLE_ID"})
	table.SetRowLine(false)
	table.SetHeaderColor(tablewriter.Colors{},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.FgRedColor, tablewriter.Bold},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor})
	table.SetColumnColor(tablewriter.Colors{tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor})
	for i, item := range items {
		table.Append([]string{fmt.Sprint(i + 1), item.Type, item.Name, item.Date, item.ID})
	}
	table.Render()

	indexes, err := promptForSelectionIndexes("select", len(items))
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, len(indexes))
	for _, index := range indexes {
		item := items[index-1]
		fmt.Printf("%s: %s\n", item.Type, item.Name)
		args = append(args, item.URL)
	}
	return args, nil
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// START: New functions for search functionality

// SearchResultItem is a unified struct to hold search results for display.
type SearchResultItem struct {
	Type   string
	Name   string
	Detail string
	URL    string
	ID     string
}

// QualityOption holds information about a downloadable quality.
type QualityOption struct {
	ID          string
	Description string
}

// setDlFlags configures the global download flags based on the user's quality selection.
func setDlFlags(quality string) {
	dl_atmos = false
	dl_aac = false

	switch quality {
	case "atmos":
		dl_atmos = true
		fmt.Println("Quality set to: Dolby Atmos")
	case "aac":
		dl_aac = true
		*aac_type = "aac"
		fmt.Println("Quality set to: High-Quality (AAC)")
	case "alac":
		fmt.Println("Quality set to: Lossless (ALAC)")
	}
}

// promptForQuality asks the user to select a download quality for the chosen media.
func promptForQuality(item SearchResultItem, token string) (string, error) {
	if item.Type == "Artist" {
		fmt.Println("Artist selected. Proceeding to list all albums/videos.")
		return "default", nil
	}

	fmt.Printf("\nFetching available qualities for: %s\n", item.Name)

	qualities := []QualityOption{
		{ID: "alac", Description: "Lossless (ALAC)"},
		{ID: "aac", Description: "High-Quality (AAC)"},
		{ID: "atmos", Description: "Dolby Atmos"},
	}
	qualityOptions := []string{}
	for _, q := range qualities {
		qualityOptions = append(qualityOptions, q.Description)
	}

	prompt := &survey.Select{
		Message:  "Select a quality to download:",
		Options:  qualityOptions,
		PageSize: 5,
	}

	selectedIndex := 0
	err := survey.AskOne(prompt, &selectedIndex)
	if err != nil {
		// This can happen if the user presses Ctrl+C
		return "", nil
	}

	return qualities[selectedIndex].ID, nil
}

// handleSearch manages the entire interactive search process.
func handleSearch(searchType string, queryParts []string, token string) (string, error) {
	query := strings.Join(queryParts, " ")
	validTypes := map[string]bool{"album": true, "song": true, "artist": true}
	if !validTypes[searchType] {
		return "", fmt.Errorf("invalid search type: %s. Use 'album', 'song', or 'artist'", searchType)
	}

	fmt.Printf("Searching for %ss: \"%s\" in storefront \"%s\"\n", searchType, query, Config.Storefront)

	offset := 0
	limit := 15 // Increased limit for better navigation

	apiSearchType := searchType + "s"

	for {
		searchResp, err := ampapi.Search(Config.Storefront, query, apiSearchType, Config.Language, token, limit, offset)
		if err != nil {
			return "", fmt.Errorf("error fetching search results: %w", err)
		}

		var items []SearchResultItem
		var displayOptions []string
		hasNext := false

		// Special options for navigation
		const prevPageOpt = "⬅️  Previous Page"
		const nextPageOpt = "➡️  Next Page"

		// Add previous page option if applicable
		if offset > 0 {
			displayOptions = append(displayOptions, prevPageOpt)
		}

		switch searchType {
		case "album":
			if searchResp.Results.Albums != nil {
				for _, item := range searchResp.Results.Albums.Data {
					year := ""
					if len(item.Attributes.ReleaseDate) >= 4 {
						year = item.Attributes.ReleaseDate[:4]
					}
					trackInfo := fmt.Sprintf("%d tracks", item.Attributes.TrackCount)
					detail := fmt.Sprintf("%s (%s, %s)", item.Attributes.ArtistName, year, trackInfo)
					displayOptions = append(displayOptions, fmt.Sprintf("%s - %s", item.Attributes.Name, detail))
					items = append(items, SearchResultItem{Type: "Album", URL: item.Attributes.URL, ID: item.ID})
				}
				hasNext = searchResp.Results.Albums.Next != ""
			}
		case "song":
			if searchResp.Results.Songs != nil {
				for _, item := range searchResp.Results.Songs.Data {
					detail := fmt.Sprintf("%s (%s)", item.Attributes.ArtistName, item.Attributes.AlbumName)
					displayOptions = append(displayOptions, fmt.Sprintf("%s - %s", item.Attributes.Name, detail))
					items = append(items, SearchResultItem{Type: "Song", URL: item.Attributes.URL, ID: item.ID})
				}
				hasNext = searchResp.Results.Songs.Next != ""
			}
		case "artist":
			if searchResp.Results.Artists != nil {
				for _, item := range searchResp.Results.Artists.Data {
					detail := ""
					if len(item.Attributes.GenreNames) > 0 {
						detail = strings.Join(item.Attributes.GenreNames, ", ")
					}
					displayOptions = append(displayOptions, fmt.Sprintf("%s (%s)", item.Attributes.Name, detail))
					items = append(items, SearchResultItem{Type: "Artist", URL: item.Attributes.URL, ID: item.ID})
				}
				hasNext = searchResp.Results.Artists.Next != ""
			}
		}

		if len(items) == 0 && offset == 0 {
			fmt.Println("No results found.")
			return "", nil
		}

		// Add next page option if applicable
		if hasNext {
			displayOptions = append(displayOptions, nextPageOpt)
		}

		prompt := &survey.Select{
			Message:  "Use arrow keys to navigate, Enter to select:",
			Options:  displayOptions,
			PageSize: limit, // Show a full page of results
		}

		selectedIndex := 0
		err = survey.AskOne(prompt, &selectedIndex)
		if err != nil {
			// User pressed Ctrl+C
			return "", nil
		}

		selectedOption := displayOptions[selectedIndex]

		// Handle pagination
		if selectedOption == nextPageOpt {
			offset += limit
			continue
		}
		if selectedOption == prevPageOpt {
			offset -= limit
			continue
		}

		// Adjust index to match the `items` slice if "Previous Page" was an option
		itemIndex := selectedIndex
		if offset > 0 {
			itemIndex--
		}

		selectedItem := items[itemIndex]

		// Automatically set single song download flag
		if selectedItem.Type == "Song" {
			dl_song = true
		}

		quality, err := promptForQuality(selectedItem, token)
		if err != nil {
			return "", fmt.Errorf("could not process quality selection: %w", err)
		}
		if quality == "" { // User cancelled quality selection
			fmt.Println("Selection cancelled.")
			return "", nil
		}

		if quality != "default" {
			setDlFlags(quality)
		}

		return selectedItem.URL, nil
	}
}

// END: New functions for search functionality

func ripTrack(track *task.Track, token string, mediaUserToken string) {
	var err error
	session.Counter.Total++
	fmt.Printf("Track %d of %d: %s\n", track.TaskNum, track.TaskTotal, track.Type)

	//mv dl dev
	if track.Type == "music-videos" {
		if len(mediaUserToken) <= 50 {
			fmt.Println("meida-user-token is not set, skip MV dl")
			session.Counter.Success++
			return
		}
		if !toolAvailable(Config.MP4DecryptPath) {
			fmt.Printf("mp4decrypt not found at '%s', skip MV dl\n", Config.MP4DecryptPath)
			session.Counter.Success++
			return
		}
		err := mvDownloader(track.ID, track.SaveDir, token, track.Storefront, mediaUserToken, track, dl_select)
		if err != nil {
			fmt.Println("\u26A0 Failed to dl MV:", err)
			session.Counter.Error++
			return
		}
		session.Counter.Success++
		return
	}

	needDlAacLc := false
	if dl_aac && Config.AacType == "aac-lc" {
		needDlAacLc = true
	}
	if track.WebM3u8 == "" && !needDlAacLc {
		if dl_atmos {
			fmt.Println("Unavailable")
			session.Counter.Unavailable++
			return
		}
		fmt.Println("Unavailable, trying to dl aac-lc")
		needDlAacLc = true
	}
	needCheck := false

	if Config.GetM3u8Mode == "all" {
		needCheck = true
	} else if Config.GetM3u8Mode == "hires" && contains(track.Resp.Attributes.AudioTraits, "hi-res-lossless") {
		needCheck = true
	}
	var EnhancedHls_m3u8 string
	if needCheck && !needDlAacLc {
		EnhancedHls_m3u8, _ = checkM3u8(track.ID, "song")
		if strings.HasSuffix(EnhancedHls_m3u8, ".m3u8") {
			track.DeviceM3u8 = EnhancedHls_m3u8
			track.M3u8 = EnhancedHls_m3u8
		}
	}
	var Quality string
	if strings.Contains(Config.SongFileFormat, "Quality") {
		if selected_audio_format != nil {
			Quality = audioFormatQuality(*selected_audio_format)
		} else if dl_atmos {
			Quality = fmt.Sprintf("%dKbps", Config.AtmosMax-2000)
		} else if needDlAacLc {
			Quality = "256Kbps"
		} else {
			_, Quality, err = extractMedia(track.M3u8, true)
			if err != nil {
				fmt.Println("Failed to extract quality from manifest.\n", err)
				session.Counter.Error++
				return
			}
		}
	}
	track.Quality = Quality

	stringsToJoin := []string{}
	if track.Resp.Attributes.IsAppleDigitalMaster {
		if Config.AppleMasterChoice != "" {
			stringsToJoin = append(stringsToJoin, Config.AppleMasterChoice)
		}
	}
	if track.Resp.Attributes.ContentRating == "explicit" {
		if Config.ExplicitChoice != "" {
			stringsToJoin = append(stringsToJoin, Config.ExplicitChoice)
		}
	}
	if track.Resp.Attributes.ContentRating == "clean" {
		if Config.CleanChoice != "" {
			stringsToJoin = append(stringsToJoin, Config.CleanChoice)
		}
	}
	Tag_string := strings.Join(stringsToJoin, " ")

	songName := strings.NewReplacer(
		"{SongId}", track.ID,
		"{SongNumer}", fmt.Sprintf("%02d", track.TaskNum),
		"{ArtistName}", LimitString(track.Resp.Attributes.ArtistName),
		"{SongName}", LimitString(track.Resp.Attributes.Name),
		"{DiscNumber}", fmt.Sprintf("%0d", track.Resp.Attributes.DiscNumber),
		"{TrackNumber}", fmt.Sprintf("%0d", track.Resp.Attributes.TrackNumber),
		"{Quality}", Quality,
		"{Tag}", Tag_string,
		"{Codec}", track.Codec,
	).Replace(Config.SongFileFormat)
	fmt.Println(songName)
	filename := fmt.Sprintf("%s.m4a", forbiddenNames.ReplaceAllString(songName, "_"))
	track.SaveName = filename
	trackPath := filepath.Join(track.SaveDir, track.SaveName)
	lrcFilename := fmt.Sprintf("%s.%s", forbiddenNames.ReplaceAllString(songName, "_"), Config.LrcFormat)

	// Determine possible post-conversion target file (so we can skip re-download)
	var convertedPath string
	considerConverted := false
	if Config.ConvertAfterDownload &&
		Config.ConvertFormat != "" &&
		strings.ToLower(Config.ConvertFormat) != "copy" &&
		!Config.ConvertKeepOriginal {
		convertedPath = strings.TrimSuffix(trackPath, filepath.Ext(trackPath)) + "." + strings.ToLower(Config.ConvertFormat)
		considerConverted = true
	}
	// Existence check now considers converted output (if original was deleted)
	existsOriginal, err := fileExists(trackPath)
	if err != nil {
		fmt.Println("Failed to check if track exists.")
	}
	if existsOriginal {
		fmt.Println("Track already exists locally.")
		markTrackSuccess(track, trackPath)
		return
	}
	if considerConverted {
		existsConverted, err2 := fileExists(convertedPath)
		if err2 == nil && existsConverted {
			fmt.Println("Converted track already exists locally.")
			markTrackSuccess(track, convertedPath)
			return
		}
	}

	//提前获取到的播放列表下track所在的专辑信息
	if track.PreType == "playlists" && Config.UseSongInfoForPlaylist {
		track.GetAlbumData(token)
	}

	//get lrc
	var lrc string = ""
	if Config.EmbedLrc || Config.SaveLrcFile {
		lrcStr, err := lyrics.Get(track.Storefront, track.ID, Config.LrcType, Config.Language, Config.LrcFormat, token, mediaUserToken)
		if err != nil {
			fmt.Println(err)
		} else {
			if Config.SaveLrcFile {
				err := writeLyrics(track.SaveDir, lrcFilename, lrcStr)
				if err != nil {
					fmt.Printf("Failed to write lyrics")
				}
			}
			if Config.EmbedLrc {
				lrc = lrcStr
			}
		}
	}

	if needDlAacLc {
		if len(mediaUserToken) <= 50 {
			fmt.Println("Invalid media-user-token")
			session.Counter.Error++
			return
		}
		_, err := runv3.Run(track.ID, trackPath, token, mediaUserToken, false, "")
		if err != nil {
			fmt.Println("Failed to dl aac-lc:", err)
			if err.Error() == "Unavailable" {
				session.Counter.Unavailable++
				return
			}
			session.Counter.Error++
			return
		}
	} else {
		trackM3u8Url, _, err := extractMedia(track.M3u8, false)
		if err != nil {
			fmt.Println("\u26A0 Failed to extract info from manifest:", err)
			session.Counter.Unavailable++
			return
		}
		//边下载边解密
		err = runv2.Run(track.ID, trackM3u8Url, trackPath, Config)
		if err != nil {
			fmt.Println("Failed to run v2:", err)
			session.Counter.Error++
			return
		}
	}
	//这里利用MP4box将fmp4转化为mp4，并添加ilst box与cover，方便后面的mp4tag添加更多自定义标签
	tags := []string{
		"tool=",
		"artist=AppleMusic",
	}
	if Config.EmbedCover {
		if (strings.Contains(track.PreID, "pl.") || strings.Contains(track.PreID, "ra.")) && Config.DlAlbumcoverForPlaylist {
			track.CoverPath, err = writeCover(track.SaveDir, track.ID, track.Resp.Attributes.Artwork.URL)
			if err != nil {
				fmt.Println("Failed to write cover.")
			}
		}
		tags = append(tags, fmt.Sprintf("cover=%s", track.CoverPath))
	}
	tagsString := strings.Join(tags, ":")
	cmd := exec.Command(Config.MP4BoxPath, "-itags", tagsString, trackPath)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Embed failed: %v\n", err)
		session.Counter.Error++
		return
	}
	if (strings.Contains(track.PreID, "pl.") || strings.Contains(track.PreID, "ra.")) && Config.DlAlbumcoverForPlaylist {
		if err := os.Remove(track.CoverPath); err != nil {
			fmt.Printf("Error deleting file: %s\n", track.CoverPath)
			session.Counter.Error++
			return
		}
	}
	track.SavePath = trackPath

	if Config.ALACFix {
		err = alacfix.Run(track.SavePath, false)
		if err != nil {
			fmt.Println("\u26A0 Failed to fix ALAC:", err)
			session.Counter.Unavailable++
			return
		}
	}

	err = writeMP4Tags(track, lrc)
	if err != nil {
		fmt.Println("\u26A0 Failed to write tags in media:", err)
		session.Counter.Unavailable++
		return
	}

	// CONVERSION FEATURE hook
	convertIfNeeded(track)

	markTrackSuccess(track, track.SavePath)
}

func ripStation(albumId string, token string, storefront string, mediaUserToken string) error {
	station := task.NewStation(storefront, albumId)
	err := station.GetResp(mediaUserToken, token, Config.Language)
	if err != nil {
		return err
	}
	fmt.Println(" -", station.Type)
	meta := station.Resp

	Codec := currentCodec()
	station.Codec = Codec
	var singerFoldername string
	if Config.ArtistFolderFormat != "" {
		singerFoldername = strings.NewReplacer(
			"{ArtistName}", "Apple Music Station",
			"{ArtistId}", "",
			"{UrlArtistName}", "Apple Music Station",
		).Replace(Config.ArtistFolderFormat)
		singerFoldername = cleanFolderName(singerFoldername)
		fmt.Println(singerFoldername)
	}
	singerFolder := saveFolderForCurrentCodec(singerFoldername)
	os.MkdirAll(singerFolder, os.ModePerm)
	station.SaveDir = singerFolder

	playlistFolder := strings.NewReplacer(
		"{ArtistName}", "Apple Music Station",
		"{PlaylistName}", LimitString(station.Name),
		"{PlaylistId}", station.ID,
		"{Quality}", "",
		"{Codec}", Codec,
		"{Tag}", "",
	).Replace(Config.PlaylistFolderFormat)
	playlistFolder = cleanFolderName(playlistFolder)
	playlistFolderPath := filepath.Join(singerFolder, forbiddenNames.ReplaceAllString(playlistFolder, "_"))
	os.MkdirAll(playlistFolderPath, os.ModePerm)
	station.SaveName = playlistFolder
	fmt.Println(playlistFolder)

	covPath, err := writeCover(playlistFolderPath, "cover", meta.Data[0].Attributes.Artwork.URL)
	if err != nil {
		fmt.Println("Failed to write cover.")
	}
	station.CoverPath = covPath

	if Config.SaveAnimatedArtwork && meta.Data[0].Attributes.EditorialVideo.MotionSquare.Video != "" {
		fmt.Println("Found Animation Artwork.")

		motionvideoUrlSquare, err := extractVideo(meta.Data[0].Attributes.EditorialVideo.MotionSquare.Video)
		if err != nil {
			fmt.Println("no motion video square.\n", err)
		} else {
			exists, err := fileExists(filepath.Join(playlistFolderPath, "square_animated_artwork.mp4"))
			if err != nil {
				fmt.Println("Failed to check if animated artwork square exists.")
			}
			if exists {
				fmt.Println("Animated artwork square already exists locally.")
			} else {
				fmt.Println("Animation Artwork Square Downloading...")
				cmd := exec.Command(Config.FFmpegPath, "-loglevel", "quiet", "-y", "-i", motionvideoUrlSquare, "-c", "copy", filepath.Join(playlistFolderPath, "square_animated_artwork.mp4"))
				if err := cmd.Run(); err != nil {
					fmt.Printf("animated artwork square dl err: %v\n", err)
				} else {
					fmt.Println("Animation Artwork Square Downloaded")
				}
			}
		}

		if Config.EmbyAnimatedArtwork {
			cmd3 := exec.Command(Config.FFmpegPath, "-i", filepath.Join(playlistFolderPath, "square_animated_artwork.mp4"), "-vf", "scale=440:-1", "-r", "24", "-f", "gif", filepath.Join(playlistFolderPath, "folder.jpg"))
			if err := cmd3.Run(); err != nil {
				fmt.Printf("animated artwork square to gif err: %v\n", err)
			}
		}
	}
	if station.Type == "stream" {
		session.Counter.Total++
		if isInArray(session.OKDict[station.ID], 1) {
			session.Counter.Success++
			return nil
		}
		songName := strings.NewReplacer(
			"{SongId}", station.ID,
			"{SongNumer}", "01",
			"{SongName}", LimitString(station.Name),
			"{DiscNumber}", "1",
			"{TrackNumber}", "1",
			"{Quality}", "256Kbps",
			"{Tag}", "",
			"{Codec}", "AAC",
		).Replace(Config.SongFileFormat)
		fmt.Println(songName)
		trackPath := filepath.Join(playlistFolderPath, fmt.Sprintf("%s.m4a", forbiddenNames.ReplaceAllString(songName, "_")))
		exists, _ := fileExists(trackPath)
		if exists {
			session.Counter.Success++
			session.OKDict[station.ID] = append(session.OKDict[station.ID], 1)

			fmt.Println("Radio already exists locally.")
			session.AddedTracks = append(session.AddedTracks, AddedTrack{
				Path:     trackPath,
				Artist:   "Apple Music Station",
				ArtistID: "",
				Album:    station.Name,
				Song:     station.Name,
			})
			return nil
		}
		assetsUrl, serverUrl, err := ampapi.GetStationAssetsUrlAndServerUrl(station.ID, mediaUserToken, token)
		if err != nil {
			fmt.Println("Failed to get station assets url.", err)
			session.Counter.Error++
			return err
		}
		trackM3U8 := strings.ReplaceAll(assetsUrl, "index.m3u8", "256/prog_index.m3u8")
		keyAndUrls, _ := runv3.Run(station.ID, trackM3U8, token, mediaUserToken, true, serverUrl)
		err = runv3.ExtMvData(keyAndUrls, trackPath, Config.MP4DecryptPath)
		if err != nil {
			fmt.Println("Failed to download station stream.", err)
			session.Counter.Error++
			return err
		}
		tags := []string{
			"tool=",
			"disk=1/1",
			"track=1",
			"tracknum=1/1",
			fmt.Sprintf("artist=%s", "Apple Music Station"),
			fmt.Sprintf("performer=%s", "Apple Music Station"),
			fmt.Sprintf("album_artist=%s", "Apple Music Station"),
			fmt.Sprintf("album=%s", station.Name),
			fmt.Sprintf("title=%s", station.Name),
		}
		if Config.EmbedCover {
			tags = append(tags, fmt.Sprintf("cover=%s", station.CoverPath))
		}
		tagsString := strings.Join(tags, ":")
		cmd := exec.Command(Config.MP4BoxPath, "-itags", tagsString, trackPath)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Embed failed: %v\n", err)
		}
		session.AddedTracks = append(session.AddedTracks, AddedTrack{
			Path:     trackPath,
			Artist:   "Apple Music Station",
			ArtistID: "",
			Album:    station.Name,
			Song:     station.Name,
		})
		session.Counter.Success++
		session.OKDict[station.ID] = append(session.OKDict[station.ID], 1)
		return nil
	}

	for i := range station.Tracks {
		station.Tracks[i].CoverPath = covPath
		station.Tracks[i].SaveDir = playlistFolderPath
		station.Tracks[i].Codec = Codec
	}

	trackTotal := len(station.Tracks)
	arr := allIndexes(trackTotal)
	selected := arr
	startIdx := len(session.AddedTracks)
	for i := range station.Tracks {
		i++
		if isInArray(selected, i) {
			ripTrack(&station.Tracks[i-1], token, mediaUserToken)
		}
	}
	if len(session.AddedTracks) > startIdx {
		if err := writeM3UPlaylist(playlistFolderPath, playlistFolder, session.AddedTracks[startIdx:]); err != nil {
			fmt.Printf("Failed to write M3U8 playlist: %v\n", err)
		}
	}
	return nil
}

func ripAlbum(albumId string, token string, storefront string, mediaUserToken string, urlArg_i string) error {
	album := task.NewAlbum(storefront, albumId)
	err := album.GetResp(token, Config.Language)
	if err != nil {
		fmt.Println("Failed to get album response.")
		return err
	}
	meta := album.Resp
	if debug_mode {
		fmt.Println(meta.Data[0].Attributes.ArtistName)
		fmt.Println(meta.Data[0].Attributes.Name)

		for trackNum, track := range meta.Data[0].Relationships.Tracks.Data {
			trackNum++
			fmt.Printf("\nTrack %d of %d:\n", trackNum, len(meta.Data[0].Relationships.Tracks.Data))
			fmt.Printf("%02d. %s\n", trackNum, track.Attributes.Name)

			manifest, err := ampapi.GetSongResp(storefront, track.ID, album.Language, token)
			if err != nil {
				fmt.Printf("Failed to get manifest for track %d: %v\n", trackNum, err)
				continue
			}

			var m3u8Url string
			if manifest.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls != "" {
				m3u8Url = manifest.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls
			}
			needCheck := false
			if Config.GetM3u8Mode == "all" {
				needCheck = true
			} else if Config.GetM3u8Mode == "hires" && contains(track.Attributes.AudioTraits, "hi-res-lossless") {
				needCheck = true
			}
			if needCheck {
				fullM3u8Url, err := checkM3u8(track.ID, "song")
				if err == nil && strings.HasSuffix(fullM3u8Url, ".m3u8") {
					m3u8Url = fullM3u8Url
				} else {
					fmt.Println("Failed to get best quality m3u8 from device m3u8 port, will use m3u8 from Web API")
				}
			}

			_, _, err = extractMedia(m3u8Url, true)
			if err != nil {
				fmt.Printf("Failed to extract quality info for track %d: %v\n", trackNum, err)
				continue
			}
		}
		return nil
	}
	trackTotal := len(meta.Data[0].Relationships.Tracks.Data)
	arr := allIndexes(trackTotal)
	var selected []int
	if dl_song || (dl_select && urlArg_i != "") {
		if urlArg_i == "" {
			return nil
		}
		for i := range album.Tracks {
			if urlArg_i == album.Tracks[i].ID {
				selected = []int{i + 1}
				break
			}
		}
		if len(selected) == 0 {
			return fmt.Errorf("song %s not found in album", urlArg_i)
		}
		if dl_select && selected_audio_format == nil {
			if err := selectSingleSongAudioFormat(urlArg_i, token, storefront); err != nil {
				return err
			}
		}
	} else if dl_select {
		fmt.Println(meta.Data[0].Attributes.ArtistName)
		fmt.Println(meta.Data[0].Attributes.Name)
		selected = album.ShowSelect()
		if len(selected) == 0 {
			fmt.Println("No tracks selected.")
			return nil
		}
	} else {
		selected = arr
	}
	Codec := currentCodec()
	album.Codec = Codec
	var singerFoldername string
	if Config.ArtistFolderFormat != "" {
		if len(meta.Data[0].Relationships.Artists.Data) > 0 {
			singerFoldername = strings.NewReplacer(
				"{UrlArtistName}", LimitString(meta.Data[0].Attributes.ArtistName),
				"{ArtistName}", LimitString(meta.Data[0].Attributes.ArtistName),
				"{ArtistId}", meta.Data[0].Relationships.Artists.Data[0].ID,
			).Replace(Config.ArtistFolderFormat)
		} else {
			singerFoldername = strings.NewReplacer(
				"{UrlArtistName}", LimitString(meta.Data[0].Attributes.ArtistName),
				"{ArtistName}", LimitString(meta.Data[0].Attributes.ArtistName),
				"{ArtistId}", "",
			).Replace(Config.ArtistFolderFormat)
		}
		singerFoldername = cleanFolderName(singerFoldername)
		fmt.Println(singerFoldername)
	}
	singerFolder := saveFolderForCurrentCodec(singerFoldername)
	os.MkdirAll(singerFolder, os.ModePerm)
	album.SaveDir = singerFolder
	var Quality string
	if strings.Contains(Config.AlbumFolderFormat, "Quality") {
		if selected_audio_format != nil {
			Quality = audioFormatQuality(*selected_audio_format)
		} else if dl_atmos {
			Quality = fmt.Sprintf("%dKbps", Config.AtmosMax-2000)
		} else if dl_aac && Config.AacType == "aac-lc" {
			Quality = "256Kbps"
		} else {
			manifest1, err := ampapi.GetSongResp(storefront, meta.Data[0].Relationships.Tracks.Data[0].ID, album.Language, token)
			if err != nil {
				fmt.Println("Failed to get manifest.\n", err)
			} else {
				if manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls == "" {
					Codec = "AAC"
					Quality = "256Kbps"
				} else {
					needCheck := false

					if Config.GetM3u8Mode == "all" {
						needCheck = true
					} else if Config.GetM3u8Mode == "hires" && contains(meta.Data[0].Relationships.Tracks.Data[0].Attributes.AudioTraits, "hi-res-lossless") {
						needCheck = true
					}
					var EnhancedHls_m3u8 string
					if needCheck {
						EnhancedHls_m3u8, _ = checkM3u8(meta.Data[0].Relationships.Tracks.Data[0].ID, "album")
						if strings.HasSuffix(EnhancedHls_m3u8, ".m3u8") {
							manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls = EnhancedHls_m3u8
						}
					}
					_, Quality, err = extractMedia(manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls, true)
					if err != nil {
						fmt.Println("Failed to extract quality from manifest.\n", err)
					}
				}
			}
		}
	}
	stringsToJoin := []string{}
	if meta.Data[0].Attributes.IsAppleDigitalMaster || meta.Data[0].Attributes.IsMasteredForItunes {
		if Config.AppleMasterChoice != "" {
			stringsToJoin = append(stringsToJoin, Config.AppleMasterChoice)
		}
	}
	if meta.Data[0].Attributes.ContentRating == "explicit" {
		if Config.ExplicitChoice != "" {
			stringsToJoin = append(stringsToJoin, Config.ExplicitChoice)
		}
	}
	if meta.Data[0].Attributes.ContentRating == "clean" {
		if Config.CleanChoice != "" {
			stringsToJoin = append(stringsToJoin, Config.CleanChoice)
		}
	}
	Tag_string := strings.Join(stringsToJoin, " ")
	var albumFolderName string
	albumFolderName = strings.NewReplacer(
		"{ReleaseDate}", meta.Data[0].Attributes.ReleaseDate,
		"{ReleaseYear}", releaseYear(meta.Data[0].Attributes.ReleaseDate),
		"{ArtistName}", LimitString(meta.Data[0].Attributes.ArtistName),
		"{AlbumName}", LimitString(meta.Data[0].Attributes.Name),
		"{UPC}", meta.Data[0].Attributes.Upc,
		"{RecordLabel}", meta.Data[0].Attributes.RecordLabel,
		"{Copyright}", meta.Data[0].Attributes.Copyright,
		"{AlbumId}", albumId,
		"{Quality}", Quality,
		"{Codec}", Codec,
		"{Tag}", Tag_string,
	).Replace(Config.AlbumFolderFormat)

	albumFolderName = cleanFolderName(albumFolderName)
	albumFolderPath := filepath.Join(singerFolder, forbiddenNames.ReplaceAllString(albumFolderName, "_"))
	os.MkdirAll(albumFolderPath, os.ModePerm)
	album.SaveName = albumFolderName
	fmt.Println(albumFolderName)
	if Config.SaveArtistCover && len(meta.Data[0].Relationships.Artists.Data) > 0 {
		if meta.Data[0].Relationships.Artists.Data[0].Attributes.Artwork.Url != "" {
			_, err = writeCover(singerFolder, "folder", meta.Data[0].Relationships.Artists.Data[0].Attributes.Artwork.Url)
			if err != nil {
				fmt.Println("Failed to write artist cover.")
			}
		}
	}
	covPath, err := writeCover(albumFolderPath, "cover", meta.Data[0].Attributes.Artwork.URL)
	if err != nil {
		fmt.Println("Failed to write cover.")
	}
	if Config.SaveAnimatedArtwork && meta.Data[0].Attributes.EditorialVideo.MotionDetailSquare.Video != "" {
		fmt.Println("Found Animation Artwork.")

		motionvideoUrlSquare, err := extractVideo(meta.Data[0].Attributes.EditorialVideo.MotionDetailSquare.Video)
		if err != nil {
			fmt.Println("no motion video square.\n", err)
		} else {
			exists, err := fileExists(filepath.Join(albumFolderPath, "square_animated_artwork.mp4"))
			if err != nil {
				fmt.Println("Failed to check if animated artwork square exists.")
			}
			if exists {
				fmt.Println("Animated artwork square already exists locally.")
			} else {
				fmt.Println("Animation Artwork Square Downloading...")
				cmd := exec.Command(Config.FFmpegPath, "-loglevel", "quiet", "-y", "-i", motionvideoUrlSquare, "-c", "copy", filepath.Join(albumFolderPath, "square_animated_artwork.mp4"))
				if err := cmd.Run(); err != nil {
					fmt.Printf("animated artwork square dl err: %v\n", err)
				} else {
					fmt.Println("Animation Artwork Square Downloaded")
				}
			}
		}

		if Config.EmbyAnimatedArtwork {
			cmd3 := exec.Command(Config.FFmpegPath, "-i", filepath.Join(albumFolderPath, "square_animated_artwork.mp4"), "-vf", "scale=440:-1", "-r", "24", "-f", "gif", filepath.Join(albumFolderPath, "folder.jpg"))
			if err := cmd3.Run(); err != nil {
				fmt.Printf("animated artwork square to gif err: %v\n", err)
			}
		}

		motionvideoUrlTall, err := extractVideo(meta.Data[0].Attributes.EditorialVideo.MotionDetailTall.Video)
		if err != nil {
			fmt.Println("no motion video tall.\n", err)
		} else {
			exists, err := fileExists(filepath.Join(albumFolderPath, "tall_animated_artwork.mp4"))
			if err != nil {
				fmt.Println("Failed to check if animated artwork tall exists.")
			}
			if exists {
				fmt.Println("Animated artwork tall already exists locally.")
			} else {
				fmt.Println("Animation Artwork Tall Downloading...")
				cmd := exec.Command(Config.FFmpegPath, "-loglevel", "quiet", "-y", "-i", motionvideoUrlTall, "-c", "copy", filepath.Join(albumFolderPath, "tall_animated_artwork.mp4"))
				if err := cmd.Run(); err != nil {
					fmt.Printf("animated artwork tall dl err: %v\n", err)
				} else {
					fmt.Println("Animation Artwork Tall Downloaded")
				}
			}
		}
	}
	for i := range album.Tracks {
		album.Tracks[i].CoverPath = covPath
		album.Tracks[i].SaveDir = albumFolderPath
		album.Tracks[i].Codec = Codec
	}
	startIdx := len(session.AddedTracks)
	for i := range album.Tracks {
		i++
		if isInArray(session.OKDict[albumId], i) {
			session.Counter.Total++
			session.Counter.Success++
			continue
		}
		if isInArray(selected, i) {
			ripTrack(&album.Tracks[i-1], token, mediaUserToken)
		}
	}
	if len(session.AddedTracks) > startIdx {
		if err := writeM3UPlaylist(albumFolderPath, albumFolderName, session.AddedTracks[startIdx:]); err != nil {
			fmt.Printf("Failed to write M3U8 playlist: %v\n", err)
		}
	}
	return nil

}
func ripPlaylist(playlistId string, token string, storefront string, mediaUserToken string) error {
	playlist := task.NewPlaylist(storefront, playlistId)
	err := playlist.GetResp(token, Config.Language)
	if err != nil {
		fmt.Println("Failed to get playlist response.")
		return err
	}
	meta := playlist.Resp
	if debug_mode {
		fmt.Println(meta.Data[0].Attributes.ArtistName)
		fmt.Println(meta.Data[0].Attributes.Name)

		for trackNum, track := range meta.Data[0].Relationships.Tracks.Data {
			trackNum++
			fmt.Printf("\nTrack %d of %d:\n", trackNum, len(meta.Data[0].Relationships.Tracks.Data))
			fmt.Printf("%02d. %s\n", trackNum, track.Attributes.Name)

			manifest, err := ampapi.GetSongResp(storefront, track.ID, playlist.Language, token)
			if err != nil {
				fmt.Printf("Failed to get manifest for track %d: %v\n", trackNum, err)
				continue
			}

			var m3u8Url string
			if manifest.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls != "" {
				m3u8Url = manifest.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls
			}
			needCheck := false
			if Config.GetM3u8Mode == "all" {
				needCheck = true
			} else if Config.GetM3u8Mode == "hires" && contains(track.Attributes.AudioTraits, "hi-res-lossless") {
				needCheck = true
			}
			if needCheck {
				fullM3u8Url, err := checkM3u8(track.ID, "song")
				if err == nil && strings.HasSuffix(fullM3u8Url, ".m3u8") {
					m3u8Url = fullM3u8Url
				} else {
					fmt.Println("Failed to get best quality m3u8 from device m3u8 port, will use m3u8 from Web API")
				}
			}

			_, _, err = extractMedia(m3u8Url, true)
			if err != nil {
				fmt.Printf("Failed to extract quality info for track %d: %v\n", trackNum, err)
				continue
			}
		}
		return nil
	}
	Codec := currentCodec()
	playlist.Codec = Codec
	var singerFoldername string
	if Config.ArtistFolderFormat != "" {
		singerFoldername = strings.NewReplacer(
			"{ArtistName}", "Apple Music",
			"{ArtistId}", "",
			"{UrlArtistName}", "Apple Music",
		).Replace(Config.ArtistFolderFormat)
		singerFoldername = cleanFolderName(singerFoldername)
		fmt.Println(singerFoldername)
	}
	singerFolder := saveFolderForCurrentCodec(singerFoldername)
	os.MkdirAll(singerFolder, os.ModePerm)
	playlist.SaveDir = singerFolder

	var Quality string
	if strings.Contains(Config.AlbumFolderFormat, "Quality") {
		if selected_audio_format != nil {
			Quality = audioFormatQuality(*selected_audio_format)
		} else if dl_atmos {
			Quality = fmt.Sprintf("%dKbps", Config.AtmosMax-2000)
		} else if dl_aac && Config.AacType == "aac-lc" {
			Quality = "256Kbps"
		} else {
			manifest1, err := ampapi.GetSongResp(storefront, meta.Data[0].Relationships.Tracks.Data[0].ID, playlist.Language, token)
			if err != nil {
				fmt.Println("Failed to get manifest.\n", err)
			} else {
				if manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls == "" {
					Codec = "AAC"
					Quality = "256Kbps"
				} else {
					needCheck := false

					if Config.GetM3u8Mode == "all" {
						needCheck = true
					} else if Config.GetM3u8Mode == "hires" && contains(meta.Data[0].Relationships.Tracks.Data[0].Attributes.AudioTraits, "hi-res-lossless") {
						needCheck = true
					}
					var EnhancedHls_m3u8 string
					if needCheck {
						EnhancedHls_m3u8, _ = checkM3u8(meta.Data[0].Relationships.Tracks.Data[0].ID, "album")
						if strings.HasSuffix(EnhancedHls_m3u8, ".m3u8") {
							manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls = EnhancedHls_m3u8
						}
					}
					_, Quality, err = extractMedia(manifest1.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls, true)
					if err != nil {
						fmt.Println("Failed to extract quality from manifest.\n", err)
					}
				}
			}
		}
	}
	stringsToJoin := []string{}
	if meta.Data[0].Attributes.IsAppleDigitalMaster || meta.Data[0].Attributes.IsMasteredForItunes {
		if Config.AppleMasterChoice != "" {
			stringsToJoin = append(stringsToJoin, Config.AppleMasterChoice)
		}
	}
	if meta.Data[0].Attributes.ContentRating == "explicit" {
		if Config.ExplicitChoice != "" {
			stringsToJoin = append(stringsToJoin, Config.ExplicitChoice)
		}
	}
	if meta.Data[0].Attributes.ContentRating == "clean" {
		if Config.CleanChoice != "" {
			stringsToJoin = append(stringsToJoin, Config.CleanChoice)
		}
	}
	Tag_string := strings.Join(stringsToJoin, " ")
	playlistFolder := strings.NewReplacer(
		"{ArtistName}", "Apple Music",
		"{PlaylistName}", LimitString(meta.Data[0].Attributes.Name),
		"{PlaylistId}", playlistId,
		"{Quality}", Quality,
		"{Codec}", Codec,
		"{Tag}", Tag_string,
	).Replace(Config.PlaylistFolderFormat)
	playlistFolder = cleanFolderName(playlistFolder)
	playlistFolderPath := filepath.Join(singerFolder, forbiddenNames.ReplaceAllString(playlistFolder, "_"))
	os.MkdirAll(playlistFolderPath, os.ModePerm)
	playlist.SaveName = playlistFolder
	fmt.Println(playlistFolder)
	trackTotal := len(meta.Data[0].Relationships.Tracks.Data)
	arr := allIndexes(trackTotal)
	var selected []int

	if !dl_select {
		selected = arr
	} else {
		selected = playlist.ShowSelect()
		if len(selected) == 0 {
			fmt.Println("No tracks selected.")
			return nil
		}
	}
	covPath, err := writeCover(playlistFolderPath, "cover", meta.Data[0].Attributes.Artwork.URL)
	if err != nil {
		fmt.Println("Failed to write cover.")
	}

	for i := range playlist.Tracks {
		playlist.Tracks[i].CoverPath = covPath
		playlist.Tracks[i].SaveDir = playlistFolderPath
		playlist.Tracks[i].Codec = Codec
	}

	if Config.SaveAnimatedArtwork && meta.Data[0].Attributes.EditorialVideo.MotionDetailSquare.Video != "" {
		fmt.Println("Found Animation Artwork.")

		motionvideoUrlSquare, err := extractVideo(meta.Data[0].Attributes.EditorialVideo.MotionDetailSquare.Video)
		if err != nil {
			fmt.Println("no motion video square.\n", err)
		} else {
			exists, err := fileExists(filepath.Join(playlistFolderPath, "square_animated_artwork.mp4"))
			if err != nil {
				fmt.Println("Failed to check if animated artwork square exists.")
			}
			if exists {
				fmt.Println("Animated artwork square already exists locally.")
			} else {
				fmt.Println("Animation Artwork Square Downloading...")
				cmd := exec.Command(Config.FFmpegPath, "-loglevel", "quiet", "-y", "-i", motionvideoUrlSquare, "-c", "copy", filepath.Join(playlistFolderPath, "square_animated_artwork.mp4"))
				if err := cmd.Run(); err != nil {
					fmt.Printf("animated artwork square dl err: %v\n", err)
				} else {
					fmt.Println("Animation Artwork Square Downloaded")
				}
			}
		}

		if Config.EmbyAnimatedArtwork {
			cmd3 := exec.Command(Config.FFmpegPath, "-i", filepath.Join(playlistFolderPath, "square_animated_artwork.mp4"), "-vf", "scale=440:-1", "-r", "24", "-f", "gif", filepath.Join(playlistFolderPath, "folder.jpg"))
			if err := cmd3.Run(); err != nil {
				fmt.Printf("animated artwork square to gif err: %v\n", err)
			}
		}

		motionvideoUrlTall, err := extractVideo(meta.Data[0].Attributes.EditorialVideo.MotionDetailTall.Video)
		if err != nil {
			fmt.Println("no motion video tall.\n", err)
		} else {
			exists, err := fileExists(filepath.Join(playlistFolderPath, "tall_animated_artwork.mp4"))
			if err != nil {
				fmt.Println("Failed to check if animated artwork tall exists.")
			}
			if exists {
				fmt.Println("Animated artwork tall already exists locally.")
			} else {
				fmt.Println("Animation Artwork Tall Downloading...")
				cmd := exec.Command(Config.FFmpegPath, "-loglevel", "quiet", "-y", "-i", motionvideoUrlTall, "-c", "copy", filepath.Join(playlistFolderPath, "tall_animated_artwork.mp4"))
				if err := cmd.Run(); err != nil {
					fmt.Printf("animated artwork tall dl err: %v\n", err)
				} else {
					fmt.Println("Animation Artwork Tall Downloaded")
				}
			}
		}
	}
	startIdx := len(session.AddedTracks)
	for i := range playlist.Tracks {
		i++
		if isInArray(session.OKDict[playlistId], i) {
			session.Counter.Total++
			session.Counter.Success++
			continue
		}
		if isInArray(selected, i) {
			ripTrack(&playlist.Tracks[i-1], token, mediaUserToken)
		}
	}
	if len(session.AddedTracks) > startIdx {
		if err := writeM3UPlaylist(playlistFolderPath, playlistFolder, session.AddedTracks[startIdx:]); err != nil {
			fmt.Printf("Failed to write M3U8 playlist: %v\n", err)
		}
	}
	return nil
}

func writeM3UPlaylist(folderPath string, name string, tracks []AddedTrack) error {
	if save_m3u8_playlist == false {
		return nil
	}
	m3uPath := filepath.Join(folderPath, forbiddenNames.ReplaceAllString(name, "_")+".m3u8")
	f, err := os.Create(m3uPath)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintln(f, "#EXTM3U")
	for _, track := range tracks {
		fmt.Fprintf(f, "#EXTINF:-1,%s - %s\n", track.Artist, track.Song)
		fmt.Fprintln(f, filepath.Base(track.Path))
	}
	return nil
}

func writeMP4Tags(track *task.Track, lrc string) error {
	t := &mp4tag.MP4Tags{
		Title:  track.Resp.Attributes.Name,
		Artist: track.Resp.Attributes.ArtistName,
		Custom: map[string]string{
			"PERFORMER":   track.Resp.Attributes.ArtistName,
			"RELEASETIME": track.Resp.Attributes.ReleaseDate,
			"ISRC":        track.Resp.Attributes.Isrc,
			"LABEL":       "",
			"UPC":         "",
		},
		Composer:    track.Resp.Attributes.ComposerName,
		CustomGenre: firstString(track.Resp.Attributes.GenreNames),
		Lyrics:      lrc,
		TrackNumber: int16(track.Resp.Attributes.TrackNumber),
		DiscNumber:  int16(track.Resp.Attributes.DiscNumber),
		Album:       track.Resp.Attributes.AlbumName,
	}

	if Config.TagSortOrder {
		t.TitleSort = track.Resp.Attributes.Name
		t.ArtistSort = track.Resp.Attributes.ArtistName
		t.ComposerSort = track.Resp.Attributes.ComposerName
		t.AlbumSort = track.Resp.Attributes.AlbumName
	}

	if Config.TagItunesID {
		if track.PreType == "albums" {
			albumID, err := strconv.ParseUint(track.PreID, 10, 64)
			if err != nil {
				return err
			}
			t.ItunesAlbumID = int32(albumID)
		}

		if len(track.Resp.Relationships.Artists.Data) > 0 {
			artistID, err := strconv.ParseUint(track.Resp.Relationships.Artists.Data[0].ID, 10, 64)
			if err != nil {
				return err
			}
			t.ItunesArtistID = int32(artistID)
		}
	}

	if (track.PreType == "playlists" || track.PreType == "stations") && !Config.UseSongInfoForPlaylist {
		t.DiscNumber = 1
		t.DiscTotal = 1
		t.TrackNumber = int16(track.TaskNum)
		t.TrackTotal = int16(track.TaskTotal)
		t.Album = track.PlaylistData.Attributes.Name
		t.AlbumArtist = track.PlaylistData.Attributes.ArtistName
		if Config.TagSortOrder {
			t.AlbumSort = track.PlaylistData.Attributes.Name
			t.AlbumArtistSort = track.PlaylistData.Attributes.ArtistName
		}
	} else if (track.PreType == "playlists" || track.PreType == "stations") && Config.UseSongInfoForPlaylist {
		t.DiscTotal = int16(track.DiscTotal)
		t.TrackTotal = int16(track.AlbumData.Attributes.TrackCount)
		t.AlbumArtist = track.AlbumData.Attributes.ArtistName
		t.Custom["UPC"] = track.AlbumData.Attributes.Upc
		t.Custom["LABEL"] = track.AlbumData.Attributes.RecordLabel
		t.Date = track.AlbumData.Attributes.ReleaseDate
		t.Copyright = track.AlbumData.Attributes.Copyright
		t.Publisher = track.AlbumData.Attributes.RecordLabel
		if Config.TagSortOrder {
			t.AlbumArtistSort = track.AlbumData.Attributes.ArtistName
		}
	} else {
		t.DiscTotal = int16(track.DiscTotal)
		t.TrackTotal = int16(track.AlbumData.Attributes.TrackCount)
		t.AlbumArtist = track.AlbumData.Attributes.ArtistName
		t.Custom["UPC"] = track.AlbumData.Attributes.Upc
		t.Date = track.AlbumData.Attributes.ReleaseDate
		t.Copyright = track.AlbumData.Attributes.Copyright
		t.Publisher = track.AlbumData.Attributes.RecordLabel
		if Config.TagSortOrder {
			t.AlbumArtistSort = track.AlbumData.Attributes.ArtistName
		}
	}

	if track.Resp.Attributes.ContentRating == "explicit" {
		t.ItunesAdvisory = mp4tag.ItunesAdvisoryExplicit
	} else if track.Resp.Attributes.ContentRating == "clean" {
		t.ItunesAdvisory = mp4tag.ItunesAdvisoryClean
	} else {
		t.ItunesAdvisory = mp4tag.ItunesAdvisoryNone
	}

	mp4, err := mp4tag.Open(track.SavePath)
	if err != nil {
		return err
	}
	defer mp4.Close()
	err = mp4.Write(t, []string{})
	if err != nil {
		return err
	}
	return nil
}

func main() {
	err := loadConfig()
	if err != nil {
		fmt.Printf("load Config failed: %v", err)
		return
	}
	token, err := ampapi.GetToken()
	if err != nil {
		if Config.AuthorizationToken != "" && Config.AuthorizationToken != "your-authorization-token" {
			token = strings.Replace(Config.AuthorizationToken, "Bearer ", "", -1)
		} else {
			fmt.Println("Failed to get token.")
			return
		}
	}
	var search_type string
	pflag.StringVar(&search_type, "search", "", "Search for 'album', 'song', or 'artist'. Provide query after flags.")
	pflag.BoolVar(&dl_atmos, "atmos", false, "Enable atmos download mode")
	pflag.BoolVar(&dl_aac, "aac", false, "Enable adm-aac download mode")
	pflag.BoolVar(&dl_select, "select", false, "Enable selective download")
	pflag.BoolVar(&dl_song, "song", false, "Enable single song download mode")
	pflag.BoolVar(&artist_select, "all-album", false, "Download all artist albums")
	pflag.BoolVar(&debug_mode, "debug", false, "Enable debug mode to show audio quality information")
	pflag.BoolVar(&print_json, "json", false, "Output JSON summary at the end")
	pflag.BoolVar(&save_m3u8_playlist, "save-m3u8-playlist", false, "Save M3U8 playlist file")
	alac_max = pflag.Int("alac-max", Config.AlacMax, "Specify the max quality for download alac")
	atmos_max = pflag.Int("atmos-max", Config.AtmosMax, "Specify the max quality for download atmos")
	aac_type = pflag.String("aac-type", Config.AacType, "Select AAC type, aac aac-binaural aac-downmix")
	mv_audio_type = pflag.String("mv-audio-type", Config.MVAudioType, "Select MV audio type, atmos ac3 aac")
	mv_max = pflag.Int("mv-max", Config.MVMax, "Specify the max quality for download MV")

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [url1 url2 ...]\n", "[main | main.exe | go run .]")
		fmt.Fprintf(os.Stderr, "Search Usage: %s --search [album|song|artist] [query]\n", "[main | main.exe | go run .]")
		fmt.Println("\nOptions:")
		pflag.PrintDefaults()
	}

	pflag.Parse()
	Config.AlacMax = *alac_max
	Config.AtmosMax = *atmos_max
	Config.AacType = *aac_type
	Config.MVAudioType = *mv_audio_type
	Config.MVMax = *mv_max

	args := pflag.Args()

	if search_type != "" {
		if len(args) == 0 {
			fmt.Println("Error: --search flag requires a query.")
			pflag.Usage()
			return
		}
		selectedUrl, err := handleSearch(search_type, args, token)
		if err != nil {
			fmt.Printf("\nSearch process failed: %v\n", err)
			return
		}
		if selectedUrl == "" {
			fmt.Println("\nExiting.")
			return
		}
		os.Args = []string{selectedUrl}
	} else {
		if len(args) == 0 {
			fmt.Println("No URLs provided. Please provide at least one URL.")
			pflag.Usage()
			return
		}
		os.Args = args
	}

	if strings.Contains(os.Args[0], "/artist/") {
		urlArtistName, urlArtistID, err := getUrlArtistName(os.Args[0], token)
		if err != nil {
			fmt.Println("Failed to get artistname.")
			return
		}
		Config.ArtistFolderFormat = strings.NewReplacer(
			"{UrlArtistName}", LimitString(urlArtistName),
			"{ArtistId}", urlArtistID,
		).Replace(Config.ArtistFolderFormat)
		if dl_select {
			artistArgs, err := checkArtistCombined(os.Args[0], token)
			if err != nil {
				fmt.Println("Failed to get artist items.")
				return
			}
			os.Args = artistArgs
		} else {
			albumArgs, err := checkArtist(os.Args[0], token, "albums")
			if err != nil {
				fmt.Println("Failed to get artist albums.")
				return
			}
			mvArgs, err := checkArtist(os.Args[0], token, "music-videos")
			if err != nil {
				fmt.Println("Failed to get artist music-videos.")
			}
			os.Args = append(albumArgs, mvArgs...)
		}
	}
	if len(os.Args) == 0 {
		fmt.Println("No items selected.")
		return
	}
	albumTotal := len(os.Args)
	for {
		for albumNum, urlRaw := range os.Args {
			selected_audio_format = nil
			fmt.Printf("Queue %d of %d: ", albumNum+1, albumTotal)
			var storefront, albumId string

			if strings.Contains(urlRaw, "/music-video/") {
				fmt.Println("Music Video")
				if debug_mode {
					continue
				}
				session.Counter.Total++
				if len(Config.MediaUserToken) <= 50 {
					fmt.Println(": meida-user-token is not set, skip MV dl")
					session.Counter.Success++
					continue
				}
				if !toolAvailable(Config.MP4DecryptPath) {
					fmt.Printf(": mp4decrypt not found at '%s', skip MV dl\n", Config.MP4DecryptPath)
					session.Counter.Success++
					continue
				}
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
				err := mvDownloader(albumId, mvSaveDir, token, storefront, Config.MediaUserToken, nil, dl_select)
				if err != nil {
					fmt.Println("\u26A0 Failed to dl MV:", err)
					session.Counter.Error++
					continue
				}
				session.Counter.Success++
				continue
			}
			if strings.Contains(urlRaw, "/song/") {
				fmt.Printf("Song->")
				storefront, songId := checkUrlSong(urlRaw)
				if storefront == "" || songId == "" {
					fmt.Println("Invalid song URL format.")
					continue
				}
				if dl_select {
					err := selectSingleSongAudioFormat(songId, token, storefront)
					if err != nil {
						fmt.Println("Failed to select audio format:", err)
						continue
					}
				}
				err := ripSong(songId, token, storefront, Config.MediaUserToken)
				if err != nil {
					fmt.Println("Failed to rip song:", err)
				}
				continue
			}
			parse, err := url.Parse(urlRaw)
			if err != nil {
				log.Fatalf("Invalid URL: %v", err)
			}
			var urlArg_i = parse.Query().Get("i")

			if strings.Contains(urlRaw, "/album/") {
				fmt.Println("Album")
				storefront, albumId = checkUrl(urlRaw)
				err := ripAlbum(albumId, token, storefront, Config.MediaUserToken, urlArg_i)
				if err != nil {
					fmt.Println("Failed to rip album:", err)
				}
			} else if strings.Contains(urlRaw, "/playlist/") {
				fmt.Println("Playlist")
				storefront, albumId = checkUrlPlaylist(urlRaw)
				err := ripPlaylist(albumId, token, storefront, Config.MediaUserToken)
				if err != nil {
					fmt.Println("Failed to rip playlist:", err)
				}
			} else if strings.Contains(urlRaw, "/station/") {
				fmt.Printf("Station")
				storefront, albumId = checkUrlStation(urlRaw)
				if len(Config.MediaUserToken) <= 50 {
					fmt.Println(": meida-user-token is not set, skip station dl")
					continue
				}
				err := ripStation(albumId, token, storefront, Config.MediaUserToken)
				if err != nil {
					fmt.Println("Failed to rip station:", err)
				}
			} else {
				fmt.Println("Invalid type")
			}
		}
		fmt.Printf("=======  [\u2714 ] Completed: %d/%d  |  [\u26A0 ] Warnings: %d  |  [\u2716 ] Errors: %d  =======\n", session.Counter.Success, session.Counter.Total, session.Counter.Unavailable+session.Counter.NotSong, session.Counter.Error)
		if session.Counter.Error == 0 {
			break
		} else if Config.ExitOnError {
			fmt.Println("Error detected, exiting...")
			os.Exit(1)
		} else {
			fmt.Println("Error detected, press Enter to try again...")
			fmt.Scanln()
			fmt.Println("Start trying again...")
		}

		session.Counter = structs.Counter{}
	}

	// Print JSON output
	if print_json {
		jsonOutput, err := json.Marshal(session.AddedTracks)
		if err != nil {
			fmt.Println("Error generating JSON output:", err)
		} else {
			fmt.Println(string(jsonOutput))
		}
	}
}

func mvDownloader(adamID string, saveDir string, token string, storefront string, mediaUserToken string, track *task.Track, selectVideo bool) error {
	MVInfo, err := ampapi.GetMusicVideoResp(storefront, adamID, Config.Language, token)
	if err != nil {
		fmt.Println("\u26A0 Failed to get MV manifest:", err)
		return nil
	}

	saveDir = cleanFolderName(saveDir)

	vidPath := filepath.Join(saveDir, fmt.Sprintf("%s_vid.mp4", adamID))
	audPath := filepath.Join(saveDir, fmt.Sprintf("%s_aud.mp4", adamID))
	mvSaveName := fmt.Sprintf("%s (%s)", MVInfo.Data[0].Attributes.Name, adamID)
	if track != nil {
		mvSaveName = fmt.Sprintf("%02d. %s", track.TaskNum, MVInfo.Data[0].Attributes.Name)
	}

	mvOutPath := filepath.Join(saveDir, fmt.Sprintf("%s.mp4", forbiddenNames.ReplaceAllString(mvSaveName, "_")))

	fmt.Println(MVInfo.Data[0].Attributes.Name)

	exists, _ := fileExists(mvOutPath)
	if exists {
		fmt.Println("MV already exists locally.")

		mvArtistName := MVInfo.Data[0].Attributes.ArtistName
		mvAlbumName := MVInfo.Data[0].Attributes.AlbumName
		mvName := MVInfo.Data[0].Attributes.Name
		mvArtistId := ""
		if len(MVInfo.Data[0].Relationships.Artists.Data) > 0 {
			mvArtistId = MVInfo.Data[0].Relationships.Artists.Data[0].ID
		}

		session.AddedTracks = append(session.AddedTracks, AddedTrack{
			Path:     mvOutPath,
			Artist:   mvArtistName,
			ArtistID: mvArtistId,
			Album:    mvAlbumName,
			Song:     mvName,
		})
		return nil
	}

	mvm3u8url, _, _, _ := runv3.GetWebplayback(adamID, token, mediaUserToken, true)
	if mvm3u8url == "" {
		return errors.New("media-user-token may wrong or expired")
	}

	os.MkdirAll(saveDir, os.ModePerm)
	videom3u8url, err := extractVideoWithSelection(mvm3u8url, selectVideo)
	if err != nil {
		return err
	}
	videokeyAndUrls, _ := runv3.Run(adamID, videom3u8url, token, mediaUserToken, true, "")
	_ = runv3.ExtMvData(videokeyAndUrls, vidPath, Config.MP4DecryptPath)
	defer os.Remove(vidPath)
	audiom3u8url, _ := extractMvAudio(mvm3u8url)
	audiokeyAndUrls, _ := runv3.Run(adamID, audiom3u8url, token, mediaUserToken, true, "")
	_ = runv3.ExtMvData(audiokeyAndUrls, audPath, Config.MP4DecryptPath)
	defer os.Remove(audPath)

	tags := []string{
		"tool=",
		fmt.Sprintf("artist=%s", MVInfo.Data[0].Attributes.ArtistName),
		fmt.Sprintf("title=%s", MVInfo.Data[0].Attributes.Name),
		fmt.Sprintf("genre=%s", firstString(MVInfo.Data[0].Attributes.GenreNames)),
		fmt.Sprintf("created=%s", MVInfo.Data[0].Attributes.ReleaseDate),
		fmt.Sprintf("ISRC=%s", MVInfo.Data[0].Attributes.Isrc),
	}

	if MVInfo.Data[0].Attributes.ContentRating == "explicit" {
		tags = append(tags, "rating=1")
	} else if MVInfo.Data[0].Attributes.ContentRating == "clean" {
		tags = append(tags, "rating=2")
	} else {
		tags = append(tags, "rating=0")
	}

	if track != nil {
		if track.PreType == "playlists" && !Config.UseSongInfoForPlaylist {
			tags = append(tags, "disk=1/1")
			tags = append(tags, fmt.Sprintf("album=%s", track.PlaylistData.Attributes.Name))
			tags = append(tags, fmt.Sprintf("track=%d", track.TaskNum))
			tags = append(tags, fmt.Sprintf("tracknum=%d/%d", track.TaskNum, track.TaskTotal))
			tags = append(tags, fmt.Sprintf("album_artist=%s", track.PlaylistData.Attributes.ArtistName))
			tags = append(tags, fmt.Sprintf("performer=%s", track.Resp.Attributes.ArtistName))
		} else if track.PreType == "playlists" && Config.UseSongInfoForPlaylist {
			tags = append(tags, fmt.Sprintf("album=%s", track.AlbumData.Attributes.Name))
			tags = append(tags, fmt.Sprintf("disk=%d/%d", track.Resp.Attributes.DiscNumber, track.DiscTotal))
			tags = append(tags, fmt.Sprintf("track=%d", track.Resp.Attributes.TrackNumber))
			tags = append(tags, fmt.Sprintf("tracknum=%d/%d", track.Resp.Attributes.TrackNumber, track.AlbumData.Attributes.TrackCount))
			tags = append(tags, fmt.Sprintf("album_artist=%s", track.AlbumData.Attributes.ArtistName))
			tags = append(tags, fmt.Sprintf("performer=%s", track.Resp.Attributes.ArtistName))
			tags = append(tags, fmt.Sprintf("copyright=%s", track.AlbumData.Attributes.Copyright))
			tags = append(tags, fmt.Sprintf("UPC=%s", track.AlbumData.Attributes.Upc))
		} else {
			tags = append(tags, fmt.Sprintf("album=%s", track.AlbumData.Attributes.Name))
			tags = append(tags, fmt.Sprintf("disk=%d/%d", track.Resp.Attributes.DiscNumber, track.DiscTotal))
			tags = append(tags, fmt.Sprintf("track=%d", track.Resp.Attributes.TrackNumber))
			tags = append(tags, fmt.Sprintf("tracknum=%d/%d", track.Resp.Attributes.TrackNumber, track.AlbumData.Attributes.TrackCount))
			tags = append(tags, fmt.Sprintf("album_artist=%s", track.AlbumData.Attributes.ArtistName))
			tags = append(tags, fmt.Sprintf("performer=%s", track.Resp.Attributes.ArtistName))
			tags = append(tags, fmt.Sprintf("copyright=%s", track.AlbumData.Attributes.Copyright))
			tags = append(tags, fmt.Sprintf("UPC=%s", track.AlbumData.Attributes.Upc))
		}
	} else {
		tags = append(tags, fmt.Sprintf("album=%s", MVInfo.Data[0].Attributes.AlbumName))
		tags = append(tags, fmt.Sprintf("disk=%d", MVInfo.Data[0].Attributes.DiscNumber))
		tags = append(tags, fmt.Sprintf("track=%d", MVInfo.Data[0].Attributes.TrackNumber))
		tags = append(tags, fmt.Sprintf("tracknum=%d", MVInfo.Data[0].Attributes.TrackNumber))
		tags = append(tags, fmt.Sprintf("performer=%s", MVInfo.Data[0].Attributes.ArtistName))
	}

	var covPath string
	if true {
		thumbURL := MVInfo.Data[0].Attributes.Artwork.URL
		baseThumbName := forbiddenNames.ReplaceAllString(mvSaveName, "_") + "_thumbnail"
		covPath, err = writeCover(saveDir, baseThumbName, thumbURL)
		if err != nil {
			fmt.Println("Failed to save MV thumbnail:", err)
		} else {
			tags = append(tags, fmt.Sprintf("cover=%s", covPath))
		}
	}
	defer os.Remove(covPath)

	tagsString := strings.Join(tags, ":")
	muxCmd := exec.Command(Config.MP4BoxPath, "-itags", tagsString, "-quiet", "-add", vidPath, "-add", audPath, "-keep-utc", "-new", mvOutPath)
	fmt.Printf("MV Remuxing...")
	if err := muxCmd.Run(); err != nil {
		fmt.Printf("MV mux failed: %v\n", err)
		return err
	}
	fmt.Printf("\rMV Remuxed.   \n")

	// Append to session.AddedTracks
	mvArtistName := MVInfo.Data[0].Attributes.ArtistName
	mvAlbumName := MVInfo.Data[0].Attributes.AlbumName
	mvName := MVInfo.Data[0].Attributes.Name
	mvArtistId := ""
	if len(MVInfo.Data[0].Relationships.Artists.Data) > 0 {
		mvArtistId = MVInfo.Data[0].Relationships.Artists.Data[0].ID
	}

	session.AddedTracks = append(session.AddedTracks, AddedTrack{
		Path:     mvOutPath,
		Artist:   mvArtistName,
		ArtistID: mvArtistId,
		Album:    mvAlbumName,
		Song:     mvName,
	})

	return nil
}

func extractMvAudio(c string) (string, error) {
	MediaUrl, err := url.Parse(c)
	if err != nil {
		return "", err
	}

	resp, err := httpClient.Get(c)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New(resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	audioString := string(body)
	from, listType, err := m3u8.DecodeFrom(strings.NewReader(audioString), true)
	if err != nil || listType != m3u8.MASTER {
		return "", errors.New("m3u8 not of media type")
	}

	audio := from.(*m3u8.MasterPlaylist)

	var audioPriority = []string{"audio-atmos", "audio-ac3", "audio-stereo-256"}
	if Config.MVAudioType == "ac3" {
		audioPriority = []string{"audio-ac3", "audio-stereo-256"}
	} else if Config.MVAudioType == "aac" {
		audioPriority = []string{"audio-stereo-256"}
	}

	re := regexp.MustCompile(`_gr(\d+)_`)

	type AudioStream struct {
		URL     string
		Rank    int
		GroupID string
	}
	var audioStreams []AudioStream

	for _, variant := range audio.Variants {
		for _, audiov := range variant.Alternatives {
			if audiov.URI != "" {
				for _, priority := range audioPriority {
					if audiov.GroupId == priority {
						matches := re.FindStringSubmatch(audiov.URI)
						if len(matches) == 2 {
							var rank int
							fmt.Sscanf(matches[1], "%d", &rank)
							streamUrl, _ := MediaUrl.Parse(audiov.URI)
							audioStreams = append(audioStreams, AudioStream{
								URL:     streamUrl.String(),
								Rank:    rank,
								GroupID: audiov.GroupId,
							})
						}
					}
				}
			}
		}
	}

	if len(audioStreams) == 0 {
		return "", errors.New("no suitable audio stream found")
	}

	sort.Slice(audioStreams, func(i, j int) bool {
		return audioStreams[i].Rank > audioStreams[j].Rank
	})
	fmt.Println("Audio: " + audioStreams[0].GroupID)
	return audioStreams[0].URL, nil
}

func checkM3u8(b string, f string) (string, error) {
	var EnhancedHls string
	if Config.GetM3u8FromDevice {
		adamID := b
		conn, err := net.Dial("tcp", Config.GetM3u8Port)
		if err != nil {
			fmt.Println("Error connecting to device:", err)
			return "none", err
		}
		defer conn.Close()
		if f == "song" {
			fmt.Println("Connected to device")
		}

		adamIDBuffer := []byte(adamID)
		lengthBuffer := []byte{byte(len(adamIDBuffer))}

		_, err = conn.Write(lengthBuffer)
		if err != nil {
			fmt.Println("Error writing length to device:", err)
			return "none", err
		}

		_, err = conn.Write(adamIDBuffer)
		if err != nil {
			fmt.Println("Error writing adamID to device:", err)
			return "none", err
		}

		response, err := bufio.NewReader(conn).ReadBytes('\n')
		if err != nil {
			fmt.Println("Error reading response from device:", err)
			return "none", err
		}

		response = bytes.TrimSpace(response)
		if len(response) > 0 {
			if f == "song" {
				fmt.Println("Received URL:", string(response))
			}
			EnhancedHls = string(response)
		} else {
			fmt.Println("Received an empty response")
		}
	}
	return EnhancedHls, nil
}

func formatAvailability(available bool, quality string) string {
	if !available {
		return "Not Available"
	}
	return quality
}

func selectSingleSongAudioFormat(songId string, token string, storefront string) error {
	manifest, err := ampapi.GetSongResp(storefront, songId, Config.Language, token)
	if err != nil {
		return err
	}
	if len(manifest.Data) == 0 {
		return errors.New("song not found")
	}

	song := manifest.Data[0]
	m3u8Url := song.Attributes.ExtendedAssetUrls.EnhancedHls
	needCheck := Config.GetM3u8Mode == "all" ||
		(Config.GetM3u8Mode == "hires" && contains(song.Attributes.AudioTraits, "hi-res-lossless"))
	if needCheck {
		fullM3u8Url, err := checkM3u8(song.ID, "song")
		if err == nil && strings.HasSuffix(fullM3u8Url, ".m3u8") {
			m3u8Url = fullM3u8Url
		} else {
			fmt.Println("Failed to get best quality m3u8 from device m3u8 port, will use m3u8 from Web API")
		}
	}
	if m3u8Url == "" {
		return errors.New("no enhanced HLS audio manifest found")
	}

	formats, err := getAudioFormats(m3u8Url)
	if err != nil {
		return err
	}
	if len(formats) == 0 {
		return errors.New("no audio formats found")
	}

	fmt.Printf("\n%s - %s\n", song.Attributes.ArtistName, song.Attributes.Name)
	selectedFormat, err := promptForAudioFormat(formats)
	if err != nil {
		return err
	}
	selected_audio_format = &selectedFormat
	applySelectedAudioFormat(selectedFormat)
	return nil
}

func getAudioFormats(m3u8Url string) ([]AudioFormat, error) {
	resp, err := httpClient.Get(m3u8Url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	from, listType, err := m3u8.DecodeFrom(strings.NewReader(string(body)), true)
	if err != nil || listType != m3u8.MASTER {
		return nil, errors.New("m3u8 not of master type")
	}
	master := from.(*m3u8.MasterPlaylist)
	sort.Slice(master.Variants, func(i, j int) bool {
		if master.Variants[i].AverageBandwidth != master.Variants[j].AverageBandwidth {
			return master.Variants[i].AverageBandwidth > master.Variants[j].AverageBandwidth
		}
		return master.Variants[i].Bandwidth > master.Variants[j].Bandwidth
	})

	formats := make([]AudioFormat, 0, len(master.Variants))
	for _, variant := range master.Variants {
		formats = append(formats, AudioFormat{
			Codecs:           variant.Codecs,
			Audio:            variant.Audio,
			URI:              variant.URI,
			Bandwidth:        variant.Bandwidth,
			AverageBandwidth: variant.AverageBandwidth,
		})
	}
	return formats, nil
}

func promptForAudioFormat(formats []AudioFormat) (AudioFormat, error) {
	renderAudioFormatsTable(formats)
	fmt.Println("Press Enter to use the first format.")
	cyanColor := color.New(color.FgCyan)
	cyanColor.Print("format: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return AudioFormat{}, err
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return formats[0], nil
	}
	selectedIndex, err := strconv.Atoi(input)
	if err != nil || selectedIndex < 1 || selectedIndex > len(formats) {
		return AudioFormat{}, fmt.Errorf("invalid format: %s", input)
	}
	return formats[selectedIndex-1], nil
}

func renderAudioFormatsTable(formats []AudioFormat) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "EXT", "TBR", "PROTO", "ACODEC", "ABR", "ASR", "MORE INFO"})
	table.SetRowLine(false)
	table.SetAutoWrapText(false)
	for i, format := range formats {
		table.Append([]string{
			fmt.Sprint(i + 1),
			"m4a",
			audioFormatTBR(format),
			"m3u8",
			format.Codecs,
			audioFormatABR(format),
			audioFormatASR(format),
			audioFormatMoreInfo(format),
		})
	}
	table.Render()
}

func applySelectedAudioFormat(format AudioFormat) {
	dl_atmos = false
	dl_aac = false

	switch format.Codecs {
	case "ec-3", "ac-3":
		dl_atmos = true
		if bitrate := audioFormatBitrate(format); bitrate > 0 {
			Config.AtmosMax = bitrate
		}
	case "mp4a.40.2":
		dl_aac = true
		Config.AacType = audioFormatAACType(format)
	case "alac":
		if sampleRate := audioFormatSampleRate(format); sampleRate > 0 {
			Config.AlacMax = sampleRate
		}
	}
}

func audioFormatTBR(format AudioFormat) string {
	bandwidth := format.AverageBandwidth
	if bandwidth == 0 {
		bandwidth = format.Bandwidth
	}
	if bandwidth == 0 {
		return ""
	}
	return fmt.Sprintf("%dk", bandwidth/1000)
}

func audioFormatABR(format AudioFormat) string {
	if bitrate := audioFormatBitrate(format); bitrate > 0 {
		return fmt.Sprintf("%dk", bitrate)
	}
	return ""
}

func audioFormatASR(format AudioFormat) string {
	if sampleRate := audioFormatSampleRate(format); sampleRate > 0 {
		return fmt.Sprintf("%dHz", sampleRate)
	}
	return ""
}

func audioFormatMoreInfo(format AudioFormat) string {
	name := audioFormatName(format)
	if format.Audio == "" {
		return name
	}
	return fmt.Sprintf("%s (%s)", name, format.Audio)
}

func audioFormatName(format AudioFormat) string {
	switch {
	case format.Codecs == "ec-3" && strings.Contains(format.Audio, "atmos"):
		return "Dolby Atmos"
	case format.Codecs == "ac-3":
		return "Dolby Audio"
	case format.Codecs == "alac":
		if sampleRate := audioFormatSampleRate(format); sampleRate > 48000 {
			return "Hi-Res Lossless"
		}
		return "Lossless"
	case format.Codecs == "mp4a.40.2" && strings.Contains(format.Audio, "binaural"):
		return "AAC Binaural"
	case format.Codecs == "mp4a.40.2" && strings.Contains(format.Audio, "downmix"):
		return "AAC Downmix"
	case format.Codecs == "mp4a.40.2":
		return "AAC"
	default:
		return "Audio"
	}
}

func audioFormatQuality(format AudioFormat) string {
	if format.Codecs == "alac" {
		split := strings.Split(format.Audio, "-")
		if len(split) >= 2 {
			bitDepth := split[len(split)-1]
			sampleRate := audioFormatSampleRate(format)
			if sampleRate > 0 {
				return fmt.Sprintf("%sB-%.1fkHz", bitDepth, float64(sampleRate)/1000.0)
			}
		}
	}
	if bitrate := audioFormatBitrate(format); bitrate > 0 {
		return fmt.Sprintf("%d Kbps", bitrate)
	}
	return audioFormatTBR(format)
}

func audioFormatBitrate(format AudioFormat) int {
	split := strings.Split(format.Audio, "-")
	if len(split) == 0 {
		return 0
	}
	bitrate, err := strconv.Atoi(split[len(split)-1])
	if err != nil {
		return 0
	}
	return bitrate
}

func audioFormatSampleRate(format AudioFormat) int {
	if format.Codecs != "alac" {
		return 0
	}
	split := strings.Split(format.Audio, "-")
	if len(split) < 2 {
		return 0
	}
	sampleRate, err := strconv.Atoi(split[len(split)-2])
	if err != nil {
		return 0
	}
	return sampleRate
}

func audioFormatAACType(format AudioFormat) string {
	switch {
	case strings.Contains(format.Audio, "binaural"):
		return "aac-binaural"
	case strings.Contains(format.Audio, "downmix"):
		return "aac-downmix"
	default:
		return "aac"
	}
}

func matchesSelectedAudioFormat(variant *m3u8.Variant) bool {
	if selected_audio_format == nil {
		return false
	}
	return variant.URI == selected_audio_format.URI &&
		variant.Codecs == selected_audio_format.Codecs &&
		variant.Audio == selected_audio_format.Audio
}

func extractMedia(b string, more_mode bool) (string, string, error) {
	masterUrl, err := url.Parse(b)
	if err != nil {
		return "", "", err
	}
	resp, err := httpClient.Get(b)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", errors.New(resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	masterString := string(body)
	from, listType, err := m3u8.DecodeFrom(strings.NewReader(masterString), true)
	if err != nil || listType != m3u8.MASTER {
		return "", "", errors.New("m3u8 not of master type")
	}
	master := from.(*m3u8.MasterPlaylist)
	var streamUrl *url.URL
	sort.Slice(master.Variants, func(i, j int) bool {
		return master.Variants[i].AverageBandwidth > master.Variants[j].AverageBandwidth
	})
	if debug_mode && more_mode {
		fmt.Println("\nDebug: All Available Variants:")
		var data [][]string
		for _, variant := range master.Variants {
			data = append(data, []string{variant.Codecs, variant.Audio, fmt.Sprint(variant.Bandwidth)})
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Codec", "Audio", "Bandwidth"})
		table.SetAutoMergeCells(true)
		table.SetRowLine(true)
		table.AppendBulk(data)
		table.Render()

		var hasAAC, hasLossless, hasHiRes, hasAtmos, hasDolbyAudio bool
		var aacQuality, losslessQuality, hiResQuality, atmosQuality, dolbyAudioQuality string

		for _, variant := range master.Variants {
			if variant.Codecs == "mp4a.40.2" { // AAC
				hasAAC = true
				split := strings.Split(variant.Audio, "-")
				if len(split) >= 3 {
					bitrate, _ := strconv.Atoi(split[2])
					currentBitrate := 0
					if aacQuality != "" {
						current := strings.Split(aacQuality, " | ")[2]
						current = strings.Split(current, " ")[0]
						currentBitrate, _ = strconv.Atoi(current)
					}
					if bitrate > currentBitrate {
						aacQuality = fmt.Sprintf("AAC | 2 Channel | %d Kbps", bitrate)
					}
				}
			} else if variant.Codecs == "ec-3" && strings.Contains(variant.Audio, "atmos") { // Dolby Atmos
				hasAtmos = true
				split := strings.Split(variant.Audio, "-")
				if len(split) > 0 {
					bitrateStr := split[len(split)-1]
					if len(bitrateStr) == 4 && bitrateStr[0] == '2' {
						bitrateStr = bitrateStr[1:]
					}
					bitrate, _ := strconv.Atoi(bitrateStr)
					currentBitrate := 0
					if atmosQuality != "" {
						current := strings.Split(strings.Split(atmosQuality, " | ")[2], " ")[0]
						currentBitrate, _ = strconv.Atoi(current)
					}
					if bitrate > currentBitrate {
						atmosQuality = fmt.Sprintf("E-AC-3 | 16 Channel | %d Kbps", bitrate)
					}
				}
			} else if variant.Codecs == "alac" { // ALAC (Lossless or Hi-Res)
				split := strings.Split(variant.Audio, "-")
				if len(split) >= 3 {
					bitDepth := split[len(split)-1]
					sampleRate := split[len(split)-2]
					sampleRateInt, _ := strconv.Atoi(sampleRate)
					if sampleRateInt > 48000 { // Hi-Res
						hasHiRes = true
						hiResQuality = fmt.Sprintf("ALAC | 2 Channel | %s-bit/%d kHz", bitDepth, sampleRateInt/1000)
					} else { // Standard Lossless
						hasLossless = true
						losslessQuality = fmt.Sprintf("ALAC | 2 Channel | %s-bit/%d kHz", bitDepth, sampleRateInt/1000)
					}
				}
			} else if variant.Codecs == "ac-3" { // Dolby Audio
				hasDolbyAudio = true
				split := strings.Split(variant.Audio, "-")
				if len(split) > 0 {
					bitrate, _ := strconv.Atoi(split[len(split)-1])
					dolbyAudioQuality = fmt.Sprintf("AC-3 |  16 Channel | %d Kbps", bitrate)
				}
			}
		}

		fmt.Println("Available Audio Formats:")
		fmt.Println("------------------------")
		fmt.Printf("AAC             : %s\n", formatAvailability(hasAAC, aacQuality))
		fmt.Printf("Lossless        : %s\n", formatAvailability(hasLossless, losslessQuality))
		fmt.Printf("Hi-Res Lossless : %s\n", formatAvailability(hasHiRes, hiResQuality))
		fmt.Printf("Dolby Atmos     : %s\n", formatAvailability(hasAtmos, atmosQuality))
		fmt.Printf("Dolby Audio     : %s\n", formatAvailability(hasDolbyAudio, dolbyAudioQuality))
		fmt.Println("------------------------")

		return "", "", nil
	}
	if selected_audio_format != nil {
		for _, variant := range master.Variants {
			if !matchesSelectedAudioFormat(variant) {
				continue
			}
			streamUrlTemp, err := masterUrl.Parse(variant.URI)
			if err != nil {
				return "", "", err
			}
			if !debug_mode && !more_mode {
				fmt.Printf("%s\n", variant.Audio)
			}
			selected := AudioFormat{
				Codecs:           variant.Codecs,
				Audio:            variant.Audio,
				URI:              variant.URI,
				Bandwidth:        variant.Bandwidth,
				AverageBandwidth: variant.AverageBandwidth,
			}
			return streamUrlTemp.String(), audioFormatQuality(selected), nil
		}
		return "", "", errors.New("selected audio format not found")
	}
	var Quality string
	for _, variant := range master.Variants {
		if dl_atmos {
			if variant.Codecs == "ec-3" && strings.Contains(variant.Audio, "atmos") {
				if debug_mode && !more_mode {
					fmt.Printf("Debug: Found Dolby Atmos variant - %s (Bitrate: %d Kbps)\n",
						variant.Audio, variant.Bandwidth/1000)
				}
				split := strings.Split(variant.Audio, "-")
				length := len(split)
				length_int, err := strconv.Atoi(split[length-1])
				if err != nil {
					return "", "", err
				}
				if length_int <= Config.AtmosMax {
					if !debug_mode && !more_mode {
						fmt.Printf("%s\n", variant.Audio)
					}
					streamUrlTemp, err := masterUrl.Parse(variant.URI)
					if err != nil {
						return "", "", err
					}
					streamUrl = streamUrlTemp
					Quality = fmt.Sprintf("%s Kbps", split[len(split)-1])
					break
				}
			} else if variant.Codecs == "ac-3" { // Add Dolby Audio support
				if debug_mode && !more_mode {
					fmt.Printf("Debug: Found Dolby Audio variant - %s (Bitrate: %d Kbps)\n",
						variant.Audio, variant.Bandwidth/1000)
				}
				streamUrlTemp, err := masterUrl.Parse(variant.URI)
				if err != nil {
					return "", "", err
				}
				streamUrl = streamUrlTemp
				split := strings.Split(variant.Audio, "-")
				Quality = fmt.Sprintf("%s Kbps", split[len(split)-1])
				break
			}
		} else if dl_aac {
			if variant.Codecs == "mp4a.40.2" {
				if debug_mode && !more_mode {
					fmt.Printf("Debug: Found AAC variant - %s (Bitrate: %d)\n", variant.Audio, variant.Bandwidth)
				}
				aacregex := regexp.MustCompile(`audio-stereo-\d+`)
				replaced := aacregex.ReplaceAllString(variant.Audio, "aac")
				if replaced == Config.AacType {
					if !debug_mode && !more_mode {
						fmt.Printf("%s\n", variant.Audio)
					}
					streamUrlTemp, err := masterUrl.Parse(variant.URI)
					if err != nil {
						panic(err)
					}
					streamUrl = streamUrlTemp
					split := strings.Split(variant.Audio, "-")
					Quality = fmt.Sprintf("%s Kbps", split[2])
					break
				}
			}
		} else {
			if variant.Codecs == "alac" {
				split := strings.Split(variant.Audio, "-")
				length := len(split)
				length_int, err := strconv.Atoi(split[length-2])
				if err != nil {
					return "", "", err
				}
				if length_int <= Config.AlacMax {
					if !debug_mode && !more_mode {
						fmt.Printf("%s-bit / %s Hz\n", split[length-1], split[length-2])
					}
					streamUrlTemp, err := masterUrl.Parse(variant.URI)
					if err != nil {
						panic(err)
					}
					streamUrl = streamUrlTemp
					KHZ := float64(length_int) / 1000.0
					Quality = fmt.Sprintf("%sB-%.1fkHz", split[length-1], KHZ)
					break
				}
			}
		}
	}
	if streamUrl == nil {
		return "", "", errors.New("no codec found")
	}
	return streamUrl.String(), Quality, nil
}

type VideoStream struct {
	URL              string
	Resolution       string
	VideoRange       string
	Codecs           string
	Width            int
	Height           int
	FrameRate        float64
	Bandwidth        uint32
	AverageBandwidth uint32
}

func extractVideo(c string) (string, error) {
	return extractVideoWithSelection(c, false)
}

func extractVideoWithSelection(c string, selectVideo bool) (string, error) {
	streams, err := getVideoStreams(c)
	if err != nil {
		return "", err
	}
	if len(streams) == 0 {
		return "", errors.New("no suitable video stream found")
	}

	stream := streams[0]
	if selectVideo && len(streams) > 1 {
		selectedStream, err := promptForVideoStream(streams)
		if err != nil {
			return "", err
		}
		stream = selectedStream
	}

	fmt.Println("Video: " + stream.Resolution + "-" + stream.VideoRange)
	return stream.URL, nil
}

func getVideoStreams(c string) ([]VideoStream, error) {
	MediaUrl, err := url.Parse(c)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Get(c)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	videoString := string(body)

	from, listType, err := m3u8.DecodeFrom(strings.NewReader(videoString), true)
	if err != nil || listType != m3u8.MASTER {
		return nil, errors.New("m3u8 not of media type")
	}

	video := from.(*m3u8.MasterPlaylist)

	maxHeight := Config.MVMax
	if maxHeight <= 0 {
		maxHeight = int(^uint(0) >> 1)
	}

	var streams []VideoStream
	for _, variant := range video.Variants {
		width, height, ok := parseVariantResolution(variant.Resolution, variant.URI)
		if !ok || height > maxHeight {
			continue
		}
		streamUrl, err := MediaUrl.Parse(variant.URI)
		if err != nil {
			return nil, err
		}
		streams = append(streams, VideoStream{
			URL:              streamUrl.String(),
			Resolution:       variant.Resolution,
			VideoRange:       variant.VideoRange,
			Codecs:           variant.Codecs,
			Width:            width,
			Height:           height,
			FrameRate:        variant.FrameRate,
			Bandwidth:        variant.Bandwidth,
			AverageBandwidth: variant.AverageBandwidth,
		})
	}

	sortVideoStreams(streams)
	return streams, nil
}

func sortVideoStreams(streams []VideoStream) {
	sort.Slice(streams, func(i, j int) bool {
		if streams[i].Height != streams[j].Height {
			return streams[i].Height > streams[j].Height
		}
		if streams[i].Width != streams[j].Width {
			return streams[i].Width > streams[j].Width
		}
		if streams[i].AverageBandwidth != streams[j].AverageBandwidth {
			return streams[i].AverageBandwidth > streams[j].AverageBandwidth
		}
		return streams[i].Bandwidth > streams[j].Bandwidth
	})
}

func promptForVideoStream(streams []VideoStream) (VideoStream, error) {
	renderVideoFormatsTable(streams)
	cyanColor := color.New(color.FgCyan)
	cyanColor.Print("format: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return VideoStream{}, err
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return streams[0], nil
	}
	selectedIndex, err := strconv.Atoi(input)
	if err != nil || selectedIndex < 1 || selectedIndex > len(streams) {
		return VideoStream{}, fmt.Errorf("invalid format: %s", input)
	}
	return streams[selectedIndex-1], nil
}

func renderVideoFormatsTable(streams []VideoStream) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "EXT", "RESOLUTION", "FPS", "TBR", "PROTO", "VCODEC", "MORE INFO"})
	table.SetRowLine(false)
	table.SetAutoWrapText(false)
	table.SetHeaderColor(tablewriter.Colors{},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.FgRedColor, tablewriter.Bold},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor})
	table.SetColumnColor(tablewriter.Colors{tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor},
		tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlackColor})
	for i, stream := range streams {
		table.Append([]string{
			fmt.Sprint(i + 1),
			"mp4",
			videoStreamResolution(stream),
			videoStreamFPS(stream),
			videoStreamTBR(stream),
			"m3u8",
			videoStreamCodec(stream),
			videoStreamMoreInfo(stream),
		})
	}
	table.Render()
	fmt.Println("Press Enter to use the best format.")
}

func videoStreamResolution(stream VideoStream) string {
	resolution := stream.Resolution
	if resolution == "" && stream.Width > 0 && stream.Height > 0 {
		resolution = fmt.Sprintf("%dx%d", stream.Width, stream.Height)
	}
	return resolution
}

func videoStreamFPS(stream VideoStream) string {
	if stream.FrameRate <= 0 {
		return ""
	}
	if stream.FrameRate == float64(int(stream.FrameRate)) {
		return fmt.Sprintf("%d", int(stream.FrameRate))
	}
	return fmt.Sprintf("%.2f", stream.FrameRate)
}

func videoStreamTBR(stream VideoStream) string {
	bitrate := videoStreamBitrate(stream)
	if bitrate == 0 {
		return ""
	}
	return fmt.Sprintf("%dk", bitrate/1000)
}

func videoStreamCodec(stream VideoStream) string {
	if stream.Codecs == "" {
		return "unknown"
	}
	return stream.Codecs
}

func videoStreamMoreInfo(stream VideoStream) string {
	parts := []string{}
	if stream.VideoRange != "" {
		parts = append(parts, stream.VideoRange)
	}
	if stream.Height >= 2160 {
		parts = append(parts, "4K")
	} else if stream.Height > 0 {
		parts = append(parts, fmt.Sprintf("%dp", stream.Height))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func videoStreamBitrate(stream VideoStream) uint32 {
	if stream.AverageBandwidth > 0 {
		return stream.AverageBandwidth
	}
	return stream.Bandwidth
}

func parseVariantResolution(resolution string, uri string) (int, int, bool) {
	resolution = strings.TrimSpace(resolution)
	if resolution != "" {
		split := strings.Split(resolution, "x")
		if len(split) == 2 {
			width, widthErr := strconv.Atoi(split[0])
			height, heightErr := strconv.Atoi(split[1])
			if widthErr == nil && heightErr == nil {
				return width, height, true
			}
		}
	}

	matches := videoResolutionRegex.FindStringSubmatch(uri)
	if len(matches) != 3 {
		return 0, 0, false
	}
	width, widthErr := strconv.Atoi(matches[1])
	height, heightErr := strconv.Atoi(matches[2])
	if widthErr != nil || heightErr != nil {
		return 0, 0, false
	}
	return width, height, true
}

func ripSong(songId string, token string, storefront string, mediaUserToken string) error {
	// Get song info to find album ID
	manifest, err := ampapi.GetSongResp(storefront, songId, Config.Language, token)
	if err != nil {
		fmt.Println("Failed to get song response.")
		return err
	}

	songData := manifest.Data[0]
	if len(songData.Relationships.Albums.Data) == 0 {
		return errors.New("song response contains no album relationship")
	}
	albumId := songData.Relationships.Albums.Data[0].ID

	// Use album approach but only download the specific song
	dl_song = true
	err = ripAlbum(albumId, token, storefront, mediaUserToken, songId)
	if err != nil {
		fmt.Println("Failed to rip song:", err)
		return err
	}

	return nil
}
