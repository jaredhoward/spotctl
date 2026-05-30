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
          repeat_state: context
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
	if !c0.Confirm {
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
	if c2.Action != "volume" || c2.Params.Level == nil || *c2.Params.Level != 60 {
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
		t.Errorf("command 5 repeat_state: got %q, want context", c5.Params.RepeatState)
	}

	warmup, ok := cfg.Sets["warmup"]
	if !ok {
		t.Fatal("expected set 'warmup'")
	}
	if len(warmup.Commands) != 1 || warmup.Commands[0].Action != "next" {
		t.Errorf("warmup commands unexpected: %+v", warmup.Commands)
	}
}

// ----- ResolvedDeviceID ------------------------------------------------------

func TestResolvedDeviceID(t *testing.T) {
	c := Command{Params: CommandParams{DeviceID: "cmd-dev"}}
	if got := c.ResolvedDeviceID("set-dev"); got != "cmd-dev" {
		t.Errorf("expected cmd-dev, got %q", got)
	}
	c.Params.DeviceID = ""
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
		{"", "", OnFailureContinue},
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
		{"", "", OnFailureContinue},
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
	level := 42
	cases := []struct {
		action  string
		params  CommandParams
		wantErr bool
	}{
		{"play", CommandParams{}, false},
		{"play", CommandParams{DeviceID: "d"}, false},
		{"pause", CommandParams{}, false},
		{"pause", CommandParams{DeviceID: "d"}, false},
		{"next", CommandParams{}, false},
		{"next", CommandParams{DeviceID: "d"}, false},
		{"previous", CommandParams{}, false},
		{"previous", CommandParams{DeviceID: "d"}, false},
		{"shuffle", CommandParams{}, false},
		{"shuffle", CommandParams{DeviceID: "d"}, false},
		{"transfer", CommandParams{}, false},
		{"transfer", CommandParams{DeviceID: "d"}, false},
		// volume requires level.
		{"volume", CommandParams{Level: &level}, false},
		{"volume", CommandParams{DeviceID: "d", Level: &level}, false},
		{"volume", CommandParams{}, true},
		// repeat requires repeat_state and it must be a valid value.
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
		t.Fatal("expected error for invalid repeat_state")
	}
	if !strings.Contains(err.Error(), "badvalue") {
		t.Errorf("expected invalid value in error, got: %v", err)
	}
}

func TestValidateRepeatMissingState(t *testing.T) {
	p := CommandParams{}
	err := p.Validate("repeat")
	if err == nil {
		t.Fatal("expected error for missing repeat_state")
	}
	if !strings.Contains(err.Error(), "repeat_state") {
		t.Errorf("expected 'repeat_state' in error, got: %v", err)
	}
}

// ----- ValidRepeatStates -----------------------------------------------------

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
