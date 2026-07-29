package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// multiHandler routes requests to per-method+path handlers, failing the test
// on any unregistered route. Used for actions that make a snapshot GET before
// their primary call.
func multiHandler(t *testing.T, routes map[string]http.HandlerFunc) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if fn, ok := routes[key]; ok {
			fn(w, r)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func TestActions(t *testing.T) {
	snapshotState, _ := json.Marshal(PlaybackState{Item: &Track{URI: "spotify:track:prior"}})
	snapshotHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(snapshotState)
	}
	// devicesHandler stands in for the diagnostic "GET /devices" call
	// Play.Dispatch makes before waking a target device.
	devicesHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"devices":[]}`))
	}

	var tests = []struct {
		name   string
		action Action
		routes map[string]http.HandlerFunc
	}{
		// Next
		{
			name:   "next with device",
			action: &Next{DeviceID: "device"},
			routes: map[string]http.HandlerFunc{
				"GET /": snapshotHandler,
				"POST /next": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("device_id"); got != "device" {
						t.Errorf("expected device_id=device, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},
		{
			name:   "next without device",
			action: &Next{DeviceID: ""},
			routes: map[string]http.HandlerFunc{
				"GET /": snapshotHandler,
				"POST /next": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("device_id"); got != "" {
						t.Errorf("expected no device_id, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},

		// Previous
		{
			name:   "previous with device",
			action: &Previous{DeviceID: "device"},
			routes: map[string]http.HandlerFunc{
				"GET /": snapshotHandler,
				"POST /previous": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("device_id"); got != "device" {
						t.Errorf("expected device_id=device, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},
		{
			name:   "previous without device",
			action: &Previous{DeviceID: ""},
			routes: map[string]http.HandlerFunc{
				"GET /": snapshotHandler,
				"POST /previous": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("device_id"); got != "" {
						t.Errorf("expected no device_id, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},

		// Pause
		{
			name:   "pause with device",
			action: &Pause{DeviceID: "device"},
			routes: map[string]http.HandlerFunc{
				"PUT /pause": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("device_id"); got != "device" {
						t.Errorf("expected device_id=device, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},
		{
			name:   "pause without device",
			action: &Pause{DeviceID: ""},
			routes: map[string]http.HandlerFunc{
				"PUT /pause": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("device_id"); got != "" {
						t.Errorf("expected no device_id, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},

		// Play
		{
			name:   "play without device id",
			action: &Play{},
			routes: map[string]http.HandlerFunc{
				// Play with no ContextURI snapshots current state first.
				"GET /": snapshotHandler,
				"PUT /play": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("device_id"); got != "" {
						t.Errorf("expected no device_id, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},
		{
			name:   "play with context",
			action: &Play{DeviceID: "device", ContextURI: "spotify:track:abc"},
			routes: map[string]http.HandlerFunc{
				// ContextURI path does not snapshot, but a DeviceID still
				// triggers the devices-lookup + wake-transfer + poll steps
				// before /play.
				"GET /devices": devicesHandler,
				"GET /":        func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
				"PUT /":        func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
				"PUT /play": func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]string
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Fatal(err)
					}
					if payload["context_uri"] != "spotify:track:abc" {
						t.Fatalf("expected context_uri set, got %v", payload)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},
		{
			name:   "play with device no context",
			action: &Play{DeviceID: "device"},
			routes: map[string]http.HandlerFunc{
				// DeviceID but no ContextURI — snapshots current state, then
				// still runs the devices-lookup + wake-transfer + poll steps
				// before /play.
				"GET /devices": devicesHandler,
				"GET /":        snapshotHandler,
				"PUT /":        func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
				"PUT /play": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("device_id"); got != "device" {
						t.Errorf("expected device_id=device, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},

		// Repeat
		{
			name:   "repeat off with device",
			action: &Repeat{DeviceID: "dev1", State: "off"},
			routes: map[string]http.HandlerFunc{
				"PUT /repeat": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("device_id"); got != "dev1" {
						t.Errorf("expected device_id=dev1, got %q", got)
					}
					if got := r.URL.Query().Get("state"); got != "off" {
						t.Errorf("expected state=off, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},
		{
			name:   "repeat track with device",
			action: &Repeat{DeviceID: "dev1", State: "track"},
			routes: map[string]http.HandlerFunc{
				"PUT /repeat": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("state"); got != "track" {
						t.Errorf("expected state=track, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},
		{
			name:   "repeat context no device",
			action: &Repeat{DeviceID: "", State: "context"},
			routes: map[string]http.HandlerFunc{
				"PUT /repeat": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("device_id"); got != "" {
						t.Errorf("expected no device_id, got %q", got)
					}
					if got := r.URL.Query().Get("state"); got != "context" {
						t.Errorf("expected state=context, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},

		// Shuffle
		{
			name:   "shuffle enabled with device",
			action: &Shuffle{DeviceID: "device", Enabled: true},
			routes: map[string]http.HandlerFunc{
				"PUT /shuffle": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("state"); got != "true" {
						t.Errorf("expected state=true, got %q", got)
					}
					if got := r.URL.Query().Get("device_id"); got != "device" {
						t.Errorf("expected device_id=device, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},
		{
			name:   "shuffle disabled without device",
			action: &Shuffle{DeviceID: "", Enabled: false},
			routes: map[string]http.HandlerFunc{
				"PUT /shuffle": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("state"); got != "false" {
						t.Errorf("expected state=false, got %q", got)
					}
					if got := r.URL.Query().Get("device_id"); got != "" {
						t.Errorf("expected no device_id, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},

		// Transfer
		{
			name:   "transfer with play",
			action: &Transfer{DeviceID: "device", Play: true},
			routes: map[string]http.HandlerFunc{
				"PUT /": func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Fatal(err)
					}
					if payload["play"] != true {
						t.Fatalf("expected play=true, got %#v", payload)
					}
					deviceIDs, ok := payload["device_ids"].([]interface{})
					if !ok || len(deviceIDs) != 1 || deviceIDs[0] != "device" {
						t.Fatalf("unexpected device_ids: %v", payload["device_ids"])
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},

		// Volume
		{
			name:   "set volume with device",
			action: &Volume{DeviceID: "device", Level: 42},
			routes: map[string]http.HandlerFunc{
				"PUT /volume": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("volume_percent"); got != "42" {
						t.Errorf("expected volume_percent=42, got %q", got)
					}
					if got := r.URL.Query().Get("device_id"); got != "device" {
						t.Errorf("expected device_id=device, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},
		{
			name:   "set volume without device",
			action: &Volume{DeviceID: "", Level: 10},
			routes: map[string]http.HandlerFunc{
				"PUT /volume": func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("volume_percent"); got != "10" {
						t.Errorf("expected volume_percent=10, got %q", got)
					}
					if got := r.URL.Query().Get("device_id"); got != "" {
						t.Errorf("expected no device_id, got %q", got)
					}
					w.WriteHeader(http.StatusNoContent)
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(multiHandler(t, tt.routes))
			defer server.Close()
			client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
			if err := tt.action.Dispatch(context.Background(), client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestActions_ErrorResponse(t *testing.T) {

	snapshotOK := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PlaybackState{Item: &Track{URI: "spotify:track:prior"}})
	}
	forbidden := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }
	devicesOK := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"devices":[]}`))
	}
	noContent := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }

	var tests = []struct {
		name   string
		action Action
		routes map[string]http.HandlerFunc
	}{
		{name: "next", action: &Next{DeviceID: "device"}, routes: map[string]http.HandlerFunc{
			"GET /": snapshotOK, "POST /next": forbidden,
		}},
		{name: "pause", action: &Pause{DeviceID: "device"}, routes: map[string]http.HandlerFunc{
			"PUT /pause": forbidden,
		}},
		{name: "play with context", action: &Play{DeviceID: "device", ContextURI: "spotify:track:abc"}, routes: map[string]http.HandlerFunc{
			"GET /devices": devicesOK, "GET /": noContent, "PUT /": noContent, "PUT /play": forbidden,
		}},
		{name: "play without context", action: &Play{DeviceID: "device"}, routes: map[string]http.HandlerFunc{
			"GET /devices": devicesOK, "GET /": snapshotOK, "PUT /": noContent, "PUT /play": forbidden,
		}},
		{name: "previous", action: &Previous{DeviceID: "device"}, routes: map[string]http.HandlerFunc{
			"GET /": snapshotOK, "POST /previous": forbidden,
		}},
		{name: "shuffle", action: &Shuffle{DeviceID: "device", Enabled: true}, routes: map[string]http.HandlerFunc{
			"PUT /shuffle": forbidden,
		}},
		{name: "transfer", action: &Transfer{DeviceID: "device", Play: false}, routes: map[string]http.HandlerFunc{
			"PUT /": forbidden,
		}},
		{name: "volume", action: &Volume{DeviceID: "device", Level: 50}, routes: map[string]http.HandlerFunc{
			"PUT /volume": forbidden,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(multiHandler(t, tt.routes))
			defer server.Close()
			client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
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
		// Play: ContextURI path
		{
			name:          "play with uri matching context",
			action:        &Play{DeviceID: "device", ContextURI: "spotify:track:abc"},
			state:         &PlaybackState{IsPlaying: true, Context: &PlaybackContext{URI: "spotify:track:abc"}},
			wantLabel:     "play uri=spotify:track:abc device=device",
			wantConfirmed: true,
		},
		{
			name:          "play with uri mismatched context",
			action:        &Play{DeviceID: "device", ContextURI: "spotify:track:abc"},
			state:         &PlaybackState{IsPlaying: true, Context: &PlaybackContext{URI: "spotify:playlist:xyz"}},
			wantLabel:     "play uri=spotify:track:abc device=device",
			wantConfirmed: false,
		},

		// Play: DeviceID path
		{
			name:          "play without uri confirmed by device",
			action:        &Play{DeviceID: "device"},
			state:         &PlaybackState{IsPlaying: true, Device: Device{ID: "device"}},
			wantLabel:     "play device=device",
			wantConfirmed: true,
		},
		{
			name:          "play without uri wrong device",
			action:        &Play{DeviceID: "device"},
			state:         &PlaybackState{IsPlaying: true, Device: Device{ID: "other"}},
			wantLabel:     "play device=device",
			wantConfirmed: false,
		},
		{
			name:          "play without uri not playing",
			action:        &Play{DeviceID: "device"},
			state:         &PlaybackState{IsPlaying: false},
			wantLabel:     "play device=device",
			wantConfirmed: false,
		},

		// Play: no constraints, priorState available — change detection
		{
			name: "play no constraints was paused now playing",
			action: &Play{
				priorState: &PlaybackState{IsPlaying: false, Device: Device{ID: "d1"}},
			},
			state:         &PlaybackState{IsPlaying: true, Device: Device{ID: "d1"}},
			wantLabel:     "play device=",
			wantConfirmed: true,
		},
		{
			name: "play no constraints device changed",
			action: &Play{
				priorState: &PlaybackState{IsPlaying: true, Device: Device{ID: "d1"}},
			},
			state:         &PlaybackState{IsPlaying: true, Device: Device{ID: "d2"}},
			wantLabel:     "play device=",
			wantConfirmed: true,
		},
		{
			name: "play no constraints already playing same device",
			action: &Play{
				priorState: &PlaybackState{IsPlaying: true, Device: Device{ID: "d1"}},
			},
			state:         &PlaybackState{IsPlaying: true, Device: Device{ID: "d1"}},
			wantLabel:     "play device=",
			wantConfirmed: false,
		},
		{
			name:          "play no constraints nil priorState returns false (unconfirmed)",
			action:        &Play{},
			state:         &PlaybackState{IsPlaying: true},
			wantLabel:     "play device=",
			wantConfirmed: false,
		},
		{
			name:          "play no constraints nil state",
			action:        &Play{},
			state:         nil,
			wantLabel:     "play device=",
			wantConfirmed: false,
		},

		// Pause
		{
			name:          "pause confirmed",
			action:        &Pause{DeviceID: "device"},
			state:         &PlaybackState{IsPlaying: false},
			wantLabel:     "pause device=device",
			wantConfirmed: true,
		},

		// Next: priorState available
		{
			name:          "next track changed",
			action:        &Next{DeviceID: "device", priorState: &PlaybackState{Item: &Track{URI: "spotify:track:old"}}},
			state:         &PlaybackState{Item: &Track{URI: "spotify:track:new"}},
			wantLabel:     "next device=device",
			wantConfirmed: true,
		},
		{
			name:          "next track unchanged",
			action:        &Next{DeviceID: "device", priorState: &PlaybackState{Item: &Track{URI: "spotify:track:old"}}},
			state:         &PlaybackState{Item: &Track{URI: "spotify:track:old"}},
			wantLabel:     "next device=device",
			wantConfirmed: false,
		},
		// Next: priorState nil — unconfirmable, returns false
		{
			name:          "next nil priorState returns false",
			action:        &Next{DeviceID: "device"},
			state:         &PlaybackState{Item: &Track{URI: "spotify:track:new"}},
			wantLabel:     "next device=device",
			wantConfirmed: false,
		},
		// Next: priorState has no item — unconfirmable
		{
			name:          "next priorState no item returns false",
			action:        &Next{DeviceID: "device", priorState: &PlaybackState{}},
			state:         &PlaybackState{Item: &Track{URI: "spotify:track:new"}},
			wantLabel:     "next device=device",
			wantConfirmed: false,
		},
		{
			name:          "next nil current state",
			action:        &Next{DeviceID: "device", priorState: &PlaybackState{Item: &Track{URI: "spotify:track:old"}}},
			state:         nil,
			wantLabel:     "next device=device",
			wantConfirmed: false,
		},

		// Previous: mirrors Next
		{
			name:          "previous track changed",
			action:        &Previous{DeviceID: "device", priorState: &PlaybackState{Item: &Track{URI: "spotify:track:old"}}},
			state:         &PlaybackState{Item: &Track{URI: "spotify:track:new"}},
			wantLabel:     "previous device=device",
			wantConfirmed: true,
		},
		{
			name:          "previous track unchanged",
			action:        &Previous{DeviceID: "device", priorState: &PlaybackState{Item: &Track{URI: "spotify:track:old"}}},
			state:         &PlaybackState{Item: &Track{URI: "spotify:track:old"}},
			wantLabel:     "previous device=device",
			wantConfirmed: false,
		},
		{
			name:          "previous nil priorState returns false",
			action:        &Previous{DeviceID: "device"},
			state:         &PlaybackState{Item: &Track{URI: "spotify:track:new"}},
			wantLabel:     "previous device=device",
			wantConfirmed: false,
		},
		{
			name:          "previous priorState no item returns false",
			action:        &Previous{DeviceID: "device", priorState: &PlaybackState{}},
			state:         &PlaybackState{Item: &Track{URI: "spotify:track:new"}},
			wantLabel:     "previous device=device",
			wantConfirmed: false,
		},

		// Shuffle
		{
			name:          "shuffle enabled confirmed",
			action:        &Shuffle{DeviceID: "device", Enabled: true},
			state:         &PlaybackState{ShuffleState: true},
			wantLabel:     "shuffle enabled=true device=device",
			wantConfirmed: true,
		},
		{
			name:          "shuffle disabled confirmed",
			action:        &Shuffle{DeviceID: "device", Enabled: false},
			state:         &PlaybackState{ShuffleState: false},
			wantLabel:     "shuffle enabled=false device=device",
			wantConfirmed: true,
		},
		{
			name:          "shuffle mismatch",
			action:        &Shuffle{DeviceID: "device", Enabled: true},
			state:         &PlaybackState{ShuffleState: false},
			wantLabel:     "shuffle enabled=true device=device",
			wantConfirmed: false,
		},

		// Repeat
		{
			name:          "repeat track confirmed",
			action:        &Repeat{DeviceID: "device", State: "track"},
			state:         &PlaybackState{RepeatState: "track"},
			wantLabel:     "repeat state=track device=device",
			wantConfirmed: true,
		},
		{
			name:          "repeat mismatch",
			action:        &Repeat{DeviceID: "device", State: "track"},
			state:         &PlaybackState{RepeatState: "off"},
			wantLabel:     "repeat state=track device=device",
			wantConfirmed: false,
		},

		// Volume
		{
			name:          "volume exact match",
			action:        &Volume{DeviceID: "device", Level: 42},
			state:         &PlaybackState{Device: Device{VolumePercent: 42}},
			wantLabel:     "volume level=42 device=device",
			wantConfirmed: true,
		},
		{
			name:          "volume within tolerance",
			action:        &Volume{DeviceID: "device", Level: 42},
			state:         &PlaybackState{Device: Device{VolumePercent: 41}},
			wantLabel:     "volume level=42 device=device",
			wantConfirmed: true,
		},
		{
			name:          "volume outside tolerance",
			action:        &Volume{DeviceID: "device", Level: 42},
			state:         &PlaybackState{Device: Device{VolumePercent: 30}},
			wantLabel:     "volume level=42 device=device",
			wantConfirmed: false,
		},

		// Transfer
		{
			name:          "transfer confirmed",
			action:        &Transfer{DeviceID: "device", Play: false},
			state:         &PlaybackState{Device: Device{ID: "device"}},
			wantLabel:     "transfer device=device play=false",
			wantConfirmed: true,
		},
		{
			name:          "transfer with play confirmed",
			action:        &Transfer{DeviceID: "device", Play: true},
			state:         &PlaybackState{Device: Device{ID: "device"}, IsPlaying: true},
			wantLabel:     "transfer device=device play=true",
			wantConfirmed: true,
		},
		{
			name:          "transfer with play not yet playing",
			action:        &Transfer{DeviceID: "device", Play: true},
			state:         &PlaybackState{Device: Device{ID: "device"}, IsPlaying: false},
			wantLabel:     "transfer device=device play=true",
			wantConfirmed: false,
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

func TestPrevious_ConfirmedNilState(t *testing.T) {
	p := &Previous{priorState: &PlaybackState{Item: &Track{URI: "spotify:track:old"}}}
	if p.Confirmed(nil) {
		t.Error("expected Confirmed=false for nil state")
	}
	noItem := &PlaybackState{}
	if p.Confirmed(noItem) {
		t.Error("expected Confirmed=false when current state has no item")
	}
}

// TestDispatch_RequestCreationError exercises the error path returned when
// urlPlayer is set to a value that makes http.NewRequestWithContext fail
// (a URL containing a null byte is rejected by the net/url parser).
func TestDispatch_RequestCreationError(t *testing.T) {
	client := &Client{accessToken: "t", httpClient: http.DefaultClient, urlPlayer: "http://\x00invalid"}
	ctx := context.Background()

	actions := []struct {
		name   string
		action Action
	}{
		{"play with context", &Play{ContextURI: "spotify:track:abc"}},
		{"play without context", &Play{}},
		{"pause", &Pause{}},
		{"next", &Next{}},
		{"previous", &Previous{}},
		{"shuffle", &Shuffle{}},
		{"repeat", &Repeat{State: "off"}},
		{"volume", &Volume{}},
		{"transfer", &Transfer{}},
	}
	for _, tt := range actions {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.action.Dispatch(ctx, client); err == nil {
				t.Fatalf("expected error for invalid URL")
			}
		})
	}
}

// TestSnapshotDispatch verifies that Next, Previous, and Play (no ContextURI)
// capture priorState during Dispatch and use it correctly in Confirmed.
func TestSnapshotDispatch(t *testing.T) {
	priorPlayback := PlaybackState{
		IsPlaying: true,
		Device:    Device{ID: "prior-device"},
		Item:      &Track{URI: "spotify:track:prior"},
	}
	snapshotBody, _ := json.Marshal(priorPlayback)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.Write(snapshotBody)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient("token")
	client.SetHTTPClient(srv.Client())
	client.urlPlayer = srv.URL

	t.Run("next captures priorState", func(t *testing.T) {
		n := &Next{DeviceID: "device"}
		if err := n.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n.priorState == nil || n.priorState.Item == nil {
			t.Fatal("expected priorState with item to be captured")
		}
		if n.priorState.Item.URI != "spotify:track:prior" {
			t.Fatalf("expected priorState URI=spotify:track:prior, got %q", n.priorState.Item.URI)
		}
		// Track changed: confirmed.
		if !n.Confirmed(&PlaybackState{Item: &Track{URI: "spotify:track:new"}}) {
			t.Fatal("expected Confirmed=true when track changed")
		}
		// Track unchanged: not confirmed.
		if n.Confirmed(&PlaybackState{Item: &Track{URI: "spotify:track:prior"}}) {
			t.Fatal("expected Confirmed=false when track unchanged")
		}
	})

	t.Run("previous captures priorState", func(t *testing.T) {
		p := &Previous{DeviceID: "device"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.priorState == nil || p.priorState.Item == nil {
			t.Fatal("expected priorState with item to be captured")
		}
		if !p.Confirmed(&PlaybackState{Item: &Track{URI: "spotify:track:new"}}) {
			t.Fatal("expected Confirmed=true when track changed")
		}
		if p.Confirmed(&PlaybackState{Item: &Track{URI: "spotify:track:prior"}}) {
			t.Fatal("expected Confirmed=false when track unchanged")
		}
	})

	t.Run("play no context captures priorState", func(t *testing.T) {
		p := &Play{DeviceID: ""}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.priorState == nil {
			t.Fatal("expected priorState to be captured for play without ContextURI")
		}
		if p.priorState.Device.ID != "prior-device" {
			t.Fatalf("expected priorState device=prior-device, got %q", p.priorState.Device.ID)
		}
		// Was playing on prior-device; now playing on a new device — confirmed.
		if !p.Confirmed(&PlaybackState{IsPlaying: true, Device: Device{ID: "new-device"}}) {
			t.Fatal("expected Confirmed=true when device changed")
		}
		// Was playing; still playing on same device — not confirmed.
		if p.Confirmed(&PlaybackState{IsPlaying: true, Device: Device{ID: "prior-device"}}) {
			t.Fatal("expected Confirmed=false when device unchanged and was already playing")
		}
	})

	t.Run("play with context does not snapshot", func(t *testing.T) {
		p := &Play{ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.priorState != nil {
			t.Fatal("expected priorState to be nil when ContextURI is set")
		}
	})
}

// TestPlay_WakeTransfer covers Play.Dispatch's wake-before-play step: a
// device-targeted play fetches the device list (diagnostics only) and
// playback state for comparison (load-bearing — decides alreadyActive),
// transfers to the device with play=false (waking it, without starting
// playback) unless it's already active, waits PlayWakeSettleDelay, then
// issues the real play request. See the comment on Play.Dispatch for why.
func TestPlay_WakeTransfer(t *testing.T) {
	t.Run("device set: fetches devices, compares state, wakes via transfer, confirms, then plays", func(t *testing.T) {
		var calls []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, r.Method+" "+r.URL.Path)
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/devices":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"devices":[]}`))
			case r.Method == http.MethodGet && r.URL.Path == "/":
				w.WriteHeader(http.StatusNoContent) // compare-state check
			case r.Method == http.MethodPut && r.URL.Path == "/":
				var payload map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload["play"] != false {
					t.Errorf("expected wake transfer play=false, got %#v", payload["play"])
				}
				deviceIDs, _ := payload["device_ids"].([]interface{})
				if len(deviceIDs) != 1 || deviceIDs[0] != "device" {
					t.Errorf("expected device_ids=[device], got %v", payload["device_ids"])
				}
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPut && r.URL.Path == "/play":
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer srv.Close()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{DeviceID: "device", ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []string{"GET /devices", "GET /", "PUT /", "GET /", "PUT /play"}
		if len(calls) != len(want) {
			t.Fatalf("expected %v, got %v", want, calls)
		}
		for i, w := range want {
			if calls[i] != w {
				t.Fatalf("expected %v, got %v", want, calls)
			}
		}
	})

	t.Run("device already active: skips wake transfer and settle delay, plays immediately", func(t *testing.T) {
		var calls []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, r.Method+" "+r.URL.Path)
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/devices":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"devices":[]}`))
			case r.Method == http.MethodGet && r.URL.Path == "/":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"is_playing":false,"device":{"id":"device","is_active":true}}`))
			case r.Method == http.MethodPut && r.URL.Path == "/play":
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer srv.Close()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{DeviceID: "device", ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []string{"GET /devices", "GET /", "PUT /play"}
		if len(calls) != len(want) {
			t.Fatalf("expected %v, got %v", want, calls)
		}
		for i, w := range want {
			if calls[i] != w {
				t.Fatalf("expected %v, got %v", want, calls)
			}
		}
	})

	t.Run("different device active: still wakes via transfer", func(t *testing.T) {
		var calls []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, r.Method+" "+r.URL.Path)
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/devices":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"devices":[]}`))
			case r.Method == http.MethodGet && r.URL.Path == "/":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"is_playing":true,"device":{"id":"other-device","is_active":true}}`))
			case r.Method == http.MethodPut && r.URL.Path == "/":
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPut && r.URL.Path == "/play":
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer srv.Close()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{DeviceID: "device", ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []string{"GET /devices", "GET /", "PUT /", "GET /", "PUT /play"}
		if len(calls) != len(want) {
			t.Fatalf("expected %v, got %v", want, calls)
		}
		for i, w := range want {
			if calls[i] != w {
				t.Fatalf("expected %v, got %v", want, calls)
			}
		}
	})

	t.Run("no device: skips devices lookup and wake transfer", func(t *testing.T) {
		var calls []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, r.Method+" "+r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(calls) != 1 || calls[0] != "PUT /play" {
			t.Fatalf("expected only play, no devices lookup or wake transfer, got %v", calls)
		}
	})

	t.Run("wake transfer failure is non-fatal, play is still attempted", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/devices":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"devices":[]}`))
			case r.Method == http.MethodGet && r.URL.Path == "/":
				w.WriteHeader(http.StatusNoContent) // compare-state check
			case r.Method == http.MethodPut && r.URL.Path == "/":
				w.WriteHeader(http.StatusNotFound) // e.g. device not found
			case r.Method == http.MethodPut && r.URL.Path == "/play":
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer srv.Close()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{DeviceID: "device", ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("expected play to succeed despite wake transfer failure, got: %v", err)
		}
	})

	t.Run("devices lookup failure is non-fatal, wake and play still attempted", func(t *testing.T) {
		var calls []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, r.Method+" "+r.URL.Path)
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/devices":
				w.WriteHeader(http.StatusInternalServerError)
			case r.Method == http.MethodPut && r.URL.Path == "/":
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodGet && r.URL.Path == "/":
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPut && r.URL.Path == "/play":
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer srv.Close()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{DeviceID: "device", ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("expected play to succeed despite devices lookup failure, got: %v", err)
		}
		if last := calls[len(calls)-1]; last != "PUT /play" {
			t.Fatalf("expected play to still be attempted, got %v", calls)
		}
	})

	t.Run("context canceled during settle delay aborts before play", func(t *testing.T) {
		old := PlayWakeSettleDelay
		PlayWakeSettleDelay = 50 * time.Millisecond
		defer func() { PlayWakeSettleDelay = old }()

		playCalled := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/devices":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"devices":[]}`))
			case r.Method == http.MethodPut && r.URL.Path == "/":
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodGet && r.URL.Path == "/":
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPut && r.URL.Path == "/play":
				playCalled = true
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{DeviceID: "device", ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(ctx, client); err == nil {
			t.Fatal("expected error from context cancellation during the settle delay")
		}
		if playCalled {
			t.Fatal("expected play request not to be sent before the settle delay completed")
		}
	})
}

func TestPlay_NeedsStabilize(t *testing.T) {
	t.Run("true before any dispatch", func(t *testing.T) {
		p := &Play{}
		if !p.NeedsStabilize() {
			t.Error("expected NeedsStabilize to default true")
		}
	})

	t.Run("false after dispatch skips wake because device already active", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/devices":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"devices":[]}`))
			case r.Method == http.MethodGet && r.URL.Path == "/":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"is_playing":false,"device":{"id":"device","is_active":true}}`))
			case r.Method == http.MethodPut && r.URL.Path == "/play":
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer srv.Close()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{DeviceID: "device", ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.NeedsStabilize() {
			t.Error("expected NeedsStabilize false after skipping an already-active wake")
		}
	})

	t.Run("true after dispatch performs a real wake", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/devices":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"devices":[]}`))
			case r.Method == http.MethodGet && r.URL.Path == "/":
				w.WriteHeader(http.StatusNoContent) // nothing active
			case r.Method == http.MethodPut && r.URL.Path == "/":
				w.WriteHeader(http.StatusNoContent) // wake transfer
			case r.Method == http.MethodPut && r.URL.Path == "/play":
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer srv.Close()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{DeviceID: "device", ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !p.NeedsStabilize() {
			t.Error("expected NeedsStabilize true after an actual wake")
		}
	})

	t.Run("true when wake transfer fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/devices":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"devices":[]}`))
			case r.Method == http.MethodGet && r.URL.Path == "/":
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPut && r.URL.Path == "/":
				w.WriteHeader(http.StatusNotFound) // wake transfer fails
			case r.Method == http.MethodPut && r.URL.Path == "/play":
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer srv.Close()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{DeviceID: "device", ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !p.NeedsStabilize() {
			t.Error("expected NeedsStabilize true when the wake transfer itself failed")
		}
	})

	t.Run("true when no device is targeted", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !p.NeedsStabilize() {
			t.Error("expected NeedsStabilize true when no device is targeted")
		}
	})

	t.Run("resets to true on a later dispatch that does need to wake", func(t *testing.T) {
		callCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/devices":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"devices":[]}`))
			case r.Method == http.MethodGet && r.URL.Path == "/":
				callCount++
				if callCount == 1 {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"is_playing":false,"device":{"id":"device","is_active":true}}`))
					return
				}
				w.WriteHeader(http.StatusNoContent) // second dispatch: nothing active
			case r.Method == http.MethodPut && r.URL.Path == "/":
				w.WriteHeader(http.StatusNoContent)
			case r.Method == http.MethodPut && r.URL.Path == "/play":
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		defer srv.Close()

		client := &Client{accessToken: "t", httpClient: srv.Client(), urlPlayer: srv.URL}
		p := &Play{DeviceID: "device", ContextURI: "spotify:track:abc"}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatal(err)
		}
		if p.NeedsStabilize() {
			t.Fatal("expected first dispatch to skip wake (already active)")
		}
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatal(err)
		}
		if !p.NeedsStabilize() {
			t.Error("expected second dispatch to require stabilize after an actual wake")
		}
	})
}
