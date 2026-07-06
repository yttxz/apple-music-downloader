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
	s.Counts.Downloaded++
	s.Downloaded = append(s.Downloaded, summaryItemFromDB(item, "", ""))
}

func (s *RunSummary) RecordSkipped(item DBItem, reason string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Counts.Skipped++
	s.Skipped = append(s.Skipped, summaryItemFromDB(item, reason, ""))
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
	s.Counts.Failed++
	s.Failed = append(s.Failed, summaryItemFromDB(item, "", errText))
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
