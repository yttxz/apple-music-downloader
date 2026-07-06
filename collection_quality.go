package main

import (
	"fmt"
	"strings"

	"apple-music-downloader/utils/ampapi"
)

func resolveCollectionCodecAndQuality(state *sessionState, folderFormat string, storefront string, language string, token string, firstTrack ampapi.TrackRespData) (string, string) {
	codec := state.currentCodec()
	if !strings.Contains(folderFormat, "Quality") {
		return codec, ""
	}
	if quality, ok := qualityFromSelectedMode(state); ok {
		return codec, quality
	}

	manifest, err := getSongResp(storefront, firstTrack.ID, language, token)
	if err != nil {
		fmt.Println("Failed to get manifest.\n", err)
		return codec, ""
	}
	if len(manifest.Data) == 0 || manifest.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls == "" {
		return "AAC", "256Kbps"
	}

	m3u8URL := manifest.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls
	if shouldCheckEnhancedHLS(firstTrack.Attributes.AudioTraits) {
		enhancedURL, _ := checkM3u8(firstTrack.ID, "album")
		if strings.HasSuffix(enhancedURL, ".m3u8") {
			m3u8URL = enhancedURL
		}
	}
	_, quality, err := extractMedia(state, m3u8URL, true)
	if err != nil {
		fmt.Println("Failed to extract quality from manifest.\n", err)
	}
	return codec, quality
}

func qualityFromSelectedMode(state *sessionState) (string, bool) {
	if state != nil && state.SelectedAudioFormat != nil {
		return audioFormatQuality(*state.SelectedAudioFormat), true
	}
	if state.isAtmosMode() {
		return fmt.Sprintf("%dKbps", state.atmosMax()-2000), true
	}
	if state.isAACMode() && state.aacType() == "aac-lc" {
		return "256Kbps", true
	}
	return "", false
}

func shouldCheckEnhancedHLS(audioTraits []string) bool {
	return Config.GetM3u8Mode == "all" ||
		(Config.GetM3u8Mode == "hires" && contains(audioTraits, "hi-res-lossless"))
}
