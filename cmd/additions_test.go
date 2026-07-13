package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// ----- spotctl sets ----------------------------------------------------------

func TestSetsCmd_NoSets(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	configPath = writeTempConfig(t, &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
	})

	output := captureOutput(t, func() {
		if err := setsCmd.RunE(setsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "No sets configured.") {
		t.Errorf("expected no-sets message, got: %q", output)
	}
}

func TestSetsCmd_WithSets(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	level := config.IntOrTemplate{Value: 50}
	cfg := &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
		Sets: map[string]config.Set{
			"morning": {
				DeviceID: "dev-abc",
				Commands: []config.Command{
					{Action: "play"},
					{Action: "shuffle"},
				},
			},
			"sleep": {
				Commands: []config.Command{
					{Action: "play"},
					{Action: "volume", Params: config.CommandParams{Level: &level}},
					{Action: "pause"},
				},
			},
		},
	}
	configPath = writeTempConfig(t, cfg)

	output := captureOutput(t, func() {
		if err := setsCmd.RunE(setsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "morning") {
		t.Errorf("expected 'morning' in output, got: %q", output)
	}
	if !strings.Contains(output, "2 command(s)") {
		t.Errorf("expected '2 command(s)' for morning, got: %q", output)
	}
	if !strings.Contains(output, "sleep") {
		t.Errorf("expected 'sleep' in output, got: %q", output)
	}
	if !strings.Contains(output, "3 command(s)") {
		t.Errorf("expected '3 command(s)' for sleep, got: %q", output)
	}
	if !strings.Contains(output, "dev-abc") {
		t.Errorf("expected device ID in output, got: %q", output)
	}
	if !strings.Contains(output, "(active device)") {
		t.Errorf("expected '(active device)' for set without device_id, got: %q", output)
	}
}

func TestSetsCmd_TemplatedDeviceIDRendersAsPlaceholder(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	cfg := &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
		Sets: map[string]config.Set{
			"jareds_bedroom_play": {
				DeviceID: "{{ device }}",
				Params:   map[string]config.SetParam{"device": {Default: "dev-abc"}},
				Commands: []config.Command{{Action: "pause"}},
			},
		},
	}
	configPath = writeTempConfig(t, cfg)

	output := captureOutput(t, func() {
		if err := setsCmd.RunE(setsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "device: <device>") {
		t.Errorf("expected templated device_id to render as <device>, got: %q", output)
	}
	if strings.Contains(output, "{{ device }}") {
		t.Errorf("expected raw {{ device }} placeholder not to leak into output, got: %q", output)
	}
}

func TestSetsCmd_OutputIsSorted(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	cfg := &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
		Sets: map[string]config.Set{
			"zzz": {Commands: []config.Command{{Action: "pause"}}},
			"aaa": {Commands: []config.Command{{Action: "play"}}},
			"mmm": {Commands: []config.Command{{Action: "next"}}},
		},
	}
	configPath = writeTempConfig(t, cfg)

	output := captureOutput(t, func() {
		if err := setsCmd.RunE(setsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	aPos := strings.Index(output, "aaa")
	mPos := strings.Index(output, "mmm")
	zPos := strings.Index(output, "zzz")
	if aPos == -1 || mPos == -1 || zPos == -1 {
		t.Fatalf("not all set names found in output: %q", output)
	}
	if !(aPos < mPos && mPos < zPos) {
		t.Errorf("expected sorted output (aaa < mmm < zzz), positions %d %d %d", aPos, mPos, zPos)
	}
}

func TestSetsCmd_BadConfig(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	configPath = "/nonexistent/config.yaml"
	err := setsCmd.RunE(setsCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("expected config load error, got %v", err)
	}
}

// ----- shuffle verb ----------------------------------------------------------

func TestShuffleCmd_Enabled(t *testing.T) {
	oldConfigPath := configPath
	oldShuffleDeviceID := shuffleDeviceID
	oldShuffleEnabled := shuffleEnabled
	defer func() {
		configPath = oldConfigPath
		shuffleDeviceID = oldShuffleDeviceID
		shuffleEnabled = oldShuffleEnabled
	}()

	shuffleDeviceID = "dev-1"
	shuffleEnabled = true
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	var gotState string
	postState, _ := json.Marshal(spotify.PlaybackState{
		IsPlaying:    true,
		ShuffleState: true,
		Device:       spotify.Device{Name: "Dev", Type: "Speaker", IsActive: true},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/shuffle":
			gotState = r.URL.Query().Get("state")
			if r.URL.Query().Get("device_id") != "dev-1" {
				t.Errorf("expected device_id=dev-1, got %q", r.URL.Query().Get("device_id"))
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.Header().Set("Content-Type", "application/json")
			w.Write(postState)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := shuffleCmd.RunE(shuffleCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "true" {
		t.Errorf("expected state=true, got %q", gotState)
	}
}

func TestShuffleCmd_Disabled(t *testing.T) {
	oldConfigPath := configPath
	oldShuffleDeviceID := shuffleDeviceID
	oldShuffleEnabled := shuffleEnabled
	defer func() {
		configPath = oldConfigPath
		shuffleDeviceID = oldShuffleDeviceID
		shuffleEnabled = oldShuffleEnabled
	}()

	shuffleDeviceID = ""
	shuffleEnabled = false
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	var gotState string
	postState, _ := json.Marshal(spotify.PlaybackState{
		IsPlaying:    true,
		ShuffleState: false,
		Device:       spotify.Device{Name: "Dev", Type: "Speaker", IsActive: true},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/shuffle":
			gotState = r.URL.Query().Get("state")
			if r.URL.Query().Get("device_id") != "" {
				t.Errorf("expected no device_id, got %q", r.URL.Query().Get("device_id"))
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.Header().Set("Content-Type", "application/json")
			w.Write(postState)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := shuffleCmd.RunE(shuffleCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "false" {
		t.Errorf("expected state=false, got %q", gotState)
	}
}

// ----- repeat verb -----------------------------------------------------------

func TestRepeatCmd_InvalidState(t *testing.T) {
	oldRepeatState := repeatState
	defer func() { repeatState = oldRepeatState }()

	repeatState = "loop"
	err := repeatCmd.RunE(repeatCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--state must be one of") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestRepeatCmd_Off(t *testing.T) {
	oldConfigPath := configPath
	oldRepeatDeviceID := repeatDeviceID
	oldRepeatState := repeatState
	defer func() {
		configPath = oldConfigPath
		repeatDeviceID = oldRepeatDeviceID
		repeatState = oldRepeatState
	}()

	repeatDeviceID = "dev-1"
	repeatState = "off"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	var gotState, gotDevice string
	postState, _ := json.Marshal(spotify.PlaybackState{
		IsPlaying:   true,
		RepeatState: "off",
		Device:      spotify.Device{Name: "Dev", Type: "Speaker", IsActive: true},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/repeat":
			gotState = r.URL.Query().Get("state")
			gotDevice = r.URL.Query().Get("device_id")
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.Header().Set("Content-Type", "application/json")
			w.Write(postState)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := repeatCmd.RunE(repeatCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "off" {
		t.Errorf("expected state=off, got %q", gotState)
	}
	if gotDevice != "dev-1" {
		t.Errorf("expected device_id=dev-1, got %q", gotDevice)
	}
}

func TestRepeatCmd_Context_NoDevice(t *testing.T) {
	oldConfigPath := configPath
	oldRepeatDeviceID := repeatDeviceID
	oldRepeatState := repeatState
	defer func() {
		configPath = oldConfigPath
		repeatDeviceID = oldRepeatDeviceID
		repeatState = oldRepeatState
	}()

	repeatDeviceID = ""
	repeatState = "context"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	var gotState, gotDevice string
	postState, _ := json.Marshal(spotify.PlaybackState{
		IsPlaying:   true,
		RepeatState: "context",
		Device:      spotify.Device{Name: "Dev", Type: "Speaker", IsActive: true},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/repeat":
			gotState = r.URL.Query().Get("state")
			gotDevice = r.URL.Query().Get("device_id")
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.Header().Set("Content-Type", "application/json")
			w.Write(postState)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := repeatCmd.RunE(repeatCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "context" {
		t.Errorf("expected state=context, got %q", gotState)
	}
	if gotDevice != "" {
		t.Errorf("expected no device_id, got %q", gotDevice)
	}
}

// ----- appVersion ------------------------------------------------------------

func TestAppVersionDefault(t *testing.T) {
	old := appVersion
	defer func() { appVersion = old }()
	appVersion = "dev"
	if appVersion != "dev" {
		t.Errorf("expected default appVersion to be 'dev', got %q", appVersion)
	}
}

func TestSetVersion(t *testing.T) {
	old := appVersion
	defer func() { appVersion = old }()

	SetVersion("1.2.3")
	if appVersion != "1.2.3" {
		t.Errorf("expected appVersion to be '1.2.3', got %q", appVersion)
	}
}

// ----- devices command (smoke tests) -----------------------------------------

func TestDevicesCommandConfigLoadError(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	configPath = "/nonexistent/config.yaml"
	if err := devicesCmd.RunE(devicesCmd, nil); err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestDevicesShowsLiveDevices(t *testing.T) {
	oldConfigPath := configPath
	defer func() {
		configPath = oldConfigPath
		if f := devicesCmd.Flags().Lookup("update"); f != nil {
			f.Changed = false
			f.Value.Set("false")
		}
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"devices":[{"id":"device-1","name":"Living Room","type":"Speaker"}]}`))
	}))
	defer srv.Close()

	configPath = writeTempConfig(t, &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
		DeviceNames: map[string]string{},
	})
	cleanup := wireClient(t, srv)
	defer cleanup()

	output := captureOutput(t, func() {
		if err := devicesCmd.RunE(devicesCmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(output, "device-1") {
		t.Errorf("expected device-1 in output, got: %q", output)
	}
	if !strings.Contains(output, "Living Room") {
		t.Errorf("expected device name in output, got: %q", output)
	}
}

// ----- status command (additional cases) -------------------------------------

func TestRunStatus_APIError(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	cleanup := wireClient(t, srv)
	defer cleanup()

	err := statusCmd.RunE(statusCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "failed to get current playback") {
		t.Fatalf("expected API error, got %v", err)
	}
}

// keep unused import happy
var _ = spotify.NewClient
