package runner

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// MaxSetDepth prevents infinite recursion in run_set commands.
const MaxSetDepth = 10

// RunSet executes all commands in set. depth prevents infinite recursion.
func RunSet(name string, set config.Set, cfg *config.Config, client *spotify.Client, depth int) error {
	if depth > MaxSetDepth {
		return &DepthExceededError{}
	}

	log.Printf("[set:%s] starting (%d commands)", name, len(set.Commands))

	for i, cmd := range set.Commands {
		label := cmd.Name
		if label == "" {
			label = fmt.Sprintf("command %d", i+1)
		}

		log.Printf("[set:%s] %s: action=%s confirm=%v", name, label, cmd.Action, cmd.Confirm)

		if err := cmd.Params.Validate(cmd.Action); err != nil {
			return handleCommandError(name, label, err, cmd.EffectiveOnError(set.OnError))
		}

		// Resolve effective device: command-level wins, falls back to set-level.
		resolved := cmd
		resolved.Params.DeviceID = cmd.ResolvedDeviceID(set.DeviceID)

		err := ExecuteCommand(resolved, cfg, client, set, depth)
		if err == nil {
			log.Printf("[set:%s] %s: done", name, label)
			continue
		}

		// DepthExceededError always propagates regardless of on_error policy.
		var depthErr *DepthExceededError
		if errors.As(err, &depthErr) {
			return err
		}

		// Timeout: apply on_timeout policy.
		var timeoutErr *CommandTimeoutError
		if errors.As(err, &timeoutErr) {
			policy := cmd.EffectiveOnTimeout(set.OnTimeout)
			log.Printf("[set:%s] %s: timed out (%v)", name, label, timeoutErr)
			switch policy {
			case config.OnFailureFail:
				return fmt.Errorf("set %q aborted: command %q timed out: %w", name, label, timeoutErr)
			case config.OnFailureSkipRemaining:
				log.Printf("[set:%s] skipping remaining commands after timeout", name)
				return nil
			default: // continue
				log.Printf("[set:%s] %s: continuing after timeout", name, label)
				continue
			}
		}

		// All other errors: apply on_error policy.
		handled := handleCommandError(name, label, err, cmd.EffectiveOnError(set.OnError))
		if handled == nil {
			continue
		}
		if _, ok := errors.AsType[*skipRemainingError](handled); ok {
			return nil
		}
		return handled
	}

	log.Printf("[set:%s] complete", name)
	return nil
}

// handleCommandError applies an OnFailure policy to a command error.
func handleCommandError(setName, label string, err error, policy config.OnFailure) error {
	log.Printf("[set:%s] %s: error: %v", setName, label, err)
	switch policy {
	case config.OnFailureFail:
		return fmt.Errorf("set %q aborted: command %q failed: %w", setName, label, err)
	case config.OnFailureSkipRemaining:
		log.Printf("[set:%s] skipping remaining commands after error", setName)
		return &skipRemainingError{}
	default: // continue
		log.Printf("[set:%s] %s: continuing after error", setName, label)
		return nil
	}
}

// skipRemainingError signals: stop executing commands but exit 0.
type skipRemainingError struct{}

func (e *skipRemainingError) Error() string { return "skip_remaining" }

// DepthExceededError signals recursion limit hit; always propagates.
type DepthExceededError struct{}

func (e *DepthExceededError) Error() string {
	return fmt.Sprintf("set recursion depth exceeded (max %d)", MaxSetDepth)
}

// ExecuteCommand dispatches a command and, if confirm:true, polls until the
// Spotify state reflects the action or the timeout expires.
func ExecuteCommand(cmd config.Command, cfg *config.Config, client *spotify.Client, set config.Set, depth int) error {
	pollInterval := cfg.PlaybackPollIntervalDuration()
	timeout := cmd.TimeoutDuration(config.DefaultConfirmTimeout)

	// Snapshot the current track URI before next/previous so confirmation can
	// detect a track change. We use item.uri (the unique Spotify track
	// identifier) rather than item.name to avoid false positives from tracks
	// that share the same title.
	var priorTrackURI string
	if cmd.Confirm && (cmd.Action == "next" || cmd.Action == "previous") {
		if state, err := client.GetCurrentPlayback(); err == nil && state != nil && state.Item != nil {
			priorTrackURI = state.Item.URI
		}
	}

	if err := DispatchAction(cmd.Params, cmd.Action, client, cfg, depth); err != nil {
		return err
	}

	if !cmd.Confirm {
		return nil
	}

	// Poll for confirmation.
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, err := client.GetCurrentPlayback()
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}
		if Confirmed(cmd, state, priorTrackURI) {
			return nil
		}
		time.Sleep(pollInterval)
	}

	return &CommandTimeoutError{Timeout: timeout, Action: cmd.Action}
}

// Confirmed returns true when the playback state reflects the completed action.
// priorTrackURI is only used for next/previous confirmation.
func Confirmed(cmd config.Command, state *spotify.PlaybackState, priorTrackURI string) bool {
	if state == nil {
		return false
	}
	switch cmd.Action {
	case "play":
		return state.IsPlaying
	case "pause":
		return !state.IsPlaying
	case "shuffle":
		return state.ShuffleState == cmd.Params.ShuffleEnabled()
	case "repeat":
		return state.RepeatState == cmd.Params.RepeatState
	case "volume":
		if cmd.Params.Level == nil {
			return true
		}
		// Spotify can report volume ±1% from the requested value due to
		// device rounding, so we treat a difference of 1 as confirmed.
		diff := state.Device.VolumePercent - *cmd.Params.Level
		if diff < 0 {
			diff = -diff
		}
		return diff <= 1
	case "transfer":
		return state.Device.ID == cmd.Params.DeviceID
	case "next", "previous":
		// Confirmed when item.uri has changed from the pre-dispatch snapshot.
		if state.Item == nil {
			return false
		}
		return state.Item.URI != priorTrackURI
	default:
		return true
	}
}

// CommandTimeoutError is returned when confirmation polling exceeds the timeout.
type CommandTimeoutError struct {
	Timeout time.Duration
	Action  string
}

func (e *CommandTimeoutError) Error() string {
	return fmt.Sprintf("confirmation timed out after %s waiting for action %q to reflect in playback state",
		e.Timeout, e.Action)
}
