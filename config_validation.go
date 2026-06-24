package main

import (
	"fmt"
	"strings"

	"apple-music-downloader/utils/structs"
)

func validateConfig(config *structs.ConfigSet) error {
	if !oneOf(config.CoverFormat, "jpg", "png", "original") {
		return fmt.Errorf("invalid cover-format %q: expected jpg, png, or original", config.CoverFormat)
	}
	if !oneOf(config.LrcType, "lyrics", "syllable-lyrics") {
		return fmt.Errorf("invalid lrc-type %q: expected lyrics or syllable-lyrics", config.LrcType)
	}
	if !oneOf(config.LrcFormat, "lrc", "ttml") {
		return fmt.Errorf("invalid lrc-format %q: expected lrc or ttml", config.LrcFormat)
	}
	if !oneOf(config.GetM3u8Mode, "all", "hires") {
		return fmt.Errorf("invalid get-m3u8-mode %q: expected all or hires", config.GetM3u8Mode)
	}
	if !oneOf(config.AacType, "aac-lc", "aac", "aac-binaural", "aac-downmix") {
		return fmt.Errorf("invalid aac-type %q: expected aac-lc, aac, aac-binaural, or aac-downmix", config.AacType)
	}
	if !oneOf(config.MVAudioType, "atmos", "ac3", "aac") {
		return fmt.Errorf("invalid mv-audio-type %q: expected atmos, ac3, or aac", config.MVAudioType)
	}
	if config.ConvertFormat != "" && !oneOf(config.ConvertFormat, "flac", "mp3", "opus", "wav", "copy") {
		return fmt.Errorf("invalid convert-format %q: expected flac, mp3, opus, wav, or copy", config.ConvertFormat)
	}
	if config.MaxMemoryLimit <= 0 {
		return fmt.Errorf("invalid max-memory-limit %d: expected a positive value", config.MaxMemoryLimit)
	}
	if config.LimitMax <= 0 {
		return fmt.Errorf("invalid limit-max %d: expected a positive value", config.LimitMax)
	}
	if config.AlacMax <= 0 {
		return fmt.Errorf("invalid alac-max %d: expected a positive value", config.AlacMax)
	}
	if config.AtmosMax <= 0 {
		return fmt.Errorf("invalid atmos-max %d: expected a positive value", config.AtmosMax)
	}
	if config.MVMax <= 0 {
		return fmt.Errorf("invalid mv-max %d: expected a positive value", config.MVMax)
	}
	if strings.TrimSpace(config.CoverSize) == "" {
		return fmt.Errorf("invalid cover-size %q: expected a non-empty value", config.CoverSize)
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
