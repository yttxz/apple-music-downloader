package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteFileAtomicSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "file.txt")
	if err := writeStringAtomic(path, "hello", 0644); err != nil {
		t.Fatalf("writeStringAtomic: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("data = %q, want hello", string(data))
	}
}

func TestWriteFileAtomicFailureDoesNotCreateFinalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	err := writeFileAtomic(path, 0644, func(f *os.File) error {
		_, _ = f.WriteString("partial")
		return errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("final file stat err = %v, want not exists", statErr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temporary file was not cleaned up: %s", entry.Name())
		}
	}
}

func TestTempPathInDestinationPreservesOutputExtension(t *testing.T) {
	dir := t.TempDir()
	for _, finalName := range []string{"artwork.mp4", "folder.gif", "track.flac", "without-extension"} {
		finalPath := filepath.Join(dir, finalName)
		tempPath, cleanup, err := tempPathInDestination(finalPath)
		if err != nil {
			t.Fatalf("tempPathInDestination(%q): %v", finalName, err)
		}
		if filepath.Dir(tempPath) != dir {
			t.Errorf("temp path directory = %q, want %q", filepath.Dir(tempPath), dir)
		}
		if ext := filepath.Ext(finalPath); ext != "" && filepath.Ext(tempPath) != ext {
			t.Errorf("temp path %q extension = %q, want %q", tempPath, filepath.Ext(tempPath), ext)
		}
		cleanup(false)
		if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
			t.Errorf("temporary path %q still exists after cleanup: %v", tempPath, err)
		}
	}
}

func TestRunCommandWithAtomicOutputIncludesStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command test is Unix-specific")
	}

	outputPath := filepath.Join(t.TempDir(), "output.mp4")
	err := runCommandWithAtomicOutput(outputPath, func(tempPath string) *exec.Cmd {
		return exec.Command("sh", "-c", "printf '%s' diagnostic >&2; exit 7")
	})
	if err == nil {
		t.Fatal("expected command failure")
	}
	if !strings.Contains(err.Error(), "diagnostic") {
		t.Fatalf("error = %q, want stderr diagnostic", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("final output stat err = %v, want not exists", statErr)
	}
}

func TestRunCommandWithAtomicOutputWorksWithFFmpeg(t *testing.T) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.mp4")
	generate := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=black:s=16x16:d=0.1", "-an", "-c:v", "libx264", "-pix_fmt", "yuv420p", inputPath)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate input: %v\n%s", err, output)
	}

	outputPath := filepath.Join(dir, "output.mp4")
	if err := runCommandWithAtomicOutput(outputPath, func(tempPath string) *exec.Cmd {
		return exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-y", "-i", inputPath, "-c", "copy", tempPath)
	}); err != nil {
		t.Fatalf("copy media through atomic output: %v", err)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output is empty")
	}
	probe := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-i", outputPath, "-f", "null", "-")
	if output, err := probe.CombinedOutput(); err != nil {
		t.Fatalf("probe output: %v\n%s", err, output)
	}
}
