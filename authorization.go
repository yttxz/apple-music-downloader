package main

import (
	"errors"
	"strings"
)

var errAuthorizationTokenUnavailable = errors.New("authorization token unavailable")

func resolveAuthorizationToken(configured string, fetch func() (string, error)) (string, error) {
	if fetch != nil {
		if token, err := fetch(); err == nil {
			if token = cleanConfiguredToken(token); strings.TrimSpace(token) != "" {
				return token, nil
			}
		}
	}

	if token := cleanConfiguredToken(configured); token != "" {
		return token, nil
	}
	return "", errAuthorizationTokenUnavailable
}
