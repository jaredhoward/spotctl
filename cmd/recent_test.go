package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

func TestRunRecent_PrintsTracks(t *testing.T) {
	played := time.Date(2026, 8, 4, 3, 10, 0, 0, time.UTC)
	resp := spotify.RecentlyPlayedResponse{
		Items: []spotify.RecentlyPlayedItem{
			{
				Track:    spotify.Track{Name: "Weightless", Artists: []spotify.Artist{{Name: "Marconi Union"}}},
				PlayedAt: played,
				Context:  &spotify.PlaybackContext{URI: "spotify:playlist:2SNUuYsP7S4K3V4xANjA46", Type: "playlist"},
			},
		},
	}

	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	oldLimit := recentLimit
	t.Cleanup(func() { recentLimit = oldLimit })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	wireClient(t, srv)
	t.Cleanup(srv.Close)

	output := captureOutput(t, func() {
		if err := recentCmd.RunE(recentCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "Weightless") {
		t.Errorf("expected track name in output, got: %q", output)
	}
	if !strings.Contains(output, "Marconi Union") {
		t.Errorf("expected artist in output, got: %q", output)
	}
	if !strings.Contains(output, "spotify:playlist:2SNUuYsP7S4K3V4xANjA46") {
		t.Errorf("expected context uri in output, got: %q", output)
	}
}

func TestRunRecent_NoTracks(t *testing.T) {
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(spotify.RecentlyPlayedResponse{})
	}))
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	wireClient(t, srv)
	t.Cleanup(srv.Close)

	output := captureOutput(t, func() {
		if err := recentCmd.RunE(recentCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "No recently played tracks found.") {
		t.Errorf("expected empty-state message, got: %q", output)
	}
}

func TestRunRecent_LimitFlag(t *testing.T) {
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	oldLimit := recentLimit
	t.Cleanup(func() { recentLimit = oldLimit })
	recentLimit = 5

	var gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(spotify.RecentlyPlayedResponse{})
	}))
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	wireClient(t, srv)
	t.Cleanup(srv.Close)

	if err := recentCmd.RunE(recentCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != "5" {
		t.Errorf("expected limit=5 sent to Spotify, got %q", gotLimit)
	}
}
