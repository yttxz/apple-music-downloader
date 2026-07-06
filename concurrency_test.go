package main

import "testing"

func TestEffectiveDownloadConcurrency(t *testing.T) {
	oldConfig := Config
	oldSelect := dl_select
	t.Cleanup(func() {
		Config = oldConfig
		dl_select = oldSelect
	})

	Config = defaultConfig()
	Config.DownloadConcurrency = 4
	dl_select = false
	if got := effectiveDownloadConcurrency(3); got != 3 {
		t.Fatalf("effectiveDownloadConcurrency = %d, want queue cap 3", got)
	}

	dl_select = true
	if got := effectiveDownloadConcurrency(3); got != 1 {
		t.Fatalf("effectiveDownloadConcurrency with select = %d, want 1", got)
	}
}
