package main

import (
	"path/filepath"
	"strings"

	"apple-music-downloader/utils/task"
)

func currentCodec() string {
	if dl_atmos {
		return "ATMOS"
	}
	if dl_aac {
		return "AAC"
	}
	return "ALAC"
}

func saveFolderForCurrentCodec(folderName string) string {
	root := Config.AlacSaveFolder
	if dl_atmos {
		root = Config.AtmosSaveFolder
	}
	if dl_aac {
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

func markTrackSuccess(track *task.Track, path string) {
	session.Counter.Success++
	session.OKDict[track.PreID] = append(session.OKDict[track.PreID], track.TaskNum)
	session.AddedTracks = append(session.AddedTracks, addedTrackForPath(track, path))
}
