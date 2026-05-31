package spotify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlayWithAndWithoutContext(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		switch called {
		case 1:
			if r.URL.Path != "/play" {
				t.Fatalf("expected /play path, got %s", r.URL.Path)
			}
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload["context_uri"] != "spotify:track:abc" {
				t.Fatalf("expected context_uri set, got %v", payload)
			}
			w.WriteHeader(http.StatusNoContent)
		case 2:
			if r.URL.Path != "/play" {
				t.Fatalf("expected /play path, got %s", r.URL.Path)
			}
			buf := make([]byte, 1)
			n, _ := r.Body.Read(buf)
			if n != 0 {
				t.Fatalf("expected empty body, got %d bytes", n)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatal("unexpected call")
		}
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Play("device", "spotify:track:abc"); err != nil {
		t.Fatal(err)
	}
	if err := client.Play("device", ""); err != nil {
		t.Fatal(err)
	}
}

func TestPlay_WithoutDevice(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("device_id") != "" {
			t.Errorf("expected no device_id, got %q", r.URL.Query().Get("device_id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Play("", ""); err != nil {
		t.Fatal(err)
	}
}

func TestTransferPlayback(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

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
		if !ok || len(deviceIDs) != 1 || deviceIDs[0] != "device" {
			t.Fatalf("unexpected device_ids payload: %v", payload["device_ids"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.TransferPlayback([]string{"device"}, true); err != nil {
		t.Fatal(err)
	}
}

func TestVolumePauseNextPreviousShuffle(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	cases := []struct {
		name    string
		path    string
		method  string
		handler func(*http.Request) bool
		invoke  func(*Client) error
	}{
		{
			name:   "set volume with device",
			path:   "/volume",
			method: http.MethodPut,
			handler: func(r *http.Request) bool {
				return r.URL.Query().Get("volume_percent") == "42" && r.URL.Query().Get("device_id") == "device"
			},
			invoke: func(c *Client) error { return c.Volume("device", 42) },
		},
		{
			name:   "set volume without device",
			path:   "/volume",
			method: http.MethodPut,
			handler: func(r *http.Request) bool {
				return r.URL.Query().Get("volume_percent") == "10" && r.URL.Query().Get("device_id") == ""
			},
			invoke: func(c *Client) error { return c.Volume("", 10) },
		},
		{
			name:    "pause with device",
			path:    "/pause",
			method:  http.MethodPut,
			handler: func(r *http.Request) bool { return r.URL.Query().Get("device_id") == "device" },
			invoke:  func(c *Client) error { return c.Pause("device") },
		},
		{
			name:    "pause without device",
			path:    "/pause",
			method:  http.MethodPut,
			handler: func(r *http.Request) bool { return r.URL.Query().Get("device_id") == "" },
			invoke:  func(c *Client) error { return c.Pause("") },
		},
		{
			name:    "next with device",
			path:    "/next",
			method:  http.MethodPost,
			handler: func(r *http.Request) bool { return r.URL.Query().Get("device_id") == "device" },
			invoke:  func(c *Client) error { return c.Next("device") },
		},
		{
			name:    "next without device",
			path:    "/next",
			method:  http.MethodPost,
			handler: func(r *http.Request) bool { return r.URL.Query().Get("device_id") == "" },
			invoke:  func(c *Client) error { return c.Next("") },
		},
		{
			name:    "previous with device",
			path:    "/previous",
			method:  http.MethodPost,
			handler: func(r *http.Request) bool { return r.URL.Query().Get("device_id") == "device" },
			invoke:  func(c *Client) error { return c.Previous("device") },
		},
		{
			name:    "previous without device",
			path:    "/previous",
			method:  http.MethodPost,
			handler: func(r *http.Request) bool { return r.URL.Query().Get("device_id") == "" },
			invoke:  func(c *Client) error { return c.Previous("") },
		},
		{
			name:   "shuffle enabled with device",
			path:   "/shuffle",
			method: http.MethodPut,
			handler: func(r *http.Request) bool {
				return r.URL.Query().Get("state") == "true" && r.URL.Query().Get("device_id") == "device"
			},
			invoke: func(c *Client) error { return c.Shuffle("device", true) },
		},
		{
			name:   "shuffle disabled without device",
			path:   "/shuffle",
			method: http.MethodPut,
			handler: func(r *http.Request) bool {
				return r.URL.Query().Get("state") == "false" && r.URL.Query().Get("device_id") == ""
			},
			invoke: func(c *Client) error { return c.Shuffle("", false) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Fatalf("expected %s %s, got %s %s", tc.method, tc.path, r.Method, r.URL.Path)
				}
				if !tc.handler(r) {
					t.Fatalf("unexpected request params: %v", r.URL.RawQuery)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			URLPlayer = server.URL
			client := &Client{accessToken: "t", httpClient: server.Client()}
			if err := tc.invoke(client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

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
	state, err := client.GetCurrentPlayback()
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
	state, err := client.GetCurrentPlayback()
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
	state, err := client.GetCurrentPlayback()
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
