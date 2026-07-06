package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadDBRecordsDownloadsAndFailures(t *testing.T) {
	db, err := OpenDownloadDB(filepath.Join(t.TempDir(), "amdl.db"))
	if err != nil {
		t.Fatalf("OpenDownloadDB: %v", err)
	}
	defer db.Close()

	item := DBItem{
		Storefront: "us",
		MediaType:  "song",
		AppleID:    "123",
		Codec:      "ALAC",
		Quality:    "24/96",
		SourceURL:  "https://music.apple.com/us/song/example/123",
		SavePath:   "/tmp/example.m4a",
	}

	if ok, _, err := db.Downloaded(item); err != nil || ok {
		t.Fatalf("Downloaded before record = (%v, %v), want false nil", ok, err)
	}
	if err := db.RecordFailure(item, errors.New("temporary failure")); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}
	failed, err := db.ListFailed()
	if err != nil {
		t.Fatalf("ListFailed: %v", err)
	}
	if len(failed) != 1 || failed[0].Attempts != 1 {
		t.Fatalf("failed = %#v, want one attempt", failed)
	}
	if err := db.RecordFailure(item, errors.New("still failing")); err != nil {
		t.Fatalf("RecordFailure second: %v", err)
	}
	failed, _ = db.ListFailed()
	if failed[0].Attempts != 2 {
		t.Fatalf("attempts = %d, want 2", failed[0].Attempts)
	}

	if err := db.RecordDownload(item); err != nil {
		t.Fatalf("RecordDownload: %v", err)
	}
	if ok, saved, err := db.Downloaded(item); err != nil || !ok || saved.SavePath != item.SavePath {
		t.Fatalf("Downloaded after record = (%v, %#v, %v)", ok, saved, err)
	}
	failed, err = db.ListFailed()
	if err != nil {
		t.Fatalf("ListFailed after success: %v", err)
	}
	if len(failed) != 0 {
		t.Fatalf("failed after success = %#v, want none", failed)
	}
}

func TestDownloadDBRepairURLs(t *testing.T) {
	db, err := OpenDownloadDB(filepath.Join(t.TempDir(), "amdl.db"))
	if err != nil {
		t.Fatalf("OpenDownloadDB: %v", err)
	}
	defer db.Close()

	item := DBItem{Storefront: "jp", MediaType: "album", AppleID: "456", Codec: "ALAC"}
	if err := db.RecordFailure(item, errors.New("failed")); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}
	urls, err := db.RepairURLs()
	if err != nil {
		t.Fatalf("RepairURLs: %v", err)
	}
	want := "https://music.apple.com/jp/album/1/456"
	if len(urls) != 1 || urls[0] != want {
		t.Fatalf("RepairURLs = %#v, want %q", urls, want)
	}
}

func TestDownloadDBRecordDownloadUpdatesQuality(t *testing.T) {
	db, err := OpenDownloadDB(filepath.Join(t.TempDir(), "amdl.db"))
	if err != nil {
		t.Fatalf("OpenDownloadDB: %v", err)
	}
	defer db.Close()

	item := DBItem{
		Storefront: "de",
		MediaType:  "song",
		AppleID:    "6780314951",
		Codec:      "ALAC",
		SourceURL:  "https://music.apple.com/de/song/example/6780314951",
		SavePath:   "AM-DL downloads/example.m4a",
	}
	if err := db.RecordDownload(item); err != nil {
		t.Fatalf("RecordDownload blank quality: %v", err)
	}
	item.Quality = "24B-96.0kHz"
	if err := db.RecordDownload(item); err != nil {
		t.Fatalf("RecordDownload updated quality: %v", err)
	}

	ok, saved, err := db.Downloaded(item)
	if err != nil || !ok {
		t.Fatalf("Downloaded = (%v, %#v, %v), want saved row", ok, saved, err)
	}
	if saved.Quality != "24B-96.0kHz" {
		t.Fatalf("saved.Quality = %q, want 24B-96.0kHz", saved.Quality)
	}
}

func TestDownloadDBConfiguresWALAndBusyTimeout(t *testing.T) {
	db, err := OpenDownloadDB(filepath.Join(t.TempDir(), "amdl.db"))
	if err != nil {
		t.Fatalf("OpenDownloadDB: %v", err)
	}
	defer db.Close()

	var journalMode string
	if err := db.db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("journal_mode pragma: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := db.db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("busy_timeout pragma: %v", err)
	}
	if busyTimeout < 5000 {
		t.Fatalf("busy_timeout = %d, want at least 5000", busyTimeout)
	}
}

func TestRecordFailureRedactsSecrets(t *testing.T) {
	oldDB := appDB
	oldConfig := Config
	oldNoDB := no_database
	t.Cleanup(func() {
		appDB = oldDB
		Config = oldConfig
		no_database = oldNoDB
	})

	db, err := OpenDownloadDB(filepath.Join(t.TempDir(), "amdl.db"))
	if err != nil {
		t.Fatalf("OpenDownloadDB: %v", err)
	}
	defer db.Close()

	appDB = db
	no_database = false
	Config = defaultConfig()
	Config.FailedDownloadsEnabled = true
	Config.AuthorizationToken = "secret-token"

	item := DBItem{Storefront: "us", MediaType: "song", AppleID: "123", Codec: "ALAC"}
	recordFailure(item, errors.New("failed with Bearer secret-token"))

	failed, err := db.ListFailed()
	if err != nil {
		t.Fatalf("ListFailed: %v", err)
	}
	if len(failed) != 1 {
		t.Fatalf("failed rows = %d, want 1", len(failed))
	}
	if strings.Contains(failed[0].Error, "secret-token") {
		t.Fatalf("failure error leaked token: %q", failed[0].Error)
	}
}
