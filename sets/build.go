package sets

import (
	"fmt"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// Build translates a config.Set into a *RunSet ready to Dispatch. depth is
// the current nesting level used to enforce MaxSetDepth.
func Build(name string, set config.Set, cfg *config.Config, depth int) (*RunSet, error) {
	if depth > MaxSetDepth {
		return nil, &DepthExceededError{Max: MaxSetDepth}
	}

	steps := make([]step, 0, len(set.Commands))
	for i, cmd := range set.Commands {
		if err := cmd.Params.Validate(cmd.Action); err != nil {
			return nil, fmt.Errorf("set %q command %d (%s): %w", name, i+1, cmd.Action, err)
		}

		deviceID := cmd.ResolvedDeviceID(set.DeviceID)

		a, err := buildAction(cmd, deviceID, cfg, depth)
		if err != nil {
			return nil, fmt.Errorf("set %q command %d (%s): %w", name, i+1, cmd.Action, err)
		}

		label := cmd.Name
		if label == "" {
			label = fmt.Sprintf("command %d (%s)", i+1, cmd.Action)
		}

		pollInterval := cfg.PlaybackPollIntervalDuration()
		timeout := cmd.TimeoutDuration(config.DefaultConfirmTimeout)

		steps = append(steps, step{
			label:  label,
			action: a,
			opts: ExecuteOptions{
				Confirm:      cmd.ConfirmEnabled(),
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
		uri, err := resolveURI(cmd.Params)
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
		return &spotify.Volume{DeviceID: deviceID, Level: *cmd.Params.Level}, nil

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
		return Build(cmd.Params.Set, sub, cfg, depth+1)

	default:
		return nil, fmt.Errorf("unknown action %q", cmd.Action)
	}
}

// resolveURI builds a Spotify context URI from CommandParams shorthand fields.
func resolveURI(p config.CommandParams) (string, error) {
	count := 0
	for _, v := range []string{p.URI, p.PlaylistID, p.TrackID, p.AlbumID, p.ArtistID} {
		if v != "" {
			count++
		}
	}
	if count > 1 {
		return "", fmt.Errorf("only one of uri/playlist/track/album/artist may be set in params")
	}
	switch {
	case p.URI != "":
		return p.URI, nil
	case p.PlaylistID != "":
		return "spotify:playlist:" + p.PlaylistID, nil
	case p.TrackID != "":
		return "spotify:track:" + p.TrackID, nil
	case p.AlbumID != "":
		return "spotify:album:" + p.AlbumID, nil
	case p.ArtistID != "":
		return "spotify:artist:" + p.ArtistID, nil
	}
	return "", nil
}
