package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	tmp, err := os.CreateTemp(filepath.Dir(finalPath), "."+filepath.Base(finalPath)+".tmp-*")
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
	if err := cmd.Run(); err != nil {
		return err
	}
	if err := renameTempIntoPlace(tempPath, finalPath); err != nil {
		return err
	}
	cleanup(true)
	return nil
}
