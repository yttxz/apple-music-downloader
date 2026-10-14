package main

import (
	"testing"

	ampapi "amdl/internal/amp-api"
)

func TestResolveCollectionCodecAndQualityFallsBackToAACWhenManifestHasNoEnhancedHLS(t *testing.T) {
	oldConfig := Config
	oldGetSongResp := getSongResp
	oldDLAAC := dl_aac
	oldDLAtmos := dl_atmos
	t.Cleanup(func() {
		Config = oldConfig
		getSongResp = oldGetSongResp
		dl_aac = oldDLAAC
		dl_atmos = oldDLAtmos
	})

	Config = defaultConfig()
	Config.AlbumFolderFormat = "{Quality}"
	dl_aac = false
	dl_atmos = false
	getSongResp = func(storefront string, id string, language string, token string) (*ampapi.SongResp, error) {
		return &ampapi.SongResp{Data: []ampapi.SongRespData{{ID: id}}}, nil
	}

	firstTrack := ampapi.TrackRespData{ID: "123"}
	codec, quality := resolveCollectionCodecAndQuality(newSessionState(""), Config.AlbumFolderFormat, "us", "en-US", "token", firstTrack)
	if codec != "AAC" || quality != "256Kbps" {
		t.Fatalf("codec/quality = %q/%q, want AAC/256Kbps", codec, quality)
	}
}

func TestShouldCheckEnhancedHLSHonorsModeAndTraits(t *testing.T) {
	oldConfig := Config
	t.Cleanup(func() { Config = oldConfig })

	Config = defaultConfig()
	Config.GetM3u8Mode = "all"
	if !shouldCheckEnhancedHLS(nil) {
		t.Fatal("all mode should check enhanced HLS")
	}

	Config.GetM3u8Mode = "hires"
	if !shouldCheckEnhancedHLS([]string{"lossless", "hi-res-lossless"}) {
		t.Fatal("hires mode should check hi-res tracks")
	}
	if shouldCheckEnhancedHLS([]string{"lossless"}) {
		t.Fatal("hires mode should not check non-hi-res tracks")
	}
}
