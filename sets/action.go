package sets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jaredhoward/spotctl/spotify"
)

const (
	defaultPollInterval    = 500 * time.Millisecond
	defaultTimeout         = 15 * time.Second
	defaultStabilizeWindow = 2 * time.Second
	// maxDispatchAttempts bounds how many times Execute re-dispatches a:
	// once after it confirms and then drops before stabilizing (see
	// staysConfirmed), and once for a transient HTTP error straight out of
	// Dispatch (see DispatchRetryBackoff) — both share this same budget.
	maxDispatchAttempts = 2
)

// DispatchRetryBackoff is how long Execute waits before redispatching after
// a transient HTTP error (502/503) straight out of Dispatch. A 429's
// Retry-After value is used instead when present. A package var, not a
// const, so tests can shrink it.
var DispatchRetryBackoff = 1 * time.Second

// Verbose enables poll/confirm/retry debug logging in Execute, set from the
// CLI --verbose flag. It is a runtime debugging aid, not persisted
// configuration.
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

// progressRegressionTolerance is how far a same-track ProgressMS can drop
// between stabilize polls before it's treated as a silent session reset
// rather than ordinary rounding/buffering jitter. Spotify has been observed
// silently re-establishing a fresh playback session mid-stream — same
// track/context, is_playing still true — a few seconds after confirming a
// play; the new session's progress starts back near zero. Confirmed() alone
// can't see this (context and is_playing both still look right), so
// staysConfirmed checks consecutive polls directly.
const progressRegressionTolerance = 1000 // milliseconds

// sessionReset reports whether current looks like a fresh playback session
// silently replacing prior mid-stream: same track item, but ProgressMS
// dropped by more than progressRegressionTolerance. A nil prior, a nil item
// on either side, or a genuine track change (different Item.URI — e.g. the
// playlist naturally advanced) is not a reset by this definition.
//
// Known false positive, not worth solving: a track on repeat:track looping
// produces the identical signature. Undetectable from PlaybackState alone.
// In practice a non-issue since the stabilize window is short (default 2s)
// and real tracks run far longer, so a loop landing inside the window
// essentially never happens outside pathological cases.
func sessionReset(prior, current *spotify.PlaybackState) bool {
	if prior == nil || prior.Item == nil || current == nil || current.Item == nil {
		return false
	}
	if prior.Item.URI != current.Item.URI {
		return false
	}
	return prior.ProgressMS-current.ProgressMS > progressRegressionTolerance
}

// ExecuteOptions controls confirmation polling for a single action.
type ExecuteOptions struct {
	Confirm      bool
	Timeout      time.Duration
	PollInterval time.Duration
	// StabilizeWindow is how long a first confirmation is re-checked before
	// Execute declares success. Defaults to defaultStabilizeWindow when zero.
	StabilizeWindow time.Duration
}

// Execute dispatches a and, when opts.Confirm is true, polls the Spotify
// playback state until the action is reflected. Once first confirmed, it
// keeps re-checking for opts.StabilizeWindow to make sure the state holds —
// some Spotify Connect devices (notably ones waking from an idle state)
// report a successful transition and then silently drop a moment later. If
// it drops within that window, Execute re-dispatches a and tries again, up
// to maxDispatchAttempts, before giving up. A transient HTTP error straight
// out of Dispatch (502, 503, or 429 — see spotify.HTTPStatusError.Retryable)
// is also retried out of that same budget, waiting DispatchRetryBackoff (or
// a 429's Retry-After, if present) first; any other error from Dispatch is
// fatal immediately.
func Execute(ctx context.Context, a spotify.Action, c *spotify.Client, opts ExecuteOptions) error {
	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	stabilizeWindow := opts.StabilizeWindow
	if stabilizeWindow <= 0 {
		stabilizeWindow = defaultStabilizeWindow
	}
	deadline := time.Now().Add(timeout)

	for attempt := 1; ; attempt++ {
		logVerbose("dispatch attempt %d/%d: %s", attempt, maxDispatchAttempts, a.Label())
		if err := a.Dispatch(spotify.WithReason(ctx, "Requested Command"), c); err != nil {
			var httpErr *spotify.HTTPStatusError
			if errors.As(err, &httpErr) && httpErr.Retryable() && attempt < maxDispatchAttempts {
				wait := DispatchRetryBackoff
				if httpErr.RetryAfter > 0 {
					wait = httpErr.RetryAfter
				}
				if time.Now().Add(wait).Before(deadline) {
					logVerbose("dispatch failed with retryable status %d: %v — retrying in %s", httpErr.StatusCode, err, wait)
					if sleepErr := sleepOrDone(ctx, wait); sleepErr != nil {
						return sleepErr
					}
					continue
				}
			}
			logVerbose("dispatch failed: %v", err)
			return err
		}
		// TEMP DEBUG: dispatchedAt times how long it takes Confirmed to
		// report true after the dispatch call returns (for the WiiM
		// investigation — see project history).
		dispatchedAt := time.Now()
		if !opts.Confirm {
			return nil
		}
		// Check Confirmed with nil first — actions like Sleep and RunSet
		// don't reflect real playback state, so they confirm immediately
		// without a state fetch and have nothing to restabilize against.
		if a.Confirmed(nil) {
			return nil
		}

		confirmedState, err := pollUntilConfirmed(ctx, a, c, deadline, pollInterval)
		if err != nil {
			return err
		}
		if confirmedState == nil {
			logVerbose("%s: never confirmed within %s", a.Label(), timeout)
			return &TimeoutError{Timeout: timeout, ActionLabel: a.Label()}
		}
		logVerbose("%s: confirmed after %s", a.Label(), time.Since(dispatchedAt))

		// Only actions that opt in via Stabilizer (currently just Play, and
		// only when it actually woke the target device — see
		// spotify.Play.NeedsStabilize) get the extra re-check. Everything
		// else trusts a single confirm: there's no wake step and no
		// observed "confirmed then silently reverted" failure to guard
		// against for them.
		if stabilizer, ok := a.(spotify.Stabilizer); !ok || !stabilizer.NeedsStabilize() {
			return nil
		}
		logVerbose("%s: checking it holds for %s", a.Label(), stabilizeWindow)

		stable, err := staysConfirmed(ctx, a, c, confirmedState, stabilizeWindow, pollInterval, deadline)
		if err != nil {
			return err
		}
		if stable {
			logVerbose("%s: held stable", a.Label())
			return nil
		}
		if attempt >= maxDispatchAttempts || !time.Now().Before(deadline) {
			logVerbose("%s: confirmed but dropped, no attempts/time remaining", a.Label())
			return &TimeoutError{Timeout: timeout, ActionLabel: a.Label() + " (confirmed but did not hold)"}
		}
		logVerbose("%s: confirmed but dropped before stabilizing — retrying", a.Label())
	}
}

// pollUntilConfirmed polls Spotify playback state at pollInterval until a is
// confirmed or deadline passes. It waits pollInterval before every check,
// including the first — checking immediately after dispatch is pointless,
// since Spotify's own backend needs a moment to reflect a change that was
// just requested. Returns the confirming state on success, or (nil, nil) on
// a plain deadline expiry. (nil, err) is returned only for context
// cancellation or too many consecutive GetCurrentPlayback failures.
func pollUntilConfirmed(ctx context.Context, a spotify.Action, c *spotify.Client, deadline time.Time, pollInterval time.Duration) (*spotify.PlaybackState, error) {
	const maxConsecutiveErrors = 5
	consecutiveErrors := 0
	for {
		if err := sleepOrDone(ctx, pollInterval); err != nil {
			return nil, err
		}
		if !time.Now().Before(deadline) {
			return nil, nil
		}
		state, err := c.GetCurrentPlayback(spotify.WithReason(ctx, "Polling for Confirmation"))
		if err != nil {
			consecutiveErrors++
			logVerbose("poll: GetCurrentPlayback failed (%d/%d consecutive): %v", consecutiveErrors, maxConsecutiveErrors, err)
			if consecutiveErrors >= maxConsecutiveErrors {
				return nil, fmt.Errorf("polling aborted after %d consecutive errors: %w", consecutiveErrors, err)
			}
			continue
		}
		consecutiveErrors = 0
		confirmed := a.Confirmed(state)
		logVerbose("poll: %s confirmed=%v", describeState(state), confirmed)
		if confirmed {
			return state, nil
		}
	}
}

// staysConfirmed re-polls for window (bounded by deadline) after an initial
// confirmation to make sure it holds. seed is the state that first confirmed
// (from pollUntilConfirmed) — comparing against it lets the very first
// stabilize tick catch a regression, not just tick-to-tick. Returns false the
// moment Confirmed reports false again, or the moment a same-track ProgressMS
// regression looks like a silent session reset (see sessionReset), or true
// once the full window elapses without either. Each wait is capped at
// whatever time remains in the window, so a window shorter than pollInterval
// (as tests use) doesn't force a full pollInterval sleep before its only
// check.
func staysConfirmed(ctx context.Context, a spotify.Action, c *spotify.Client, seed *spotify.PlaybackState, window, pollInterval time.Duration, deadline time.Time) (bool, error) {
	stableUntil := time.Now().Add(window)
	if stableUntil.After(deadline) {
		stableUntil = deadline
	}
	const maxConsecutiveErrors = 5
	consecutiveErrors := 0
	lastState := seed
	for {
		wait := time.Until(stableUntil)
		if wait <= 0 {
			return true, nil
		}
		if wait > pollInterval {
			wait = pollInterval
		}
		if err := sleepOrDone(ctx, wait); err != nil {
			return false, err
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}
		state, err := c.GetCurrentPlayback(spotify.WithReason(ctx, "Checking Stability"))
		if err != nil {
			consecutiveErrors++
			logVerbose("stabilize: GetCurrentPlayback failed (%d/%d consecutive): %v", consecutiveErrors, maxConsecutiveErrors, err)
			if consecutiveErrors >= maxConsecutiveErrors {
				return false, fmt.Errorf("polling aborted after %d consecutive errors: %w", consecutiveErrors, err)
			}
			continue
		}
		consecutiveErrors = 0
		confirmed := a.Confirmed(state)
		logVerbose("stabilize: %s confirmed=%v", describeState(state), confirmed)
		if !confirmed {
			return false, nil
		}
		if sessionReset(lastState, state) {
			logVerbose("stabilize: %s: progress regressed on same track (prior=%dms now=%dms) — treating as a silent session reset", a.Label(), lastState.ProgressMS, state.ProgressMS)
			return false, nil
		}
		lastState = state
	}
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
