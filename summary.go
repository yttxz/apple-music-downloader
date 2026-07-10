package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type SummaryCounts struct {
	Downloaded int `json:"downloaded"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}

type SummaryItem struct {
	Storefront string `json:"storefront"`
	MediaType  string `json:"media_type"`
	AppleID    string `json:"apple_id"`
	Codec      string `json:"codec"`
	Quality    string `json:"quality"`
	Path       string `json:"path,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Error      string `json:"error,omitempty"`
}

type RunSummary struct {
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Counts      SummaryCounts `json:"counts"`
	Downloaded  []SummaryItem `json:"downloaded"`
	Skipped     []SummaryItem `json:"skipped"`
	Failed      []SummaryItem `json:"failed"`

	mu sync.Mutex
}

func newRunSummary() *RunSummary {
	return &RunSummary{StartedAt: time.Now()}
}

func (s *RunSummary) RecordDownloaded(item DBItem) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeItemLocked(item)
	s.Downloaded = append(s.Downloaded, summaryItemFromDB(item, "", ""))
	s.syncCountsLocked()
}

func (s *RunSummary) RecordSkipped(item DBItem, reason string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeItemLocked(item)
	s.Skipped = append(s.Skipped, summaryItemFromDB(item, reason, ""))
	s.syncCountsLocked()
}

func (s *RunSummary) RecordFailed(item DBItem, err error) {
	if s == nil {
		return
	}
	errText := item.Error
	if err != nil {
		errText = err.Error()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeItemLocked(item)
	s.Failed = append(s.Failed, summaryItemFromDB(item, "", errText))
	s.syncCountsLocked()
}

func (s *RunSummary) removeItemLocked(item DBItem) {
	if item.AppleID == "" {
		return
	}
	s.Downloaded = removeSummaryItem(s.Downloaded, item)
	s.Skipped = removeSummaryItem(s.Skipped, item)
	s.Failed = removeSummaryItem(s.Failed, item)
}

func (s *RunSummary) syncCountsLocked() {
	s.Counts = SummaryCounts{
		Downloaded: len(s.Downloaded),
		Skipped:    len(s.Skipped),
		Failed:     len(s.Failed),
	}
}

func removeSummaryItem(items []SummaryItem, target DBItem) []SummaryItem {
	kept := items[:0]
	for _, item := range items {
		if item.Storefront == target.Storefront &&
			item.MediaType == target.MediaType &&
			item.AppleID == target.AppleID &&
			item.Codec == target.Codec {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

func (s *RunSummary) PrintJSON() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.CompletedAt = time.Now()
	data, err := json.MarshalIndent(s, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, string(data))
	return err
}

func summaryItemFromDB(item DBItem, reason string, errText string) SummaryItem {
	return SummaryItem{
		Storefront: item.Storefront,
		MediaType:  item.MediaType,
		AppleID:    item.AppleID,
		Codec:      item.Codec,
		Quality:    item.Quality,
		Path:       item.SavePath,
		Reason:     reason,
		Error:      errText,
	}
}
