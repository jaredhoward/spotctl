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

// statusSetup wires up config, token refresh, and a test HTTP server for
// status tests. Returns a cleanup func — call defer cleanup() in each test.
func statusSetup(t *testing.T, handler http.HandlerFunc) (cleanup func()) {
	t.Helper()

	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer

	srv := httptest.NewServer(handler)

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }
	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}

	return func() {
		srv.Close()
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
	}
}

func TestRunStatus_NoPlayback(t *testing.T) {
	cleanup := statusSetup(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer cleanup()

	output := captureOutput(t, func() {
		if err := statusCmd.RunE(statusCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "No active playback found.") {
		t.Errorf("expected no-playback message, got: %q", output)
	}
}

func TestRunStatus_WithFullPlayback(t *testing.T) {
	state := spotify.PlaybackState{
		IsPlaying:    true,
		ShuffleState: true,
		RepeatState:  "context",
		ProgressMS:   60000,
		Device: spotify.Device{
			Name:          "Bedroom Speaker",
			Type:          "Speaker",
			IsActive:      true,
			VolumePercent: 55,
		},
		Item: &spotify.Track{
			Name:       "Test Track",
			URI:        "spotify:track:abc",
			DurationMS: 180000,
			Artists:    []spotify.Artist{{Name: "Artist One"}, {Name: "Artist Two"}},
		},
		Context: &spotify.PlaybackContext{
			URI:  "spotify:playlist:xyz",
			Type: "playlist",
		},
	}

	cleanup := statusSetup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	})
	defer cleanup()

	output := captureOutput(t, func() {
		if err := statusCmd.RunE(statusCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{
		"Bedroom Speaker",
		"Speaker",
		"(active)",
		"playing",
		"shuffle: true",
		"repeat: context",
		"volume: 55%",
		"Test Track",
		"Artist One, Artist Two",
		"01:00",
		"03:00",
		"spotify:playlist:xyz",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got:\n%s", want, output)
		}
	}
}

func TestRunStatus_Paused_NoItemNoContext(t *testing.T) {
	// Paused state with no track and no context — device-only output.
	state := spotify.PlaybackState{
		IsPlaying: false,
		Device:    spotify.Device{Name: "Kitchen", Type: "Speaker", VolumePercent: 30},
	}

	cleanup := statusSetup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	})
	defer cleanup()

	output := captureOutput(t, func() {
		if err := statusCmd.RunE(statusCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "paused") {
		t.Errorf("expected 'paused' in output, got: %q", output)
	}
	if strings.Contains(output, "Track:") {
		t.Errorf("expected no Track line when item is nil, got: %q", output)
	}
	if strings.Contains(output, "Context:") {
		t.Errorf("expected no Context line when context is nil, got: %q", output)
	}
}

func TestRunStatus_WithItem_NoContext(t *testing.T) {
	// Playing a track with no context (e.g. playing a single track, not a playlist).
	state := spotify.PlaybackState{
		IsPlaying: true,
		Device:    spotify.Device{Name: "Speaker", Type: "Speaker", VolumePercent: 40},
		Item: &spotify.Track{
			Name:       "Solo Track",
			DurationMS: 240000,
			Artists:    []spotify.Artist{{Name: "Solo Artist"}},
		},
	}

	cleanup := statusSetup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	})
	defer cleanup()

	output := captureOutput(t, func() {
		if err := statusCmd.RunE(statusCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "Solo Track") {
		t.Errorf("expected track name in output, got: %q", output)
	}
	if !strings.Contains(output, "Solo Artist") {
		t.Errorf("expected artist name in output, got: %q", output)
	}
	if strings.Contains(output, "Context:") {
		t.Errorf("expected no Context line when context URI is empty, got: %q", output)
	}
}

func TestRunStatus_WithContext_EmptyURI(t *testing.T) {
	// Context present but URI is empty — should not print Context line.
	state := spotify.PlaybackState{
		IsPlaying: true,
		Device:    spotify.Device{Name: "Speaker", Type: "Speaker", VolumePercent: 40},
		Context:   &spotify.PlaybackContext{URI: "", Type: "playlist"},
	}

	cleanup := statusSetup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	})
	defer cleanup()

	output := captureOutput(t, func() {
		if err := statusCmd.RunE(statusCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Contains(output, "Context:") {
		t.Errorf("expected no Context line for empty URI, got: %q", output)
	}
}

func TestRunStatus_APIError(t *testing.T) {
	cleanup := statusSetup(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	err := statusCmd.RunE(statusCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "failed to get current playback") {
		t.Fatalf("expected API error, got %v", err)
	}
}
