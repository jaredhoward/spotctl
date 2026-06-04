package config

import (
	"bytes"
	"fmt"
	"text/template"
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

// ValidRepeatStates is the set of values accepted by the repeat action.
var ValidRepeatStates = map[string]bool{"off": true, "track": true, "context": true}

// Set is a named list of commands to execute in order.
type Set struct {
	// DeviceID is the default device for all commands in this set. A command
	// may override it by specifying its own params.device_id.
	DeviceID  string               `yaml:"device_id,omitempty"`
	OnError   OnFailure            `yaml:"on_error,omitempty"`
	OnTimeout OnFailure            `yaml:"on_timeout,omitempty"`
	// Confirm is the set-level default for command confirmation. Commands may
	// override it. When nil (not set), the global default of true is used.
	Confirm   *bool                `yaml:"confirm,omitempty"`
	// Timeout is the set-level default timeout for confirmed commands.
	// Commands may override it. When empty, DefaultConfirmTimeout is used.
	Timeout   string               `yaml:"timeout,omitempty"`
	// Params declares the parameters this set accepts, with optional defaults
	// and required flags. Callers supply values via run_set args or --arg flags.
	Params    map[string]SetParam  `yaml:"params,omitempty"`
	Commands  []Command            `yaml:"commands"`
}

// ResolveParams validates and merges caller-supplied args with declared
// set-level param defaults. Returns an error if a required param is absent
// from both args and the default. Unknown keys in args are silently ignored.
func (s *Set) ResolveParams(args map[string]string) (map[string]string, error) {
	resolved := make(map[string]string, len(s.Params))
	for name, decl := range s.Params {
		if val, ok := args[name]; ok {
			resolved[name] = val
		} else if decl.Default != "" {
			resolved[name] = decl.Default
		} else if decl.Required {
			return nil, fmt.Errorf("missing required arg %q", name)
		}
	}
	return resolved, nil
}

// Command is a single action within a set.
type Command struct {
	Name      string        `yaml:"name,omitempty"`
	Action    string        `yaml:"action"`
	DeviceID  string        `yaml:"device_id,omitempty"`
	Params    CommandParams `yaml:"params,omitempty"`
	// Confirm is a pointer so that an explicit confirm:false in YAML can be
	// distinguished from the field being absent. When nil (not set), the
	// default is true — confirmation is on unless explicitly opted out.
	Confirm   *bool         `yaml:"confirm,omitempty"`
	Timeout   string        `yaml:"timeout,omitempty"`
	OnError   OnFailure     `yaml:"on_error,omitempty"`
	OnTimeout OnFailure     `yaml:"on_timeout,omitempty"`
}

// ResolvedDeviceID returns the command's own device_id when set, otherwise
// falls back to the set-level device_id.
func (c *Command) ResolvedDeviceID(setDeviceID string) string {
	if c.DeviceID != "" {
		return c.DeviceID
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
// default, else OnFailureFail.
func (c *Command) EffectiveOnError(setDefault OnFailure) OnFailure {
	if c.OnError != "" {
		return c.OnError
	}
	if setDefault != "" {
		return setDefault
	}
	return OnFailureFail
}

// EffectiveConfirm returns the command's confirm if explicitly set, else the
// set-level default, else true.
func (c *Command) EffectiveConfirm(setDefault *bool) bool {
	if c.Confirm != nil {
		return *c.Confirm
	}
	if setDefault != nil {
		return *setDefault
	}
	return true
}

// EffectiveTimeout returns the command's timeout if set, else the set-level
// timeout parsed as a duration, else def.
func (c *Command) EffectiveTimeout(setDefault string, def time.Duration) time.Duration {
	if c.Timeout != "" {
		return c.TimeoutDuration(def)
	}
	if setDefault != "" {
		if d, err := time.ParseDuration(setDefault); err == nil && d > 0 {
			return d
		}
	}
	return def
}

// EffectiveOnTimeout mirrors EffectiveOnError for timeout.
func (c *Command) EffectiveOnTimeout(setDefault OnFailure) OnFailure {
	if c.OnTimeout != "" {
		return c.OnTimeout
	}
	if setDefault != "" {
		return setDefault
	}
	return OnFailureFail
}

// SetParam declares a single parameter a set accepts, with an optional
// default and a required flag.
type SetParam struct {
	Default  string `yaml:"default,omitempty"`
	Required bool   `yaml:"required,omitempty"`
}

// CommandParams holds all possible parameters for any action type.
type CommandParams struct {
	URI         string            `yaml:"uri,omitempty"`
	PlaylistID  string            `yaml:"playlist,omitempty"`
	TrackID     string            `yaml:"track,omitempty"`
	AlbumID     string            `yaml:"album,omitempty"`
	ArtistID    string            `yaml:"artist,omitempty"`
	Level       *int              `yaml:"level,omitempty"`
	Play        *bool             `yaml:"play,omitempty"`
	Enabled     *bool             `yaml:"enabled,omitempty"`
	Duration    string            `yaml:"duration,omitempty"`
	Set         string            `yaml:"set,omitempty"`
	RepeatState string            `yaml:"state,omitempty"` // off | track | context
	// Args passes caller-supplied parameter values into a run_set target.
	Args        map[string]string `yaml:"args,omitempty"`
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
// device_id is not validated here — it lives on the Command and is resolved
// at the set level before dispatch.
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
			return fmt.Errorf("action %q requires params.state (off, track, context)", action)
		}
		if !ValidRepeatStates[p.RepeatState] {
			return fmt.Errorf("action %q: invalid state %q — must be off, track, or context", action, p.RepeatState)
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

// interpolateString executes a Go template against the resolved param map,
// returning the rendered string or the original if no placeholders are present.
func interpolateString(s string, data map[string]string) (string, error) {
	if s == "" {
		return s, nil
	}
	tmpl, err := template.New("").Option("missingkey=zero").Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid template %q: %w", s, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution failed for %q: %w", s, err)
	}
	return buf.String(), nil
}

// InterpolateParams returns a copy of CommandParams with all string fields
// rendered against the resolved param map. Non-string fields (Level, Play,
// Enabled) are copied as-is. Unknown template keys render as empty strings.
func (p *CommandParams) InterpolateParams(resolved map[string]string) (CommandParams, error) {
	out := *p // shallow copy; pointer fields shared until overwritten below
	type strField struct {
		src *string
		dst *string
	}
	fields := []strField{
		{&p.URI, &out.URI},
		{&p.PlaylistID, &out.PlaylistID},
		{&p.TrackID, &out.TrackID},
		{&p.AlbumID, &out.AlbumID},
		{&p.ArtistID, &out.ArtistID},
		{&p.Duration, &out.Duration},
		{&p.Set, &out.Set},
		{&p.RepeatState, &out.RepeatState},
	}
	for _, f := range fields {
		val, err := interpolateString(*f.src, resolved)
		if err != nil {
			return CommandParams{}, err
		}
		*f.dst = val
	}
	return out, nil
}
