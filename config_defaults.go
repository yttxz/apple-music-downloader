package main

import (
	"os"
	"path/filepath"
	"strings"

	"apple-music-downloader/utils/structs"
)

const defaultConfigPath = "config.yaml"

func defaultConfig() structs.ConfigSet {
	return structs.ConfigSet{
		Storefront:                 "us",
		LrcType:                    "lyrics",
		LrcFormat:                  "lrc",
		EmbedLrc:                   true,
		EmbedCover:                 true,
		CoverSize:                  "5000x5000",
		CoverFormat:                "jpg",
		TagSortOrder:               true,
		TagItunesID:                true,
		AlacSaveFolder:             "AM-DL downloads",
		AtmosSaveFolder:            "AM-DL-Atmos downloads",
		AacSaveFolder:              "AM-DL-AAC downloads",
		MVSaveFolder:               "AM-DL-MV downloads",
		MaxMemoryLimit:             256,
		DecryptM3u8Port:            "127.0.0.1:10020",
		GetM3u8Port:                "127.0.0.1:20020",
		GetM3u8FromDevice:          true,
		GetM3u8Mode:                "hires",
		AacType:                    "aac-lc",
		AlacMax:                    192000,
		AtmosMax:                   2768,
		LimitMax:                   200,
		MVAudioType:                "atmos",
		MVMax:                      2160,
		ConvertFormat:              "flac",
		ConvertSkipIfSourceMatch:   true,
		ConvertWithMetadata:        true,
		ConvertWarnLossyToLossless: true,
		ConvertSkipLossyToLossless: true,
		FFmpegPath:                 "ffmpeg",
		MP4BoxPath:                 "MP4Box",
		MP4DecryptPath:             "mp4decrypt",
		DatabaseEnabled:            true,
		DatabasePath:               "amdl.db",
		FailedDownloadsEnabled:     true,
		DownloadConcurrency:        1,
		MVSegmentConcurrency:       6,
		RequestsPerMinute:          60,
		RetryCount:                 3,
		RetryBackoffMS:             500,
		RequestTimeoutSeconds:      30,
		QualityFallback:            "skip",
	}
}

func configPathFromArgs(args []string) string {
	for i, arg := range args {
		if arg == "--config" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimPrefix(arg, "--config=")
		}
	}
	return defaultConfigPath
}

func helpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "help" {
			return true
		}
	}
	return false
}

func isPlaceholderToken(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" ||
		value == "your-media-user-token" ||
		value == "your-authorization-token" ||
		value == "enter your account storefront"
}

func cleanConfiguredToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "Bearer ")
	if isPlaceholderToken(value) {
		return ""
	}
	return value
}

func validOutputRoot(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == string(os.PathSeparator) {
		return false
	}
	return !strings.HasPrefix(clean, "..")
}
