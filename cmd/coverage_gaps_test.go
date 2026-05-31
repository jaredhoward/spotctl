package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// ----- status: inactive device branch ----------------------------------------

func TestRunStatus_InactiveDevice(t *testing.T) {
	state := spotify.PlaybackState{
		IsPlaying: false,
		Device: spotify.Device{
			Name:     "Kitchen Speaker",
			Type:     "Speaker",
			IsActive: false,
		},
		Item: &spotify.Track{
			Name:    "Some Track",
			Artists: []spotify.Artist{{Name: "Some Artist"}},
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

	// deviceActivity(false) returns "" — so "(active)" must NOT appear
	if strings.Contains(output, "(active)") {
		t.Errorf("did not expect '(active)' for inactive device, got:\n%s", output)
	}
	if !strings.Contains(output, "Kitchen Speaker") {
		t.Errorf("expected device name in output, got:\n%s", output)
	}
}

// ----- play command: error path from resolveURI ------------------------------

func TestPlayCmdRunE_MultipleURIFlags_Error(t *testing.T) {
	oldConfigPath := configPath
	oldURI := uri
	oldTrackID := trackID
	defer func() {
		configPath = oldConfigPath
		uri = oldURI
		trackID = oldTrackID
		resetPlayCmdFlags(t)
	}()

	// Simulate --uri and --track both being set
	if err := playCmd.Flags().Set("uri", "spotify:track:aaa"); err != nil {
		t.Fatal(err)
	}
	if err := playCmd.Flags().Set("track", "bbb"); err != nil {
		t.Fatal(err)
	}
	uri = "spotify:track:aaa"
	trackID = "bbb"

	err := playCmd.RunE(playCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "only one of") {
		t.Fatalf("expected multiple-URI error, got %v", err)
	}
}

// ----- play command: album and track flags -----------------------------------

func TestPlayCmdRunE_WithAlbum(t *testing.T) {
	oldConfigPath := configPath
	oldAlbumID := albumID
	defer func() {
		configPath = oldConfigPath
		albumID = oldAlbumID
		resetPlayCmdFlags(t)
	}()

	if err := playCmd.Flags().Set("album", "al123"); err != nil {
		t.Fatal(err)
	}
	albumID = "al123"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := readBody(r)
		gotBody = b
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := playCmd.RunE(playCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:album:al123") {
		t.Errorf("expected album URI in request body, got: %q", gotBody)
	}
}

func TestPlayCmdRunE_WithTrack(t *testing.T) {
	oldConfigPath := configPath
	oldTrackID := trackID
	defer func() {
		configPath = oldConfigPath
		trackID = oldTrackID
		resetPlayCmdFlags(t)
	}()

	if err := playCmd.Flags().Set("track", "tr456"); err != nil {
		t.Fatal(err)
	}
	trackID = "tr456"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := readBody(r)
		gotBody = b
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := playCmd.RunE(playCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:track:tr456") {
		t.Errorf("expected track URI in request body, got: %q", gotBody)
	}
}

// ----- transfer command: API error -------------------------------------------

func TestTransferCmdRunE_APIError(t *testing.T) {
	oldConfigPath := configPath
	oldTransferDeviceID := transferDeviceID
	defer func() {
		configPath = oldConfigPath
		transferDeviceID = oldTransferDeviceID
	}()

	transferDeviceID = "device-1"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	err := transferCmd.RunE(transferCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "transfer failed") {
		t.Fatalf("expected transfer failure error, got %v", err)
	}
}

// ----- volume command: API error ---------------------------------------------

func TestVolumeCmdRunE_APIError(t *testing.T) {
	oldConfigPath := configPath
	oldVolumeDeviceID := volumeDeviceID
	defer func() {
		configPath = oldConfigPath
		volumeDeviceID = oldVolumeDeviceID
		resetVolumeCmdFlags(t)
	}()

	volumeDeviceID = "device-1"
	if err := volumeCmd.Flags().Set("level", "50"); err != nil {
		t.Fatal(err)
	}
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	err := volumeCmd.RunE(volumeCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "volume failed") {
		t.Fatalf("expected volume failure error, got %v", err)
	}
}

// ----- devices command: offline devices and --update path --------------------

func TestDevicesShowsOfflineDevices(t *testing.T) {
	oldConfigPath := configPath
	defer func() {
		configPath = oldConfigPath
		if f := devicesCmd.Flags().Lookup("update"); f != nil {
			f.Changed = false
			f.Value.Set("false")
		}
	}()

	// Server returns no live devices
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"devices":[]}`))
	}))
	defer srv.Close()

	configPath = writeTempConfig(t, &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
		DeviceNames: map[string]string{"offline-id": "Offline Speaker"},
	})
	cleanup := wireClient(t, srv)
	defer cleanup()

	output := captureOutput(t, func() {
		if err := devicesCmd.RunE(devicesCmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(output, "offline-id") {
		t.Errorf("expected offline device ID in output, got: %q", output)
	}
	if !strings.Contains(output, "(offline)") {
		t.Errorf("expected '(offline)' label in output, got: %q", output)
	}
}

func TestDevicesNoDevicesAtAll(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"devices":[]}`))
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

	if !strings.Contains(output, "No Spotify Connect devices found") {
		t.Errorf("expected no-devices message, got: %q", output)
	}
}

func TestDevicesUpdate_SavesDeviceNames(t *testing.T) {
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
		w.Write([]byte(`{"devices":[{"id":"live-id","name":"Living Room","type":"Speaker","volume_percent":50}]}`))
	}))
	defer srv.Close()

	configPath = writeTempConfig(t, &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
		DeviceNames: map[string]string{},
	})
	cleanup := wireClient(t, srv)
	defer cleanup()

	if err := devicesCmd.Flags().Set("update", "true"); err != nil {
		t.Fatal(err)
	}

	output := captureOutput(t, func() {
		if err := devicesCmd.RunE(devicesCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "Updated config") {
		t.Errorf("expected 'Updated config' in output, got: %q", output)
	}

	// Verify it actually saved
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DeviceNames["live-id"] != "Living Room" {
		t.Errorf("expected device name to be saved, got: %v", loaded.DeviceNames)
	}
}

func TestDevicesActiveDevice(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"devices":[{"id":"act-id","name":"Active One","type":"Computer","is_active":true,"volume_percent":70}]}`))
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

	if !strings.Contains(output, " *") {
		t.Errorf("expected active device marker '*' in output, got: %q", output)
	}
}

// ----- control commands: API error paths -------------------------------------

func TestPauseCmdRunE_APIError(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	cleanup := wireClient(t, srv)
	defer cleanup()

	err := pauseCmd.RunE(pauseCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "pause failed") {
		t.Fatalf("expected pause failure error, got %v", err)
	}
}

func TestNextCmdRunE_APIError(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	cleanup := wireClient(t, srv)
	defer cleanup()

	err := nextCmd.RunE(nextCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "next failed") {
		t.Fatalf("expected next failure error, got %v", err)
	}
}

func TestPreviousCmdRunE_APIError(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	cleanup := wireClient(t, srv)
	defer cleanup()

	err := previousCmd.RunE(previousCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "previous failed") {
		t.Fatalf("expected previous failure error, got %v", err)
	}
}

func TestShuffleCmdRunE_APIError(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	cleanup := wireClient(t, srv)
	defer cleanup()

	err := shuffleCmd.RunE(shuffleCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "shuffle failed") {
		t.Fatalf("expected shuffle failure error, got %v", err)
	}
}

func TestRepeatCmdRunE_APIError(t *testing.T) {
	oldRepeatState := repeatState
	oldConfigPath := configPath
	defer func() {
		repeatState = oldRepeatState
		configPath = oldConfigPath
	}()

	repeatState = "off"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	err := repeatCmd.RunE(repeatCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "repeat failed") {
		t.Fatalf("expected repeat failure error, got %v", err)
	}
}

// readBody reads r.Body and returns it as a string.
func readBody(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	return buf.String(), err
}
