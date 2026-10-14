package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	task "amdl/internal/model"
)

func TestConvertIfNeededUsesFakeFFmpegSafely(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake ffmpeg shell script test is Unix-specific")
	}

	oldConfig := Config
	t.Cleanup(func() { Config = oldConfig })

	dir := t.TempDir()
	logPath := filepath.Join(dir, "ffmpeg.args")
	fakeFFmpeg := filepath.Join(dir, "ffmpeg")
	script := `#!/bin/sh
set -eu
last=""
: > "$FAKE_FFMPEG_LOG"
for arg in "$@"; do
  printf '<%s>\n' "$arg" >> "$FAKE_FFMPEG_LOG"
  last="$arg"
done
printf 'converted' > "$last"
`
	if err := os.WriteFile(fakeFFmpeg, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile fake ffmpeg: %v", err)
	}
	t.Setenv("FAKE_FFMPEG_LOG", logPath)

	srcPath := filepath.Join(dir, "song.m4a")
	if err := os.WriteFile(srcPath, []byte("source"), 0644); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}

	Config = defaultConfig()
	Config.ConvertAfterDownload = true
	Config.ConvertFormat = "mp3"
	Config.ConvertKeepOriginal = true
	Config.FFmpegPath = fakeFFmpeg
	Config.ConvertExtraArgs = "-metadata 'title=hello;touch should-not-run'"

	track := &task.Track{SavePath: srcPath, SaveName: filepath.Base(srcPath), Codec: "ALAC"}
	convertIfNeeded(track)

	outPath := strings.TrimSuffix(srcPath, ".m4a") + ".mp3"
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("converted output missing: %v", err)
	}
	if _, err := os.Stat(srcPath); err != nil {
		t.Fatalf("original should be kept: %v", err)
	}
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile log: %v", err)
	}
	log := string(logData)
	if !strings.Contains(log, "<title=hello;touch should-not-run>") {
		t.Fatalf("fake ffmpeg args did not preserve shell-style quoted arg:\n%s", log)
	}
	if _, err := os.Stat(filepath.Join(dir, "should-not-run")); !os.IsNotExist(err) {
		t.Fatalf("unexpected shell side effect file exists or stat failed: %v", err)
	}
}
