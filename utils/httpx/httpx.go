package httpx

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"time"
)

type RetryConfig struct {
	Timeout           time.Duration
	RetryCount        int
	RetryBackoff      time.Duration
	RequestsPerMinute int
}

type RetryTransport struct {
	Base              http.RoundTripper
	RetryCount        int
	RetryBackoff      time.Duration
	RequestsPerMinute int

	mu       sync.Mutex
	nextSlot time.Time
}

func NewClient(config RetryConfig) *http.Client {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &RetryTransport{
			Base:              http.DefaultTransport,
			RetryCount:        config.RetryCount,
			RetryBackoff:      config.RetryBackoff,
			RequestsPerMinute: config.RequestsPerMinute,
		},
	}
}

func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return t.base().RoundTrip(req)
	}

	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
	}

	attempts := t.RetryCount + 1
	if attempts < 1 {
		attempts = 1
	}
	backoff := t.RetryBackoff
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}

	var lastResp *http.Response
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff * time.Duration(attempt))
		}
		t.waitForRateLimit(req)

		nextReq := req.Clone(req.Context())
		if body != nil {
			nextReq.Body = io.NopCloser(bytes.NewReader(body))
		}
		resp, err := t.base().RoundTrip(nextReq)
		if err == nil && !isTransientStatus(resp.StatusCode) {
			return resp, nil
		}
		if attempt == attempts-1 {
			return resp, err
		}
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		lastResp = resp
		lastErr = err
	}
	if lastResp != nil && lastErr == nil {
		return lastResp, nil
	}
	return nil, lastErr
}

func (t *RetryTransport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func (t *RetryTransport) waitForRateLimit(req *http.Request) {
	if t.RequestsPerMinute <= 0 {
		return
	}
	interval := time.Minute / time.Duration(t.RequestsPerMinute)
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if now.Before(t.nextSlot) {
		timer := time.NewTimer(t.nextSlot.Sub(now))
		t.mu.Unlock()
		select {
		case <-timer.C:
		case <-req.Context().Done():
			timer.Stop()
		}
		t.mu.Lock()
		now = time.Now()
	}
	t.nextSlot = now.Add(interval)
}

func isTransientStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
