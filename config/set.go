package config

import (
	"fmt"
	"time"
)

// OnFailure controls what a set does when a command times out or errors.
type OnFailure string

const (
	OnFailureFail          OnFailure = "fail"
	OnFailureContinue      OnFailure = "continue"
	OnFailureSkipRemaining OnFailure = "skip_remaining"
)

// DefaultConfirmTimeout is used when a command has confirm:true but no explicit timeout.
const DefaultConfirmTimeout = 15 * time.Second

// DefaultPollInterval is how often confirmation polls the playback state.
const DefaultPollInterval = 500 * time.Millisecond

// ValidRepeatStates is the set of values accepted by the repeat action.
var ValidRepeatStates = map[string]bool{"off": true, "track": true, "context": true}

// Set is a named list of commands to execute in order.
type Set struct {
	// DeviceID is the default device for all commands in this set. A command
	// may override it by specifying its own params.device_id.
	DeviceID  string    `yaml:"device_id,omitempty"`
	OnError   OnFailure `yaml:"on_error,omitempty"`
	OnTimeout OnFailure `yaml:"on_timeout,omitempty"`
	Commands  []Command `yaml:"commands"`
}

// Command is a single action within a set.
type Command struct {
	Name      string        `yaml:"name,omitempty"`
	Action    string        `yaml:"action"`
	Params    CommandParams `yaml:"params,omitempty"`
	Confirm   bool          `yaml:"confirm,omitempty"`
	Timeout   string        `yaml:"timeout,omitempty"`
	OnError   OnFailure     `yaml:"on_error,omitempty"`
	OnTimeout OnFailure     `yaml:"on_timeout,omitempty"`
}

// ResolvedDeviceID returns the command's own device_id when set, otherwise
// falls back to the set-level device_id.
func (c *Command) ResolvedDeviceID(setDeviceID string) string {
	if c.Params.DeviceID != "" {
		return c.Params.DeviceID
	}
	return setDeviceID
}

// TimeoutDuration parses Timeout and returns the duration, falling back to def.
func (c *Command) TimeoutDuration(def time.Duration) time.Duration {
	if c.Timeout == "" {
		return def
	}
	d, err := time.ParseDuration(c.Timeout)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// EffectiveOnError returns the command's on_error if set, else the set
// default, else OnFailureContinue.
func (c *Command) EffectiveOnError(setDefault OnFailure) OnFailure {
	if c.OnError != "" {
		return c.OnError
	}
	if setDefault != "" {
		return setDefault
	}
	return OnFailureContinue
}

// EffectiveOnTimeout mirrors EffectiveOnError for timeout.
func (c *Command) EffectiveOnTimeout(setDefault OnFailure) OnFailure {
	if c.OnTimeout != "" {
		return c.OnTimeout
	}
	if setDefault != "" {
		return setDefault
	}
	return OnFailureContinue
}

// CommandParams holds all possible parameters for any action type.
type CommandParams struct {
	DeviceID    string `yaml:"device_id,omitempty"`
	URI         string `yaml:"uri,omitempty"`
	PlaylistID  string `yaml:"playlist,omitempty"`
	TrackID     string `yaml:"track,omitempty"`
	AlbumID     string `yaml:"album,omitempty"`
	Shuffle     *bool  `yaml:"shuffle,omitempty"`
	Level       *int   `yaml:"level,omitempty"`
	Play        *bool  `yaml:"play,omitempty"`
	Enabled     *bool  `yaml:"enabled,omitempty"`
	Duration    string `yaml:"duration,omitempty"`
	Set         string `yaml:"set,omitempty"`
	RepeatState string `yaml:"repeat_state,omitempty"` // off | track | context
}

// ShuffleEnabled returns the value of Enabled, defaulting to true.
func (p *CommandParams) ShuffleEnabled() bool {
	if p.Enabled == nil {
		return true
	}
	return *p.Enabled
}

// TransferPlay returns the value of Play, defaulting to false.
func (p *CommandParams) TransferPlay() bool {
	if p.Play == nil {
		return false
	}
	return *p.Play
}

// Validate checks that required params are present for the given action.
// device_id is not validated here — it may be supplied at the set level or
// left empty to target the active Spotify device.
func (p *CommandParams) Validate(action string) error {
	switch action {
	case "play", "pause", "next", "previous", "shuffle", "transfer":
		return nil
	case "volume":
		if p.Level == nil {
			return fmt.Errorf("action %q requires params.level", action)
		}
	case "repeat":
		if p.RepeatState == "" {
			return fmt.Errorf("action %q requires params.repeat_state (off, track, context)", action)
		}
		if !ValidRepeatStates[p.RepeatState] {
			return fmt.Errorf("action %q: invalid repeat_state %q — must be off, track, or context", action, p.RepeatState)
		}
	case "sleep":
		if p.Duration == "" {
			return fmt.Errorf("action %q requires params.duration", action)
		}
		if _, err := time.ParseDuration(p.Duration); err != nil {
			return fmt.Errorf("action %q: invalid params.duration %q: %w", action, p.Duration, err)
		}
	case "run_set":
		if p.Set == "" {
			return fmt.Errorf("action %q requires params.set", action)
		}
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	return nil
}
