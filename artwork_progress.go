package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const artworkProgressThreshold int64 = 1 << 20

var (
	artworkProgressInterval           = 5 * time.Second
	artworkProgressOutput   io.Writer = os.Stdout
	artworkProgressMu       sync.Mutex
)

type artworkProgressReader struct {
	reader      io.Reader
	label       string
	total       int64
	downloaded  int64
	nextReport  time.Time
	active      bool
	finished    bool
	now         func() time.Time
	writeReport func(string, int64, int64, bool)
}

func newArtworkProgressReader(reader io.Reader, total int64, label string) *artworkProgressReader {
	p := &artworkProgressReader{
		reader:      reader,
		label:       label,
		total:       total,
		active:      total < 0 || total >= artworkProgressThreshold,
		now:         time.Now,
		writeReport: printArtworkProgress,
	}
	if p.active {
		now := p.now()
		p.nextReport = now.Add(artworkProgressInterval)
		p.writeReport(p.label, 0, p.total, false)
	}
	return p
}

func (p *artworkProgressReader) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)
	p.downloaded += int64(n)
	if p.active && !p.finished {
		now := p.now()
		complete := err == io.EOF || (p.total >= 0 && p.downloaded >= p.total)
		if complete || !now.Before(p.nextReport) {
			p.writeReport(p.label, p.downloaded, p.total, complete)
			p.nextReport = now.Add(artworkProgressInterval)
			p.finished = complete
		}
	}
	return n, err
}

func (p *artworkProgressReader) Finish() {
	if !p.active || p.finished {
		return
	}
	p.writeReport(p.label, p.downloaded, p.total, true)
	p.finished = true
}

func printArtworkProgress(label string, downloaded, total int64, complete bool) {
	artworkProgressMu.Lock()
	defer artworkProgressMu.Unlock()

	status := formatArtworkBytes(downloaded)
	if total > 0 {
		percent := downloaded * 100 / total
		if percent > 100 {
			percent = 100
		}
		status = fmt.Sprintf("%s / %s (%d%%)", status, formatArtworkBytes(total), percent)
	}
	if complete {
		fmt.Fprintf(artworkProgressOutput, "%s downloaded: %s\n", label, status)
		return
	}
	fmt.Fprintf(artworkProgressOutput, "%s downloading: %s\n", label, status)
}

func formatArtworkBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KiB", "MiB", "GiB"}
	for _, suffix := range units {
		value /= unit
		if value < unit || suffix == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%d B", size)
}
