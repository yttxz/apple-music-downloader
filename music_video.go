package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	ampapi "amdl/internal/amp-api"
	"amdl/internal/app"
	mvmedia "amdl/internal/media/mv"
	task "amdl/internal/model"
)

func mvDownloader(state *sessionState, adamID string, saveDir string, token string, storefront string, mediaUserToken string, track *task.Track, selectVideo bool) error {
	info, err := getMusicVideoResp(storefront, adamID, Config.Language, token)
	if err != nil {
		return fmt.Errorf("get MV manifest: %w", err)
	}
	if info == nil || len(info.Data) == 0 {
		return errors.New("MV manifest contains no data")
	}
	saveDir = cleanFolderName(saveDir)
	name := fmt.Sprintf("%s (%s)", info.Data[0].Attributes.Name, adamID)
	if track != nil {
		name = fmt.Sprintf("%02d. %s", track.TaskNum, info.Data[0].Attributes.Name)
	}
	outPath := filepath.Join(saveDir, forbiddenNames.ReplaceAllString(name, "_")+".mp4")
	if exists, err := fileExists(outPath); err != nil {
		return err
	} else if exists {
		state.AddedTracks = append(state.AddedTracks, addedTrackForMusicVideo(info.Data[0], outPath))
		return nil
	}
	manifest, err := getMVWebplayback(Config.LiteServer, adamID)
	if err != nil {
		return fmt.Errorf("get MV playback manifest: %w", err)
	}
	videoURL, playReady, err := extractMVVideoURL(manifest, selectVideo)
	if err != nil {
		return err
	}
	ctx := context.Background()
	video, err := fetchMVStream(ctx, adamID, videoURL, Config.LiteServer, playReady)
	if err != nil {
		return fmt.Errorf("download MV video stream: %w", err)
	}
	videoPath, cleanupVideo, err := tempPathInDestination(filepath.Join(saveDir, adamID+"_vid.mp4"))
	if err != nil {
		return err
	}
	defer cleanupVideo(false)
	if err := extractMVData(ctx, video, videoPath); err != nil {
		return fmt.Errorf("decrypt MV video stream: %w", err)
	}
	audioURL, err := extractMVAudioURL(manifest)
	if err != nil {
		return fmt.Errorf("select MV audio stream: %w", err)
	}
	audio, err := fetchMVStream(ctx, adamID, audioURL, Config.LiteServer, false)
	if err != nil {
		return fmt.Errorf("download MV audio stream: %w", err)
	}
	audioPath, cleanupAudio, err := tempPathInDestination(filepath.Join(saveDir, adamID+"_aud.mp4"))
	if err != nil {
		return err
	}
	defer cleanupAudio(false)
	if err := extractMVData(ctx, audio, audioPath); err != nil {
		return fmt.Errorf("decrypt MV audio stream: %w", err)
	}
	tempOut, cleanupOut, err := tempPathInDestination(outPath)
	if err != nil {
		return err
	}
	defer cleanupOut(false)
	if err := mvmedia.Mux(videoPath, audioPath, tempOut); err != nil {
		return fmt.Errorf("mux MV: %w", err)
	}
	coverPath := ""
	if Config.EmbedCover {
		coverPath, err = writeCover(saveDir, forbiddenNames.ReplaceAllString(name, "_")+"_thumbnail", info.Data[0].Attributes.Artwork.URL)
		if err != nil {
			fmt.Println("Failed to save MV thumbnail:", err)
		}
		if coverPath != "" {
			defer os.Remove(coverPath)
		}
	}
	if err := app.NewRunner(upstreamConfig(Config)).WriteMVTags(tempOut, info, track, coverPath); err != nil {
		return fmt.Errorf("tag MV: %w", err)
	}
	if err := renameTempIntoPlace(tempOut, outPath); err != nil {
		return err
	}
	state.AddedTracks = append(state.AddedTracks, addedTrackForMusicVideo(info.Data[0], outPath))
	return nil
}

func addedTrackForMusicVideo(mv ampapi.MusicVideoRespData, path string) AddedTrack {
	artistID := ""
	if len(mv.Relationships.Artists.Data) > 0 {
		artistID = mv.Relationships.Artists.Data[0].ID
	}
	return AddedTrack{Path: path, Artist: mv.Attributes.ArtistName, ArtistID: artistID, Album: mv.Attributes.AlbumName, Song: mv.Attributes.Name}
}

func extractMvAudio(url string) (string, error) {
	return app.NewRunner(upstreamConfig(Config)).ExtractMVAudio(url)
}
