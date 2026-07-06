package config

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"time"
)

// Now returns the current time. It is a package-level var so tests can
// override it to get deterministic pool picks (see pickFromPool).
var Now = time.Now

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
// set-level param defaults/pools. setName scopes pool selection so different
// sets sharing a param name (e.g. "uri") don't rotate in lockstep. Returns an
// error if a required param is absent from args, default, and pool. Unknown
// keys in args are silently ignored.
func (s *Set) ResolveParams(args map[string]string, setName string) (map[string]string, error) {
	resolved := make(map[string]string, len(s.Params))
	for name, decl := range s.Params {
		if val, ok := args[name]; ok {
			if val == "" && decl.Required {
				return nil, fmt.Errorf("missing required arg %q", name)
			}
			resolved[name] = val
		} else if len(decl.Pool) > 0 {
			resolved[name] = pickFromPool(setName, name, decl.Pool, Now())
		} else if decl.Default != "" {
			resolved[name] = decl.Default
		} else if decl.Required {
			return nil, fmt.Errorf("missing required arg %q", name)
		} else {
			// Optional param with no default and not required: resolve to ""
			// so any {{ name }} placeholder in command params does not error.
			// Undeclared placeholders (typos) are unaffected since they never
			// appear in s.Params and so are never added here.
			resolved[name] = ""
		}
	}
	return resolved, nil
}

// pickFromPool deterministically picks a pool entry for the given calendar
// day — no state is persisted anywhere. The starting position in the pool is
// a hash of setName and paramName (so different sets/params don't all land on
// the same entry on the same day); from there the pick advances by one pool
// position per calendar day. This guarantees: re-running on the same day
// reproduces the same pick, consecutive days never repeat (advancing by one
// step in a pool of size > 1 can never return to the same index), and the
// full pool cycles through evenly every len(pool) days.
func pickFromPool(setName, paramName string, pool []string, now time.Time) string {
	n := len(pool)
	if n == 1 {
		return pool[0]
	}
	offset := poolIndex(setName, paramName, n)
	idx := (offset + dayCount(now)) % n
	return pool[idx]
}

// poolIndex hashes (setName, paramName) into a starting index in [0, n).
func poolIndex(setName, paramName string, n int) int {
	h := fnv.New32a()
	h.Write([]byte(setName + "\x00" + paramName))
	return int(h.Sum32() % uint32(n))
}

// dayCount returns a value that increases by exactly 1 for each successive
// calendar date of now (in now's own location), independent of time-of-day.
// Using an absolute epoch anchor rather than a formatted date string avoids
// string-hash timezone/format edge cases while staying trivially testable.
func dayCount(now time.Time) int {
	y, m, d := now.Date()
	return int(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
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

// ResolveDeviceID interpolates a device_id string (from Set.DeviceID or
// Command.DeviceID) against a set's resolved params, so device_id: '{{ name }}'
// can reference a declared param the same way uri or volume already do. A
// literal device_id with no {{ }} placeholder passes through unchanged.
func ResolveDeviceID(deviceID string, resolved map[string]string) (string, error) {
	return interpolateString(deviceID, resolved)
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
// default, a required flag, or a pool of candidate values to pick from
// deterministically by date (see pickFromPool). Pool is mutually exclusive
// with Default and Required — enforced in Config.validate.
type SetParam struct {
	Default  string   `yaml:"default,omitempty"`
	Required bool     `yaml:"required,omitempty"`
	Pool     []string `yaml:"pool,omitempty"`
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

// ResolveContextURI builds a Spotify context URI from the shorthand fields
// (URI, PlaylistID, TrackID, AlbumID, ArtistID). At most one may be set;
// returns an error if more than one is non-empty.
func (p *CommandParams) ResolveContextURI() (string, error) {
	count := 0
	for _, v := range []string{p.URI, p.PlaylistID, p.TrackID, p.AlbumID, p.ArtistID} {
		if v != "" {
			count++
		}
	}
	if count > 1 {
		return "", fmt.Errorf("only one of uri/playlist/track/album/artist may be set")
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
	var missing []string
	result := placeholderExpr.ReplaceAllStringFunc(s, func(match string) string {
		key := strings.TrimSpace(placeholderExpr.FindStringSubmatch(match)[1])
		val, ok := data[key]
		if !ok {
			missing = append(missing, key)
			return ""
		}
		return val
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("placeholders have no value (declare them in params): %s", strings.Join(missing, ", "))
	}
	return result, nil
}

// InterpolateParams returns a copy of CommandParams with all string fields
// rendered against the resolved param map. Non-string fields (Level, Play,
// Enabled) are copied as-is. Unknown placeholder keys render as empty strings.
func (p *CommandParams) InterpolateParams(resolved map[string]string) (CommandParams, error) {
	out := *p // shallow copy; pointer fields (Level, Play, Enabled) are overwritten below — add new pointer fields there too
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
