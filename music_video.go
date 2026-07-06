package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"apple-music-downloader/utils/ampapi"
	"apple-music-downloader/utils/task"

	"github.com/grafov/m3u8"
)

func mvDownloader(state *sessionState, adamID string, saveDir string, token string, storefront string, mediaUserToken string, track *task.Track, selectVideo bool) error {
	MVInfo, err := getMusicVideoResp(storefront, adamID, Config.Language, token)
	if err != nil {
		fmt.Println("\u26A0 Failed to get MV manifest:", err)
		return err
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
		state.AddedTracks = append(state.AddedTracks, addedTrackForMusicVideo(MVInfo.Data[0], mvOutPath))
		return nil
	}

	mvm3u8url, _, _, err := getMVWebplayback(adamID, token, mediaUserToken, true)
	if err != nil {
		return fmt.Errorf("get MV playback manifest: %w", err)
	}
	if mvm3u8url == "" {
		return errors.New("media-user-token may wrong or expired")
	}

	if err := ensureDir(saveDir); err != nil {
		return fmt.Errorf("create MV folder %s: %w", saveDir, err)
	}
	videom3u8url, err := extractMVVideoURL(mvm3u8url, selectVideo)
	if err != nil {
		return err
	}
	videokeyAndUrls, err := runMVDownload(adamID, videom3u8url, token, mediaUserToken, true, "")
	if err != nil {
		return fmt.Errorf("download MV video stream: %w", err)
	}
	if err := extractMVData(videokeyAndUrls, vidPath, Config.MP4DecryptPath); err != nil {
		return fmt.Errorf("decrypt MV video stream: %w", err)
	}
	defer os.Remove(vidPath)
	audiom3u8url, err := extractMVAudioURL(mvm3u8url)
	if err != nil {
		return fmt.Errorf("select MV audio stream: %w", err)
	}
	audiokeyAndUrls, err := runMVDownload(adamID, audiom3u8url, token, mediaUserToken, true, "")
	if err != nil {
		return fmt.Errorf("download MV audio stream: %w", err)
	}
	if err := extractMVData(audiokeyAndUrls, audPath, Config.MP4DecryptPath); err != nil {
		return fmt.Errorf("decrypt MV audio stream: %w", err)
	}
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
	thumbURL := MVInfo.Data[0].Attributes.Artwork.URL
	baseThumbName := forbiddenNames.ReplaceAllString(mvSaveName, "_") + "_thumbnail"
	covPath, err = writeCover(saveDir, baseThumbName, thumbURL)
	if err != nil {
		fmt.Println("Failed to save MV thumbnail:", err)
	} else {
		tags = append(tags, fmt.Sprintf("cover=%s", covPath))
	}
	defer os.Remove(covPath)

	tagsString := strings.Join(tags, ":")
	tempMvOutPath, cleanupMvOut, err := tempPathInDestination(mvOutPath)
	if err != nil {
		return err
	}
	defer cleanupMvOut(false)
	_ = os.Remove(tempMvOutPath)
	muxCmd := exec.Command(Config.MP4BoxPath, "-itags", tagsString, "-quiet", "-add", vidPath, "-add", audPath, "-keep-utc", "-new", tempMvOutPath)
	fmt.Printf("MV Remuxing...")
	if err := muxCmd.Run(); err != nil {
		fmt.Printf("MV mux failed: %v\n", err)
		return err
	}
	if err := renameTempIntoPlace(tempMvOutPath, mvOutPath); err != nil {
		return err
	}
	cleanupMvOut(true)
	fmt.Printf("\rMV Remuxed.   \n")

	state.AddedTracks = append(state.AddedTracks, addedTrackForMusicVideo(MVInfo.Data[0], mvOutPath))

	return nil
}

func addedTrackForMusicVideo(mv ampapi.MusicVideoRespData, path string) AddedTrack {
	artistID := ""
	if len(mv.Relationships.Artists.Data) > 0 {
		artistID = mv.Relationships.Artists.Data[0].ID
	}
	return AddedTrack{
		Path:     path,
		Artist:   mv.Attributes.ArtistName,
		ArtistID: artistID,
		Album:    mv.Attributes.AlbumName,
		Song:     mv.Attributes.Name,
	}
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
