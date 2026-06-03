package sets

import (
	"context"
	"time"

	"github.com/jaredhoward/spotctl/spotify"
)

const (
	defaultPollInterval = 500 * time.Millisecond
	defaultTimeout      = 15 * time.Second
)

// ExecuteOptions controls confirmation polling for a single action.
type ExecuteOptions struct {
	Confirm      bool
	Timeout      time.Duration
	PollInterval time.Duration
}

// Execute dispatches a and, when opts.Confirm is true, polls the Spotify
// playback state until the action is reflected or the deadline is exceeded.
func Execute(ctx context.Context, a spotify.Action, c *spotify.Client, opts ExecuteOptions) error {
	if err := a.Dispatch(ctx, c); err != nil {
		return err
	}
	if !opts.Confirm {
		return nil
	}

	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// Check Confirmed with nil first — actions like Sleep that don't need
		// a state fetch will return true immediately without an API call.
		if a.Confirmed(nil) {
			return nil
		}
		state, err := c.GetCurrentPlayback()
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}
		if a.Confirmed(state) {
			return nil
		}
		time.Sleep(pollInterval)
	}

	return &TimeoutError{Timeout: timeout, ActionLabel: a.Label()}
}
