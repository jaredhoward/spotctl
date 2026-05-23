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

func devicesTestSetup(t *testing.T, cfg *config.Config, spotifyDevices []spotify.Device) (cleanup func()) {
	t.Helper()

	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer

	configPath = writeTempConfig(t, cfg)
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(spotify.DevicesResponse{Devices: spotifyDevices})
	}))
	t.Cleanup(server.Close)

	spotify.URLPlayer = server.URL
	newSpotifyClient = func(accessToken string) *spotify.Client {
		c := spotify.NewClient(accessToken)
		c.SetHTTPClient(server.Client())
		return c
	}

	return func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		if f := devicesCmd.Flags().Lookup("update"); f != nil {
			f.Changed = false
			f.Value.Set("false")
		}
	}
}

func TestDevicesShowsLiveDevices(t *testing.T) {
	cleanup := devicesTestSetup(t, &config.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		DeviceNames:  map[string]string{},
	}, []spotify.Device{
		{ID: "device-1", Name: "Living Room", Type: "Speaker"},
	})
	defer cleanup()

	output := captureOutput(t, func() {
		if err := devicesCmd.RunE(devicesCmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(output, "device-1") {
		t.Errorf("expected live device in output, got: %q", output)
	}
	if !strings.Contains(output, "Living Room") {
		t.Errorf("expected device name in output, got: %q", output)
	}
}

func TestDevicesShowsOfflineConfigDevices(t *testing.T) {
	// Spotify returns nothing; config knows about two devices.
	cleanup := devicesTestSetup(t, &config.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		DeviceNames: map[string]string{
			"device-offline-1": "Bedroom Speaker",
			"device-offline-2": "Kitchen Display",
		},
	}, []spotify.Device{})
	defer cleanup()

	output := captureOutput(t, func() {
		if err := devicesCmd.RunE(devicesCmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(output, "device-offline-1") {
		t.Errorf("expected offline device-1 in output, got: %q", output)
	}
	if !strings.Contains(output, "Bedroom Speaker") {
		t.Errorf("expected offline device-1 name in output, got: %q", output)
	}
	if !strings.Contains(output, "device-offline-2") {
		t.Errorf("expected offline device-2 in output, got: %q", output)
	}
	if !strings.Contains(output, "Kitchen Display") {
		t.Errorf("expected offline device-2 name in output, got: %q", output)
	}
	if !strings.Contains(output, "(offline)") {
		t.Errorf("expected offline marker in output, got: %q", output)
	}
}

func TestDevicesMergesLiveAndOffline(t *testing.T) {
	// Spotify returns one device; config also knows about a second that's offline.
	cleanup := devicesTestSetup(t, &config.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		DeviceNames: map[string]string{
			"device-1":       "Living Room",
			"device-offline": "Bedroom Speaker",
		},
	}, []spotify.Device{
		{ID: "device-1", Name: "Living Room", Type: "Speaker"},
	})
	defer cleanup()

	output := captureOutput(t, func() {
		if err := devicesCmd.RunE(devicesCmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(output, "device-1") {
		t.Errorf("expected live device in output, got: %q", output)
	}
	if !strings.Contains(output, "device-offline") {
		t.Errorf("expected offline device in output, got: %q", output)
	}
	if !strings.Contains(output, "(offline)") {
		t.Errorf("expected offline marker in output, got: %q", output)
	}
	// The live device should NOT be marked offline.
	if strings.Count(output, "(offline)") != 1 {
		t.Errorf("expected exactly one offline marker, got: %q", output)
	}
}

func TestDevicesSortedByName(t *testing.T) {
	// Mix of live and offline devices with names that would sort differently
	// than their insertion order. Verifies the output is name-ordered.
	cleanup := devicesTestSetup(t, &config.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		DeviceNames: map[string]string{
			"device-z": "Zebra Room",   // offline, name sorts last
			"device-a": "Apple TV",     // offline, name sorts first
		},
	}, []spotify.Device{
		{ID: "device-m", Name: "Master Bedroom", Type: "Speaker", IsActive: true},
		{ID: "device-k", Name: "Kitchen", Type: "Speaker"},
	})
	defer cleanup()

	output := captureOutput(t, func() {
		if err := devicesCmd.RunE(devicesCmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	positions := map[string]int{
		"Apple TV":       strings.Index(output, "Apple TV"),
		"Kitchen":        strings.Index(output, "Kitchen"),
		"Master Bedroom": strings.Index(output, "Master Bedroom"),
		"Zebra Room":     strings.Index(output, "Zebra Room"),
	}

	for name, pos := range positions {
		if pos == -1 {
			t.Errorf("expected %q in output, got: %q", name, output)
		}
	}

	// Expected order: Apple TV < Kitchen < Master Bedroom < Zebra Room
	if !(positions["Apple TV"] < positions["Kitchen"] &&
		positions["Kitchen"] < positions["Master Bedroom"] &&
		positions["Master Bedroom"] < positions["Zebra Room"]) {
		t.Errorf("devices not sorted by name; positions: %v\noutput:\n%s", positions, output)
	}
}

func TestDevicesCommandUpdateSavesNewName(t *testing.T) {
	cleanup := devicesTestSetup(t, &config.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		DeviceNames:  map[string]string{},
	}, []spotify.Device{
		{ID: "device-1", Name: "Living Room", Type: "Speaker"},
	})
	defer cleanup()

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

	saved, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.DeviceNames["device-1"] != "Living Room" {
		t.Errorf("expected saved device name, got: %v", saved.DeviceNames)
	}
}

func TestDevicesCommandUpdateNoChange(t *testing.T) {
	cleanup := devicesTestSetup(t, &config.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		DeviceNames:  map[string]string{"device-1": "Living Room"},
	}, []spotify.Device{
		{ID: "device-1", Name: "Living Room", Type: "Speaker"},
	})
	defer cleanup()

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

func TestDevicesEmptyEverywhere(t *testing.T) {
	// No live devices, no config entries — should print the empty message, not error.
	cleanup := devicesTestSetup(t, &config.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		DeviceNames:  map[string]string{},
	}, []spotify.Device{})
	defer cleanup()

	output := captureOutput(t, func() {
		if err := devicesCmd.RunE(devicesCmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(output, "No Spotify Connect devices found") {
		t.Errorf("expected empty-state message, got: %q", output)
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
