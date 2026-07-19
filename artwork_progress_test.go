package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestArtworkProgressReaderReportsLargeTransfer(t *testing.T) {
	oldOutput := artworkProgressOutput
	oldInterval := artworkProgressInterval
	t.Cleanup(func() {
		artworkProgressOutput = oldOutput
		artworkProgressInterval = oldInterval
	})

	var output bytes.Buffer
	artworkProgressOutput = &output
	artworkProgressInterval = 5 * time.Second

	total := int64(2 << 20)
	progress := newArtworkProgressReader(io.LimitReader(zeroReader{}, total), total, "Album artwork")
	progress.nextReport = time.Unix(1, 0)
	progress.now = func() time.Time { return time.Unix(10, 0) }

	if _, err := io.Copy(io.Discard, progress); err != nil {
		t.Fatal(err)
	}
	progress.Finish()

	text := output.String()
	if !strings.Contains(text, "Album artwork downloading: 0 B / 2.0 MiB (0%)") {
		t.Fatalf("missing initial progress in %q", text)
	}
	if !strings.Contains(text, "Album artwork downloaded: 2.0 MiB / 2.0 MiB (100%)") {
		t.Fatalf("missing completion progress in %q", text)
	}
}

func TestArtworkProgressReaderSkipsSmallTransfer(t *testing.T) {
	var reports int
	progress := newArtworkProgressReader(strings.NewReader("small"), 5, "Artwork")
	progress.writeReport = func(string, int64, int64, bool) { reports++ }
	if _, err := io.Copy(io.Discard, progress); err != nil {
		t.Fatal(err)
	}
	progress.Finish()
	if reports != 0 {
		t.Fatalf("small transfer reports = %d, want 0", reports)
	}
}

type zeroReader struct{}

func (zeroReader) Read(buf []byte) (int, error) {
	for i := range buf {
		buf[i] = 0
	}
	return len(buf), nil
}
