package spotify

import (
	"context"
	"time"
)

// SavedTrack is one entry in the user's Liked Songs. Track is nil when
// Spotify returns null for it.
type SavedTrack struct {
	AddedAt time.Time `json:"added_at"`
	Track   *Track    `json:"track"`
}

// GetSavedTracks fetches a page of the user's saved tracks ("Liked Songs").
// limit and offset behave as in GetUserPlaylists. Liked Songs is not a
// playlist and has no playlist ID, so it isn't reachable via
// GetPlaylistItems. Requires the user-library-read scope.
func (c *Client) GetSavedTracks(ctx context.Context, limit, offset int) (*Page[SavedTrack], error) {
	params, err := pagingParams(limit, offset)
	if err != nil {
		return nil, err
	}
	var out Page[SavedTrack]
	if err := c.getJSON(ctx, "/v1/me/tracks", params, "get saved tracks", "Get Saved Tracks", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
