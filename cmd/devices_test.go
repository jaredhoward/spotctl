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

func TestDevicesCommandUpdateSavesNewName(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		if f := devicesCmd.Flags().Lookup("update"); f != nil {
			f.Changed = false
			f.Value.Set("false")
		}
	}()

	cfgFile := writeTempConfig(t, &config.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		DeviceNames:  map[string]string{},
	})
	configPath = cfgFile
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(spotify.DevicesResponse{Devices: []spotify.Device{
			{ID: "device-1", Name: "Living Room", Type: "Speaker"},
		}})
	}))
	defer server.Close()

	spotify.URLPlayer = server.URL
	newSpotifyClient = func(accessToken string) *spotify.Client {
		c := spotify.NewClient(accessToken)
		c.SetHTTPClient(server.Client())
		return c
	}

	if err := devicesCmd.Flags().Set("update", "true"); err != nil {
		t.Fatal(err)
	}

	output := captureOutput(t, func() {
		if err := devicesCmd.RunE(devicesCmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(output, "Updated config with discovered device names") {
		t.Errorf("expected config-saved message, got: %q", output)
	}

	saved, err := config.Load(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if saved.DeviceNames["device-1"] != "Living Room" {
		t.Errorf("expected saved device name, got: %v", saved.DeviceNames)
	}
}

func TestDevicesCommandUpdateNoChange(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		if f := devicesCmd.Flags().Lookup("update"); f != nil {
			f.Changed = false
			f.Value.Set("false")
		}
	}()

	configPath = writeTempConfig(t, &config.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		DeviceNames:  map[string]string{"device-1": "Living Room"},
	})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(spotify.DevicesResponse{Devices: []spotify.Device{
			{ID: "device-1", Name: "Living Room", Type: "Speaker"},
		}})
	}))
	defer server.Close()

	spotify.URLPlayer = server.URL
	newSpotifyClient = func(accessToken string) *spotify.Client {
		c := spotify.NewClient(accessToken)
		c.SetHTTPClient(server.Client())
		return c
	}

	if err := devicesCmd.Flags().Set("update", "true"); err != nil {
		t.Fatal(err)
	}

	output := captureOutput(t, func() {
		if err := devicesCmd.RunE(devicesCmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	if strings.Contains(output, "Updated config") {
		t.Errorf("did not expect config-saved message when nothing changed, got: %q", output)
	}
}

func TestDevicesCommandConfigLoadError(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	configPath = "/nonexistent/config.yaml"
	if err := devicesCmd.RunE(devicesCmd, nil); err == nil {
		t.Fatal("expected error for missing config")
	}
}
