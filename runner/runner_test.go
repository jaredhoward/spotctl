package runner_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/runner"
	"github.com/jaredhoward/spotctl/spotify"
)

// ----- small helpers ---------------------------------------------------------

func intPtr(i int) *int    { return &i }
func boolPtr(b bool) *bool { return &b }

// mockServer creates an httptest.Server whose mux maps "METHOD /path" strings
// to handler functions. Unmatched requests are logged and return 404.
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

// newClient builds a Spotify client pointed at srv with a dummy token.
func newClient(t *testing.T, srv *httptest.Server) *spotify.Client {
	t.Helper()
	c := spotify.NewClient("test-token")
	c.SetHTTPClient(srv.Client())
	return c
}

// newCfg builds a minimal *config.Config with the given sets.
func newCfg(sets map[string]config.Set) *config.Config {
	return &config.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RefreshToken: "refresh",
		Sets:         sets,
	}
}

// stateBody marshals a PlaybackState to JSON bytes.
func stateBody(state spotify.PlaybackState) []byte {
	b, _ := json.Marshal(state)
	return b
}

// useTestServer points URLPlayer at srv for the duration of the test.
func useTestServer(t *testing.T, srv *httptest.Server) {
	t.Helper()
	original := spotify.URLPlayer
	spotify.URLPlayer = srv.URL
	t.Cleanup(func() { spotify.URLPlayer = original })
}

func readBody(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	b, err := io.ReadAll(r.Body)
	return string(b), err
}

// ----- DispatchAction: sleep -------------------------------------------------

func TestDispatchAction_Sleep(t *testing.T) {
	p := config.CommandParams{Duration: "20ms"}
	start := time.Now()
	if err := runner.DispatchAction(p, "sleep", nil, newCfg(nil), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Error("sleep did not sleep long enough")
	}
}

// ----- DispatchAction: run_set -----------------------------------------------

func TestDispatchAction_RunSet_Success(t *testing.T) {
	pauseCount := 0
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCount++; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	inner := config.Set{
		Commands: []config.Command{
			{Action: "pause", Params: config.CommandParams{DeviceID: "d1"}},
		},
	}
	cfg := newCfg(map[string]config.Set{"inner": inner})
	p := config.CommandParams{Set: "inner"}

	if err := runner.DispatchAction(p, "run_set", newClient(t, srv), cfg, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pauseCount != 1 {
		t.Errorf("expected inner set to run once, got pause count=%d", pauseCount)
	}
}

func TestDispatchAction_RunSet_NotFound(t *testing.T) {
	p := config.CommandParams{Set: "does-not-exist"}
	err := runner.DispatchAction(p, "run_set", nil, newCfg(nil), 0)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestDispatchAction_UnknownAction(t *testing.T) {
	err := runner.DispatchAction(config.CommandParams{}, "bogus", nil, newCfg(nil), 0)
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("expected unknown action error, got %v", err)
	}
}

// ----- DispatchAction: play variants -----------------------------------------

func TestDispatchAction_Play_WithPlaylist(t *testing.T) {
	var gotBody string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			b, _ := readBody(r)
			gotBody = b
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	p := config.CommandParams{DeviceID: "d1", PlaylistID: "pl123"}
	if err := runner.DispatchAction(p, "play", newClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:playlist:pl123") {
		t.Errorf("expected playlist URI in body, got: %q", gotBody)
	}
}

func TestDispatchAction_Play_WithTrack(t *testing.T) {
	var gotBody string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			b, _ := readBody(r)
			gotBody = b
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	p := config.CommandParams{DeviceID: "d1", TrackID: "tr456"}
	if err := runner.DispatchAction(p, "play", newClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:track:tr456") {
		t.Errorf("expected track URI in body, got: %q", gotBody)
	}
}

func TestDispatchAction_Play_WithAlbum(t *testing.T) {
	var gotBody string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			b, _ := readBody(r)
			gotBody = b
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	p := config.CommandParams{DeviceID: "d1", AlbumID: "al789"}
	if err := runner.DispatchAction(p, "play", newClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:album:al789") {
		t.Errorf("expected album URI in body, got: %q", gotBody)
	}
}

func TestDispatchAction_Play_WithArtist(t *testing.T) {
	var gotBody string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			b, _ := readBody(r)
			gotBody = b
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	p := config.CommandParams{DeviceID: "d1", ArtistID: "ar999"}
	if err := runner.DispatchAction(p, "play", newClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:artist:ar999") {
		t.Errorf("expected artist URI in body, got: %q", gotBody)
	}
}

func TestDispatchAction_Play_MultipleURIParamsError(t *testing.T) {
	p := config.CommandParams{PlaylistID: "pl1", TrackID: "tr1"}
	err := runner.DispatchAction(p, "play", nil, nil, 0)
	if err == nil || !strings.Contains(err.Error(), "only one of") {
		t.Fatalf("expected multiple URI error, got %v", err)
	}
}

// ----- DispatchAction: transfer ----------------------------------------------

func TestDispatchAction_Transfer_WithPlay(t *testing.T) {
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	play := true
	p := config.CommandParams{DeviceID: "d1", Play: &play}
	if err := runner.DispatchAction(p, "transfer", newClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ----- DispatchAction: shuffle -----------------------------------------------

func TestDispatchAction_Shuffle_Disabled(t *testing.T) {
	var gotState string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /shuffle": func(w http.ResponseWriter, r *http.Request) {
			gotState = r.URL.Query().Get("state")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	enabled := false
	p := config.CommandParams{DeviceID: "d1", Enabled: &enabled}
	if err := runner.DispatchAction(p, "shuffle", newClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "false" {
		t.Errorf("expected state=false, got %q", gotState)
	}
}

// ----- DispatchAction: repeat ------------------------------------------------

func TestDispatchAction_Repeat(t *testing.T) {
	var gotState, gotDevice string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /repeat": func(w http.ResponseWriter, r *http.Request) {
			gotState = r.URL.Query().Get("state")
			gotDevice = r.URL.Query().Get("device_id")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	p := config.CommandParams{DeviceID: "d1", RepeatState: "track"}
	if err := runner.DispatchAction(p, "repeat", newClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "track" {
		t.Errorf("expected state=track, got %q", gotState)
	}
	if gotDevice != "d1" {
		t.Errorf("expected device_id=d1, got %q", gotDevice)
	}
}

// ----- ResolveURIFromParams --------------------------------------------------

func TestResolveURIFromParams(t *testing.T) {
	cases := []struct {
		p       config.CommandParams
		want    string
		wantErr bool
	}{
		{config.CommandParams{}, "", false},
		{config.CommandParams{URI: "spotify:artist:abc"}, "spotify:artist:abc", false},
		{config.CommandParams{PlaylistID: "pl1"}, "spotify:playlist:pl1", false},
		{config.CommandParams{TrackID: "tr1"}, "spotify:track:tr1", false},
		{config.CommandParams{AlbumID: "al1"}, "spotify:album:al1", false},
		{config.CommandParams{ArtistID: "ar1"}, "spotify:artist:ar1", false},
		{config.CommandParams{PlaylistID: "pl1", TrackID: "tr1"}, "", true},
	}
	for _, tc := range cases {
		got, err := runner.ResolveURIFromParams(tc.p)
		if (err != nil) != tc.wantErr {
			t.Errorf("ResolveURIFromParams(%+v): err=%v wantErr=%v", tc.p, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("ResolveURIFromParams(%+v): got %q, want %q", tc.p, got, tc.want)
		}
	}
}

// ----- RunSet: set-level device propagation ----------------------------------

func TestRunSet_SetLevelDeviceApplied(t *testing.T) {
	var gotDeviceID string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) {
			gotDeviceID = r.URL.Query().Get("device_id")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{DeviceID: "set-device", Commands: []config.Command{{Action: "pause"}}}
	if err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDeviceID != "set-device" {
		t.Errorf("expected set-level device_id, got %q", gotDeviceID)
	}
}

func TestRunSet_CommandDeviceOverridesSet(t *testing.T) {
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
		Commands: []config.Command{
			{Action: "pause", Params: config.CommandParams{DeviceID: "cmd-device"}},
		},
	}
	if err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDeviceID != "cmd-device" {
		t.Errorf("expected command-level device_id to win, got %q", gotDeviceID)
	}
}

func TestRunSet_NoDeviceOmitsParam(t *testing.T) {
	var gotDeviceID string
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) {
			gotDeviceID = r.URL.Query().Get("device_id")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{{Action: "pause"}}}
	if err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDeviceID != "" {
		t.Errorf("expected no device_id in request, got %q", gotDeviceID)
	}
}

// ----- RunSet: basic execution -----------------------------------------------

func TestRunSet_PlayNoConfirm(t *testing.T) {
	playCalled := false
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			playCalled = true
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{
		{Action: "play", Params: config.CommandParams{DeviceID: "d1"}},
	}}
	if err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0); err != nil {
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
			w.Write(stateBody(spotify.PlaybackState{IsPlaying: true}))
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{
		{Action: "play", Params: config.CommandParams{DeviceID: "d1"}, Confirm: true, Timeout: "5s"},
	}}
	if err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pollCount < 1 {
		t.Error("expected at least one state poll")
	}
}

func TestRunSet_ConfirmTimeout_Continue(t *testing.T) {
	pauseCalled := false
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play":  func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /":      func(w http.ResponseWriter, r *http.Request) { w.Write(stateBody(spotify.PlaybackState{IsPlaying: false})) },
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCalled = true; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{
		OnTimeout: config.OnFailureContinue,
		Commands: []config.Command{
			{Action: "play", Params: config.CommandParams{DeviceID: "d1"}, Confirm: true, Timeout: "50ms"},
			{Action: "pause", Params: config.CommandParams{DeviceID: "d1"}},
		},
	}
	if err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !pauseCalled {
		t.Error("expected pause after timeout+continue")
	}
}

func TestRunSet_ConfirmTimeout_Fail(t *testing.T) {
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /":     func(w http.ResponseWriter, r *http.Request) { w.Write(stateBody(spotify.PlaybackState{IsPlaying: false})) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{
		{Action: "play", Params: config.CommandParams{DeviceID: "d1"}, Confirm: true, Timeout: "50ms", OnTimeout: config.OnFailureFail},
	}}
	err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestRunSet_ConfirmTimeout_SkipRemaining(t *testing.T) {
	pauseCalled := false
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play":  func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /":      func(w http.ResponseWriter, r *http.Request) { w.Write(stateBody(spotify.PlaybackState{IsPlaying: false})) },
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCalled = true; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{
		{Action: "play", Params: config.CommandParams{DeviceID: "d1"}, Confirm: true, Timeout: "50ms", OnTimeout: config.OnFailureSkipRemaining},
		{Action: "pause", Params: config.CommandParams{DeviceID: "d1"}},
	}}
	if err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if pauseCalled {
		t.Error("pause should not run after skip_remaining")
	}
}

// ----- RunSet: command errors ------------------------------------------------

func TestRunSet_CommandError_Continue(t *testing.T) {
	pauseCalled := false
	srv := mockServer(t, map[string]http.HandlerFunc{
		"POST /next": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCalled = true; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{
		OnError: config.OnFailureContinue,
		Commands: []config.Command{
			{Action: "next", Params: config.CommandParams{DeviceID: "d1"}},
			{Action: "pause", Params: config.CommandParams{DeviceID: "d1"}},
		},
	}
	if err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pauseCalled {
		t.Error("expected pause after next error with on_error:continue")
	}
}

func TestRunSet_CommandError_Fail(t *testing.T) {
	srv := mockServer(t, map[string]http.HandlerFunc{
		"POST /next": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{
		OnError: config.OnFailureFail,
		Commands: []config.Command{
			{Action: "next", Params: config.CommandParams{DeviceID: "d1"}},
		},
	}
	err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0)
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("expected abort error, got %v", err)
	}
}

func TestRunSet_CommandError_SkipRemaining(t *testing.T) {
	pauseCalled := false
	srv := mockServer(t, map[string]http.HandlerFunc{
		"POST /next": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCalled = true; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{
		OnError: config.OnFailureSkipRemaining,
		Commands: []config.Command{
			{Action: "next", Params: config.CommandParams{DeviceID: "d1"}},
			{Action: "pause", Params: config.CommandParams{DeviceID: "d1"}},
		},
	}
	if err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if pauseCalled {
		t.Error("pause should not run after skip_remaining")
	}
}

func TestRunSet_CommandOverridesSetDefault(t *testing.T) {
	srv := mockServer(t, map[string]http.HandlerFunc{
		"POST /next": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{
		OnError: config.OnFailureContinue,
		Commands: []config.Command{
			{Action: "next", Params: config.CommandParams{DeviceID: "d1"}, OnError: config.OnFailureFail},
		},
	}
	err := runner.RunSet("test", set, newCfg(nil), newClient(t, srv), 0)
	if err == nil {
		t.Fatal("expected error when command overrides with on_error:fail")
	}
}

// ----- RunSet: sleep & composition -------------------------------------------

func TestRunSet_Sleep(t *testing.T) {
	set := config.Set{Commands: []config.Command{
		{Action: "sleep", Params: config.CommandParams{Duration: "20ms"}},
	}}
	start := time.Now()
	if err := runner.RunSet("test", set, newCfg(nil), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Error("sleep command did not sleep long enough")
	}
}

func TestRunSet_Composable(t *testing.T) {
	pauseCount := 0
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCount++; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	inner := config.Set{Commands: []config.Command{
		{Action: "pause", Params: config.CommandParams{DeviceID: "d1"}},
	}}
	outer := config.Set{Commands: []config.Command{
		{Action: "run_set", Params: config.CommandParams{Set: "inner"}},
		{Action: "run_set", Params: config.CommandParams{Set: "inner"}},
	}}
	if err := runner.RunSet("outer", outer, newCfg(map[string]config.Set{"inner": inner}), newClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pauseCount != 2 {
		t.Errorf("expected inner set to run twice, got %d", pauseCount)
	}
}

func TestRunSet_MaxDepth(t *testing.T) {
	sets := map[string]config.Set{
		"self": {Commands: []config.Command{
			{Action: "run_set", Params: config.CommandParams{Set: "self"}},
		}},
	}
	cfg := newCfg(sets)
	err := runner.RunSet("self", cfg.Sets["self"], cfg, nil, runner.MaxSetDepth)
	if err == nil || !strings.Contains(err.Error(), "recursion depth exceeded") {
		t.Fatalf("expected depth-exceeded error, got %v", err)
	}
}

func TestRunSet_UnknownNestedSet(t *testing.T) {
	set := config.Set{
		OnError: config.OnFailureFail,
		Commands: []config.Command{
			{Action: "run_set", Params: config.CommandParams{Set: "does-not-exist"}},
		},
	}
	err := runner.RunSet("outer", set, newCfg(nil), nil, 0)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// ----- Confirmed -------------------------------------------------------------

func TestConfirmed_Play(t *testing.T) {
	cmd := config.Command{Action: "play"}
	if !runner.Confirmed(cmd, &spotify.PlaybackState{IsPlaying: true}, "") {
		t.Error("expected confirmed when is_playing=true")
	}
	if runner.Confirmed(cmd, &spotify.PlaybackState{IsPlaying: false}, "") {
		t.Error("expected not confirmed when is_playing=false")
	}
}

func TestConfirmed_Pause(t *testing.T) {
	cmd := config.Command{Action: "pause"}
	if !runner.Confirmed(cmd, &spotify.PlaybackState{IsPlaying: false}, "") {
		t.Error("expected confirmed when not playing")
	}
	if runner.Confirmed(cmd, &spotify.PlaybackState{IsPlaying: true}, "") {
		t.Error("expected not confirmed when playing")
	}
}

func TestConfirmed_Shuffle(t *testing.T) {
	enabled := true
	cmd := config.Command{Action: "shuffle", Params: config.CommandParams{Enabled: &enabled}}
	if !runner.Confirmed(cmd, &spotify.PlaybackState{ShuffleState: true}, "") {
		t.Error("expected confirmed when shuffle_state=true")
	}
	if runner.Confirmed(cmd, &spotify.PlaybackState{ShuffleState: false}, "") {
		t.Error("expected not confirmed when shuffle_state=false")
	}
}

func TestConfirmed_Repeat(t *testing.T) {
	cmd := config.Command{Action: "repeat", Params: config.CommandParams{RepeatState: "context"}}
	if !runner.Confirmed(cmd, &spotify.PlaybackState{RepeatState: "context"}, "") {
		t.Error("expected confirmed when repeat_state matches")
	}
	if runner.Confirmed(cmd, &spotify.PlaybackState{RepeatState: "off"}, "") {
		t.Error("expected not confirmed when repeat_state differs")
	}
}

func TestConfirmed_Volume(t *testing.T) {
	level := 42
	cmd := config.Command{Action: "volume", Params: config.CommandParams{Level: &level}}
	if !runner.Confirmed(cmd, &spotify.PlaybackState{Device: spotify.Device{VolumePercent: 42}}, "") {
		t.Error("expected confirmed when volume matches exactly")
	}
	if !runner.Confirmed(cmd, &spotify.PlaybackState{Device: spotify.Device{VolumePercent: 41}}, "") {
		t.Error("expected confirmed when volume is within ±1 (41)")
	}
	if !runner.Confirmed(cmd, &spotify.PlaybackState{Device: spotify.Device{VolumePercent: 43}}, "") {
		t.Error("expected confirmed when volume is within ±1 (43)")
	}
	if runner.Confirmed(cmd, &spotify.PlaybackState{Device: spotify.Device{VolumePercent: 30}}, "") {
		t.Error("expected not confirmed when volume differs by more than 1")
	}
}

func TestConfirmed_Volume_NilLevel(t *testing.T) {
	cmd := config.Command{Action: "volume", Params: config.CommandParams{Level: nil}}
	if !runner.Confirmed(cmd, &spotify.PlaybackState{}, "") {
		t.Error("expected confirmed when level is nil")
	}
}

func TestConfirmed_Transfer(t *testing.T) {
	cmd := config.Command{Action: "transfer", Params: config.CommandParams{DeviceID: "target"}}
	if !runner.Confirmed(cmd, &spotify.PlaybackState{Device: spotify.Device{ID: "target"}}, "") {
		t.Error("expected confirmed when active device matches")
	}
	if runner.Confirmed(cmd, &spotify.PlaybackState{Device: spotify.Device{ID: "other"}}, "") {
		t.Error("expected not confirmed when active device differs")
	}
}

func TestConfirmed_Next(t *testing.T) {
	cmd := config.Command{Action: "next"}
	priorURI := "spotify:track:aaaa"
	state := &spotify.PlaybackState{Item: &spotify.Track{URI: "spotify:track:bbbb"}}
	if !runner.Confirmed(cmd, state, priorURI) {
		t.Error("expected confirmed when track URI changed")
	}
	state.Item.URI = priorURI
	if runner.Confirmed(cmd, state, priorURI) {
		t.Error("expected not confirmed when track URI unchanged")
	}
}

func TestConfirmed_Previous(t *testing.T) {
	cmd := config.Command{Action: "previous"}
	priorURI := "spotify:track:cccc"
	state := &spotify.PlaybackState{Item: &spotify.Track{URI: "spotify:track:dddd"}}
	if !runner.Confirmed(cmd, state, priorURI) {
		t.Error("expected confirmed when track URI changed")
	}
	state.Item.URI = priorURI
	if runner.Confirmed(cmd, state, priorURI) {
		t.Error("expected not confirmed when track URI unchanged")
	}
}

func TestConfirmed_NilState(t *testing.T) {
	cmd := config.Command{Action: "play"}
	if runner.Confirmed(cmd, nil, "") {
		t.Error("expected not confirmed for nil state")
	}
}

func TestConfirmed_NextNilItem(t *testing.T) {
	cmd := config.Command{Action: "next"}
	if runner.Confirmed(cmd, &spotify.PlaybackState{Item: nil}, "spotify:track:aaaa") {
		t.Error("expected not confirmed when item is nil")
	}
}

func TestConfirmed_RunSet_DefaultTrue(t *testing.T) {
	cmd := config.Command{Action: "run_set"}
	if !runner.Confirmed(cmd, &spotify.PlaybackState{}, "") {
		t.Error("expected confirmed=true for run_set (default branch)")
	}
}

// ----- ExecuteCommand: prior track URI snapshot ------------------------------

func TestExecuteCommand_Next_SnapshotsURI(t *testing.T) {
	callCount := 0
	srv := mockServer(t, map[string]http.HandlerFunc{
		"GET /": func(w http.ResponseWriter, r *http.Request) {
			callCount++
			uri := "spotify:track:before"
			if callCount > 1 {
				uri = "spotify:track:after"
			}
			w.Write(stateBody(spotify.PlaybackState{
				IsPlaying: true,
				Item:      &spotify.Track{URI: uri},
			}))
		},
		"POST /next": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	cmd := config.Command{
		Action:  "next",
		Params:  config.CommandParams{DeviceID: "d1"},
		Confirm: true,
		Timeout: "5s",
	}
	if err := runner.ExecuteCommand(cmd, newCfg(nil), newClient(t, srv), config.Set{}, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected snapshot call + at least one confirmation poll, got %d GET calls", callCount)
	}
}

func TestExecuteCommand_ConfirmPollError(t *testing.T) {
	pollCount := 0
	srv := mockServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /": func(w http.ResponseWriter, r *http.Request) {
			pollCount++
			if pollCount < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Write(stateBody(spotify.PlaybackState{IsPlaying: true}))
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	cmd := config.Command{
		Action:  "play",
		Params:  config.CommandParams{DeviceID: "d1"},
		Confirm: true,
		Timeout: "5s",
	}
	if err := runner.ExecuteCommand(cmd, newCfg(nil), newClient(t, srv), config.Set{}, 0); err != nil {
		t.Fatalf("expected success after transient poll errors, got %v", err)
	}
	if pollCount < 3 {
		t.Errorf("expected at least 3 polls, got %d", pollCount)
	}
}

// ----- error types -----------------------------------------------------------

func TestDepthExceededError_Message(t *testing.T) {
	e := &runner.DepthExceededError{}
	if !strings.Contains(e.Error(), "recursion depth exceeded") {
		t.Errorf("unexpected message: %q", e.Error())
	}
}

func TestCommandTimeoutError_Message(t *testing.T) {
	e := &runner.CommandTimeoutError{Timeout: 5 * time.Second, Action: "play"}
	if !strings.Contains(e.Error(), "play") || !strings.Contains(e.Error(), "5s") {
		t.Errorf("unexpected message: %q", e.Error())
	}
}

// ----- CommandLabel ----------------------------------------------------------

func TestCommandLabel_Play_URI(t *testing.T) {
	c := config.Command{Action: "play", Params: config.CommandParams{URI: "spotify:album:xyz"}}
	got := runner.CommandLabel(1, c)
	if !strings.Contains(got, "play") || !strings.Contains(got, "uri=spotify:album:xyz") {
		t.Errorf("unexpected label: %q", got)
	}
}

func TestCommandLabel_Volume(t *testing.T) {
	level := 75
	c := config.Command{Action: "volume", Params: config.CommandParams{Level: &level}}
	got := runner.CommandLabel(1, c)
	if !strings.Contains(got, "level=75") {
		t.Errorf("unexpected label: %q", got)
	}
}

func TestCommandLabel_Sleep(t *testing.T) {
	c := config.Command{Action: "sleep", Params: config.CommandParams{Duration: "30s"}}
	got := runner.CommandLabel(1, c)
	if !strings.Contains(got, "duration=30s") {
		t.Errorf("unexpected label: %q", got)
	}
}

func TestCommandLabel_WithName(t *testing.T) {
	c := config.Command{Action: "pause", Name: "bedtime pause"}
	got := runner.CommandLabel(1, c)
	if !strings.Contains(got, "bedtime pause") {
		t.Errorf("expected name in label, got: %q", got)
	}
}

func TestCommandLabel_Confirm(t *testing.T) {
	c := config.Command{Action: "play", Confirm: true}
	got := runner.CommandLabel(1, c)
	if !strings.Contains(got, "confirm") {
		t.Errorf("expected 'confirm' in label, got: %q", got)
	}
}

// ----- errors.As compatibility -----------------------------------------------

func TestDepthExceededError_As(t *testing.T) {
	err := runner.RunSet("self", config.Set{Commands: []config.Command{
		{Action: "run_set", Params: config.CommandParams{Set: "self"}},
	}}, newCfg(map[string]config.Set{
		"self": {Commands: []config.Command{
			{Action: "run_set", Params: config.CommandParams{Set: "self"}},
		}},
	}), nil, runner.MaxSetDepth)

	var depthErr *runner.DepthExceededError
	if !errors.As(err, &depthErr) {
		t.Errorf("expected DepthExceededError via errors.As, got %T: %v", err, err)
	}
}
