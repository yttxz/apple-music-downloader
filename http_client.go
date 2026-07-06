package main

import (
	"apple-music-downloader/utils/ampapi"
	"apple-music-downloader/utils/httpx"
	"apple-music-downloader/utils/lyrics"
	"apple-music-downloader/utils/runv3"
	"net/http"
	"time"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

func configureHTTPClients() {
	timeout := time.Duration(Config.RequestTimeoutSeconds) * time.Second
	backoff := time.Duration(Config.RetryBackoffMS) * time.Millisecond
	retryConfig := httpx.RetryConfig{
		Timeout:           timeout,
		RetryCount:        Config.RetryCount,
		RetryBackoff:      backoff,
		RequestsPerMinute: Config.RequestsPerMinute,
	}
	httpClient = httpx.NewClient(retryConfig)
	ampapi.ConfigureHTTPClient(retryConfig)
	lyrics.ConfigureHTTPClient(retryConfig)
	runv3.ConfigureHTTPClient(retryConfig)
	runv3.ConfigureMVSegmentConcurrency(Config.MVSegmentConcurrency)
}
