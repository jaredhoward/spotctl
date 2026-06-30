package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type PlaybackContext struct {
	URI  string `json:"uri"`
	Type string `json:"type"`
}

type Artist struct {
	Name string `json:"name"`
}

type Track struct {
	URI        string   `json:"uri"`
	Name       string   `json:"name"`
	DurationMS int      `json:"duration_ms"`
	Artists    []Artist `json:"artists"`
}

type PlaybackState struct {
	Device       Device           `json:"device"`
	IsPlaying    bool             `json:"is_playing"`
	ShuffleState bool             `json:"shuffle_state"`
	RepeatState  string           `json:"repeat_state"`
	ProgressMS   int              `json:"progress_ms"`
	Item         *Track           `json:"item"`
	Context      *PlaybackContext `json:"context"`
}

func (c *Client) GetCurrentPlayback(ctx context.Context) (*PlaybackState, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, URLPlayer, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create current playback request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("current playback request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("current playback returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var playback PlaybackState
	if err := json.NewDecoder(resp.Body).Decode(&playback); err != nil {
		return nil, fmt.Errorf("could not decode current playback response: %w", err)
	}

	return &playback, nil
}
