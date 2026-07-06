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

	m3u8URL := preferredEnhancedHLS(firstTrack.ID, manifest.Data[0].Attributes.ExtendedAssetUrls.EnhancedHls, firstTrack.Attributes.AudioTraits, "album", false)
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

func preferredEnhancedHLS(trackID string, fallbackURL string, audioTraits []string, wrapperMode string, warnOnFallback bool) string {
	if !shouldCheckEnhancedHLS(audioTraits) {
		return fallbackURL
	}
	fullM3u8URL, err := checkM3u8(trackID, wrapperMode)
	if err == nil && strings.HasSuffix(fullM3u8URL, ".m3u8") {
		return fullM3u8URL
	}
	if warnOnFallback {
		fmt.Println("Failed to get best quality m3u8 from device m3u8 port, will use m3u8 from Web API")
	}
	return fallbackURL
}
