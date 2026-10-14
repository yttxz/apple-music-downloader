package main

import (
	"fmt"
	"path/filepath"
	"strings"

	task "amdl/internal/model"
)

func saveFolderForCurrentCodec(state *sessionState, folderName string) string {
	root := Config.AlacSaveFolder
	if state != nil && state.isAtmosMode() {
		root = Config.AtmosSaveFolder
	}
	if state != nil && state.isAACMode() {
		root = Config.AacSaveFolder
	}
	return filepath.Join(root, forbiddenNames.ReplaceAllString(folderName, "_"))
}

func cleanFolderName(name string) string {
	if strings.HasSuffix(name, ".") {
		name = strings.ReplaceAll(name, ".", "")
	}
	return strings.TrimSpace(name)
}

func formatChoiceTags(isAppleMaster bool, contentRating string) string {
	var tags []string
	if isAppleMaster && Config.AppleMasterChoice != "" {
		tags = append(tags, Config.AppleMasterChoice)
	}
	if contentRating == "explicit" && Config.ExplicitChoice != "" {
		tags = append(tags, Config.ExplicitChoice)
	}
	if contentRating == "clean" && Config.CleanChoice != "" {
		tags = append(tags, Config.CleanChoice)
	}
	return strings.Join(tags, " ")
}

func allIndexes(total int) []int {
	indexes := make([]int, total)
	for i := 0; i < total; i++ {
		indexes[i] = i + 1
	}
	return indexes
}

func trackArtistID(track *task.Track) string {
	if len(track.Resp.Relationships.Artists.Data) == 0 {
		return ""
	}
	return track.Resp.Relationships.Artists.Data[0].ID
}

func addedTrackForPath(track *task.Track, path string) AddedTrack {
	return AddedTrack{
		Path:     path,
		Artist:   track.Resp.Attributes.ArtistName,
		ArtistID: trackArtistID(track),
		Album:    track.Resp.Attributes.AlbumName,
		Song:     track.Resp.Attributes.Name,
	}
}

func markTrackSuccess(state *sessionState, track *task.Track, path string) {
	state.Counter.Success++
	state.OKDict[track.PreID] = append(state.OKDict[track.PreID], track.TaskNum)
	state.AddedTracks = append(state.AddedTracks, addedTrackForPath(track, path))
	item := trackDBItem(state, track, path)
	recordDownload(item)
}

func markTrackSkipped(state *sessionState, track *task.Track, path string, reason string) {
	state.Counter.Success++
	state.OKDict[track.PreID] = append(state.OKDict[track.PreID], track.TaskNum)
	if shouldListSkippedTrack(reason, path) {
		state.AddedTracks = append(state.AddedTracks, addedTrackForPath(track, path))
	}
	item := trackDBItem(state, track, path)
	if reason == "file-exists" {
		if appDB != nil && !no_database {
			if err := appDB.RecordDownload(item); err != nil {
				fmt.Println("Failed to record existing file:", err)
			}
		}
	}
	recordSkip(item, reason)
}

func markTrackFailure(state *sessionState, track *task.Track, err error) {
	recordFailure(trackDBItem(state, track, plannedTrackPath(track, "")), err)
}

func shouldListSkippedTrack(reason string, path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	return reason == "already-downloaded" || reason == "file-exists"
}
