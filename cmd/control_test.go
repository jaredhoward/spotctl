package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPut || r.URL.Path != "/pause" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("device_id") != "dev-1" {
			t.Errorf("expected device_id=dev-1, got %q", r.URL.Query().Get("device_id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := pauseCmd.RunE(pauseCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected pause API to be called")
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
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	actionCalled := false
	snapshotState, _ := json.Marshal(spotify.PlaybackState{Item: &spotify.Track{URI: "spotify:track:prior"}})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			// Snapshot call from Next.Dispatch.
			w.Header().Set("Content-Type", "application/json")
			w.Write(snapshotState)
		case r.Method == http.MethodPost && r.URL.Path == "/next":
			actionCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := nextCmd.RunE(nextCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !actionCalled {
		t.Error("expected next API to be called")
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
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	actionCalled := false
	snapshotState, _ := json.Marshal(spotify.PlaybackState{Item: &spotify.Track{URI: "spotify:track:prior"}})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			// Snapshot call from Previous.Dispatch.
			w.Header().Set("Content-Type", "application/json")
			w.Write(snapshotState)
		case r.Method == http.MethodPost && r.URL.Path == "/previous":
			actionCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := previousCmd.RunE(previousCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !actionCalled {
		t.Error("expected previous API to be called")
	}
}
