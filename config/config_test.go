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

	level := 60
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
						Action: "play",
						Params: CommandParams{
							DeviceID: "dev1",
							URI:      "spotify:playlist:abc",
						},
						Confirm: true,
					},
					{
						Action: "volume",
						Params: CommandParams{DeviceID: "dev1", Level: &level},
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
	if sleep.Commands[0].Params.DeviceID != "dev1" {
		t.Errorf("device_id not preserved: %+v", sleep.Commands[0].Params)
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
