package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
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
	for _, name := range []string{"uri", "playlist", "track", "album"} {
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

func captureOutputAndLogs(t *testing.T, fn func()) (stdout, logs string) {
	t.Helper()
	oldStdout := os.Stdout
	rStdout, wStdout, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = wStdout
	oldLogWriter := log.Writer()
	logBuf := &bytes.Buffer{}
	log.SetOutput(logBuf)
	defer log.SetOutput(oldLogWriter)
	fn()
	wStdout.Close()
	os.Stdout = oldStdout
	stdoutData, err := io.ReadAll(rStdout)
	if err != nil {
		t.Fatal(err)
	}
	return string(stdoutData), logBuf.String()
}

func TestPlayCmdRunE_NoDevice(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldPlayDeviceID := playDeviceID
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		playDeviceID = oldPlayDeviceID
	}()

	playDeviceID = ""
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }
	resetPlayCmdFlags(t)

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// no device_id expected
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

	if err := playCmd.RunE(playCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected play API to be called")
	}
}

func TestPlayCmdRunE_WithDevice(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldPlayDeviceID := playDeviceID
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		playDeviceID = oldPlayDeviceID
	}()

	playDeviceID = "device-1"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }
	resetPlayCmdFlags(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("device_id") != "device-1" {
			t.Errorf("expected device_id=device-1, got %q", r.URL.Query().Get("device_id"))
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
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldTransferDeviceID := transferDeviceID
	oldTransferPlay := transferPlay
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		transferDeviceID = oldTransferDeviceID
		transferPlay = oldTransferPlay
	}()

	transferDeviceID = "device-1"
	transferPlay = true
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

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
	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}

	if err := transferCmd.RunE(transferCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVolumeCmdRunE_InvalidLevel(t *testing.T) {
	oldVolumeLevel := volumeLevel
	defer func() { volumeLevel = oldVolumeLevel }()
	volumeLevel = 101

	err := volumeCmd.RunE(volumeCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "volume must be between 0 and 100") {
		t.Fatalf("expected invalid volume error, got %v", err)
	}
}

func TestVolumeCmdRunE_Success(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldVolumeDeviceID := volumeDeviceID
	oldVolumeLevel := volumeLevel
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		volumeDeviceID = oldVolumeDeviceID
		volumeLevel = oldVolumeLevel
	}()

	volumeDeviceID = "device-1"
	volumeLevel = 42
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

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
	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}

	if err := volumeCmd.RunE(volumeCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
