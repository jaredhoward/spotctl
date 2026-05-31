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

func TestDeviceActivity_Inactive(t *testing.T) {
	if got := deviceActivity(false); got != "" {
		t.Errorf("expected empty string for inactive device, got %q", got)
	}
}

func TestDeviceActivity_Active(t *testing.T) {
	if got := deviceActivity(true); got != "(active)" {
		t.Errorf("expected '(active)', got %q", got)
	}
}

func TestFormatDurationMS(t *testing.T) {
	cases := []struct {
		ms   int
		want string
	}{
		{0, "00:00"},
		{1000, "00:01"},
		{60000, "01:00"},
		{90500, "01:30"},
		{3661000, "61:01"},
	}
	for _, tc := range cases {
		got := formatDurationMS(tc.ms)
		if got != tc.want {
			t.Errorf("formatDurationMS(%d): got %q, want %q", tc.ms, got, tc.want)
		}
	}
}

func TestJoinArtists_Empty(t *testing.T) {
	if got := joinArtists(nil); got != "" {
		t.Errorf("expected empty string for nil artists, got %q", got)
	}
}

func TestRunStatus_NoItem_NoContext(t *testing.T) {
	state := spotify.PlaybackState{
		IsPlaying:   false,
		RepeatState: "off",
		Device: spotify.Device{
			Name:     "Test Device",
			Type:     "Computer",
			IsActive: false,
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

	if !strings.Contains(output, "paused") {
		t.Errorf("expected 'paused' in output, got: %q", output)
	}
	// No item or context — those lines should be absent.
	if strings.Contains(output, "Track:") {
		t.Errorf("did not expect 'Track:' with no item, got: %q", output)
	}
	if strings.Contains(output, "Context:") {
		t.Errorf("did not expect 'Context:' with no context, got: %q", output)
	}
	// Inactive device — deviceActivity should return "".
	if strings.Contains(output, "(active)") {
		t.Errorf("did not expect '(active)' for inactive device, got: %q", output)
	}
}

func TestRunStatus_ItemNoContext(t *testing.T) {
	state := spotify.PlaybackState{
		IsPlaying: true,
		Device:    spotify.Device{Name: "Phone", Type: "Smartphone", IsActive: true},
		Item: &spotify.Track{
			Name:       "Solo Track",
			DurationMS: 240000,
			Artists:    []spotify.Artist{{Name: "Solo Artist"}},
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

	if !strings.Contains(output, "Solo Track") {
		t.Errorf("expected track name in output, got: %q", output)
	}
	if strings.Contains(output, "Context:") {
		t.Errorf("did not expect context line when context is nil, got: %q", output)
	}
}
