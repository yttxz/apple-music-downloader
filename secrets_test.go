package main

import (
	"errors"
	"strings"
	"testing"
)

func TestRedactTextHidesTokens(t *testing.T) {
	oldConfig := Config
	t.Cleanup(func() { Config = oldConfig })
	Config.MediaUserToken = "media-secret"
	Config.AuthorizationToken = "auth-secret"

	input := "Bearer auth-secret https://example.test/path?token=abc&media-user-token=media-secret authorization: auth-secret"
	got := redactText(input)
	for _, secret := range []string{"auth-secret", "media-secret", "token=abc"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redactText leaked %q in %q", secret, got)
		}
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("redactText = %q, want redaction marker", got)
	}
}

func TestRedactError(t *testing.T) {
	err := redactError(errors.New("request failed: token=secret"))
	if err == nil {
		t.Fatal("redactError returned nil")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("redactError leaked secret: %q", err.Error())
	}
}
