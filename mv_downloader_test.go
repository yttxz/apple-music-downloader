package main

import (
	widevinerip "amdl/internal/widevine-rip"
	"context"
	"errors"
	"strings"
	"testing"

	ampapi "amdl/internal/amp-api"
)

func TestMVDownloaderReturnsManifestError(t *testing.T) {
	oldConfig := Config
	oldGetMusicVideoResp := getMusicVideoResp
	t.Cleanup(func() {
		Config = oldConfig
		getMusicVideoResp = oldGetMusicVideoResp
	})

	Config = defaultConfig()
	getMusicVideoResp = func(storefront string, id string, language string, token string) (*ampapi.MusicVideoResp, error) {
		return nil, errors.New("manifest failed")
	}

	err := mvDownloader(newSessionState(""), "123", t.TempDir(), "token", "us", "media-user-token-with-enough-length-for-test", nil, false)
	if err == nil || !strings.Contains(err.Error(), "manifest failed") {
		t.Fatalf("mvDownloader error = %v, want manifest failed", err)
	}
}

func TestMVDownloaderReturnsVideoDecryptError(t *testing.T) {
	oldConfig := Config
	oldGetMusicVideoResp := getMusicVideoResp
	oldGetMVWebplayback := getMVWebplayback
	oldRunMVDownload := fetchMVStream
	oldExtractMVData := extractMVData
	oldExtractMVVideoURL := extractMVVideoURL
	oldExtractMVAudioURL := extractMVAudioURL
	t.Cleanup(func() {
		Config = oldConfig
		getMusicVideoResp = oldGetMusicVideoResp
		getMVWebplayback = oldGetMVWebplayback
		fetchMVStream = oldRunMVDownload
		extractMVData = oldExtractMVData
		extractMVVideoURL = oldExtractMVVideoURL
		extractMVAudioURL = oldExtractMVAudioURL
	})

	Config = defaultConfig()
	getMusicVideoResp = func(storefront string, id string, language string, token string) (*ampapi.MusicVideoResp, error) {
		resp := &ampapi.MusicVideoResp{Data: []ampapi.MusicVideoRespData{{ID: id}}}
		resp.Data[0].Attributes.Name = "Video"
		resp.Data[0].Attributes.ArtistName = "Artist"
		resp.Data[0].Attributes.AlbumName = "Album"
		return resp, nil
	}
	getMVWebplayback = func(server string, adamId string) (string, error) {
		return "master.m3u8", nil
	}
	extractMVVideoURL = func(c string, selectVideo bool) (string, bool, error) {
		return "video.m3u8", false, nil
	}
	extractMVAudioURL = func(c string) (string, error) {
		return "audio.m3u8", nil
	}
	fetchMVStream = func(ctx context.Context, adamId, url, server string, playReady bool) (widevinerip.EncryptedStream, error) {
		return widevinerip.EncryptedStream{}, nil
	}
	extractMVData = func(ctx context.Context, stream widevinerip.EncryptedStream, savePath string) error {
		return errors.New("decrypt failed")
	}

	err := mvDownloader(newSessionState(""), "123", t.TempDir(), "token", "us", "media-user-token-with-enough-length-for-test", nil, false)
	if err == nil || !strings.Contains(err.Error(), "decrypt MV video stream") || !strings.Contains(err.Error(), "decrypt failed") {
		t.Fatalf("mvDownloader error = %v, want video decrypt failure", err)
	}
}
