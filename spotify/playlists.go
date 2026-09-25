package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Page is Spotify's paging object. Next is empty on the last page.
type Page[T any] struct {
	Href     string `json:"href"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
	Total    int    `json:"total"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Items    []T    `json:"items"`
}

// PlaylistOwner is the user who owns a playlist.
type PlaylistOwner struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// playlistItemsRef is the {href, total} stub a simplified playlist carries
// in place of its actual items.
type playlistItemsRef struct {
	Total int `json:"total"`
}

// Playlist is a simplified playlist object, as returned by the
// current-user-playlists endpoint.
type Playlist struct {
	ID            string            `json:"id"`
	URI           string            `json:"uri"`
	Name          string            `json:"name"`
	Owner         PlaylistOwner     `json:"owner"`
	Collaborative bool              `json:"collaborative"`
	Items         *playlistItemsRef `json:"items"`
}

// TotalItems returns the number of items in the playlist, or 0 if Spotify
// omitted the item-count stub.
func (p Playlist) TotalItems() int {
	if p.Items == nil {
		return 0
	}
	return p.Items.Total
}

// PlaylistItem is one entry in a playlist. Item is nil when Spotify returns
// null for it (e.g. the track has been removed from the catalogue).
type PlaylistItem struct {
	AddedAt time.Time `json:"added_at"`
	IsLocal bool      `json:"is_local"`
	Item    *Track    `json:"item"`
}

// GetUserPlaylists fetches a page of the current user's playlists. limit
// caps the page size (valid range 1-50); 0 leaves the query param unset so
// Spotify applies its own default (20). offset is the index of the first
// playlist to return.
func (c *Client) GetUserPlaylists(ctx context.Context, limit, offset int) (*Page[Playlist], error) {
	params, err := pagingParams(limit, offset)
	if err != nil {
		return nil, err
	}
	var out Page[Playlist]
	if err := c.getJSON(ctx, "/v1/me/playlists", params, "get user playlists", "Get User Playlists", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPlaylistItems fetches a page of a playlist's items. playlist is a
// playlist ID, URI, or open.spotify.com link (see ParsePlaylistID). limit and
// offset behave as in GetUserPlaylists.
//
// Spotify only serves items for playlists the current user owns or
// collaborates on; anything else is a 403, as is a token that was issued
// without the playlist-read-private scope.
func (c *Client) GetPlaylistItems(ctx context.Context, playlist string, limit, offset int) (*Page[PlaylistItem], error) {
	id, err := ParsePlaylistID(playlist)
	if err != nil {
		return nil, err
	}
	params, err := pagingParams(limit, offset)
	if err != nil {
		return nil, err
	}
	var out Page[PlaylistItem]
	if err := c.getJSON(ctx, "/v1/playlists/"+id+"/items", params, "get playlist items", "Get Playlist Items", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

var playlistIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// ParsePlaylistID extracts the bare playlist ID from s, which may be the ID
// itself, a spotify:playlist:<id> URI, or an open.spotify.com/playlist/<id>
// link (with or without a query string, e.g. the ?si= share tracker).
func ParsePlaylistID(s string) (string, error) {
	id := strings.TrimSpace(s)
	if rest, ok := strings.CutPrefix(id, "spotify:playlist:"); ok {
		id = rest
	} else if u, err := url.Parse(id); err == nil && u.Host != "" {
		rest, ok := strings.CutPrefix(u.Path, "/playlist/")
		if !ok {
			return "", fmt.Errorf("invalid playlist %q: expected a playlist ID, spotify:playlist:<id> URI, or open.spotify.com/playlist/<id> link", s)
		}
		id = rest
	}
	if !playlistIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid playlist %q: expected a playlist ID, spotify:playlist:<id> URI, or open.spotify.com/playlist/<id> link", s)
	}
	return id, nil
}

// pagingParams validates limit/offset and builds the query. A zero limit is
// omitted so Spotify applies its default.
func pagingParams(limit, offset int) (url.Values, error) {
	if limit < 0 || limit > 50 {
		return nil, fmt.Errorf("invalid limit %d: must be 0–50", limit)
	}
	if offset < 0 {
		return nil, fmt.Errorf("invalid offset %d: must not be negative", offset)
	}
	params := url.Values{}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}
	return params, nil
}

// getJSON issues an authenticated GET against the API base and decodes a 200
// response into out. Any other status is returned as an *HTTPStatusError so
// callers can branch on it (e.g. 403) with errors.As.
func (c *Client) getJSON(ctx context.Context, path string, params url.Values, action, label string, out any) error {
	u := c.apiBase + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", action, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.doRequest(req, label)
	if err != nil {
		return fmt.Errorf("%s request failed: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &HTTPStatusError{Action: action, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode %s response: %w", action, err)
	}
	return nil
}
