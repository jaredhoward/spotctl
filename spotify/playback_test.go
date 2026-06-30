package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetCurrentPlaybackSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/" {
			t.Fatalf("expected GET /, got %s %s", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PlaybackState{
			Device:       Device{ID: "device-1", Name: "Living Room", Type: "Speaker", IsActive: true, VolumePercent: 50},
			IsPlaying:    true,
			ShuffleState: false,
			RepeatState:  "off",
			ProgressMS:   12345,
			Item:         &Track{Name: "Test Track", DurationMS: 300000, Artists: []Artist{{Name: "Artist 1"}}},
			Context:      &PlaybackContext{URI: "spotify:playlist:test", Type: "playlist"},
		})
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	playback, err := client.GetCurrentPlayback(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if playback == nil || playback.Device.ID != "device-1" {
		t.Fatalf("unexpected playback state: %#v", playback)
	}
}

func TestGetCurrentPlaybackNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	playback, err := client.GetCurrentPlayback(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if playback != nil {
		t.Fatalf("expected nil playback for 204 response, got %#v", playback)
	}
}

func TestGetCurrentPlaybackErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	if _, err := client.GetCurrentPlayback(context.Background()); err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestGetCurrentPlaybackDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	if _, err := client.GetCurrentPlayback(context.Background()); err == nil {
		t.Fatal("expected decode error")
	}
}
