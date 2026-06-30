package sets

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// MaxSetDepth is the maximum allowed run_set nesting depth.
const MaxSetDepth = 10

// RunSet executes a named sequence of actions in order, applying per-command
// error and timeout policies. It satisfies spotify.Action so it can be nested
// via run_set commands.
type RunSet struct {
	Name  string
	Steps []step
}

// step pairs an action with its execution options and policy overrides.
type step struct {
	label     string
	action    spotify.Action
	opts      ExecuteOptions
	onError   config.OnFailure
	onTimeout config.OnFailure
}

func (r *RunSet) Dispatch(ctx context.Context, c *spotify.Client) error {
	fmt.Fprintf(os.Stderr, "[set:%s] starting (%d steps)\n", r.Name, len(r.Steps))

	for _, s := range r.Steps {
		fmt.Fprintf(os.Stderr, "[set:%s] %s: confirm=%v\n", r.Name, s.label, s.opts.Confirm)

		err := Execute(ctx, s.action, c, s.opts)
		if err == nil {
			fmt.Fprintf(os.Stderr, "[set:%s] %s: done\n", r.Name, s.label)
			continue
		}

		// DepthExceededError always propagates.
		var depthErr *DepthExceededError
		if errors.As(err, &depthErr) {
			return err
		}

		// Timeout: apply on_timeout policy.
		var timeoutErr *TimeoutError
		if errors.As(err, &timeoutErr) {
			fmt.Fprintf(os.Stderr, "[set:%s] %s: timed out (%v)\n", r.Name, s.label, timeoutErr)
			switch s.onTimeout {
			case config.OnFailureFail:
				return fmt.Errorf("set %q aborted: step %q timed out: %w", r.Name, s.label, timeoutErr)
			case config.OnFailureSkipRemaining:
				fmt.Fprintf(os.Stderr, "[set:%s] skipping remaining steps after timeout\n", r.Name)
				return nil
			default: // continue
				fmt.Fprintf(os.Stderr, "[set:%s] %s: continuing after timeout\n", r.Name, s.label)
				continue
			}
		}

		// All other errors: apply on_error policy.
		fmt.Fprintf(os.Stderr, "[set:%s] %s: error: %v\n", r.Name, s.label, err)
		switch s.onError {
		case config.OnFailureFail:
			return fmt.Errorf("set %q aborted: step %q failed: %w", r.Name, s.label, err)
		case config.OnFailureSkipRemaining:
			fmt.Fprintf(os.Stderr, "[set:%s] skipping remaining steps after error\n", r.Name)
			return nil
		default: // continue
			fmt.Fprintf(os.Stderr, "[set:%s] %s: continuing after error\n", r.Name, s.label)
		}
	}

	fmt.Fprintf(os.Stderr, "[set:%s] complete\n", r.Name)
	return nil
}

func (r *RunSet) Confirmed(_ *spotify.PlaybackState) bool { return true }
func (r *RunSet) Label() string                           { return "run_set set=" + r.Name }
