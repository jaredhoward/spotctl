package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

func TestPauseCmd_NoDevice(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldPauseDeviceID := pauseDeviceID
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		pauseDeviceID = oldPauseDeviceID
	}()

	pauseDeviceID = ""
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// device_id should be absent from the URL
		if r.URL.Query().Get("device_id") != "" {
			t.Errorf("expected no device_id query param, got %q", r.URL.Query().Get("device_id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}

	if err := pauseCmd.RunE(pauseCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected pause API to be called")
	}
}

func TestPauseCmd_WithDevice(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldPauseDeviceID := pauseDeviceID
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		pauseDeviceID = oldPauseDeviceID
	}()

	pauseDeviceID = "dev-1"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("device_id") != "dev-1" {
			t.Errorf("expected device_id=dev-1, got %q", r.URL.Query().Get("device_id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}

	if err := pauseCmd.RunE(pauseCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNextCmd_NoDevice(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldNextDeviceID := nextDeviceID
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		nextDeviceID = oldNextDeviceID
	}()

	nextDeviceID = ""
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("device_id") != "" {
			t.Errorf("expected no device_id, got %q", r.URL.Query().Get("device_id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}

	if err := nextCmd.RunE(nextCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreviousCmd_WithDevice(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldPreviousDeviceID := previousDeviceID
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		previousDeviceID = oldPreviousDeviceID
	}()

	previousDeviceID = "dev-2"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("device_id") != "dev-2" {
			t.Errorf("expected device_id=dev-2, got %q", r.URL.Query().Get("device_id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}

	if err := previousCmd.RunE(previousCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
