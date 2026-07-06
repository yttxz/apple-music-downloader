package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type DBItem struct {
	Storefront string `json:"storefront"`
	MediaType  string `json:"media_type"`
	AppleID    string `json:"apple_id"`
	Codec      string `json:"codec"`
	Quality    string `json:"quality"`
	SourceURL  string `json:"source_url"`
	SavePath   string `json:"save_path,omitempty"`
	Error      string `json:"error,omitempty"`
	Attempts   int    `json:"attempt_count,omitempty"`
}

type DownloadDB struct {
	mu sync.Mutex
	db *sql.DB
}

func OpenDownloadDB(path string) (*DownloadDB, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := ensureDir(dir); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	state := &DownloadDB{db: db}
	if err := state.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return state, nil
}

func (d *DownloadDB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}

func (d *DownloadDB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS downloads (
			storefront TEXT NOT NULL,
			media_type TEXT NOT NULL,
			apple_id TEXT NOT NULL,
			codec TEXT NOT NULL,
			quality TEXT NOT NULL,
			source_url TEXT NOT NULL,
			save_path TEXT NOT NULL,
			completed_at TEXT NOT NULL,
			UNIQUE(storefront, media_type, apple_id, codec)
		)`,
		`CREATE TABLE IF NOT EXISTS failed_downloads (
			storefront TEXT NOT NULL,
			media_type TEXT NOT NULL,
			apple_id TEXT NOT NULL,
			codec TEXT NOT NULL,
			source_url TEXT NOT NULL,
			error TEXT NOT NULL,
			attempt_count INTEGER NOT NULL DEFAULT 1,
			last_attempt_at TEXT NOT NULL,
			UNIQUE(storefront, media_type, apple_id, codec)
		)`,
		`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, ?)`,
	}
	now := time.Now().Format(time.RFC3339)
	for _, stmt := range stmts {
		if _, err := d.db.Exec(stmt, now); err != nil {
			return err
		}
	}
	return nil
}

func (d *DownloadDB) Downloaded(item DBItem) (bool, DBItem, error) {
	if d == nil || d.db == nil || item.AppleID == "" {
		return false, DBItem{}, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	row := d.db.QueryRow(
		`SELECT save_path, quality, source_url FROM downloads WHERE storefront=? AND media_type=? AND apple_id=? AND codec=?`,
		item.Storefront, item.MediaType, item.AppleID, item.Codec,
	)
	var saved DBItem
	saved = item
	if err := row.Scan(&saved.SavePath, &saved.Quality, &saved.SourceURL); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, DBItem{}, nil
		}
		return false, DBItem{}, err
	}
	return true, saved, nil
}

func (d *DownloadDB) RecordDownload(item DBItem) error {
	if d == nil || d.db == nil || item.AppleID == "" {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.db.Exec(
		`INSERT INTO downloads(storefront, media_type, apple_id, codec, quality, source_url, save_path, completed_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(storefront, media_type, apple_id, codec) DO UPDATE SET
			quality=excluded.quality,
			source_url=excluded.source_url,
			save_path=excluded.save_path,
			completed_at=excluded.completed_at`,
		item.Storefront, item.MediaType, item.AppleID, item.Codec, item.Quality, item.SourceURL, item.SavePath, time.Now().Format(time.RFC3339),
	)
	if err != nil {
		return err
	}
	_, _ = d.db.Exec(
		`DELETE FROM failed_downloads WHERE storefront=? AND media_type=? AND apple_id=? AND codec=?`,
		item.Storefront, item.MediaType, item.AppleID, item.Codec,
	)
	return nil
}

func (d *DownloadDB) RecordFailure(item DBItem, err error) error {
	if d == nil || d.db == nil || item.AppleID == "" {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	if errText == "" {
		errText = item.Error
	}
	if errText == "" {
		errText = "unknown error"
	}
	_, execErr := d.db.Exec(
		`INSERT INTO failed_downloads(storefront, media_type, apple_id, codec, source_url, error, attempt_count, last_attempt_at)
		 VALUES(?, ?, ?, ?, ?, ?, 1, ?)
		 ON CONFLICT(storefront, media_type, apple_id, codec) DO UPDATE SET
			source_url=excluded.source_url,
			error=excluded.error,
			attempt_count=failed_downloads.attempt_count + 1,
			last_attempt_at=excluded.last_attempt_at`,
		item.Storefront, item.MediaType, item.AppleID, item.Codec, item.SourceURL, errText, time.Now().Format(time.RFC3339),
	)
	return execErr
}

func (d *DownloadDB) ListDownloads() ([]DBItem, error) {
	if d == nil || d.db == nil {
		return nil, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	rows, err := d.db.Query(`SELECT storefront, media_type, apple_id, codec, quality, source_url, save_path FROM downloads ORDER BY completed_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DBItem
	for rows.Next() {
		var item DBItem
		if err := rows.Scan(&item.Storefront, &item.MediaType, &item.AppleID, &item.Codec, &item.Quality, &item.SourceURL, &item.SavePath); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *DownloadDB) ListFailed() ([]DBItem, error) {
	if d == nil || d.db == nil {
		return nil, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	rows, err := d.db.Query(`SELECT storefront, media_type, apple_id, codec, source_url, error, attempt_count FROM failed_downloads ORDER BY last_attempt_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DBItem
	for rows.Next() {
		var item DBItem
		if err := rows.Scan(&item.Storefront, &item.MediaType, &item.AppleID, &item.Codec, &item.SourceURL, &item.Error, &item.Attempts); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *DownloadDB) RepairURLs() ([]string, error) {
	failed, err := d.ListFailed()
	if err != nil {
		return nil, err
	}
	urls := make([]string, 0, len(failed))
	seen := map[string]bool{}
	for _, item := range failed {
		u := item.SourceURL
		if u == "" {
			u = urlForDBItem(item)
		}
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		urls = append(urls, u)
	}
	return urls, nil
}

func urlForDBItem(item DBItem) string {
	storefront := item.Storefront
	if storefront == "" {
		storefront = Config.Storefront
	}
	switch item.MediaType {
	case "album":
		return fmt.Sprintf("https://music.apple.com/%s/album/1/%s", storefront, item.AppleID)
	case "song", "track":
		return fmt.Sprintf("https://music.apple.com/%s/song/1/%s", storefront, item.AppleID)
	case "playlist":
		return fmt.Sprintf("https://music.apple.com/%s/playlist/1/%s", storefront, item.AppleID)
	case "station":
		return fmt.Sprintf("https://music.apple.com/%s/station/1/%s", storefront, item.AppleID)
	case "music-video":
		return fmt.Sprintf("https://music.apple.com/%s/music-video/1/%s", storefront, item.AppleID)
	default:
		return ""
	}
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}
