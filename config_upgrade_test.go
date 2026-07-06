package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingManagedConfigKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("database-enabled: true\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	missing, err := missingManagedConfigKeys(path)
	if err != nil {
		t.Fatalf("missingManagedConfigKeys: %v", err)
	}
	if len(missing) == 0 {
		t.Fatal("missingManagedConfigKeys returned no missing keys")
	}
	if missing[0] != "database-path" {
		t.Fatalf("first missing key = %q, want database-path", missing[0])
	}
}

func TestMissingManagedConfigKeysComplete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	var data []byte
	for _, key := range managedConfigOrder {
		data = append(data, []byte(key+": null\n")...)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	missing, err := missingManagedConfigKeys(path)
	if err != nil {
		t.Fatalf("missingManagedConfigKeys: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing = %#v, want none", missing)
	}
}
