package main

import (
	"errors"
	"testing"
)

func summaryTestItem(codec string) DBItem {
	return DBItem{
		Storefront: "us",
		MediaType:  "song",
		AppleID:    "123",
		Codec:      codec,
		Quality:    "24B-96.0kHz",
	}
}

func TestRunSummarySuccessReplacesFailure(t *testing.T) {
	summary := newRunSummary()
	item := summaryTestItem("ALAC")

	summary.RecordFailed(item, errors.New("temporary failure"))
	item.SavePath = "/music/song.m4a"
	summary.RecordDownloaded(item)

	if summary.Counts != (SummaryCounts{Downloaded: 1}) {
		t.Fatalf("counts = %#v, want one downloaded", summary.Counts)
	}
	if len(summary.Downloaded) != 1 || len(summary.Failed) != 0 {
		t.Fatalf("downloaded=%#v failed=%#v", summary.Downloaded, summary.Failed)
	}
}

func TestRunSummaryRepeatedFailureKeepsLatestResult(t *testing.T) {
	summary := newRunSummary()
	item := summaryTestItem("ALAC")

	summary.RecordFailed(item, errors.New("first failure"))
	summary.RecordFailed(item, errors.New("last failure"))

	if summary.Counts != (SummaryCounts{Failed: 1}) || len(summary.Failed) != 1 {
		t.Fatalf("counts=%#v failed=%#v", summary.Counts, summary.Failed)
	}
	if summary.Failed[0].Error != "last failure" {
		t.Fatalf("failure error = %q, want last failure", summary.Failed[0].Error)
	}
}

func TestRunSummarySuccessReplacesSkip(t *testing.T) {
	summary := newRunSummary()
	item := summaryTestItem("ALAC")

	summary.RecordSkipped(item, "already-downloaded")
	summary.RecordDownloaded(item)

	if summary.Counts != (SummaryCounts{Downloaded: 1}) || len(summary.Skipped) != 0 {
		t.Fatalf("counts=%#v skipped=%#v", summary.Counts, summary.Skipped)
	}
}

func TestRunSummaryKeepsDifferentCodecsSeparate(t *testing.T) {
	summary := newRunSummary()
	summary.RecordFailed(summaryTestItem("ALAC"), errors.New("lossless failed"))
	summary.RecordDownloaded(summaryTestItem("AAC"))

	if summary.Counts != (SummaryCounts{Downloaded: 1, Failed: 1}) {
		t.Fatalf("counts = %#v, want one downloaded and one failed", summary.Counts)
	}
}

func TestRunSummaryDoesNotReconcileIdentitylessItems(t *testing.T) {
	summary := newRunSummary()
	item := summaryTestItem("ALAC")
	item.AppleID = ""

	summary.RecordFailed(item, errors.New("failure"))
	summary.RecordDownloaded(item)

	if summary.Counts != (SummaryCounts{Downloaded: 1, Failed: 1}) {
		t.Fatalf("counts = %#v, want append-only identityless results", summary.Counts)
	}
}
