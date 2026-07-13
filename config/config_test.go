package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestPlaybackPollIntervalDurationDefault(t *testing.T) {
	cfg := &Config{}
	if got := cfg.PlaybackPollIntervalDuration(); got != DefaultPlaybackPollInterval {
		t.Fatalf("expected default %v, got %v", DefaultPlaybackPollInterval, got)
	}
}

func TestPlaybackPollIntervalDurationValid(t *testing.T) {
	cfg := &Config{PlaybackPollInterval: "250ms"}
	if got := cfg.PlaybackPollIntervalDuration(); got != 250*time.Millisecond {
		t.Fatalf("expected 250ms, got %v", got)
	}
}

func TestPlaybackPollIntervalDurationInvalidFallsBack(t *testing.T) {
	cfg := &Config{PlaybackPollInterval: "notaduration"}
	if got := cfg.PlaybackPollIntervalDuration(); got != DefaultPlaybackPollInterval {
		t.Fatalf("expected default on invalid value, got %v", got)
	}
}

func TestPlaybackPollIntervalDurationZeroFallsBack(t *testing.T) {
	cfg := &Config{PlaybackPollInterval: "0s"}
	if got := cfg.PlaybackPollIntervalDuration(); got != DefaultPlaybackPollInterval {
		t.Fatalf("expected default on zero duration, got %v", got)
	}
}

func TestPlaybackPollIntervalDurationNegativeFallsBack(t *testing.T) {
	cfg := &Config{PlaybackPollInterval: "-1s"}
	if got := cfg.PlaybackPollIntervalDuration(); got != DefaultPlaybackPollInterval {
		t.Fatalf("expected default on negative duration, got %v", got)
	}
}

func TestClientB64(t *testing.T) {
	cfg := &Config{ClientID: "myclient", ClientSecret: "mysecret"}
	// base64("myclient:mysecret")
	want := "bXljbGllbnQ6bXlzZWNyZXQ="
	if got := cfg.ClientB64(); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load("/nonexistent/path/config.yaml"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	f, err := os.CreateTemp("", "spotctl-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(":\tinvalid yaml\t:")
	f.Close()

	if _, err := Load(f.Name()); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadMissingFields(t *testing.T) {
	cases := []struct {
		name    string
		content string
		missing string
	}{
		{"missing client_id", "client_secret: s\nrefresh_token: r\n", "client_id"},
		{"missing client_secret", "client_id: id\nrefresh_token: r\n", "client_secret"},
		{"missing refresh_token", "client_id: id\nclient_secret: s\n", "refresh_token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.CreateTemp("", "spotctl-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())
			f.WriteString(tc.content)
			f.Close()

			_, err = Load(f.Name())
			if err == nil {
				t.Fatalf("expected validation error for %s", tc.missing)
			}
		})
	}
}

func TestLoadAndSaveRoundTrip(t *testing.T) {
	f, err := os.CreateTemp("", "spotctl-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	level := IntOrTemplate{Value: 60}
	original := &Config{
		ClientID:             "id",
		ClientSecret:         "secret",
		RefreshToken:         "refresh",
		RedirectURI:          "https://example.com/callback",
		PlaybackPollInterval: "1s",
		Sets: map[string]Set{
			"sleep": {
				Commands: []Command{
					{
						Action:   "play",
						DeviceID: "dev1",
						Params:   CommandParams{URI: "spotify:playlist:abc"},
						Confirm:  func() *bool { b := true; return &b }(),
					},
					{
						Action:   "volume",
						DeviceID: "dev1",
						Params:   CommandParams{Level: &level},
					},
				},
			},
		},
		DeviceNames: map[string]string{"dev1": "Bedroom Speaker"},
	}

	if err := Save(f.Name(), original); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	if loaded.ClientID != original.ClientID {
		t.Errorf("ClientID mismatch: got %q", loaded.ClientID)
	}
	if loaded.PlaybackPollInterval != original.PlaybackPollInterval {
		t.Errorf("PlaybackPollInterval mismatch: got %q", loaded.PlaybackPollInterval)
	}
	sleep, ok := loaded.Sets["sleep"]
	if !ok {
		t.Fatal("set 'sleep' not preserved after round-trip")
	}
	if len(sleep.Commands) != 2 {
		t.Errorf("expected 2 commands in sleep set, got %d", len(sleep.Commands))
	}
	if sleep.Commands[0].DeviceID != "dev1" {
		t.Errorf("device_id not preserved: %+v", sleep.Commands[0])
	}
	if loaded.DeviceNames["dev1"] != "Bedroom Speaker" {
		t.Errorf("device name not preserved: %+v", loaded.DeviceNames)
	}
}

func TestSaveFailsOnUnwritablePath(t *testing.T) {
	cfg := &Config{ClientID: "id", ClientSecret: "s", RefreshToken: "r"}
	if err := Save("/nonexistent/dir/config.yaml", cfg); err == nil {
		t.Fatal("expected error saving to unwritable path")
	}
}

func TestSaveNewFileDefaultsTo0600(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"
	cfg := &Config{ClientID: "id", ClientSecret: "s", RefreshToken: "r"}

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("expected new config file to be 0600, got %o", got)
	}
}

func TestSavePreservesExistingPermissions(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"
	cfg := &Config{ClientID: "id", ClientSecret: "s", RefreshToken: "r"}

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}

	cfg.RefreshToken = "rotated"
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0644 {
		t.Errorf("expected Save to preserve 0644 permissions, got %o", got)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RefreshToken != "rotated" {
		t.Errorf("expected rotated content to be saved, got %q", loaded.RefreshToken)
	}
}

func TestSaveLeavesNoTempFilesBehind(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"
	cfg := &Config{ClientID: "id", ClientSecret: "s", RefreshToken: "r"}

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	// Save again to exercise the permission-preservation path too.
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected only config.yaml in dir, got %v", names)
	}
}

func TestSaveDoesNotDestroyExistingFileOnTempCreateFailure(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"
	original := &Config{ClientID: "id", ClientSecret: "s", RefreshToken: "original"}
	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}

	// Make the directory read-only so the temp file create fails, simulating
	// a write failure partway through Save without touching the destination.
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0700)

	failing := &Config{ClientID: "id", ClientSecret: "s", RefreshToken: "should-not-be-saved"}
	if err := Save(path, failing); err == nil {
		t.Fatal("expected Save to fail when the directory is read-only")
	}

	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RefreshToken != "original" {
		t.Errorf("expected original content to survive a failed Save, got %q", loaded.RefreshToken)
	}
}

func TestValidateMultipleMissingFields(t *testing.T) {
	cfg := &Config{}
	err := cfg.validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	msg := err.Error()
	for _, field := range []string{"client_id", "client_secret", "refresh_token"} {
		if !strings.Contains(msg, field) {
			t.Errorf("expected %q in error: %s", field, msg)
		}
	}
}

func validConfigWithParam(param SetParam) *Config {
	return &Config{
		ClientID: "id", ClientSecret: "s", RefreshToken: "r",
		Sets: map[string]Set{
			"speaker_sleep": {
				Params: map[string]SetParam{"uri": param},
			},
		},
	}
}

func TestValidate_PoolAndDefaultRejected(t *testing.T) {
	cfg := validConfigWithParam(SetParam{Pool: []string{"a", "b"}, Default: "a"})
	err := cfg.validate()
	if err == nil {
		t.Fatal("expected validation error for pool+default")
	}
	if !strings.Contains(err.Error(), "speaker_sleep") || !strings.Contains(err.Error(), "uri") {
		t.Errorf("expected set/param name in error, got: %v", err)
	}
}

func TestValidate_PoolAndRequiredRejected(t *testing.T) {
	cfg := validConfigWithParam(SetParam{Pool: []string{"a", "b"}, Required: true})
	err := cfg.validate()
	if err == nil {
		t.Fatal("expected validation error for pool+required")
	}
	if !strings.Contains(err.Error(), "speaker_sleep") || !strings.Contains(err.Error(), "uri") {
		t.Errorf("expected set/param name in error, got: %v", err)
	}
}

func TestValidate_PoolAloneIsValid(t *testing.T) {
	cfg := validConfigWithParam(SetParam{Pool: []string{"a", "b"}})
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected pool alone to be valid, got: %v", err)
	}
}

func TestValidate_PoolMethodRandomIsValid(t *testing.T) {
	cfg := validConfigWithParam(SetParam{Pool: []string{"a", "b"}, Method: PoolMethodRandom})
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected pool with method random to be valid, got: %v", err)
	}
}

func TestValidate_PoolMethodDateIsValid(t *testing.T) {
	cfg := validConfigWithParam(SetParam{Pool: []string{"a", "b"}, Method: PoolMethodDate})
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected pool with method date to be valid, got: %v", err)
	}
}

func TestValidate_PoolMethodInvalidValueRejected(t *testing.T) {
	cfg := validConfigWithParam(SetParam{Pool: []string{"a", "b"}, Method: "bogus"})
	err := cfg.validate()
	if err == nil {
		t.Fatal("expected validation error for invalid method value")
	}
	msg := err.Error()
	for _, want := range []string{"speaker_sleep", "uri", "method", "random", "date"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected %q in error, got: %v", want, msg)
		}
	}
}

func TestValidate_MethodWithoutPoolRejected(t *testing.T) {
	cfg := validConfigWithParam(SetParam{Default: "a", Method: PoolMethodRandom})
	err := cfg.validate()
	if err == nil {
		t.Fatal("expected validation error for method without pool")
	}
	if !strings.Contains(err.Error(), "method requires pool") {
		t.Errorf("expected \"method requires pool\" in error, got: %v", err)
	}
}

