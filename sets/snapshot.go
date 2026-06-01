package sets

import (
	"context"
	"log"

	"github.com/jaredhoward/spotctl/spotify"
)

// snapshotAction wraps next/previous to snapshot the current track URI before
// dispatch, so Confirmed can detect that the track actually changed.
type snapshotAction struct {
	inner    spotify.Action
	priorURI string
}

func (s *snapshotAction) Dispatch(ctx context.Context, c *spotify.Client) error {
	if state, err := c.GetCurrentPlayback(); err == nil && state != nil && state.Item != nil {
		s.priorURI = state.Item.URI
	} else if err != nil {
		log.Printf("snapshotAction: could not get prior track URI: %v", err)
	}
	return s.inner.Dispatch(ctx, c)
}

func (s *snapshotAction) Confirmed(state *spotify.PlaybackState) bool {
	if state == nil || state.Item == nil {
		return false
	}
	return state.Item.URI != s.priorURI
}

func (s *snapshotAction) Label() string { return s.inner.Label() }
