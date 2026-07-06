package httpx

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRetryTransportRetriesTransientGET(t *testing.T) {
	attempts := 0
	client := &http.Client{
		Timeout: time.Second,
		Transport: &RetryTransport{
			Base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				attempts++
				status := http.StatusOK
				if attempts == 1 {
					status = http.StatusServiceUnavailable
				}
				return &http.Response{
					StatusCode: status,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			}),
			RetryCount:   1,
			RetryBackoff: time.Millisecond,
		},
	}
	resp, err := client.Get("https://example.invalid/test")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestRetryTransportDoesNotRetryPOST(t *testing.T) {
	attempts := 0
	client := &http.Client{
		Timeout: time.Second,
		Transport: &RetryTransport{
			Base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				attempts++
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			}),
			RetryCount:   3,
			RetryBackoff: time.Millisecond,
		},
	}
	resp, err := client.Post("https://example.invalid/test", "text/plain", nil)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	_ = resp.Body.Close()
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestNewClientSetsTimeout(t *testing.T) {
	client := NewClient(RetryConfig{
		Timeout:      2 * time.Second,
		RetryCount:   1,
		RetryBackoff: time.Millisecond,
	})
	if client.Timeout != 2*time.Second {
		t.Fatalf("timeout = %s, want 2s", client.Timeout)
	}
}
