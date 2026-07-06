package main

import (
	"errors"
	"os"
	"path/filepath"
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
