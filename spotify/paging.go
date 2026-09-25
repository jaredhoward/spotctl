package spotify

import (
	"context"
	"fmt"
)

// maxPageSize is the largest page Spotify serves for the list endpoints.
const maxPageSize = 50

// allPages walks a paged endpoint from the start, at the largest page size,
// until every item has been read. It stops on an empty page even if the
// reported total says more remain, so a total that shifts mid-walk can't
// spin forever.
func allPages[T any](fetch func(limit, offset int) (*Page[T], error)) ([]T, error) {
	var all []T
	for offset := 0; ; {
		page, err := fetch(maxPageSize, offset)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Items...)
		offset += len(page.Items)
		if len(page.Items) == 0 || offset >= page.Total {
			return all, nil
		}
	}
}

// GetAllUserPlaylists fetches every playlist in the current user's library.
func (c *Client) GetAllUserPlaylists(ctx context.Context) ([]Playlist, error) {
	return allPages(func(limit, offset int) (*Page[Playlist], error) {
		return c.GetUserPlaylists(ctx, limit, offset)
	})
}

// GetAllPlaylistItems fetches every item in a playlist (see GetPlaylistItems).
func (c *Client) GetAllPlaylistItems(ctx context.Context, playlist string) ([]PlaylistItem, error) {
	return allPages(func(limit, offset int) (*Page[PlaylistItem], error) {
		return c.GetPlaylistItems(ctx, playlist, limit, offset)
	})
}

// GetAllSavedTracks fetches every one of the user's saved tracks.
func (c *Client) GetAllSavedTracks(ctx context.Context) ([]SavedTrack, error) {
	return allPages(func(limit, offset int) (*Page[SavedTrack], error) {
		return c.GetSavedTracks(ctx, limit, offset)
	})
}

// GetCurrentUserID returns the Spotify user ID of the token's owner. Only the
// ID is decoded; the profile also carries fields (e.g. email) spotctl has no
// use for.
func (c *Client) GetCurrentUserID(ctx context.Context) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	if err := c.getJSON(ctx, "/v1/me", nil, "get current user", "Get Current User", &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("current user response had no id")
	}
	return out.ID, nil
}
