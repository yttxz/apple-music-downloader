package main

import (
	ampapi "amdl/internal/amp-api"
	"amdl/internal/download"
	widevinerip "amdl/internal/widevine-rip"
	"amdl/internal/wrapper"
	"amdl/utils/httpx"

	"net/http"
	"time"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

var artworkHTTPClient = &http.Client{
	Timeout: 180 * time.Second,
}

func configureHTTPClients() error {
	timeout := time.Duration(Config.RequestTimeoutSeconds) * time.Second
	artworkTimeout := time.Duration(Config.ArtworkTimeoutSeconds) * time.Second
	backoff := time.Duration(Config.RetryBackoffMS) * time.Millisecond
	retryConfig := httpx.RetryConfig{
		Timeout:           timeout,
		RetryCount:        Config.RetryCount,
		RetryBackoff:      backoff,
		RequestsPerMinute: Config.RequestsPerMinute,
	}
	if err := download.Init(Config.Proxy); err != nil {
		return err
	}
	httpClient = httpx.NewClient(retryConfig)
	httpClient.Transport.(*httpx.RetryTransport).Base = download.Client.Transport
	artworkRetryConfig := retryConfig
	artworkRetryConfig.Timeout = artworkTimeout
	artworkHTTPClient = httpx.NewClient(artworkRetryConfig)
	artworkHTTPClient.Transport.(*httpx.RetryTransport).Base = download.Client.Transport
	ampapi.ConfigureHTTPClientWithTransport(retryConfig, download.Client.Transport)
	wrapper.ControlClient = &http.Client{Timeout: timeout, Transport: download.Client.Transport}
	widevinerip.ConfigureStreamDownloads(Config.MVSegmentConcurrency, Config.RetryCount)
	return nil
}
