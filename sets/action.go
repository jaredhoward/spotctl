package sets

import (
	"context"
	"fmt"
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

	const maxConsecutiveErrors = 5

	deadline := time.Now().Add(timeout)
	consecutiveErrors := 0
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
		state, err := c.GetCurrentPlayback(ctx)
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				return fmt.Errorf("polling aborted after %d consecutive errors: %w", consecutiveErrors, err)
			}
			if err := sleepOrDone(ctx, pollInterval); err != nil {
				return err
			}
			continue
		}
		consecutiveErrors = 0
		if a.Confirmed(state) {
			return nil
		}
		if err := sleepOrDone(ctx, pollInterval); err != nil {
			return err
		}
	}

	return &TimeoutError{Timeout: timeout, ActionLabel: a.Label()}
}

// sleepOrDone waits for d, returning ctx.Err() early if ctx is cancelled
// before the wait completes.
func sleepOrDone(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
