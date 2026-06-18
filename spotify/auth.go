package spotify

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var RefreshAccessToken = refreshAccessToken

// ErrInvalidGrant indicates Spotify rejected the refresh token itself
// (revoked, expired, or otherwise no longer valid) rather than a transient
// request problem. Callers must discard the stored refresh token and send
// the user through the sign-in flow again rather than retrying.
var ErrInvalidGrant = errors.New("refresh token is invalid or expired")

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// tokenErrorResponse mirrors Spotify's OAuth error body, e.g.
// {"error": "invalid_grant", "error_description": "..."}.
type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func refreshAccessToken(clientB64, refreshToken string) (string, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequest(http.MethodPost, URLToken, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("could not create token request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+clientB64)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		var tokenErr tokenErrorResponse
		if jsonErr := json.Unmarshal(body, &tokenErr); jsonErr == nil && tokenErr.Error == "invalid_grant" {
			return "", fmt.Errorf("%w: %s", ErrInvalidGrant, tokenErr.ErrorDescription)
		}

		return "", fmt.Errorf("token request returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("could not decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	return tokenResp.AccessToken, nil
}
