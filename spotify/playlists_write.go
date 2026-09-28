package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// MaxPlaylistDescription is Spotify's cap on a playlist description.
	MaxPlaylistDescription = 300
	// maxAddItemsBatch is the most URIs Spotify accepts in one add-items call.
	maxAddItemsBatch = 100
)

var trackURIPattern = regexp.MustCompile(`^spotify:track:[A-Za-z0-9]+$`)

// ParseTrackURI returns s (trimmed) if it is a spotify:track:<id> URI.
func ParseTrackURI(s string) (string, error) {
	uri := strings.TrimSpace(s)
	if !trackURIPattern.MatchString(uri) {
		return "", fmt.Errorf("invalid track %q: expected a spotify:track:<id> URI", s)
	}
	return uri, nil
}

// CreatePlaylist creates an empty, private playlist owned by the current user.
// Public playlists are deliberately not supported: spotctl only requests the
// playlist-modify-private scope. description may be empty. Requires that scope.
func (c *Client) CreatePlaylist(ctx context.Context, name, description string) (*Playlist, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("playlist name must not be empty")
	}
	if n := utf8.RuneCountInString(description); n > MaxPlaylistDescription {
		return nil, fmt.Errorf("description is %d characters; Spotify allows at most %d", n, MaxPlaylistDescription)
	}
	body := map[string]any{"name": name, "public": false}
	if description != "" {
		body["description"] = description
	}
	var out Playlist
	if err := c.sendJSON(ctx, http.MethodPost, "/v1/me/playlists", body, "create playlist", "Create Playlist", &out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, fmt.Errorf("create playlist response had no id")
	}
	return &out, nil
}

// AddPlaylistItems appends tracks to the end of a playlist, in order, in
// batches of 100 (Spotify's per-call limit). playlist is anything
// ParsePlaylistID accepts; every entry of uris must be a spotify:track:<id>
// URI. All URIs are validated before the first request is sent, but a failure
// part-way through leaves the earlier batches added. Requires the
// playlist-modify-private scope.
func (c *Client) AddPlaylistItems(ctx context.Context, playlist string, uris []string) error {
	id, err := ParsePlaylistID(playlist)
	if err != nil {
		return err
	}
	for _, u := range uris {
		if _, err := ParseTrackURI(u); err != nil {
			return err
		}
	}
	for start := 0; start < len(uris); start += maxAddItemsBatch {
		end := min(start+maxAddItemsBatch, len(uris))
		body := map[string]any{"uris": uris[start:end]}
		if err := c.sendJSON(ctx, http.MethodPost, "/v1/playlists/"+id+"/items", body, "add playlist items", "Add Playlist Items", nil); err != nil {
			return fmt.Errorf("tracks %d–%d of %d: %w", start+1, end, len(uris), err)
		}
	}
	return nil
}

// sendJSON issues an authenticated request with a JSON body and treats 200
// and 201 as success, decoding the response into out when out is non-nil.
// Any other status is returned as an *HTTPStatusError, like getJSON.
func (c *Client) sendJSON(ctx context.Context, method, path string, body any, action, label string, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode %s request: %w", action, err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiBase+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", action, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req, label)
	if err != nil {
		return fmt.Errorf("%s request failed: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return &HTTPStatusError{Action: action, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(respBody))}
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode %s response: %w", action, err)
	}
	return nil
}
