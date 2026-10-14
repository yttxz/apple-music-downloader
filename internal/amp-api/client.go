package ampapi

import (
	"amdl/internal/download"
	"amdl/utils/httpx"
	"errors"
	"net/http"
)

var ErrNoData = errors.New("response contains no data")

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"

var apiClient *http.Client

func ConfigureHTTPClient(config httpx.RetryConfig) {
	apiClient = httpx.NewClient(config)
}

func setCatalogHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Origin", "https://music.apple.com")
}

// The standalone upstream runner initializes the shared download client.
// The fork configures a bounded catalog client separately from large media streams.
func catalogClient() *http.Client {
	if apiClient != nil {
		return apiClient
	}
	return download.Client
}

func ConfigureHTTPClientWithTransport(config httpx.RetryConfig, base http.RoundTripper) {
	ConfigureHTTPClient(config)
	apiClient.Transport.(*httpx.RetryTransport).Base = base
}
