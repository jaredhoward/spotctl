package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ErrInvalidGrant indicates Spotify rejected the refresh token itself
// (revoked, expired, or otherwise no longer valid) rather than a transient
// request problem. Callers must discard the stored refresh token and send
// the user through the sign-in flow again rather than retrying.
var ErrInvalidGrant = errors.New("refresh token is invalid or expired")

// RefreshResult holds the tokens returned by a successful token refresh.
// NewRefreshToken is non-empty only when Spotify rotated the refresh token;
// callers should persist it to replace the stored token.
type RefreshResult struct {
	AccessToken     string
	NewRefreshToken string
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// tokenErrorResponse mirrors Spotify's OAuth error body, e.g.
// {"error": "invalid_grant", "error_description": "..."}.
type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

var RefreshAccessToken = refreshAccessToken

func refreshAccessToken(ctx context.Context, clientB64, refreshToken string) (RefreshResult, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, URLToken, strings.NewReader(data.Encode()))
	if err != nil {
		return RefreshResult{}, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+clientB64)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := (&http.Client{Timeout: defaultHTTPTimeout}).Do(req)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		var tokenErr tokenErrorResponse
		if jsonErr := json.Unmarshal(body, &tokenErr); jsonErr == nil && tokenErr.Error == "invalid_grant" {
			return RefreshResult{}, fmt.Errorf("%w: %s", ErrInvalidGrant, tokenErr.ErrorDescription)
		}

		return RefreshResult{}, fmt.Errorf("token request returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return RefreshResult{}, fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return RefreshResult{}, fmt.Errorf("empty access token in response")
	}

	return RefreshResult{
		AccessToken:     tokenResp.AccessToken,
		NewRefreshToken: tokenResp.RefreshToken,
	}, nil
}
