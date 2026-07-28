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

func TestPauseCmd_Success(t *testing.T) {
	oldConfigPath := configPath
	oldPauseDeviceID := pauseDeviceID
	defer func() {
		configPath = oldConfigPath
		pauseDeviceID = oldPauseDeviceID
	}()

	pauseDeviceID = "dev-1"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh", ConfirmStabilizeWindow: "5ms"})

	pauseCalled := false
	pausedState, _ := json.Marshal(spotify.PlaybackState{
		IsPlaying: false,
		Device:    spotify.Device{Name: "Dev", Type: "Speaker", IsActive: true, VolumePercent: 50},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/pause":
			if r.URL.Query().Get("device_id") != "dev-1" {
				t.Errorf("expected device_id=dev-1, got %q", r.URL.Query().Get("device_id"))
			}
			pauseCalled = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/":
			// Confirmation poll and final status fetch — return paused state.
			w.Header().Set("Content-Type", "application/json")
			w.Write(pausedState)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	output := captureOutput(t, func() {
		if err := pauseCmd.RunE(pauseCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !pauseCalled {
		t.Error("expected pause API to be called")
	}
	if !strings.Contains(output, "paused") {
		t.Errorf("expected status output to contain 'paused', got: %q", output)
	}
}

func TestNextCmd_Success(t *testing.T) {
	oldConfigPath := configPath
	oldNextDeviceID := nextDeviceID
	defer func() {
		configPath = oldConfigPath
		nextDeviceID = oldNextDeviceID
	}()

	nextDeviceID = "dev-1"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh", ConfirmStabilizeWindow: "5ms"})

	nextCalled := false
	preState, _ := json.Marshal(spotify.PlaybackState{
		IsPlaying: true,
		Device:    spotify.Device{Name: "Dev", Type: "Speaker", IsActive: true},
		Item:      &spotify.Track{URI: "spotify:track:before", Name: "Before", DurationMS: 180000, Artists: []spotify.Artist{{Name: "Artist"}}},
	})
	postState, _ := json.Marshal(spotify.PlaybackState{
		IsPlaying: true,
		Device:    spotify.Device{Name: "Dev", Type: "Speaker", IsActive: true},
		Item:      &spotify.Track{URI: "spotify:track:after", Name: "After", DurationMS: 180000, Artists: []spotify.Artist{{Name: "Artist"}}},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.Header().Set("Content-Type", "application/json")
			if nextCalled {
				w.Write(postState)
			} else {
				w.Write(preState)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/next":
			nextCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	output := captureOutput(t, func() {
		if err := nextCmd.RunE(nextCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !nextCalled {
		t.Error("expected next API to be called")
	}
	if !strings.Contains(output, "After") {
		t.Errorf("expected new track name in status output, got: %q", output)
	}
}

func TestPreviousCmd_Success(t *testing.T) {
	oldConfigPath := configPath
	oldPreviousDeviceID := previousDeviceID
	defer func() {
		configPath = oldConfigPath
		previousDeviceID = oldPreviousDeviceID
	}()

	previousDeviceID = "dev-1"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh", ConfirmStabilizeWindow: "5ms"})

	previousCalled := false
	preState, _ := json.Marshal(spotify.PlaybackState{
		IsPlaying: true,
		Device:    spotify.Device{Name: "Dev", Type: "Speaker", IsActive: true},
		Item:      &spotify.Track{URI: "spotify:track:current", Name: "Current", DurationMS: 180000, Artists: []spotify.Artist{{Name: "Artist"}}},
	})
	postState, _ := json.Marshal(spotify.PlaybackState{
		IsPlaying: true,
		Device:    spotify.Device{Name: "Dev", Type: "Speaker", IsActive: true},
		Item:      &spotify.Track{URI: "spotify:track:prior", Name: "Prior", DurationMS: 180000, Artists: []spotify.Artist{{Name: "Artist"}}},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.Header().Set("Content-Type", "application/json")
			if previousCalled {
				w.Write(postState)
			} else {
				w.Write(preState)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/previous":
			previousCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	output := captureOutput(t, func() {
		if err := previousCmd.RunE(previousCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !previousCalled {
		t.Error("expected previous API to be called")
	}
	if !strings.Contains(output, "Prior") {
		t.Errorf("expected prior track name in status output, got: %q", output)
	}
}
