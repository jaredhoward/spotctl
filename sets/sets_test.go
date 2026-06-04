package sets_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jaredhoward/spotctl/config"
	sets "github.com/jaredhoward/spotctl/sets"
	"github.com/jaredhoward/spotctl/spotify"
)

// ---- test helpers -----------------------------------------------------------

func boolPtr(b bool) *bool { return &b }

func mockServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if fn, ok := handlers[key]; ok {
			fn(w, r)
			return
		}
		t.Logf("unhandled request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
}

func newClient(t *testing.T, srv *httptest.Server) *spotify.Client {
	t.Helper()
	c := spotify.NewClient("test-token")
	c.SetHTTPClient(srv.Client())
	return c
}

func newCfg(s map[string]config.Set) *config.Config {
	return &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
		Sets: s,
	}
}

func stateBody(state spotify.PlaybackState) []byte {
	b, _ := json.Marshal(state)
	return b
}

func useTestServer(t *testing.T, srv *httptest.Server) {
	t.Helper()
	original := spotify.URLPlayer
	spotify.URLPlayer = srv.URL
	t.Cleanup(func() { spotify.URLPlayer = original })
}

func readBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

// ---- Build: URI resolution --------------------------------------------------

func TestBuild_Play_Playlist(t *testing.T) {
	var gotBody string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			gotBody = readBody(r)
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{{Action: "play", Params: config.CommandParams{PlaylistID: "pl123"}, Confirm: boolPtr(false)}}}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:playlist:pl123") {
		t.Errorf("expected playlist URI in body, got: %q", gotBody)
	}
}

func TestBuild_Play_Track(t *testing.T) {
	var gotBody string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			gotBody = readBody(r)
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{{Action: "play", Params: config.CommandParams{TrackID: "tr456"}, Confirm: boolPtr(false)}}}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:track:tr456") {
		t.Errorf("expected track URI in body, got: %q", gotBody)
	}
}

func TestBuild_Play_Album(t *testing.T) {
	var gotBody string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			gotBody = readBody(r)
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{{Action: "play", Params: config.CommandParams{AlbumID: "al789"}, Confirm: boolPtr(false)}}}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:album:al789") {
		t.Errorf("expected album URI in body, got: %q", gotBody)
	}
}

func TestBuild_Play_Artist(t *testing.T) {
	var gotBody string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			gotBody = readBody(r)
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{{Action: "play", Params: config.CommandParams{ArtistID: "ar999"}, Confirm: boolPtr(false)}}}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:artist:ar999") {
		t.Errorf("expected artist URI in body, got: %q", gotBody)
	}
}

func TestBuild_Play_MultipleURIError(t *testing.T) {
	set := config.Set{Commands: []config.Command{
		{Action: "play", Params: config.CommandParams{PlaylistID: "pl1", TrackID: "tr1"}},
	}}
	_, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err == nil || !strings.Contains(err.Error(), "only one of") {
		t.Fatalf("expected multiple-URI error, got %v", err)
	}
}

// ---- Build: device resolution -----------------------------------------------

func TestBuild_SetLevelDeviceApplied(t *testing.T) {
	var gotDeviceID string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) {
			gotDeviceID = r.URL.Query().Get("device_id")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{DeviceID: "set-device", Commands: []config.Command{{Action: "pause", Confirm: boolPtr(false)}}}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDeviceID != "set-device" {
		t.Errorf("expected set-level device_id, got %q", gotDeviceID)
	}
}

func TestBuild_CommandDeviceOverridesSet(t *testing.T) {
	var gotDeviceID string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) {
			gotDeviceID = r.URL.Query().Get("device_id")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{
		DeviceID: "set-device",
		Commands: []config.Command{{Action: "pause", DeviceID: "cmd-device", Confirm: boolPtr(false)}},
	}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDeviceID != "cmd-device" {
		t.Errorf("expected command-level device_id to win, got %q", gotDeviceID)
	}
}

// ---- RunSet: execution, confirm, error/timeout policies ---------------------

func TestRunSet_PlayNoConfirm(t *testing.T) {
	playCalled := false
	srv := mockServer(t, map[string]http.HandlerFunc{
		"GET /":     func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"PUT /play": func(w http.ResponseWriter, r *http.Request) { playCalled = true; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{{Action: "play", DeviceID: "d1", Confirm: boolPtr(false)}}}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !playCalled {
		t.Error("expected play to be called")
	}
}

func TestRunSet_PlayAndConfirm(t *testing.T) {
	pollCount := 0
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /": func(w http.ResponseWriter, r *http.Request) {
			pollCount++
			w.Write(stateBody(spotify.PlaybackState{
				IsPlaying: true,
				Device:    spotify.Device{ID: "d1"},
			}))
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{
		{Action: "play", DeviceID: "d1", Confirm: boolPtr(true), Timeout: "5s"},
	}}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pollCount < 1 {
		t.Error("expected at least one state poll")
	}
}

func TestRunSet_ConfirmTimeout_Continue(t *testing.T) {
	pauseCalled := false
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /": func(w http.ResponseWriter, r *http.Request) {
			w.Write(stateBody(spotify.PlaybackState{IsPlaying: false}))
		},
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCalled = true; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{
		OnTimeout: config.OnFailureContinue,
		Commands: []config.Command{
			{Action: "play", DeviceID: "d1", Confirm: boolPtr(true), Timeout: "50ms"},
			{Action: "pause", DeviceID: "d1"},
		},
	}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pauseCalled {
		t.Error("expected pause after timeout+continue")
	}
}

func TestRunSet_ConfirmTimeout_Fail(t *testing.T) {
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /": func(w http.ResponseWriter, r *http.Request) {
			w.Write(stateBody(spotify.PlaybackState{IsPlaying: false}))
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{
		{Action: "play", DeviceID: "d1", Confirm: boolPtr(true), Timeout: "50ms", OnTimeout: config.OnFailureFail},
	}}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = rs.Dispatch(context.Background(), newClient(t, srv))
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestRunSet_ConfirmTimeout_SkipRemaining(t *testing.T) {
	pauseCalled := false
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /": func(w http.ResponseWriter, r *http.Request) {
			w.Write(stateBody(spotify.PlaybackState{IsPlaying: false}))
		},
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCalled = true; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{
		{Action: "play", DeviceID: "d1", Confirm: boolPtr(true), Timeout: "50ms", OnTimeout: config.OnFailureSkipRemaining},
		{Action: "pause", DeviceID: "d1"},
	}}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if pauseCalled {
		t.Error("pause should not run after skip_remaining")
	}
}

func TestRunSet_CommandError_Continue(t *testing.T) {
	pauseCalled := false
	srv := mockServer(t, map[string]http.HandlerFunc{
		"GET /":      func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"POST /next": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCalled = true; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{
		OnError: config.OnFailureContinue,
		Commands: []config.Command{
			{Action: "next", DeviceID: "d1"},
			{Action: "pause", DeviceID: "d1", Confirm: boolPtr(false)},
		},
	}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pauseCalled {
		t.Error("expected pause after next error with on_error:continue")
	}
}

func TestRunSet_CommandError_Fail(t *testing.T) {
	srv := mockServer(t, map[string]http.HandlerFunc{
		"GET /":      func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"POST /next": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{
		OnError:  config.OnFailureFail,
		Commands: []config.Command{{Action: "next", DeviceID: "d1"}},
	}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = rs.Dispatch(context.Background(), newClient(t, srv))
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("expected abort error, got %v", err)
	}
}

func TestRunSet_CommandError_SkipRemaining(t *testing.T) {
	pauseCalled := false
	srv := mockServer(t, map[string]http.HandlerFunc{
		"GET /":      func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"POST /next": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCalled = true; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{
		OnError: config.OnFailureSkipRemaining,
		Commands: []config.Command{
			{Action: "next", DeviceID: "d1"},
			{Action: "pause", DeviceID: "d1"},
		},
	}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if pauseCalled {
		t.Error("pause should not run after skip_remaining")
	}
}

func TestRunSet_CommandOverridesSetDefault(t *testing.T) {
	srv := mockServer(t, map[string]http.HandlerFunc{
		"GET /":      func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"POST /next": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{
		OnError:  config.OnFailureContinue,
		Commands: []config.Command{{Action: "next", DeviceID: "d1", OnError: config.OnFailureFail}},
	}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err == nil {
		t.Fatal("expected error when command overrides with on_error:fail")
	}
}

// ---- Sleep ------------------------------------------------------------------

func TestRunSet_Sleep(t *testing.T) {
	set := config.Set{Commands: []config.Command{
		{Action: "sleep", Params: config.CommandParams{Duration: "20ms"}},
	}}
	rs, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := rs.Dispatch(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Error("sleep did not sleep long enough")
	}
}

// ---- run_set composition ----------------------------------------------------

func TestRunSet_Composable(t *testing.T) {
	pauseCount := 0
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCount++; w.WriteHeader(http.StatusNoContent) },
		"GET /": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"is_playing":false}`))
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	inner := config.Set{Commands: []config.Command{{Action: "pause", DeviceID: "d1"}}}
	outer := config.Set{Commands: []config.Command{
		{Action: "run_set", Params: config.CommandParams{Set: "inner"}},
		{Action: "run_set", Params: config.CommandParams{Set: "inner"}},
	}}
	rs, err := sets.Build("outer", outer, newCfg(map[string]config.Set{"inner": inner}), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Dispatch(context.Background(), newClient(t, srv)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pauseCount != 2 {
		t.Errorf("expected inner set to run twice, got %d", pauseCount)
	}
}

func TestBuild_MaxDepth(t *testing.T) {
	s := map[string]config.Set{
		"self": {Commands: []config.Command{
			{Action: "run_set", Params: config.CommandParams{Set: "self"}},
		}},
	}
	cfg := newCfg(s)
	_, err := sets.Build("self", cfg.Sets["self"], cfg, sets.MaxSetDepth, nil)
	var depthErr *sets.DepthExceededError
	if !errors.As(err, &depthErr) {
		t.Fatalf("expected DepthExceededError, got %v", err)
	}
}

func TestBuild_UnknownNestedSet(t *testing.T) {
	set := config.Set{Commands: []config.Command{
		{Action: "run_set", Params: config.CommandParams{Set: "does-not-exist"}},
	}}
	_, err := sets.Build("outer", set, newCfg(nil), 0, nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestBuild_UnknownAction(t *testing.T) {
	set := config.Set{Commands: []config.Command{{Action: "bogus"}}}
	_, err := sets.Build("test", set, newCfg(nil), 0, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("expected unknown action error, got %v", err)
	}
}

// ---- Spotify action types: Confirmed ----------------------------------------

func TestPlay_Confirmed(t *testing.T) {
	a := &spotify.Play{}
	if !a.Confirmed(&spotify.PlaybackState{IsPlaying: true}) {
		t.Error("expected confirmed when is_playing=true and no constraints")
	}
	if a.Confirmed(&spotify.PlaybackState{IsPlaying: false}) {
		t.Error("expected not confirmed when is_playing=false")
	}
	if a.Confirmed(nil) {
		t.Error("expected not confirmed for nil state")
	}

	ad := &spotify.Play{DeviceID: "d1"}
	if !ad.Confirmed(&spotify.PlaybackState{IsPlaying: true, Device: spotify.Device{ID: "d1"}}) {
		t.Error("expected confirmed when device matches")
	}
	if ad.Confirmed(&spotify.PlaybackState{IsPlaying: true, Device: spotify.Device{ID: "other"}}) {
		t.Error("expected not confirmed when device differs")
	}
}

func TestPause_Confirmed(t *testing.T) {
	a := &spotify.Pause{}
	if !a.Confirmed(&spotify.PlaybackState{IsPlaying: false}) {
		t.Error("expected confirmed when not playing")
	}
	if a.Confirmed(&spotify.PlaybackState{IsPlaying: true}) {
		t.Error("expected not confirmed when playing")
	}
}

func TestShuffle_Confirmed(t *testing.T) {
	a := &spotify.Shuffle{Enabled: true}
	if !a.Confirmed(&spotify.PlaybackState{ShuffleState: true}) {
		t.Error("expected confirmed when shuffle_state=true")
	}
	if a.Confirmed(&spotify.PlaybackState{ShuffleState: false}) {
		t.Error("expected not confirmed")
	}
}

func TestRepeat_Confirmed(t *testing.T) {
	a := &spotify.Repeat{State: "context"}
	if !a.Confirmed(&spotify.PlaybackState{RepeatState: "context"}) {
		t.Error("expected confirmed when repeat_state matches")
	}
	if a.Confirmed(&spotify.PlaybackState{RepeatState: "off"}) {
		t.Error("expected not confirmed when repeat_state differs")
	}
}

func TestVolume_Confirmed(t *testing.T) {
	a := &spotify.Volume{Level: 42}
	if !a.Confirmed(&spotify.PlaybackState{Device: spotify.Device{VolumePercent: 42}}) {
		t.Error("expected confirmed at exact volume")
	}
	if !a.Confirmed(&spotify.PlaybackState{Device: spotify.Device{VolumePercent: 41}}) {
		t.Error("expected confirmed within ±1 (41)")
	}
	if !a.Confirmed(&spotify.PlaybackState{Device: spotify.Device{VolumePercent: 43}}) {
		t.Error("expected confirmed within ±1 (43)")
	}
	if a.Confirmed(&spotify.PlaybackState{Device: spotify.Device{VolumePercent: 30}}) {
		t.Error("expected not confirmed when diff > 1")
	}
	if a.Confirmed(nil) {
		t.Error("expected not confirmed for nil state")
	}
}

func TestTransfer_Confirmed(t *testing.T) {
	a := &spotify.Transfer{DeviceID: "target"}
	if !a.Confirmed(&spotify.PlaybackState{Device: spotify.Device{ID: "target"}}) {
		t.Error("expected confirmed when active device matches")
	}
	if a.Confirmed(&spotify.PlaybackState{Device: spotify.Device{ID: "other"}}) {
		t.Error("expected not confirmed when active device differs")
	}
}

// ---- Execute: confirm polling with transient errors -------------------------

func TestExecute_ConfirmPollError(t *testing.T) {
	pollCount := 0
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /": func(w http.ResponseWriter, r *http.Request) {
			pollCount++
			if pollCount < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Write(stateBody(spotify.PlaybackState{
				IsPlaying: true,
				Device:    spotify.Device{ID: "d1"},
			}))
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	a := &spotify.Play{DeviceID: "d1"}
	err := sets.Execute(context.Background(), a, newClient(t, srv), sets.ExecuteOptions{
		Confirm:      true,
		Timeout:      5 * time.Second,
		PollInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("expected success after transient poll errors, got %v", err)
	}
	if pollCount < 3 {
		t.Errorf("expected at least 3 polls, got %d", pollCount)
	}
}

func TestCommandLabel(t *testing.T) {
	cases := []struct {
		name string
		cmd  config.Command
		want []string
	}{
		{
			name: "play with uri and metadata",
			cmd: config.Command{
				Action:   "play",
				DeviceID: "dev1",
				Confirm:  boolPtr(true),
				Name:     "my play",
				Params:   config.CommandParams{URI: "spotify:track:abc"},
			},
			want: []string{"play", "uri=spotify:track:abc", "device=dev1", "confirm", "(my play)"},
		},
		{
			name: "volume with level",
			cmd: config.Command{
				Action: "volume",
				Params: config.CommandParams{Level: func(i int) *int { return &i }(5)},
			},
			want: []string{"volume", "level=5"},
		},
		{
			name: "shuffle disabled",
			cmd:  config.Command{Action: "shuffle", Params: config.CommandParams{Enabled: func() *bool { b := false; return &b }()}},
			want: []string{"shuffle", "enabled=false"},
		},
		{
			name: "repeat state",
			cmd:  config.Command{Action: "repeat", Params: config.CommandParams{RepeatState: "track"}},
			want: []string{"repeat", "state=track"},
		},
		{
			name: "sleep duration",
			cmd:  config.Command{Action: "sleep", Params: config.CommandParams{Duration: "20ms"}},
			want: []string{"sleep", "duration=20ms"},
		},
		{
			name: "run_set label",
			cmd:  config.Command{Action: "run_set", Params: config.CommandParams{Set: "inner"}},
			want: []string{"run_set", "set=inner"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := sets.CommandLabel(1, tt.cmd)
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("expected %q in label %q", want, got)
				}
			}
		})
	}
}

func TestRunSet_ConfirmedAndLabel(t *testing.T) {
	r := sets.RunSet{Name: "myset"}
	if !r.Confirmed(nil) {
		t.Fatal("expected RunSet.Confirmed to return true")
	}
	if got := r.Label(); got != "run_set set=myset" {
		t.Fatalf("expected run_set label, got %q", got)
	}
}

// ---- Error types ------------------------------------------------------------

func TestTimeoutError_Message(t *testing.T) {
	e := &sets.TimeoutError{Timeout: 5 * time.Second, ActionLabel: "play device=d1"}
	if !strings.Contains(e.Error(), "play device=d1") || !strings.Contains(e.Error(), "5s") {
		t.Errorf("unexpected message: %q", e.Error())
	}
}

func TestDepthExceededError_Message(t *testing.T) {
	e := &sets.DepthExceededError{Max: 10}
	if !strings.Contains(e.Error(), "recursion depth exceeded") {
		t.Errorf("unexpected message: %q", e.Error())
	}
}
