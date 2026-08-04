package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RecentlyPlayedItem is one entry in the user's play history.
type RecentlyPlayedItem struct {
	Track    Track            `json:"track"`
	PlayedAt time.Time        `json:"played_at"`
	Context  *PlaybackContext `json:"context"`
}

// RecentlyPlayedResponse is the decoded response from the recently-played
// endpoint.
type RecentlyPlayedResponse struct {
	Items []RecentlyPlayedItem `json:"items"`
}

// GetRecentlyPlayed fetches the user's most recently played tracks. limit
// caps the number of items Spotify returns (valid range 1-50); 0 leaves the
// query param unset so Spotify applies its own default (20).
func (c *Client) GetRecentlyPlayed(ctx context.Context, limit int) (*RecentlyPlayedResponse, error) {
	if limit < 0 || limit > 50 {
		return nil, fmt.Errorf("invalid limit %d: must be 0–50", limit)
	}

	var extra []url.Values
	if limit > 0 {
		extra = append(extra, url.Values{"limit": {strconv.Itoa(limit)}})
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		playerURL(c.urlPlayer, "/recently-played", "", extra...), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create recently played request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.doRequest(req, "Get Recently Played Tracks")
	if err != nil {
		return nil, fmt.Errorf("recently played request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("recently played request returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out RecentlyPlayedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode recently played response: %w", err)
	}

	return &out, nil
}
