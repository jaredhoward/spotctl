package sets

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jaredhoward/spotctl/spotify"
)

const (
	defaultPollInterval = 500 * time.Millisecond
	defaultTimeout      = 15 * time.Second
)

// Verbose enables poll/confirm debug logging in Execute, set from the CLI
// --verbose flag. It is a runtime debugging aid, not persisted configuration.
var Verbose bool

// timestampFormat gives verbose log lines millisecond-precision timestamps
// so elapsed time between lines (e.g. dispatch to confirm) can be read
// directly off --verbose output instead of measured separately.
const timestampFormat = "2006-01-02 15:04:05.000"

// logVerbose writes a debug line to stderr when Verbose is enabled.
func logVerbose(format string, args ...any) {
	if !Verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "[%s] [verbose] "+format+"\n", append([]any{time.Now().Format(timestampFormat)}, args...)...)
}

// describeState renders a concise summary of a playback state for verbose
// logging.
func describeState(state *spotify.PlaybackState) string {
	if state == nil {
		return "state=<none>"
	}
	ctx := "-"
	if state.Context != nil {
		ctx = state.Context.URI
	}
	return fmt.Sprintf("is_playing=%v device=%q(%s) context=%s", state.IsPlaying, state.Device.Name, state.Device.ID, ctx)
}

// ExecuteOptions controls confirmation polling for a single action.
type ExecuteOptions struct {
	Confirm      bool
	Timeout      time.Duration
	PollInterval time.Duration
}

// Execute dispatches a and, when opts.Confirm is true, polls the Spotify
// playback state until the action is reflected or the deadline is exceeded.
func Execute(ctx context.Context, a spotify.Action, c *spotify.Client, opts ExecuteOptions) error {
	logVerbose("dispatch: %s", a.Label())
	if err := a.Dispatch(ctx, c); err != nil {
		logVerbose("dispatch failed: %v", err)
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
			logVerbose("poll: GetCurrentPlayback failed (%d/%d consecutive): %v", consecutiveErrors, maxConsecutiveErrors, err)
			if consecutiveErrors >= maxConsecutiveErrors {
				return fmt.Errorf("polling aborted after %d consecutive errors: %w", consecutiveErrors, err)
			}
			if err := sleepOrDone(ctx, pollInterval); err != nil {
				return err
			}
			continue
		}
		consecutiveErrors = 0
		confirmed := a.Confirmed(state)
		logVerbose("poll: %s confirmed=%v", describeState(state), confirmed)
		if confirmed {
			return nil
		}
		if err := sleepOrDone(ctx, pollInterval); err != nil {
			return err
		}
	}

	logVerbose("%s: never confirmed within %s", a.Label(), timeout)
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
