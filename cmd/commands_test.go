package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

func resetPlayCmdFlags(t *testing.T) {
	t.Helper()
	for _, name := range []string{"uri", "playlist", "track", "album", "artist"} {
		if flag := playCmd.Flags().Lookup(name); flag != nil {
			flag.Changed = false
		}
	}
}

func resetVolumeCmdFlags(t *testing.T) {
	t.Helper()
	if flag := volumeCmd.Flags().Lookup("level"); flag != nil {
		flag.Changed = false
		flag.Value.Set("0")
	}
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	output, err := io.ReadAll(r)
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}

func TestPlayCmdRunE_NoDevice(t *testing.T) {
	oldConfigPath := configPath
	oldPlayDeviceID := playDeviceID
	defer func() {
		configPath = oldConfigPath
		playDeviceID = oldPlayDeviceID
	}()

	playDeviceID = ""
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	resetPlayCmdFlags(t)

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Query().Get("device_id") != "" {
			t.Errorf("expected no device_id, got %q", r.URL.Query().Get("device_id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := playCmd.RunE(playCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected play API to be called")
	}
}

func TestPlayCmdRunE_WithDevice(t *testing.T) {
	oldConfigPath := configPath
	oldPlayDeviceID := playDeviceID
	defer func() {
		configPath = oldConfigPath
		playDeviceID = oldPlayDeviceID
	}()

	playDeviceID = "device-1"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	resetPlayCmdFlags(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("device_id") != "device-1" {
			t.Errorf("expected device_id=device-1, got %q", r.URL.Query().Get("device_id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := playCmd.RunE(playCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransferCmdRunE_RequiresDevice(t *testing.T) {
	oldTransferDeviceID := transferDeviceID
	defer func() { transferDeviceID = oldTransferDeviceID }()
	transferDeviceID = ""

	err := transferCmd.RunE(transferCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--device") {
		t.Fatalf("expected missing device error, got %v", err)
	}
}

func TestTransferCmdRunE_Success(t *testing.T) {
	oldConfigPath := configPath
	oldTransferDeviceID := transferDeviceID
	oldTransferPlay := transferPlay
	defer func() {
		configPath = oldConfigPath
		transferDeviceID = oldTransferDeviceID
		transferPlay = oldTransferPlay
	}()

	transferDeviceID = "device-1"
	transferPlay = true
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["play"] != true {
			t.Fatalf("expected play=true, got %v", payload["play"])
		}
		ids, _ := payload["device_ids"].([]interface{})
		if len(ids) != 1 || ids[0] != "device-1" {
			t.Fatalf("unexpected device_ids: %v", payload["device_ids"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := transferCmd.RunE(transferCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ----- volume command --------------------------------------------------------

func TestVolumeCmdRunE_NoLevel(t *testing.T) {
	defer resetVolumeCmdFlags(t)

	err := volumeCmd.RunE(volumeCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "requires --level") {
		t.Fatalf("expected missing-level error, got %v", err)
	}
}

func TestVolumeCmdRunE_InvalidLevel(t *testing.T) {
	defer resetVolumeCmdFlags(t)

	if err := volumeCmd.Flags().Set("level", "101"); err != nil {
		t.Fatal(err)
	}

	err := volumeCmd.RunE(volumeCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "volume must be between 0 and 100") {
		t.Fatalf("expected invalid volume error, got %v", err)
	}
}

func TestVolumeCmdRunE_Success(t *testing.T) {
	oldConfigPath := configPath
	oldVolumeDeviceID := volumeDeviceID
	defer func() {
		configPath = oldConfigPath
		volumeDeviceID = oldVolumeDeviceID
		resetVolumeCmdFlags(t)
	}()

	volumeDeviceID = "device-1"
	if err := volumeCmd.Flags().Set("level", "42"); err != nil {
		t.Fatal(err)
	}
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("device_id") != "device-1" {
			t.Errorf("expected device_id=device-1, got %q", r.URL.Query().Get("device_id"))
		}
		if r.URL.Query().Get("volume_percent") != "42" {
			t.Errorf("expected volume_percent=42, got %q", r.URL.Query().Get("volume_percent"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := volumeCmd.RunE(volumeCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ----- status command --------------------------------------------------------

func TestRunStatus_NoPlayback(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	cleanup := wireClient(t, srv)
	defer cleanup()

	output := captureOutput(t, func() {
		if err := statusCmd.RunE(statusCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "No active playback found.") {
		t.Errorf("expected no-playback message, got: %q", output)
	}
}

func TestRunStatus_WithFullPlayback(t *testing.T) {
	state := spotify.PlaybackState{
		IsPlaying:    true,
		ShuffleState: true,
		RepeatState:  "context",
		ProgressMS:   60000,
		Device: spotify.Device{
			Name:          "Bedroom Speaker",
			Type:          "Speaker",
			IsActive:      true,
			VolumePercent: 55,
		},
		Item: &spotify.Track{
			Name:       "Test Track",
			URI:        "spotify:track:abc",
			DurationMS: 180000,
			Artists:    []spotify.Artist{{Name: "Artist One"}, {Name: "Artist Two"}},
		},
		Context: &spotify.PlaybackContext{
			URI:  "spotify:playlist:xyz",
			Type: "playlist",
		},
	}

	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	}))
	defer srv.Close()
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	cleanup := wireClient(t, srv)
	defer cleanup()

	output := captureOutput(t, func() {
		if err := statusCmd.RunE(statusCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{
		"Bedroom Speaker", "Speaker", "(active)", "playing",
		"shuffle: true", "repeat: context", "volume: 55%",
		"Test Track", "Artist One, Artist Two",
		"01:00", "03:00", "spotify:playlist:xyz",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got:\n%s", want, output)
		}
	}
}
