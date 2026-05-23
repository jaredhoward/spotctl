package cmd

import (
	"path/filepath"
	"testing"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

func TestWithPlayerClientUsesConfigAndTokenRefresh(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldDeviceID := deviceID
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		deviceID = oldDeviceID
	}()

	tmpDir := t.TempDir()
	configPath = filepath.Join(tmpDir, "config.yaml")
	cfg := &config.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		RedirectURI:  "https://example.com/callback",
	}
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	spotify.RefreshAccessToken = func(clientB64, refreshToken string) (string, error) {
		if clientB64 == cfg.ClientB64() && refreshToken == cfg.RefreshToken {
			return "access-token", nil
		}
		return "", nil
	}

	newSpotifyClient = func(accessToken string) *spotify.Client {
		if accessToken != "access-token" {
			t.Fatalf("unexpected access token %q", accessToken)
		}
		return &spotify.Client{}
	}

	deviceID = "device-1"
	err := withPlayerClient(func(c *spotify.Client, id string) error {
		if id != deviceID {
			t.Fatalf("expected device id %q, got %q", deviceID, id)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWithPlayerClientMissingDevice(t *testing.T) {
	oldDeviceID := deviceID
	deviceID = ""
	defer func() { deviceID = oldDeviceID }()

	if err := withPlayerClient(func(c *spotify.Client, id string) error { return nil }); err == nil {
		t.Fatal("expected error when device id is missing")
	}
}

func TestExecutePlaybackActionPrintsSuccess(t *testing.T) {
	oldConfigPath := configPath
	oldDeviceID := deviceID
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	defer func() {
		configPath = oldConfigPath
		deviceID = oldDeviceID
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
	}()

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	deviceID = "device-1"
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }
	newSpotifyClient = func(_ string) *spotify.Client { return &spotify.Client{} }

	output := captureOutput(t, func() {
		if err := executePlaybackAction("pause", func(c *spotify.Client, id string) error {
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	if output != "Pause on device device-1\n" {
		t.Fatalf("unexpected output: %q", output)
	}
}
