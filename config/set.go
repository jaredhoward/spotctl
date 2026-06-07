package config

import (
	"fmt"
	"regexp"
	"strings"
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

// IntOrTemplate holds a volume level that may be a literal int or a
// {{ name }} placeholder resolved at interpolation time.
type IntOrTemplate struct {
	Value int
	Expr  string // non-empty when a template placeholder was specified
}

func (v *IntOrTemplate) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try int first.
	var i int
	if err := unmarshal(&i); err == nil {
		v.Value = i
		return nil
	}
	// Fall back to string (template expression).
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	v.Expr = s
	return nil
}

func (v IntOrTemplate) MarshalYAML() (interface{}, error) {
	if v.Expr != "" {
		return v.Expr, nil
	}
	return v.Value, nil
}

// Resolved returns the int value after interpolation, or an error if the
// expression hasn't been resolved yet.
func (v *IntOrTemplate) Resolved() (int, error) {
	if v.Expr != "" {
		return 0, fmt.Errorf("level expression %q was not resolved before use", v.Expr)
	}
	return v.Value, nil
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
	Level       *IntOrTemplate    `yaml:"level,omitempty"`
	Play        *bool             `yaml:"play,omitempty"`
	Enabled     *bool             `yaml:"enabled,omitempty"`
	Duration    string            `yaml:"duration,omitempty"`
	Set         string            `yaml:"set,omitempty"`
	RepeatState string            `yaml:"state,omitempty"` // off | track | context
	// Forwarded captures any YAML keys under params not claimed by named fields.
	// For run_set, these are passed as args to the target set.
	Forwarded   map[string]string `yaml:",inline"`
}

// ForwardedArgs collects all param values suitable for passing to a run_set
// target: named string fields that are set, the Level value if present, and
// anything in the inline Forwarded map. The structural `set` field is excluded.
func (p *CommandParams) ForwardedArgs() map[string]string {
	out := make(map[string]string)
	for k, v := range p.Forwarded {
		out[k] = v
	}
	if p.URI != "" {
		out["uri"] = p.URI
	}
	if p.PlaylistID != "" {
		out["playlist"] = p.PlaylistID
	}
	if p.TrackID != "" {
		out["track"] = p.TrackID
	}
	if p.AlbumID != "" {
		out["album"] = p.AlbumID
	}
	if p.ArtistID != "" {
		out["artist"] = p.ArtistID
	}
	if p.Duration != "" {
		out["duration"] = p.Duration
	}
	if p.RepeatState != "" {
		out["state"] = p.RepeatState
	}
	if p.Level != nil {
		if p.Level.Expr != "" {
			out["volume"] = p.Level.Expr
		} else {
			out["volume"] = fmt.Sprintf("%d", p.Level.Value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

// placeholderExpr matches {{ name }} style placeholders used in command params.
var placeholderExpr = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

// interpolateString replaces {{ name }} placeholders with values from data.
// Unknown keys render as empty strings.
func interpolateString(s string, data map[string]string) (string, error) {
	if s == "" || !strings.Contains(s, "{{") {
		return s, nil
	}
	return placeholderExpr.ReplaceAllStringFunc(s, func(match string) string {
		key := strings.TrimSpace(placeholderExpr.FindStringSubmatch(match)[1])
		return data[key]
	}), nil
}

// InterpolateParams returns a copy of CommandParams with all string fields
// rendered against the resolved param map. Non-string fields (Level, Play,
// Enabled) are copied as-is. Unknown placeholder keys render as empty strings.
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
	// Interpolate Level.Expr if present.
	if p.Level != nil && p.Level.Expr != "" {
		interpolated, err := interpolateString(p.Level.Expr, resolved)
		if err != nil {
			return CommandParams{}, err
		}
		var i int
		if _, err := fmt.Sscanf(interpolated, "%d", &i); err != nil {
			return CommandParams{}, fmt.Errorf("level expression %q resolved to %q which is not an integer", p.Level.Expr, interpolated)
		}
		out.Level = &IntOrTemplate{Value: i}
	}
	return out, nil
}
