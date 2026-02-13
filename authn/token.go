package authn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// TokenProvider retrieves Azure access tokens for the given scope.
type TokenProvider interface {
	GetToken(scope string) (string, time.Time, error)
}

// AzdTokenProvider implements TokenProvider by calling the azd CLI.
type AzdTokenProvider struct{}

// azdTokenResponse represents the JSON response from the azd auth token command.
type azdTokenResponse struct {
	Token     string `json:"token"`
	ExpiresOn string `json:"expiresOn"`
}

// TokenResponse is the HTTP API response returned by the token server.
type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresOn string `json:"expiresOn"`
}

// TokenRequest is the HTTP API request accepted by the token server.
type TokenRequest struct {
	Scopes []string `json:"scopes"`
}

// GetToken retrieves a token for the given scope by calling the azd CLI.
// It runs "azd auth token --scope <scope> --output json" and parses the result.
//
// SECURITY: Never log the returned token string or raw command output.
func GetToken(scope string) (string, time.Time, error) {
	if len(scope) > 512 || !strings.HasPrefix(scope, "https://") {
		return "", time.Time{}, fmt.Errorf("invalid scope format")
	}
	for _, c := range scope {
		if c < 0x20 || c > 0x7e {
			return "", time.Time{}, fmt.Errorf("invalid characters in scope")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "azd", "auth", "token", "--scope", scope, "--output", "json")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	// SECURITY: output may contain token - never log it
	if err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return "", time.Time{}, fmt.Errorf("azd auth token failed: %s", stderrStr)
		}
		return "", time.Time{}, fmt.Errorf("azd auth token failed: %w", err)
	}

	var response azdTokenResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse azd auth token response (output may be malformed)")
	}

	// Try multiple time formats
	var expiresOn time.Time
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999 -0700 MST",
	}

	for _, format := range formats {
		expiresOn, err = time.Parse(format, strings.TrimSpace(response.ExpiresOn))
		if err == nil {
			break
		}
	}

	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse expiresOn value %q: no supported format matched", response.ExpiresOn)
	}

	return response.Token, expiresOn, nil
}
