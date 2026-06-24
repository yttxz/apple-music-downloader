package ampapi

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withMockClient(t *testing.T, fn roundTripFunc) {
	t.Helper()
	oldClient := apiClient
	apiClient = &http.Client{Transport: fn}
	t.Cleanup(func() {
		apiClient = oldClient
	})
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGetSongRespReturnsErrorOnEmptyData(t *testing.T) {
	withMockClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":[]}`), nil
	})

	_, err := GetSongResp("us", "123", "en-US", "token")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "song 123 not found") {
		t.Fatalf("error = %q, want song not found", err.Error())
	}
}

func TestGetAlbumRespReturnsErrorOnEmptyData(t *testing.T) {
	withMockClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":[]}`), nil
	})

	_, err := GetAlbumResp("us", "456", "en-US", "token")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "album 456 not found") {
		t.Fatalf("error = %q, want album not found", err.Error())
	}
}

func TestGetPlaylistRespReturnsErrorOnEmptyData(t *testing.T) {
	withMockClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":[]}`), nil
	})

	_, err := GetPlaylistResp("us", "pl.abc", "en-US", "token")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "playlist pl.abc not found") {
		t.Fatalf("error = %q, want playlist not found", err.Error())
	}
}

func TestGetStationAssetsReturnsErrorWithoutAssets(t *testing.T) {
	withMockClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"results":{"assets":[]}}`), nil
	})

	_, _, err := GetStationAssetsUrlAndServerUrl("ra.123", "media-user-token", "token")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "station assets not found") {
		t.Fatalf("error = %q, want station assets not found", err.Error())
	}
}

func TestGetAlbumRespHandlesEmptyPaginatedTrackPage(t *testing.T) {
	requests := 0
	withMockClient(t, func(req *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return jsonResponse(http.StatusOK, `{"data":[{"id":"456","relationships":{"tracks":{"next":"/v1/catalog/us/albums/456/tracks?offset=100","data":[]}}}]}`), nil
		}
		return jsonResponse(http.StatusOK, `{"data":[]}`), nil
	})

	resp, err := GetAlbumResp("us", "456", "en-US", "token")
	if err != nil {
		t.Fatalf("GetAlbumResp returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("len(resp.Data) = %d, want 1", len(resp.Data))
	}
}
