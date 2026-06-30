package sets

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jaredhoward/spotctl/spotify"
)

// Sleep pauses execution for Duration. It satisfies spotify.Action but makes
// no Spotify API call and has no state to confirm.
type Sleep struct {
	Duration time.Duration
}

func (s *Sleep) Dispatch(ctx context.Context, _ *spotify.Client) error {
	log.Printf("sleeping %s", s.Duration)
	select {
	case <-time.After(s.Duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Sleep) Confirmed(_ *spotify.PlaybackState) bool { return true }
func (s *Sleep) Label() string                           { return fmt.Sprintf("sleep duration=%s", s.Duration) }
