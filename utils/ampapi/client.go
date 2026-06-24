package ampapi

import (
	"errors"
	"net/http"
	"time"
)

var ErrNoData = errors.New("response contains no data")

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"

var apiClient = &http.Client{
	Timeout: 30 * time.Second,
}

func setCatalogHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Origin", "https://music.apple.com")
}
