package sets

import (
	"fmt"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// Build translates a config.Set into a *RunSet ready to Dispatch. depth is
// the current nesting level used to enforce MaxSetDepth. args supplies
// caller-provided parameter values which are merged with set-level defaults.
func Build(name string, set config.Set, cfg *config.Config, depth int, args map[string]string) (*RunSet, error) {
	if depth > MaxSetDepth {
		return nil, &DepthExceededError{Max: MaxSetDepth}
	}

	resolved, err := set.ResolveParams(args, name)
	if err != nil {
		return nil, fmt.Errorf("set %q: %w", name, err)
	}

	setDeviceID, err := config.ResolveDeviceID(set.DeviceID, resolved)
	if err != nil {
		return nil, fmt.Errorf("set %q device_id: %w", name, err)
	}

	steps := make([]step, 0, len(set.Commands))
	for i, cmd := range set.Commands {
		interpolated, err := cmd.Params.InterpolateParams(resolved)
		if err != nil {
			return nil, fmt.Errorf("set %q command %d (%s): param interpolation: %w", name, i+1, cmd.Action, err)
		}
		cmd.Params = interpolated

		if err := cmd.Params.Validate(cmd.Action); err != nil {
			return nil, fmt.Errorf("set %q command %d (%s): %w", name, i+1, cmd.Action, err)
		}

		cmd.DeviceID, err = config.ResolveDeviceID(cmd.DeviceID, resolved)
		if err != nil {
			return nil, fmt.Errorf("set %q command %d (%s): device_id: %w", name, i+1, cmd.Action, err)
		}

		deviceID := cmd.ResolvedDeviceID(setDeviceID)

		a, err := buildAction(cmd, deviceID, cfg, depth)
		if err != nil {
			return nil, fmt.Errorf("set %q command %d (%s): %w", name, i+1, cmd.Action, err)
		}

		label := cmd.Name
		if label == "" {
			label = fmt.Sprintf("command %d (%s)", i+1, cmd.Action)
		}
		if detail := ActionDetail(cmd); detail != "" {
			label = fmt.Sprintf("%s [%s]", label, detail)
		}

		pollInterval := cfg.PlaybackPollIntervalDuration()
		timeout := cmd.EffectiveTimeout(set.Timeout, config.DefaultConfirmTimeout)

		steps = append(steps, step{
			label:  label,
			action: a,
			opts: ExecuteOptions{
				Confirm:      cmd.EffectiveConfirm(set.Confirm),
				Timeout:      timeout,
				PollInterval: pollInterval,
			},
			onError:   cmd.EffectiveOnError(set.OnError),
			onTimeout: cmd.EffectiveOnTimeout(set.OnTimeout),
		})
	}

	return &RunSet{Name: name, Steps: steps}, nil
}

// buildAction constructs the concrete spotify.Action for a single config.Command.
func buildAction(cmd config.Command, deviceID string, cfg *config.Config, depth int) (spotify.Action, error) {
	switch cmd.Action {
	case "play":
		uri, err := cmd.Params.ResolveContextURI()
		if err != nil {
			return nil, err
		}
		return &spotify.Play{DeviceID: deviceID, ContextURI: uri}, nil

	case "pause":
		return &spotify.Pause{DeviceID: deviceID}, nil

	case "next":
		return &spotify.Next{DeviceID: deviceID}, nil

	case "previous":
		return &spotify.Previous{DeviceID: deviceID}, nil

	case "shuffle":
		return &spotify.Shuffle{DeviceID: deviceID, Enabled: cmd.Params.ShuffleEnabled()}, nil

	case "repeat":
		return &spotify.Repeat{DeviceID: deviceID, State: cmd.Params.RepeatState}, nil

	case "volume":
		level, err := cmd.Params.Level.Resolved()
		if err != nil {
			return nil, err
		}
		return &spotify.Volume{DeviceID: deviceID, Level: level}, nil

	case "transfer":
		return &spotify.Transfer{DeviceID: deviceID, Play: cmd.Params.TransferPlay()}, nil

	case "sleep":
		d, _ := time.ParseDuration(cmd.Params.Duration) // already validated
		return &Sleep{Duration: d}, nil

	case "run_set":
		sub, ok := cfg.Sets[cmd.Params.Set]
		if !ok {
			return nil, fmt.Errorf("set %q not found", cmd.Params.Set)
		}
		forwarded := cmd.Params.ForwardedArgs()
		if deviceID != "" {
			if forwarded == nil {
				forwarded = make(map[string]string)
			}
			forwarded["device"] = deviceID
		}
		return Build(cmd.Params.Set, sub, cfg, depth+1, forwarded)

	default:
		return nil, fmt.Errorf("unknown action %q", cmd.Action)
	}
}

