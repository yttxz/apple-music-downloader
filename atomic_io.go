package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func writeFileAtomic(path string, perm os.FileMode, write func(*os.File) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := write(tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func writeBytesAtomic(path string, data []byte, perm os.FileMode) error {
	return writeFileAtomic(path, perm, func(f *os.File) error {
		_, err := f.Write(data)
		return err
	})
}

func writeStringAtomic(path string, data string, perm os.FileMode) error {
	return writeFileAtomic(path, perm, func(f *os.File) error {
		_, err := f.WriteString(data)
		return err
	})
}

func copyToFileAtomic(path string, perm os.FileMode, r io.Reader) error {
	return writeFileAtomic(path, perm, func(f *os.File) error {
		_, err := io.Copy(f, r)
		return err
	})
}

func tempPathInDestination(finalPath string) (string, func(bool), error) {
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return "", nil, err
	}
	base := filepath.Base(finalPath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	// filepath.Ext treats a dotfile such as ".cover" as having an extension
	// equal to its entire name. Keep that name as the stem instead.
	if stem == "" {
		stem = base
		ext = ""
	}
	// Keep the final extension at the end of the temporary name. Tools such as
	// FFmpeg infer the output container from the path extension.
	tmpPattern := "." + stem + ".tmp-*" + ext
	tmp, err := os.CreateTemp(filepath.Dir(finalPath), tmpPattern)
	if err != nil {
		return "", nil, err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", nil, err
	}
	cleanup := func(commit bool) {
		if !commit {
			_ = os.Remove(tmpPath)
		}
	}
	return tmpPath, cleanup, nil
}

func renameTempIntoPlace(tempPath, finalPath string) error {
	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("replace %s: %w", finalPath, err)
	}
	return nil
}

func runCommandWithAtomicOutput(finalPath string, build func(tempPath string) *exec.Cmd) error {
	tempPath, cleanup, err := tempPathInDestination(finalPath)
	if err != nil {
		return err
	}
	defer cleanup(false)
	_ = os.Remove(tempPath)
	cmd := build(tempPath)
	var stderr bytes.Buffer
	if cmd.Stderr == nil {
		cmd.Stderr = &stderr
	}
	if err := cmd.Run(); err != nil {
		if diagnostic := strings.TrimSpace(stderr.String()); diagnostic != "" {
			return fmt.Errorf("%w: %s", err, diagnostic)
		}
		return err
	}
	if err := renameTempIntoPlace(tempPath, finalPath); err != nil {
		return err
	}
	cleanup(true)
	return nil
}
