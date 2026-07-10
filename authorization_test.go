package main

import (
	"errors"
	"testing"
)

func TestResolveAuthorizationTokenPrefersFreshToken(t *testing.T) {
	token, err := resolveAuthorizationToken("configured-token", func() (string, error) {
		return "Bearer fresh-token", nil
	})
	if err != nil {
		t.Fatalf("resolveAuthorizationToken: %v", err)
	}
	if token != "fresh-token" {
		t.Fatalf("token = %q, want fresh-token", token)
	}
}

func TestResolveAuthorizationTokenFallsBackToConfiguredToken(t *testing.T) {
	token, err := resolveAuthorizationToken(" Bearer configured-token ", func() (string, error) {
		return "", errors.New("fresh token unavailable")
	})
	if err != nil {
		t.Fatalf("resolveAuthorizationToken: %v", err)
	}
	if token != "configured-token" {
		t.Fatalf("token = %q, want configured-token", token)
	}
}

func TestResolveAuthorizationTokenRejectsEmptySources(t *testing.T) {
	token, err := resolveAuthorizationToken("your-authorization-token", func() (string, error) {
		return "", errors.New("fetch failed with secret details")
	})
	if token != "" {
		t.Fatalf("token = %q, want empty", token)
	}
	if !errors.Is(err, errAuthorizationTokenUnavailable) {
		t.Fatalf("error = %v, want errAuthorizationTokenUnavailable", err)
	}
}

func TestResolveAuthorizationTokenTreatsEmptyFreshTokenAsFailure(t *testing.T) {
	token, err := resolveAuthorizationToken("configured-token", func() (string, error) {
		return "   ", nil
	})
	if err != nil || token != "configured-token" {
		t.Fatalf("resolveAuthorizationToken = (%q, %v), want configured-token", token, err)
	}
}
