package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// ----- Set YAML round-trip ---------------------------------------------------

const setYAML = `
client_id: id
client_secret: secret
refresh_token: refresh
sets:
  morning_vibes:
    device_id: set-device
    on_error: continue
    on_timeout: fail
    confirm: false
    timeout: 20s
    commands:
      - name: start playlist
        action: play
        params:
          playlist: pl123
        confirm: true
        timeout: 15s
        on_timeout: fail
      - name: enable shuffle
        action: shuffle
        params:
          enabled: true
        confirm: true
        timeout: 5s
      - name: set volume
        action: volume
        params:
          level: 60
      - name: sleep a bit
        action: sleep
        params:
          duration: 2s
      - name: run warmup
        action: run_set
        params:
          set: warmup
      - name: set repeat
        action: repeat
        params:
          state: context
  warmup:
    commands:
      - action: next
`

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "set-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestSetRoundTrip(t *testing.T) {
	cfg, err := Load(writeYAML(t, setYAML))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Sets) != 2 {
		t.Fatalf("expected 2 sets, got %d", len(cfg.Sets))
	}

	mv, ok := cfg.Sets["morning_vibes"]
	if !ok {
		t.Fatal("expected set 'morning_vibes'")
	}
	if mv.DeviceID != "set-device" {
		t.Errorf("set device_id: got %q", mv.DeviceID)
	}
	if mv.OnError != OnFailureContinue {
		t.Errorf("on_error: got %q, want %q", mv.OnError, OnFailureContinue)
	}
	if mv.OnTimeout != OnFailureFail {
		t.Errorf("on_timeout: got %q, want %q", mv.OnTimeout, OnFailureFail)
	}
	if mv.Confirm == nil || *mv.Confirm {
		t.Error("set confirm: expected explicit false")
	}
	if mv.Timeout != "20s" {
		t.Errorf("set timeout: got %q, want 20s", mv.Timeout)
	}
	if len(mv.Commands) != 6 {
		t.Fatalf("expected 6 commands, got %d", len(mv.Commands))
	}

	c0 := mv.Commands[0]
	if c0.Action != "play" {
		t.Errorf("command 0 action: got %q, want play", c0.Action)
	}
	if c0.Params.PlaylistID != "pl123" {
		t.Errorf("command 0 playlist: got %q", c0.Params.PlaylistID)
	}
	if c0.Confirm == nil || !*c0.Confirm {
		t.Error("command 0 confirm: expected true")
	}
	if c0.OnTimeout != OnFailureFail {
		t.Errorf("command 0 on_timeout: got %q", c0.OnTimeout)
	}

	c1 := mv.Commands[1]
	if c1.Action != "shuffle" || c1.Params.Enabled == nil || !*c1.Params.Enabled {
		t.Errorf("command 1 shuffle unexpected: %+v", c1)
	}

	c2 := mv.Commands[2]
	if c2.Action != "volume" || c2.Params.Level == nil || c2.Params.Level.Value != 60 {
		t.Errorf("command 2 volume unexpected: %+v", c2)
	}

	c3 := mv.Commands[3]
	if c3.Action != "sleep" || c3.Params.Duration != "2s" {
		t.Errorf("command 3 sleep unexpected: %+v", c3)
	}

	c4 := mv.Commands[4]
	if c4.Action != "run_set" || c4.Params.Set != "warmup" {
		t.Errorf("command 4 run_set unexpected: %+v", c4)
	}

	c5 := mv.Commands[5]
	if c5.Action != "repeat" {
		t.Errorf("command 5 action: got %q, want repeat", c5.Action)
	}
	if c5.Params.RepeatState != "context" {
		t.Errorf("command 5 state: got %q, want context", c5.Params.RepeatState)
	}

	warmup, ok := cfg.Sets["warmup"]
	if !ok {
		t.Fatal("expected set 'warmup'")
	}
	if len(warmup.Commands) != 1 || warmup.Commands[0].Action != "next" {
		t.Errorf("warmup commands unexpected: %+v", warmup.Commands)
	}
}

// ----- EffectiveConfirm -----------------------------------------------------

func TestEffectiveConfirm(t *testing.T) {
	true_ := true
	false_ := false

	cases := []struct {
		cmdConfirm *bool
		setConfirm *bool
		want       bool
	}{
		// both nil: global default true
		{nil, nil, true},
		// set-level false, command unset: inherits false
		{nil, &false_, false},
		// set-level true, command unset: inherits true
		{nil, &true_, true},
		// command overrides set-level false → true
		{&true_, &false_, true},
		// command overrides set-level true → false
		{&false_, &true_, false},
		// command explicit, set nil
		{&false_, nil, false},
		{&true_, nil, true},
	}
	for _, tc := range cases {
		c := Command{Confirm: tc.cmdConfirm}
		if got := c.EffectiveConfirm(tc.setConfirm); got != tc.want {
			t.Errorf("EffectiveConfirm(cmd=%v, set=%v) = %v, want %v",
				tc.cmdConfirm, tc.setConfirm, got, tc.want)
		}
	}
}

// ----- EffectiveTimeout ------------------------------------------------------

func TestEffectiveTimeout(t *testing.T) {
	def := 15 * time.Second
	cases := []struct {
		cmdTimeout string
		setTimeout string
		want       time.Duration
	}{
		// both empty: global default
		{"", "", def},
		// set-level only
		{"", "20s", 20 * time.Second},
		// command overrides set
		{"5s", "20s", 5 * time.Second},
		// command overrides empty set
		{"10s", "", 10 * time.Second},
		// invalid set-level falls back to default
		{"", "bad", def},
		// invalid set-level zero falls back to default
		{"", "0s", def},
		// command invalid falls back to default (TimeoutDuration behaviour)
		{"bad", "20s", def},
	}
	for _, tc := range cases {
		c := Command{Timeout: tc.cmdTimeout}
		if got := c.EffectiveTimeout(tc.setTimeout, def); got != tc.want {
			t.Errorf("EffectiveTimeout(cmd=%q, set=%q) = %v, want %v",
				tc.cmdTimeout, tc.setTimeout, got, tc.want)
		}
	}
}

// ----- ResolveParams --------------------------------------------------------

func TestResolveParams(t *testing.T) {
	t.Run("required present", func(t *testing.T) {
		s := Set{Params: map[string]SetParam{"uri": {Required: true}}}
		got, err := s.ResolveParams(map[string]string{"uri": "spotify:playlist:abc"}, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["uri"] != "spotify:playlist:abc" {
			t.Errorf("uri: got %q", got["uri"])
		}
	})

	t.Run("required missing", func(t *testing.T) {
		s := Set{Params: map[string]SetParam{"uri": {Required: true}}}
		_, err := s.ResolveParams(nil, "test")
		if err == nil {
			t.Fatal("expected error for missing required arg")
		}
		if !strings.Contains(err.Error(), "uri") {
			t.Errorf("expected param name in error, got: %v", err)
		}
	})

	t.Run("default used when arg absent", func(t *testing.T) {
		s := Set{Params: map[string]SetParam{"volume": {Default: "35"}}}
		got, err := s.ResolveParams(nil, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["volume"] != "35" {
			t.Errorf("volume: got %q, want 35", got["volume"])
		}
	})

	t.Run("arg overrides default", func(t *testing.T) {
		s := Set{Params: map[string]SetParam{"volume": {Default: "35"}}}
		got, err := s.ResolveParams(map[string]string{"volume": "50"}, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["volume"] != "50" {
			t.Errorf("volume: got %q, want 50", got["volume"])
		}
	})

	t.Run("unknown arg ignored", func(t *testing.T) {
		s := Set{Params: map[string]SetParam{"uri": {Required: true}}}
		got, err := s.ResolveParams(map[string]string{"uri": "x", "unknown": "y"}, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, exists := got["unknown"]; exists {
			t.Error("expected unknown key to be ignored")
		}
	})

	t.Run("no params declared", func(t *testing.T) {
		s := Set{}
		got, err := s.ResolveParams(map[string]string{"anything": "value"}, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty resolved map, got %v", got)
		}
	})

	t.Run("explicit empty string for required param errors", func(t *testing.T) {
		s := Set{Params: map[string]SetParam{"track": {Required: true}}}
		_, err := s.ResolveParams(map[string]string{"track": ""}, "test")
		if err == nil {
			t.Fatal("expected error for empty string passed to a required param")
		}
		if !strings.Contains(err.Error(), "track") {
			t.Errorf("expected param name in error, got: %v", err)
		}
	})

	t.Run("explicit empty string for optional param is accepted", func(t *testing.T) {
		s := Set{Params: map[string]SetParam{"device": {}}}
		got, err := s.ResolveParams(map[string]string{"device": ""}, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["device"] != "" {
			t.Errorf("device: got %q, want empty string", got["device"])
		}
	})

	t.Run("optional param absent resolves to empty string", func(t *testing.T) {
		s := Set{Params: map[string]SetParam{"device": {}}}
		got, err := s.ResolveParams(nil, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		val, ok := got["device"]
		if !ok {
			t.Fatal("expected optional param to be present in resolved map")
		}
		if val != "" {
			t.Errorf("device: got %q, want empty string", val)
		}
	})

	t.Run("pool ignored when arg provided", func(t *testing.T) {
		s := Set{Params: map[string]SetParam{
			"uri": {Pool: []string{"a", "b", "c"}},
		}}
		got, err := s.ResolveParams(map[string]string{"uri": "spotify:track:override"}, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["uri"] != "spotify:track:override" {
			t.Errorf("uri: got %q, want override", got["uri"])
		}
	})

	t.Run("empty pool falls back to default", func(t *testing.T) {
		s := Set{Params: map[string]SetParam{
			"uri": {Pool: []string{}, Default: "spotify:track:fallback"},
		}}
		got, err := s.ResolveParams(nil, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["uri"] != "spotify:track:fallback" {
			t.Errorf("uri: got %q, want fallback default", got["uri"])
		}
	})
}

// ----- pool selection --------------------------------------------------------

func TestResolveParams_PoolSameDayIsIdempotent(t *testing.T) {
	oldNow := Now
	defer func() { Now = oldNow }()
	Now = func() time.Time { return time.Date(2026, 7, 6, 22, 0, 0, 0, time.UTC) }

	s := Set{Params: map[string]SetParam{
		"uri": {Pool: []string{"spotify:playlist:a", "spotify:playlist:b", "spotify:playlist:c"}, Method: PoolMethodDate},
	}}

	got1, err := s.ResolveParams(nil, "jareds_sleep")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got2, err := s.ResolveParams(nil, "jareds_sleep")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got1["uri"] != got2["uri"] {
		t.Errorf("expected same-day picks to match, got %q and %q", got1["uri"], got2["uri"])
	}
}

func TestResolveParams_PoolNeverRepeatsAdjacentDay(t *testing.T) {
	oldNow := Now
	defer func() { Now = oldNow }()

	s := Set{Params: map[string]SetParam{
		"uri": {Pool: []string{"spotify:playlist:a", "spotify:playlist:b", "spotify:playlist:c"}, Method: PoolMethodDate},
	}}

	base := time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)
	var prev string
	for i := 0; i < 30; i++ {
		day := base.AddDate(0, 0, i)
		Now = func() time.Time { return day }
		got, err := s.ResolveParams(nil, "jareds_sleep")
		if err != nil {
			t.Fatalf("day %d: unexpected error: %v", i, err)
		}
		if i > 0 && got["uri"] == prev {
			t.Errorf("day %d: pick %q repeats previous day's pick", i, got["uri"])
		}
		prev = got["uri"]
	}
}

func TestResolveParams_PoolSingleEntry(t *testing.T) {
	s := Set{Params: map[string]SetParam{
		"uri": {Pool: []string{"spotify:playlist:only"}},
	}}
	for i := 0; i < 5; i++ {
		got, err := s.ResolveParams(nil, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["uri"] != "spotify:playlist:only" {
			t.Errorf("uri: got %q, want the single pool entry", got["uri"])
		}
	}
}

func TestPoolIndex_Deterministic(t *testing.T) {
	a := poolIndex("set", "param", 5)
	b := poolIndex("set", "param", 5)
	if a != b {
		t.Errorf("expected identical inputs to produce identical index, got %d and %d", a, b)
	}
	if a < 0 || a >= 5 {
		t.Errorf("index %d out of range [0,5)", a)
	}
}

func TestResolveParams_PoolCyclesEvenlyOverFullPeriod(t *testing.T) {
	oldNow := Now
	defer func() { Now = oldNow }()

	pool := []string{"spotify:playlist:a", "spotify:playlist:b", "spotify:playlist:c"}
	s := Set{Params: map[string]SetParam{"uri": {Pool: pool, Method: PoolMethodDate}}}

	base := time.Date(2026, 3, 1, 22, 0, 0, 0, time.UTC)
	seen := make(map[string]bool)
	for i := 0; i < len(pool); i++ {
		day := base.AddDate(0, 0, i)
		Now = func() time.Time { return day }
		got, err := s.ResolveParams(nil, "jareds_sleep")
		if err != nil {
			t.Fatalf("day %d: unexpected error: %v", i, err)
		}
		seen[got["uri"]] = true
	}
	if len(seen) != len(pool) {
		t.Errorf("expected all %d pool entries to appear within one full cycle, saw %d: %v", len(pool), len(seen), seen)
	}
}

func TestResolveParams_PoolDefaultMethodIsRandom(t *testing.T) {
	oldRandIntn := RandIntn
	defer func() { RandIntn = oldRandIntn }()
	RandIntn = func(n int) int { return 1 }

	pool := []string{"spotify:playlist:a", "spotify:playlist:b", "spotify:playlist:c"}
	s := Set{Params: map[string]SetParam{"uri": {Pool: pool}}}

	got, err := s.ResolveParams(nil, "jareds_daily_mix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["uri"] != pool[1] {
		t.Errorf("expected default method to be random and pick pool[1], got %q", got["uri"])
	}
}

func TestResolveParams_PoolMethodRandomIgnoresNow(t *testing.T) {
	oldNow := Now
	oldRandIntn := RandIntn
	defer func() { Now = oldNow; RandIntn = oldRandIntn }()
	Now = func() time.Time { return time.Date(2026, 7, 6, 22, 0, 0, 0, time.UTC) }

	pool := []string{"spotify:playlist:a", "spotify:playlist:b", "spotify:playlist:c"}
	s := Set{Params: map[string]SetParam{"uri": {Pool: pool, Method: PoolMethodRandom}}}

	RandIntn = func(n int) int { return 0 }
	got1, err := s.ResolveParams(nil, "jareds_daily_mix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	RandIntn = func(n int) int { return 2 }
	got2, err := s.ResolveParams(nil, "jareds_daily_mix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got1["uri"] == got2["uri"] {
		t.Errorf("expected random method to ignore Now and follow RandIntn, got same pick %q both times", got1["uri"])
	}
}

// ----- ResolveContextURI -----------------------------------------------------

func TestResolveContextURI(t *testing.T) {
	cases := []struct {
		name    string
		params  CommandParams
		want    string
		wantErr bool
	}{
		{"uri set", CommandParams{URI: "spotify:artist:abc"}, "spotify:artist:abc", false},
		{"playlist set", CommandParams{PlaylistID: "p1"}, "spotify:playlist:p1", false},
		{"track set", CommandParams{TrackID: "t1"}, "spotify:track:t1", false},
		{"album set", CommandParams{AlbumID: "a1"}, "spotify:album:a1", false},
		{"artist set", CommandParams{ArtistID: "ar1"}, "spotify:artist:ar1", false},
		{"none set", CommandParams{}, "", false},
		{"uri and playlist both set", CommandParams{URI: "spotify:artist:abc", PlaylistID: "p1"}, "", true},
		{"all five set", CommandParams{URI: "u", PlaylistID: "p", TrackID: "t", AlbumID: "a", ArtistID: "ar"}, "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.params.ResolveContextURI()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ----- InterpolateParams -----------------------------------------------------

func TestInterpolateParams(t *testing.T) {
	t.Run("uri interpolated", func(t *testing.T) {
		p := CommandParams{URI: "{{ uri }}"}
		got, err := p.InterpolateParams(map[string]string{"uri": "spotify:playlist:abc"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.URI != "spotify:playlist:abc" {
			t.Errorf("URI: got %q", got.URI)
		}
	})

	t.Run("multiple fields interpolated", func(t *testing.T) {
		p := CommandParams{
			URI:      "{{ uri }}",
			Duration: "{{ dur }}",
		}
		got, err := p.InterpolateParams(map[string]string{"uri": "spotify:track:x", "dur": "5s"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.URI != "spotify:track:x" {
			t.Errorf("URI: got %q", got.URI)
		}
		if got.Duration != "5s" {
			t.Errorf("Duration: got %q", got.Duration)
		}
	})

	t.Run("unknown key returns error", func(t *testing.T) {
		p := CommandParams{URI: "{{ missing }}"}
		_, err := p.InterpolateParams(map[string]string{})
		if err == nil {
			t.Fatal("expected error for unknown placeholder key")
		}
		if !strings.Contains(err.Error(), "missing") {
			t.Errorf("expected error to mention key name, got %v", err)
		}
	})

	t.Run("no placeholders pass through unchanged", func(t *testing.T) {
		p := CommandParams{URI: "spotify:playlist:abc", RepeatState: "context"}
		got, err := p.InterpolateParams(map[string]string{"uri": "other"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.URI != "spotify:playlist:abc" {
			t.Errorf("URI: got %q", got.URI)
		}
		if got.RepeatState != "context" {
			t.Errorf("RepeatState: got %q", got.RepeatState)
		}
	})

	t.Run("non-string fields unaffected", func(t *testing.T) {
		level := IntOrTemplate{Value: 42}
		p := CommandParams{Level: &level, URI: "{{ uri }}"}
		got, err := p.InterpolateParams(map[string]string{"uri": "spotify:album:z"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Level == nil || got.Level.Value != 42 {
			t.Errorf("Level: expected 42, got %v", got.Level)
		}
	})
}

// ----- ResolvedDeviceID ------------------------------------------------------

func TestResolvedDeviceID(t *testing.T) {
	c := Command{DeviceID: "cmd-dev", Params: CommandParams{}}
	if got := c.ResolvedDeviceID("set-dev"); got != "cmd-dev" {
		t.Errorf("expected cmd-dev, got %q", got)
	}
	c.DeviceID = ""
	if got := c.ResolvedDeviceID("set-dev"); got != "set-dev" {
		t.Errorf("expected set-dev, got %q", got)
	}
	if got := c.ResolvedDeviceID(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ----- TimeoutDuration -------------------------------------------------------

func TestTimeoutDuration(t *testing.T) {
	cases := []struct {
		timeout string
		def     time.Duration
		want    time.Duration
	}{
		{"", 10 * time.Second, 10 * time.Second},
		{"5s", 10 * time.Second, 5 * time.Second},
		{"500ms", 10 * time.Second, 500 * time.Millisecond},
		{"bad", 10 * time.Second, 10 * time.Second},
		{"-1s", 10 * time.Second, 10 * time.Second},
		{"0s", 10 * time.Second, 10 * time.Second},
	}
	for _, tc := range cases {
		c := Command{Timeout: tc.timeout}
		if got := c.TimeoutDuration(tc.def); got != tc.want {
			t.Errorf("TimeoutDuration(%q, %v) = %v, want %v", tc.timeout, tc.def, got, tc.want)
		}
	}
}

// ----- EffectiveOnError / EffectiveOnTimeout ---------------------------------

func TestEffectiveOnError(t *testing.T) {
	cases := []struct {
		cmdVal     OnFailure
		setDefault OnFailure
		want       OnFailure
	}{
		{"", "", OnFailureFail},
		{"", OnFailureFail, OnFailureFail},
		{OnFailureSkipRemaining, OnFailureFail, OnFailureSkipRemaining},
		{OnFailureContinue, OnFailureFail, OnFailureContinue},
	}
	for _, tc := range cases {
		c := Command{OnError: tc.cmdVal}
		if got := c.EffectiveOnError(tc.setDefault); got != tc.want {
			t.Errorf("EffectiveOnError(cmd=%q, set=%q) = %q, want %q", tc.cmdVal, tc.setDefault, got, tc.want)
		}
	}
}

func TestEffectiveOnTimeout(t *testing.T) {
	cases := []struct {
		cmdVal     OnFailure
		setDefault OnFailure
		want       OnFailure
	}{
		{"", "", OnFailureFail},
		{"", OnFailureFail, OnFailureFail},
		{OnFailureSkipRemaining, OnFailureFail, OnFailureSkipRemaining},
	}
	for _, tc := range cases {
		c := Command{OnTimeout: tc.cmdVal}
		if got := c.EffectiveOnTimeout(tc.setDefault); got != tc.want {
			t.Errorf("EffectiveOnTimeout(cmd=%q, set=%q) = %q, want %q", tc.cmdVal, tc.setDefault, got, tc.want)
		}
	}
}

// ----- CommandParams helpers -------------------------------------------------

func TestShuffleEnabled(t *testing.T) {
	p := CommandParams{}
	if !p.ShuffleEnabled() {
		t.Error("expected default ShuffleEnabled to be true")
	}
	f := false
	p.Enabled = &f
	if p.ShuffleEnabled() {
		t.Error("expected ShuffleEnabled false when Enabled=false")
	}
}

func TestTransferPlay(t *testing.T) {
	p := CommandParams{}
	if p.TransferPlay() {
		t.Error("expected default TransferPlay to be false")
	}
	tr := true
	p.Play = &tr
	if !p.TransferPlay() {
		t.Error("expected TransferPlay true when Play=true")
	}
}

// ----- Validate --------------------------------------------------------------

func TestValidateAllActions(t *testing.T) {
	cases := []struct {
		action  string
		params  CommandParams
		wantErr bool
	}{
		{"play", CommandParams{}, false},
		{"play", CommandParams{URI: "spotify:track:123"}, false},
		{"play", CommandParams{PlaylistID: "pl123"}, false},
		{"play", CommandParams{TrackID: "tr456"}, false},
		{"play", CommandParams{AlbumID: "al789"}, false},
		{"play", CommandParams{ArtistID: "ar123"}, false},
		{"pause", CommandParams{}, false},
		{"next", CommandParams{}, false},
		{"previous", CommandParams{}, false},
		{"shuffle", CommandParams{}, false},
		{"transfer", CommandParams{}, false},
		// volume requires level.
		{"volume", CommandParams{Level: &IntOrTemplate{Value: 42}}, false},
		{"volume", CommandParams{}, true},
		// repeat requires state and it must be a valid value.
		{"repeat", CommandParams{RepeatState: "off"}, false},
		{"repeat", CommandParams{RepeatState: "track"}, false},
		{"repeat", CommandParams{RepeatState: "context"}, false},
		{"repeat", CommandParams{RepeatState: "loop"}, true},
		{"repeat", CommandParams{}, true},
		// sleep requires valid duration.
		{"sleep", CommandParams{Duration: "1s"}, false},
		{"sleep", CommandParams{Duration: "bad"}, true},
		{"sleep", CommandParams{}, true},
		// run_set requires set name.
		{"run_set", CommandParams{Set: "s"}, false},
		{"run_set", CommandParams{}, true},
		{"bogus", CommandParams{}, true},
	}
	for _, tc := range cases {
		err := tc.params.Validate(tc.action)
		if (err != nil) != tc.wantErr {
			t.Errorf("Validate(%q, %+v): err=%v, wantErr=%v", tc.action, tc.params, err, tc.wantErr)
		}
	}
}

// ----- Config without sets still loads --------------------------------------

func TestLoadConfigWithoutSets(t *testing.T) {
	yaml := "client_id: id\nclient_secret: s\nrefresh_token: r\n"
	cfg, err := Load(writeYAML(t, yaml))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.Sets) != 0 {
		t.Errorf("expected empty sets, got %v", cfg.Sets)
	}
}

// ----- OnFailure constants ---------------------------------------------------

func TestOnFailureValues(t *testing.T) {
	cases := []struct {
		val  OnFailure
		want string
	}{
		{OnFailureFail, "fail"},
		{OnFailureContinue, "continue"},
		{OnFailureSkipRemaining, "skip_remaining"},
	}
	for _, tc := range cases {
		if string(tc.val) != tc.want {
			t.Errorf("OnFailure value: got %q, want %q", tc.val, tc.want)
		}
	}
}

// ----- URI variants in params ------------------------------------------------

func TestCommandParamsURIVariants(t *testing.T) {
	yaml := `
client_id: id
client_secret: secret
refresh_token: refresh
sets:
  test:
    commands:
      - action: play
        params:
          playlist: pl123
      - action: play
        params:
          track: tr456
      - action: play
        params:
          album: al789
      - action: play
        params:
          uri: spotify:artist:xyz
`
	cfg, err := Load(writeYAML(t, yaml))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	cmds := cfg.Sets["test"].Commands
	if len(cmds) != 4 {
		t.Fatalf("expected 4 commands, got %d", len(cmds))
	}
	if cmds[0].Params.PlaylistID != "pl123" {
		t.Errorf("playlist: got %q", cmds[0].Params.PlaylistID)
	}
	if cmds[1].Params.TrackID != "tr456" {
		t.Errorf("track: got %q", cmds[1].Params.TrackID)
	}
	if cmds[2].Params.AlbumID != "al789" {
		t.Errorf("album: got %q", cmds[2].Params.AlbumID)
	}
	if cmds[3].Params.URI != "spotify:artist:xyz" {
		t.Errorf("uri: got %q", cmds[3].Params.URI)
	}
}

// ----- command name is optional ----------------------------------------------

func TestCommandNameOptional(t *testing.T) {
	yaml := `
client_id: id
client_secret: secret
refresh_token: refresh
sets:
  test:
    commands:
      - action: sleep
        params:
          duration: 1s
`
	cfg, err := Load(writeYAML(t, yaml))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Sets["test"].Commands[0].Name != "" {
		t.Error("expected empty name")
	}
}

// ----- Validate sleep bad duration message -----------------------------------

func TestValidateSleepBadDurationMessage(t *testing.T) {
	p := CommandParams{Duration: "nope"}
	err := p.Validate("sleep")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected 'invalid' in error, got: %v", err)
	}
}

// ----- Validate repeat -------------------------------------------------------

func TestValidateRepeatInvalidStateMessage(t *testing.T) {
	p := CommandParams{RepeatState: "badvalue"}
	err := p.Validate("repeat")
	if err == nil {
		t.Fatal("expected error for invalid state")
	}
	if !strings.Contains(err.Error(), "badvalue") {
		t.Errorf("expected invalid value in error, got: %v", err)
	}
}

func TestValidateRepeatMissingState(t *testing.T) {
	p := CommandParams{}
	err := p.Validate("repeat")
	if err == nil {
		t.Fatal("expected error for missing state")
	}
	if !strings.Contains(err.Error(), "state") {
		t.Errorf("expected 'state' in error, got: %v", err)
	}
}

// ----- IntOrTemplate --------------------------------------------------------

func TestIntOrTemplate_UnmarshalYAML(t *testing.T) {
	t.Run("int value", func(t *testing.T) {
		yaml := `
client_id: id
client_secret: secret
refresh_token: refresh
sets:
  test:
    commands:
      - action: volume
        params:
          level: 42
`
		cfg, err := Load(writeYAML(t, yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		level := cfg.Sets["test"].Commands[0].Params.Level
		if level == nil || level.Value != 42 || level.Expr != "" {
			t.Errorf("expected Value=42, got %+v", level)
		}
	})

	t.Run("template expression", func(t *testing.T) {
		yaml := `
client_id: id
client_secret: secret
refresh_token: refresh
sets:
  test:
    commands:
      - action: volume
        params:
          level: '{{ volume }}'
`
		cfg, err := Load(writeYAML(t, yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		level := cfg.Sets["test"].Commands[0].Params.Level
		if level == nil || level.Expr != "{{ volume }}" || level.Value != 0 {
			t.Errorf("expected Expr={{ volume }}, got %+v", level)
		}
	})

	t.Run("zero is valid", func(t *testing.T) {
		yaml := `
client_id: id
client_secret: secret
refresh_token: refresh
sets:
  test:
    commands:
      - action: volume
        params:
          level: 0
`
		cfg, err := Load(writeYAML(t, yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		level := cfg.Sets["test"].Commands[0].Params.Level
		if level == nil || level.Value != 0 {
			t.Errorf("expected Value=0, got %+v", level)
		}
	})
}

func TestIntOrTemplate_MarshalYAML(t *testing.T) {
	t.Run("marshals int", func(t *testing.T) {
		v := IntOrTemplate{Value: 35}
		out, err := v.MarshalYAML()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != 35 {
			t.Errorf("expected 35, got %v", out)
		}
	})

	t.Run("marshals expr as string", func(t *testing.T) {
		v := IntOrTemplate{Expr: "{{ volume }}"}
		out, err := v.MarshalYAML()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "{{ volume }}" {
			t.Errorf("expected expr string, got %v", out)
		}
	})
}

func TestIntOrTemplate_Resolved(t *testing.T) {
	t.Run("resolved int", func(t *testing.T) {
		v := IntOrTemplate{Value: 42}
		got, err := v.Resolved()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 42 {
			t.Errorf("expected 42, got %d", got)
		}
	})

	t.Run("unresolved expr errors", func(t *testing.T) {
		v := IntOrTemplate{Expr: "{{ volume }}"}
		_, err := v.Resolved()
		if err == nil {
			t.Fatal("expected error for unresolved expr")
		}
		if !strings.Contains(err.Error(), "volume") {
			t.Errorf("expected expr in error, got: %v", err)
		}
	})
}

// ----- ForwardedArgs ---------------------------------------------------------

func TestForwardedArgs(t *testing.T) {
	t.Run("uri forwarded", func(t *testing.T) {
		p := CommandParams{Set: "my_set", URI: "spotify:playlist:abc"}
		got := p.ForwardedArgs()
		if got["uri"] != "spotify:playlist:abc" {
			t.Errorf("expected uri forwarded, got %v", got)
		}
	})

	t.Run("playlist forwarded", func(t *testing.T) {
		p := CommandParams{Set: "my_set", PlaylistID: "pl123"}
		got := p.ForwardedArgs()
		if got["playlist"] != "pl123" {
			t.Errorf("expected playlist forwarded, got %v", got)
		}
	})

	t.Run("track forwarded", func(t *testing.T) {
		p := CommandParams{Set: "my_set", TrackID: "tr456"}
		got := p.ForwardedArgs()
		if got["track"] != "tr456" {
			t.Errorf("expected track forwarded, got %v", got)
		}
	})

	t.Run("album forwarded", func(t *testing.T) {
		p := CommandParams{Set: "my_set", AlbumID: "al789"}
		got := p.ForwardedArgs()
		if got["album"] != "al789" {
			t.Errorf("expected album forwarded, got %v", got)
		}
	})

	t.Run("artist forwarded", func(t *testing.T) {
		p := CommandParams{Set: "my_set", ArtistID: "ar999"}
		got := p.ForwardedArgs()
		if got["artist"] != "ar999" {
			t.Errorf("expected artist forwarded, got %v", got)
		}
	})

	t.Run("duration forwarded", func(t *testing.T) {
		p := CommandParams{Set: "my_set", Duration: "5s"}
		got := p.ForwardedArgs()
		if got["duration"] != "5s" {
			t.Errorf("expected duration forwarded, got %v", got)
		}
	})

	t.Run("repeat state forwarded", func(t *testing.T) {
		p := CommandParams{Set: "my_set", RepeatState: "context"}
		got := p.ForwardedArgs()
		if got["state"] != "context" {
			t.Errorf("expected state forwarded, got %v", got)
		}
	})

	t.Run("level int forwarded", func(t *testing.T) {
		p := CommandParams{Set: "my_set", Level: &IntOrTemplate{Value: 50}}
		got := p.ForwardedArgs()
		if got["volume"] != "50" {
			t.Errorf("expected volume=50 forwarded, got %v", got)
		}
	})

	t.Run("level expr forwarded", func(t *testing.T) {
		p := CommandParams{Set: "my_set", Level: &IntOrTemplate{Expr: "{{ volume }}"}}
		got := p.ForwardedArgs()
		if got["volume"] != "{{ volume }}" {
			t.Errorf("expected expr forwarded, got %v", got)
		}
	})

	t.Run("empty params returns nil", func(t *testing.T) {
		p := CommandParams{Set: "my_set"}
		if got := p.ForwardedArgs(); got != nil {
			t.Errorf("expected nil for empty params, got %v", got)
		}
	})
}

// ----- InterpolateParams: level expr -----------------------------------------

func TestInterpolateParams_LevelExpr(t *testing.T) {
	t.Run("level expr resolved to int", func(t *testing.T) {
		p := CommandParams{Level: &IntOrTemplate{Expr: "{{ volume }}"}}
		got, err := p.InterpolateParams(map[string]string{"volume": "42"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Level == nil || got.Level.Value != 42 || got.Level.Expr != "" {
			t.Errorf("expected resolved Level=42, got %+v", got.Level)
		}
	})

	t.Run("level expr resolves to non-int errors", func(t *testing.T) {
		p := CommandParams{Level: &IntOrTemplate{Expr: "{{ volume }}"}}
		_, err := p.InterpolateParams(map[string]string{"volume": "loud"})
		if err == nil {
			t.Fatal("expected error for non-int level")
		}
		if !strings.Contains(err.Error(), "not an integer") {
			t.Errorf("expected 'not an integer' in error, got: %v", err)
		}
	})
}

func TestForwardedArgs_InlineForwarded(t *testing.T) {
	p := CommandParams{
		Set:      "my_set",
		Forwarded: map[string]string{"custom_key": "custom_val"},
	}
	got := p.ForwardedArgs()
	if got["custom_key"] != "custom_val" {
		t.Errorf("expected custom_key=custom_val in forwarded args, got %v", got)
	}
}

func TestInterpolateParams_LevelExprMissingKey(t *testing.T) {
	p := CommandParams{Level: &IntOrTemplate{Expr: "{{ missing_vol }}"}}
	_, err := p.InterpolateParams(map[string]string{})
	if err == nil {
		t.Fatal("expected error when Level.Expr key is missing from resolved map")
	}
	if !strings.Contains(err.Error(), "missing_vol") {
		t.Errorf("expected missing key in error, got: %v", err)
	}
}

func TestIntOrTemplate_UnmarshalYAML_InvalidType(t *testing.T) {
	yaml := `
client_id: id
client_secret: secret
refresh_token: refresh
sets:
  test:
    commands:
      - action: volume
        params:
          level:
            - 1
            - 2
`
	_, err := Load(writeYAML(t, yaml))
	if err == nil {
		t.Fatal("expected error when level is a YAML sequence (not int or string)")
	}
}

// ----- ValidRepeatStates ----------------------------------------------------

func TestValidRepeatStates(t *testing.T) {
	for _, state := range []string{"off", "track", "context"} {
		if !ValidRepeatStates[state] {
			t.Errorf("expected %q to be a valid repeat state", state)
		}
	}
	if ValidRepeatStates["loop"] {
		t.Error("expected 'loop' to be invalid")
	}
}

// ----- ResolveDeviceID --------------------------------------------------------

func TestResolveDeviceID(t *testing.T) {
	t.Run("literal passes through unchanged", func(t *testing.T) {
		got, err := ResolveDeviceID("dev123", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "dev123" {
			t.Errorf("got %q, want dev123", got)
		}
	})

	t.Run("empty string passes through unchanged", func(t *testing.T) {
		got, err := ResolveDeviceID("", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("placeholder resolved from map", func(t *testing.T) {
		got, err := ResolveDeviceID("{{ device }}", map[string]string{"device": "dev456"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "dev456" {
			t.Errorf("got %q, want dev456", got)
		}
	})

	t.Run("missing placeholder errors", func(t *testing.T) {
		_, err := ResolveDeviceID("{{ device }}", map[string]string{})
		if err == nil {
			t.Fatal("expected error for unresolved placeholder")
		}
		if !strings.Contains(err.Error(), "device") {
			t.Errorf("expected 'device' in error, got: %v", err)
		}
	})
}
