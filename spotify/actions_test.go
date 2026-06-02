package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestActions(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	var tests = []struct {
		name    string
		action  Action
		path    string
		method  string
		handler http.HandlerFunc
	}{
		// Next actions
		{
			name: "next with device", action: &Next{DeviceID: "device"},
			path: "/next", method: http.MethodPost,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("device_id"); got != "device" {
					t.Errorf("expected device_id=device, got %q", got)
				}
			},
		},
		{
			name: "next without device", action: &Next{DeviceID: ""},
			path: "/next", method: http.MethodPost,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("device_id"); got != "" {
					t.Errorf("expected device_id=, got %q", got)
				}
			},
		},

		// Pause actions
		{
			name: "pause with device", action: &Pause{DeviceID: "device"},
			path: "/pause", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("device_id"); got != "device" {
					t.Errorf("expected device_id=device, got %q", got)
				}
			},
		},
		{
			name: "pause without device", action: &Pause{DeviceID: ""},
			path: "/pause", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("device_id"); got != "" {
					t.Errorf("expected device_id=, got %q", got)
				}
			},
		},

		// Play actions
		{
			name: "play without device id", action: &Play{},
			path: "/play", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("device_id"); got != "" {
					t.Errorf("expected no device_id, got %q", got)
				}
			},
		},
		{
			name: "play with context", action: &Play{DeviceID: "device", ContextURI: "spotify:track:abc"},
			path: "/play", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]string
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload["context_uri"] != "spotify:track:abc" {
					t.Fatalf("expected context_uri set, got %v", payload)
				}
			},
		},
		{
			name: "play without context", action: &Play{DeviceID: "device", ContextURI: ""},
			path: "/play", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				buf := make([]byte, 1)
				n, _ := r.Body.Read(buf)
				if n != 0 {
					t.Fatalf("expected empty body, got %d bytes", n)
				}
			},
		},

		// Previous actions
		{
			name: "previous with device", action: &Previous{DeviceID: "device"},
			path: "/previous", method: http.MethodPost,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("device_id"); got != "device" {
					t.Errorf("expected device_id=device, got %q", got)
				}
			},
		},
		{
			name: "previous without device", action: &Previous{DeviceID: ""},
			path: "/previous", method: http.MethodPost,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("device_id"); got != "" {
					t.Errorf("expected device_id=, got %q", got)
				}
			},
		},

		// Repeat actions
		{
			name: "repeat off with device", action: &Repeat{DeviceID: "dev1", State: "off"},
			path: "/repeat", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("device_id"); got != "dev1" {
					t.Errorf("expected device_id=, got %q", got)
				}
				if got := r.URL.Query().Get("state"); got != "off" {
					t.Errorf("expected state=off, got %q", got)
				}
			},
		},
		{
			name: "repeat track with device", action: &Repeat{DeviceID: "dev1", State: "track"},
			path: "/repeat", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("device_id"); got != "dev1" {
					t.Errorf("expected device_id=dev1, got %q", got)
				}
				if got := r.URL.Query().Get("state"); got != "track" {
					t.Errorf("expected state=track, got %q", got)
				}
			},
		},
		{
			name: "repeat context no device", action: &Repeat{DeviceID: "", State: "context"},
			path: "/repeat", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("device_id"); got != "" {
					t.Errorf("expected device_id=, got %q", got)
				}
				if got := r.URL.Query().Get("state"); got != "context" {
					t.Errorf("expected state=context, got %q", got)
				}
			},
		},

		// Shuffle actions
		{
			name: "shuffle enabled with device", action: &Shuffle{DeviceID: "device", Enabled: true},
			path: "/shuffle", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("state"); got != "true" {
					t.Errorf("expected state=true, got %q", got)
				}
				if got := r.URL.Query().Get("device_id"); got != "device" {
					t.Errorf("expected device_id=device, got %q", got)
				}
			},
		},
		{
			name: "shuffle disabled without device", action: &Shuffle{DeviceID: "", Enabled: false},
			path: "/shuffle", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("state"); got != "false" {
					t.Errorf("expected state=false, got %q", got)
				}
				if got := r.URL.Query().Get("device_id"); got != "" {
					t.Errorf("expected device_id=, got %q", got)
				}
			},
		},

		// Transfer actions
		{
			name: "transfer with play", action: &Transfer{DeviceID: "device", Play: true},
			path: "/", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
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
			},
		},

		// Volume actions
		{
			name: "set volume with device", action: &Volume{DeviceID: "device", Level: 42},
			path: "/volume", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("volume_percent"); got != "42" {
					t.Errorf("expected volume_percent=42, got %q", got)
				}
				if got := r.URL.Query().Get("device_id"); got != "device" {
					t.Errorf("expected device_id=device, got %q", got)
				}
			},
		},
		{
			name: "set volume without device", action: &Volume{DeviceID: "", Level: 10},
			path: "/volume", method: http.MethodPut,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("volume_percent"); got != "10" {
					t.Errorf("expected volume_percent=10, got %q", got)
				}
				if got := r.URL.Query().Get("device_id"); got != "" {
					t.Errorf("expected device_id=, got %q", got)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method || r.URL.Path != tt.path {
					t.Fatalf("expected %s %s, got %s %s", tt.method, tt.path, r.Method, r.URL.Path)
				}
				tt.handler(w, r)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			URLPlayer = server.URL
			client := &Client{accessToken: "t", httpClient: server.Client()}

			if err := tt.action.Dispatch(context.Background(), client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestActions_ErrorResponse(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}

	var tests = []struct {
		name   string
		action Action
	}{
		{
			name:   "next",
			action: &Next{DeviceID: "device"},
		},
		{
			name:   "pause",
			action: &Pause{DeviceID: "device"},
		},
		{
			name:   "play",
			action: &Play{DeviceID: "device", ContextURI: "spotify:track:abc"},
		},
		{
			name:   "previous",
			action: &Previous{DeviceID: "device"},
		},
		{
			name:   "shuffle",
			action: &Shuffle{DeviceID: "device", Enabled: true},
		},
		{
			name:   "transfer",
			action: &Transfer{DeviceID: "device", Play: false},
		},
		{
			name:   "volume",
			action: &Volume{DeviceID: "device", Level: 50},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.action.Dispatch(context.Background(), client); err == nil {
				t.Fatal("expected error for non-2xx response")
			}
		})
	}
}

func TestSpotifyActionLabelsAndConfirmed(t *testing.T) {
	cases := []struct {
		name          string
		action        Action
		state         *PlaybackState
		wantLabel     string
		wantConfirmed bool
	}{
		{
			name:          "play with uri and matching context",
			action:        &Play{DeviceID: "device", ContextURI: "spotify:track:abc"},
			state:         &PlaybackState{IsPlaying: true, Context: &PlaybackContext{URI: "spotify:track:abc"}},
			wantLabel:     "play uri=spotify:track:abc device=device",
			wantConfirmed: true,
		},
		{
			name:          "play with uri and mismatched context",
			action:        &Play{DeviceID: "device", ContextURI: "spotify:track:abc"},
			state:         &PlaybackState{IsPlaying: true, Context: &PlaybackContext{URI: "spotify:playlist:xyz"}},
			wantLabel:     "play uri=spotify:track:abc device=device",
			wantConfirmed: false,
		},
		{
			name:          "play without uri",
			action:        &Play{DeviceID: "device"},
			state:         &PlaybackState{IsPlaying: false},
			wantLabel:     "play device=device",
			wantConfirmed: false,
		},
		{
			name:          "pause",
			action:        &Pause{DeviceID: "device"},
			state:         &PlaybackState{IsPlaying: false},
			wantLabel:     "pause device=device",
			wantConfirmed: true,
		},
		{
			name:          "next",
			action:        &Next{DeviceID: "device"},
			state:         nil,
			wantLabel:     "next device=device",
			wantConfirmed: false,
		},
		{
			name:          "previous",
			action:        &Previous{DeviceID: "device"},
			state:         nil,
			wantLabel:     "previous device=device",
			wantConfirmed: false,
		},
		{
			name:          "shuffle",
			action:        &Shuffle{DeviceID: "device", Enabled: true},
			state:         &PlaybackState{ShuffleState: true},
			wantLabel:     "shuffle enabled=true device=device",
			wantConfirmed: true,
		},
		{
			name:          "repeat",
			action:        &Repeat{DeviceID: "device", State: "track"},
			state:         &PlaybackState{RepeatState: "track"},
			wantLabel:     "repeat state=track device=device",
			wantConfirmed: true,
		},
		{
			name:          "volume",
			action:        &Volume{DeviceID: "device", Level: 42},
			state:         &PlaybackState{Device: Device{VolumePercent: 41}},
			wantLabel:     "volume level=42 device=device",
			wantConfirmed: true,
		},
		{
			name:          "transfer",
			action:        &Transfer{DeviceID: "device", Play: false},
			state:         &PlaybackState{Device: Device{ID: "device"}},
			wantLabel:     "transfer device=device play=false",
			wantConfirmed: true,
		},
		{
			name:          "transfer with play",
			action:        &Transfer{DeviceID: "device", Play: true},
			state:         &PlaybackState{Device: Device{ID: "device"}, IsPlaying: true},
			wantLabel:     "transfer device=device play=true",
			wantConfirmed: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.action.Label(); got != tt.wantLabel {
				t.Fatalf("expected label %q, got %q", tt.wantLabel, got)
			}
			if got := tt.action.Confirmed(tt.state); got != tt.wantConfirmed {
				t.Fatalf("expected Confirmed=%v, got %v", tt.wantConfirmed, got)
			}
		})
	}
}
