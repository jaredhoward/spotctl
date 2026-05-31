package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaredhoward/spotctl/config"
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

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost || r.URL.Path != "/next" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := nextCmd.RunE(nextCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
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

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost || r.URL.Path != "/previous" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := previousCmd.RunE(previousCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected previous API to be called")
	}
}
