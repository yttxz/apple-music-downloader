package main

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"apple-music-downloader/utils/structs"

	"github.com/kballard/go-shellquote"
)

var coverSizePattern = regexp.MustCompile(`^\d{2,5}x\d{2,5}$`)

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
	if !oneOf(config.QualityFallback, "skip", "best-available") {
		return fmt.Errorf("invalid quality-fallback %q: expected skip or best-available", config.QualityFallback)
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
	if !coverSizePattern.MatchString(config.CoverSize) {
		return fmt.Errorf("invalid cover-size %q: expected WIDTHxHEIGHT", config.CoverSize)
	}
	if config.DownloadConcurrency <= 0 || config.DownloadConcurrency > 16 {
		return fmt.Errorf("invalid download-concurrency %d: expected 1-16", config.DownloadConcurrency)
	}
	if config.MVSegmentConcurrency <= 0 || config.MVSegmentConcurrency > 32 {
		return fmt.Errorf("invalid mv-segment-concurrency %d: expected 1-32", config.MVSegmentConcurrency)
	}
	if config.RequestsPerMinute < -1 || config.RequestsPerMinute == 0 {
		return fmt.Errorf("invalid requests-per-minute %d: expected -1 or a positive value", config.RequestsPerMinute)
	}
	if config.RetryCount < 0 || config.RetryCount > 10 {
		return fmt.Errorf("invalid retry-count %d: expected 0-10", config.RetryCount)
	}
	if config.RetryBackoffMS < 0 {
		return fmt.Errorf("invalid retry-backoff-ms %d: expected a non-negative value", config.RetryBackoffMS)
	}
	if config.RequestTimeoutSeconds <= 0 || config.RequestTimeoutSeconds > 3600 {
		return fmt.Errorf("invalid request-timeout-seconds %d: expected 1-3600", config.RequestTimeoutSeconds)
	}
	if !validOutputRoot(config.AlacSaveFolder) || !validOutputRoot(config.AtmosSaveFolder) || !validOutputRoot(config.AacSaveFolder) || !validOutputRoot(config.MVSaveFolder) {
		return fmt.Errorf("invalid output folder: output folders must be non-empty and must not point above the current directory")
	}
	if !config.AllowNonlocalWrapper {
		if err := validateLoopbackAddress(config.DecryptM3u8Port, "decrypt-m3u8-port"); err != nil {
			return err
		}
		if err := validateLoopbackAddress(config.GetM3u8Port, "get-m3u8-port"); err != nil {
			return err
		}
	}
	if err := validateConvertExtraArgs(config.ConvertExtraArgs); err != nil {
		return err
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

func validateLoopbackAddress(addr string, name string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil || port == "" {
		return fmt.Errorf("invalid %s %q: expected host:port", name, addr)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("invalid %s %q: expected loopback address unless allow-nonlocal-wrapper is true", name, addr)
	}
	return nil
}

func validateConvertExtraArgs(extraArgs string) error {
	extraArgs = strings.TrimSpace(extraArgs)
	if extraArgs == "" {
		return nil
	}
	args, err := shellquote.Split(extraArgs)
	if err != nil {
		return fmt.Errorf("invalid convert-extra-args: %w", err)
	}
	for _, arg := range args {
		switch arg {
		case "-i", "-y":
			return fmt.Errorf("invalid convert-extra-args: %s is controlled by the app", arg)
		}
		if strings.HasPrefix(arg, "file:") {
			return fmt.Errorf("invalid convert-extra-args: file protocol paths are not allowed")
		}
	}
	return nil
}
