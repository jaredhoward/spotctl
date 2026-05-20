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

func writeTempConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()
	file, err := os.CreateTemp("", "spotctl-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	t.Cleanup(func() { os.Remove(file.Name()) })

	if err := config.Save(file.Name(), cfg); err != nil {
		t.Fatal(err)
	}

	return file.Name()
}

func resetPlayCmdFlags(t *testing.T) {
	t.Helper()
	for _, name := range []string{"device", "uri", "playlist", "track", "album", "shuffle"} {
		if flag := playCmd.Flags().Lookup(name); flag != nil {
			flag.Changed = false
		}
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

func TestPlayCommandRunERequiresDevice(t *testing.T) {
	oldConfigPath := configPath
	oldDeviceID := deviceID
	oldPreset := preset
	oldURI := uri
	oldPlaylistID := playlistID
	oldTrackID := trackID
	oldAlbumID := albumID
	oldShuffle := shuffle
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	defer func() {
		configPath = oldConfigPath
		deviceID = oldDeviceID
		preset = oldPreset
		uri = oldURI
		playlistID = oldPlaylistID
		trackID = oldTrackID
		albumID = oldAlbumID
		shuffle = oldShuffle
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
	}()

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	deviceID = ""
	preset = ""
	uri = ""
	playlistID = ""
	trackID = ""
	albumID = ""
	shuffle = false
	resetPlayCmdFlags(t)

	spotify.RefreshAccessToken = func(clientB64, refreshToken string) (string, error) {
		return "token", nil
	}
	newSpotifyClient = func(accessToken string) *spotify.Client {
		t.Fatalf("newSpotifyClient should not be called")
		return nil
	}

	err := playCmd.RunE(playCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "device ID is required") {
		t.Fatalf("expected missing device error, got %v", err)
	}
}

func TestPlayCommandRunESuccessResumesWithoutContext(t *testing.T) {
	oldConfigPath := configPath
	oldDeviceID := deviceID
	oldPreset := preset
	oldURI := uri
	oldPlaylistID := playlistID
	oldTrackID := trackID
	oldAlbumID := albumID
	oldShuffle := shuffle
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldAPIBase := spotify.APIBase
	defer func() {
		configPath = oldConfigPath
		deviceID = oldDeviceID
		preset = oldPreset
		uri = oldURI
		playlistID = oldPlaylistID
		trackID = oldTrackID
		albumID = oldAlbumID
		shuffle = oldShuffle
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.APIBase = oldAPIBase
	}()

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	deviceID = "device-1"
	preset = ""
	uri = ""
	playlistID = ""
	trackID = ""
	albumID = ""
	shuffle = false
	resetPlayCmdFlags(t)

	spotify.RefreshAccessToken = func(clientB64, refreshToken string) (string, error) {
		return "token", nil
	}

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPut || r.URL.Path != "/play" {
			t.Fatalf("expected PUT /play, got %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("device_id") != "device-1" {
			t.Fatalf("expected device_id device-1, got %q", r.URL.Query().Get("device_id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	spotify.APIBase = server.URL
	newSpotifyClient = func(accessToken string) *spotify.Client {
		c := spotify.NewClient(accessToken)
		c.SetHTTPClient(server.Client())
		return c
	}

	if err := playCmd.RunE(playCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected play API to be called")
	}
}

func TestTransferCommandRunERequiresDevice(t *testing.T) {
	oldTransferDeviceID := transferDeviceID
	oldTransferPlay := transferPlay
	defer func() {
		transferDeviceID = oldTransferDeviceID
		transferPlay = oldTransferPlay
	}()

	transferDeviceID = ""
	transferPlay = false

	err := transferCmd.RunE(transferCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "device ID is required") {
		t.Fatalf("expected missing device error, got %v", err)
	}
}

func TestTransferCommandRunESuccess(t *testing.T) {
	oldConfigPath := configPath
	oldTransferDeviceID := transferDeviceID
	oldTransferPlay := transferPlay
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldAPIBase := spotify.APIBase
	defer func() {
		configPath = oldConfigPath
		transferDeviceID = oldTransferDeviceID
		transferPlay = oldTransferPlay
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.APIBase = oldAPIBase
	}()

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	transferDeviceID = "device-1"
	transferPlay = true

	spotify.RefreshAccessToken = func(clientB64, refreshToken string) (string, error) {
		return "token", nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/" {
			t.Fatalf("expected PUT /, got %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["play"] != true {
			t.Fatalf("expected play true, got %#v", payload)
		}
		deviceIDs, ok := payload["device_ids"].([]interface{})
		if !ok || len(deviceIDs) != 1 || deviceIDs[0] != "device-1" {
			t.Fatalf("unexpected device_ids payload: %v", payload["device_ids"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	spotify.APIBase = server.URL
	newSpotifyClient = func(accessToken string) *spotify.Client {
		c := spotify.NewClient(accessToken)
		c.SetHTTPClient(server.Client())
		return c
	}

	output := captureOutput(t, func() {
		if err := transferCmd.RunE(transferCmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(output, "Transferred and started playback to device device-1") {
		t.Fatalf("unexpected output: %q", output)
	}
}

func TestVolumeCommandRunERequiresValidLevel(t *testing.T) {
	oldVolumeDeviceID := volumeDeviceID
	oldVolumeLevel := volumeLevel
	defer func() {
		volumeDeviceID = oldVolumeDeviceID
		volumeLevel = oldVolumeLevel
	}()

	volumeDeviceID = "device-1"
	volumeLevel = 101

	err := volumeCmd.RunE(volumeCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "volume must be between 0 and 100") {
		t.Fatalf("expected invalid volume error, got %v", err)
	}
}

func TestVolumeCommandRunESuccess(t *testing.T) {
	oldConfigPath := configPath
	oldVolumeDeviceID := volumeDeviceID
	oldVolumeLevel := volumeLevel
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldAPIBase := spotify.APIBase
	defer func() {
		configPath = oldConfigPath
		volumeDeviceID = oldVolumeDeviceID
		volumeLevel = oldVolumeLevel
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.APIBase = oldAPIBase
	}()

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	volumeDeviceID = "device-1"
	volumeLevel = 42

	spotify.RefreshAccessToken = func(clientB64, refreshToken string) (string, error) {
		return "token", nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/volume" {
			t.Fatalf("expected PUT /volume, got %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("device_id") != "device-1" || r.URL.Query().Get("volume_percent") != "42" {
			t.Fatalf("unexpected query values: %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	spotify.APIBase = server.URL
	newSpotifyClient = func(accessToken string) *spotify.Client {
		c := spotify.NewClient(accessToken)
		c.SetHTTPClient(server.Client())
		return c
	}

	output := captureOutput(t, func() {
		if err := volumeCmd.RunE(volumeCmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(output, "Set volume to 42% on device device-1") {
		t.Fatalf("unexpected output: %q", output)
	}
}
