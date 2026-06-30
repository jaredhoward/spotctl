package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoExpectSuccess_NonTwoXX_IncludesBody(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request body"))
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/play?device_id=device", URLPlayer), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer t")
	err = client.doExpectSuccess(req, "play")
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
	if !strings.Contains(err.Error(), "bad request body") {
		t.Errorf("expected response body in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected status code in error, got: %v", err)
	}
}

func TestGetCurrentPlayback_ErrorStatus(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	state, err := client.GetCurrentPlayback(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if state != nil {
		t.Error("expected nil state on error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected status in error, got: %v", err)
	}
}

func TestGetCurrentPlayback_NoContent(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	state, err := client.GetCurrentPlayback(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Error("expected nil state for 204 No Content")
	}
}

func TestGetCurrentPlayback_WithState(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	expected := PlaybackState{
		IsPlaying:    true,
		ShuffleState: true,
		RepeatState:  "context",
		ProgressMS:   30000,
		Device:       Device{ID: "dev1", Name: "Speaker", VolumePercent: 50, IsActive: true},
		Item: &Track{
			URI:        "spotify:track:abc",
			Name:       "Song",
			DurationMS: 200000,
			Artists:    []Artist{{Name: "Artist"}},
		},
		Context: &PlaybackContext{URI: "spotify:playlist:xyz", Type: "playlist"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	state, err := client.GetCurrentPlayback(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.Device.ID != "dev1" {
		t.Errorf("device ID: got %q", state.Device.ID)
	}
	if state.Item == nil || state.Item.URI != "spotify:track:abc" {
		t.Errorf("item URI: got %v", state.Item)
	}
	if state.Context == nil || state.Context.URI != "spotify:playlist:xyz" {
		t.Errorf("context URI: got %v", state.Context)
	}
	if !state.IsPlaying {
		t.Error("expected IsPlaying=true")
	}
}

func TestPlayerURL_WithAndWithoutDevice(t *testing.T) {
	cases := []struct {
		base     string
		path     string
		deviceID string
		want     string
	}{
		{"http://base", "/play", "dev1", "http://base/play?device_id=dev1"},
		{"http://base", "/play", "", "http://base/play"},
		{"http://base", "", "dev1", "http://base?device_id=dev1"},
		{"http://base", "", "", "http://base"},
	}
	for _, tc := range cases {
		got := playerURL(tc.base, tc.path, tc.deviceID)
		if got != tc.want {
			t.Errorf("playerURL(%q, %q, %q) = %q, want %q", tc.base, tc.path, tc.deviceID, got, tc.want)
		}
	}
}
