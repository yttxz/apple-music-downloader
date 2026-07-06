package main

import (
	"errors"
	"regexp"
	"strings"
)

var (
	bearerTokenPattern  = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._~+/=-]+`)
	querySecretPattern  = regexp.MustCompile(`(?i)([?&](?:authorization|media-user-token|token|access_token|id_token|secret|signature|sig|key)=)[^&\s]+`)
	headerSecretPattern = regexp.MustCompile(`(?i)\b(authorization|media-user-token|token|access-token|secret|signature|sig|key)(\s*[:=]\s*)[^\s,;]+`)
)

func redactText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	value = bearerTokenPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	value = querySecretPattern.ReplaceAllString(value, "${1}[REDACTED]")
	value = headerSecretPattern.ReplaceAllString(value, "${1}${2}[REDACTED]")
	for _, secret := range []string{Config.MediaUserToken, Config.AuthorizationToken} {
		secret = strings.TrimSpace(strings.TrimPrefix(secret, "Bearer "))
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}

func redactError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(redactText(err.Error()))
}
