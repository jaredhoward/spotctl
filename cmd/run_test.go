package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// ----- small helpers ---------------------------------------------------------

func intPtr(i int) *int    { return &i }
func boolPtr(b bool) *bool { return &b }

// setServer creates an httptest.Server whose mux maps "METHOD /path" strings
// to handler functions. Unmatched requests are logged and return 404.
func setServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if fn, ok := handlers[key]; ok {
			fn(w, r)
			return
		}
		t.Logf("unhandled request in setServer: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
}

// setClient builds a Spotify client pointed at srv with a dummy token.
func setClient(t *testing.T, srv *httptest.Server) *spotify.Client {
	t.Helper()
	c := spotify.NewClient("test-token")
	c.SetHTTPClient(srv.Client())
	return c
}

// setCfg builds a minimal *config.Config with the given sets.
func setCfg(sets map[string]config.Set) *config.Config {
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

// useTestServer points URLPlayer at srv for the duration of the test and
// restores the original value afterwards.
func useTestServer(t *testing.T, srv *httptest.Server) {
	t.Helper()
	original := spotify.URLPlayer
	spotify.URLPlayer = srv.URL
	t.Cleanup(func() { spotify.URLPlayer = original })
}

// ----- CommandParams.Validate ------------------------------------------------

func TestCommandParamsValidate(t *testing.T) {
	cases := []struct {
		action  string
		params  config.CommandParams
		wantErr bool
	}{
		{"play", config.CommandParams{}, false},
		{"play", config.CommandParams{DeviceID: "d1"}, false},
		{"pause", config.CommandParams{}, false},
		{"pause", config.CommandParams{DeviceID: "d1"}, false},
		{"next", config.CommandParams{}, false},
		{"next", config.CommandParams{DeviceID: "d1"}, false},
		{"previous", config.CommandParams{}, false},
		{"previous", config.CommandParams{DeviceID: "d1"}, false},
		{"shuffle", config.CommandParams{}, false},
		{"shuffle", config.CommandParams{DeviceID: "d1"}, false},
		{"transfer", config.CommandParams{}, false},
		{"transfer", config.CommandParams{DeviceID: "d1"}, false},
		{"volume", config.CommandParams{Level: intPtr(50)}, false},
		{"volume", config.CommandParams{DeviceID: "d1", Level: intPtr(50)}, false},
		{"volume", config.CommandParams{}, true},
		{"sleep", config.CommandParams{Duration: "2s"}, false},
		{"sleep", config.CommandParams{Duration: "bad"}, true},
		{"sleep", config.CommandParams{}, true},
		{"run_set", config.CommandParams{Set: "s"}, false},
		{"run_set", config.CommandParams{}, true},
		{"unknown", config.CommandParams{}, true},
	}
	for _, tc := range cases {
		err := tc.params.Validate(tc.action)
		if (err != nil) != tc.wantErr {
			t.Errorf("Validate(%q): got err=%v, wantErr=%v", tc.action, err, tc.wantErr)
		}
	}
}

// ----- Command helper methods ------------------------------------------------

func TestCommandEffectiveOnError(t *testing.T) {
	cmd := config.Command{}
	if got := cmd.EffectiveOnError(""); got != config.OnFailureContinue {
		t.Errorf("expected continue, got %q", got)
	}
	if got := cmd.EffectiveOnError(config.OnFailureFail); got != config.OnFailureFail {
		t.Errorf("expected fail, got %q", got)
	}
	cmd.OnError = config.OnFailureSkipRemaining
	if got := cmd.EffectiveOnError(config.OnFailureFail); got != config.OnFailureSkipRemaining {
		t.Errorf("expected skip_remaining, got %q", got)
	}
}

func TestCommandTimeoutDuration(t *testing.T) {
	cmd := config.Command{}
	if got := cmd.TimeoutDuration(5 * time.Second); got != 5*time.Second {
		t.Errorf("expected default 5s, got %v", got)
	}
	cmd.Timeout = "3s"
	if got := cmd.TimeoutDuration(5 * time.Second); got != 3*time.Second {
		t.Errorf("expected 3s, got %v", got)
	}
	cmd.Timeout = "bad"
	if got := cmd.TimeoutDuration(5 * time.Second); got != 5*time.Second {
		t.Errorf("expected fallback 5s for bad duration, got %v", got)
	}
}

// ----- ResolvedDeviceID ------------------------------------------------------

func TestResolvedDeviceID(t *testing.T) {
	cmd := config.Command{Params: config.CommandParams{DeviceID: "cmd-dev"}}
	if got := cmd.ResolvedDeviceID("set-dev"); got != "cmd-dev" {
		t.Errorf("expected cmd-dev, got %q", got)
	}
	cmd.Params.DeviceID = ""
	if got := cmd.ResolvedDeviceID("set-dev"); got != "set-dev" {
		t.Errorf("expected set-dev, got %q", got)
	}
	if got := cmd.ResolvedDeviceID(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ----- sentinel errors -------------------------------------------------------

func TestSkipRemainingError_Message(t *testing.T) {
	e := &skipRemainingError{}
	if e.Error() != "skip_remaining" {
		t.Errorf("unexpected message: %q", e.Error())
	}
}

func TestDepthExceededError_Message(t *testing.T) {
	e := &depthExceededError{}
	if !strings.Contains(e.Error(), "recursion depth exceeded") {
		t.Errorf("unexpected message: %q", e.Error())
	}
}

func TestCommandTimeoutError_Message(t *testing.T) {
	e := &commandTimeoutError{timeout: 5 * time.Second, action: "play"}
	if !strings.Contains(e.Error(), "play") || !strings.Contains(e.Error(), "5s") {
		t.Errorf("unexpected message: %q", e.Error())
	}
}

// ----- Set-level device propagation ------------------------------------------

func TestRunSet_SetLevelDeviceApplied(t *testing.T) {
	var gotDeviceID string
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) {
			gotDeviceID = r.URL.Query().Get("device_id")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{DeviceID: "set-device", Commands: []config.Command{{Action: "pause"}}}
	if err := runSet("test", set, setCfg(nil), setClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDeviceID != "set-device" {
		t.Errorf("expected set-level device_id, got %q", gotDeviceID)
	}
}

func TestRunSet_CommandDeviceOverridesSet(t *testing.T) {
	var gotDeviceID string
	srv := setServer(t, map[string]http.HandlerFunc{
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
	if err := runSet("test", set, setCfg(nil), setClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDeviceID != "cmd-device" {
		t.Errorf("expected command-level device_id to win, got %q", gotDeviceID)
	}
}

func TestRunSet_NoDeviceOmitsParam(t *testing.T) {
	var gotDeviceID string
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) {
			gotDeviceID = r.URL.Query().Get("device_id")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{{Action: "pause"}}}
	if err := runSet("test", set, setCfg(nil), setClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDeviceID != "" {
		t.Errorf("expected no device_id in request, got %q", gotDeviceID)
	}
}

// ----- confirmed() -----------------------------------------------------------

func TestConfirmed_Play(t *testing.T) {
	cmd := config.Command{Action: "play"}
	if !confirmed(cmd, &spotify.PlaybackState{IsPlaying: true}, "") {
		t.Error("expected confirmed when is_playing=true")
	}
	if confirmed(cmd, &spotify.PlaybackState{IsPlaying: false}, "") {
		t.Error("expected not confirmed when is_playing=false")
	}
}

func TestConfirmed_Pause(t *testing.T) {
	cmd := config.Command{Action: "pause"}
	if !confirmed(cmd, &spotify.PlaybackState{IsPlaying: false}, "") {
		t.Error("expected confirmed when not playing")
	}
	if confirmed(cmd, &spotify.PlaybackState{IsPlaying: true}, "") {
		t.Error("expected not confirmed when playing")
	}
}

func TestConfirmed_Shuffle(t *testing.T) {
	enabled := true
	cmd := config.Command{Action: "shuffle", Params: config.CommandParams{Enabled: &enabled}}
	if !confirmed(cmd, &spotify.PlaybackState{ShuffleState: true}, "") {
		t.Error("expected confirmed when shuffle_state=true")
	}
	if confirmed(cmd, &spotify.PlaybackState{ShuffleState: false}, "") {
		t.Error("expected not confirmed when shuffle_state=false")
	}
}

func TestConfirmed_Volume(t *testing.T) {
	level := 42
	cmd := config.Command{Action: "volume", Params: config.CommandParams{Level: &level}}
	if !confirmed(cmd, &spotify.PlaybackState{Device: spotify.Device{VolumePercent: 42}}, "") {
		t.Error("expected confirmed when volume matches")
	}
	if confirmed(cmd, &spotify.PlaybackState{Device: spotify.Device{VolumePercent: 30}}, "") {
		t.Error("expected not confirmed when volume differs")
	}
}

func TestConfirmed_Volume_NilLevel(t *testing.T) {
	// nil Level → confirmed immediately (nothing to check)
	cmd := config.Command{Action: "volume", Params: config.CommandParams{Level: nil}}
	if !confirmed(cmd, &spotify.PlaybackState{}, "") {
		t.Error("expected confirmed when level is nil")
	}
}

func TestConfirmed_Transfer(t *testing.T) {
	cmd := config.Command{Action: "transfer", Params: config.CommandParams{DeviceID: "target"}}
	if !confirmed(cmd, &spotify.PlaybackState{Device: spotify.Device{ID: "target"}}, "") {
		t.Error("expected confirmed when active device matches")
	}
	if confirmed(cmd, &spotify.PlaybackState{Device: spotify.Device{ID: "other"}}, "") {
		t.Error("expected not confirmed when active device differs")
	}
}

func TestConfirmed_Next(t *testing.T) {
	cmd := config.Command{Action: "next"}
	priorURI := "spotify:track:aaaa"
	state := &spotify.PlaybackState{Item: &spotify.Track{URI: "spotify:track:bbbb"}}
	if !confirmed(cmd, state, priorURI) {
		t.Error("expected confirmed when track URI changed")
	}
	state.Item.URI = priorURI
	if confirmed(cmd, state, priorURI) {
		t.Error("expected not confirmed when track URI unchanged")
	}
}

func TestConfirmed_Previous(t *testing.T) {
	cmd := config.Command{Action: "previous"}
	priorURI := "spotify:track:cccc"
	state := &spotify.PlaybackState{Item: &spotify.Track{URI: "spotify:track:dddd"}}
	if !confirmed(cmd, state, priorURI) {
		t.Error("expected confirmed when track URI changed")
	}
	state.Item.URI = priorURI
	if confirmed(cmd, state, priorURI) {
		t.Error("expected not confirmed when track URI unchanged")
	}
}

func TestConfirmed_NilState(t *testing.T) {
	cmd := config.Command{Action: "play"}
	if confirmed(cmd, nil, "") {
		t.Error("expected not confirmed for nil state")
	}
}

func TestConfirmed_NextNilItem(t *testing.T) {
	cmd := config.Command{Action: "next"}
	if confirmed(cmd, &spotify.PlaybackState{Item: nil}, "spotify:track:aaaa") {
		t.Error("expected not confirmed when item is nil")
	}
}

func TestConfirmed_RunSet_DefaultTrue(t *testing.T) {
	// run_set and any unrecognised action hit the default branch → true
	cmd := config.Command{Action: "run_set"}
	if !confirmed(cmd, &spotify.PlaybackState{}, "") {
		t.Error("expected confirmed=true for run_set (default branch)")
	}
}

// ----- executeCommand: prior track URI snapshot ------------------------------

// TestExecuteCommand_Next_SnapshotsURI verifies that when confirm:true is set
// on a next command, executeCommand snapshots the current track URI before
// dispatching and uses it for confirmation polling.
func TestExecuteCommand_Next_SnapshotsURI(t *testing.T) {
	callCount := 0
	srv := setServer(t, map[string]http.HandlerFunc{
		// First GET: current state (prior track)
		// Subsequent GETs: confirmation polls (new track)
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
	set := config.Set{}
	if err := executeCommand(cmd, setCfg(nil), setClient(t, srv), set, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected snapshot call + at least one confirmation poll, got %d GET calls", callCount)
	}
}

// TestExecuteCommand_ConfirmPollError verifies that a transient state poll
// error is tolerated and polling continues until success or timeout.
func TestExecuteCommand_ConfirmPollError(t *testing.T) {
	pollCount := 0
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /": func(w http.ResponseWriter, r *http.Request) {
			pollCount++
			if pollCount < 3 {
				// Return a non-200/204 to simulate a transient error.
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			// Third poll succeeds.
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
	if err := executeCommand(cmd, setCfg(nil), setClient(t, srv), config.Set{}, 0); err != nil {
		t.Fatalf("expected success after transient poll errors, got %v", err)
	}
	if pollCount < 3 {
		t.Errorf("expected at least 3 polls, got %d", pollCount)
	}
}

// ----- resolveURIFromParams --------------------------------------------------

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
		{config.CommandParams{PlaylistID: "pl1", TrackID: "tr1"}, "", true},
	}
	for _, tc := range cases {
		got, err := resolveURIFromParams(tc.p)
		if (err != nil) != tc.wantErr {
			t.Errorf("resolveURIFromParams(%+v): err=%v wantErr=%v", tc.p, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveURIFromParams(%+v): got %q, want %q", tc.p, got, tc.want)
		}
	}
}

// ----- runSet: basic execution -----------------------------------------------

func TestRunSet_PlayNoConfirm(t *testing.T) {
	playCalled := false
	srv := setServer(t, map[string]http.HandlerFunc{
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
	if err := runSet("test", set, setCfg(nil), setClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !playCalled {
		t.Error("expected play to be called")
	}
}

func TestRunSet_PlayAndConfirm(t *testing.T) {
	pollCount := 0
	srv := setServer(t, map[string]http.HandlerFunc{
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
	if err := runSet("test", set, setCfg(nil), setClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pollCount < 1 {
		t.Error("expected at least one state poll")
	}
}

func TestRunSet_ConfirmTimeout_Continue(t *testing.T) {
	pauseCalled := false
	srv := setServer(t, map[string]http.HandlerFunc{
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
	if err := runSet("test", set, setCfg(nil), setClient(t, srv), 0); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !pauseCalled {
		t.Error("expected pause after timeout+continue")
	}
}

func TestRunSet_ConfirmTimeout_Fail(t *testing.T) {
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /":     func(w http.ResponseWriter, r *http.Request) { w.Write(stateBody(spotify.PlaybackState{IsPlaying: false})) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	set := config.Set{Commands: []config.Command{
		{Action: "play", Params: config.CommandParams{DeviceID: "d1"}, Confirm: true, Timeout: "50ms", OnTimeout: config.OnFailureFail},
	}}
	err := runSet("test", set, setCfg(nil), setClient(t, srv), 0)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestRunSet_ConfirmTimeout_SkipRemaining(t *testing.T) {
	pauseCalled := false
	srv := setServer(t, map[string]http.HandlerFunc{
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
	if err := runSet("test", set, setCfg(nil), setClient(t, srv), 0); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if pauseCalled {
		t.Error("pause should not run after skip_remaining")
	}
}

// ----- runSet: command errors ------------------------------------------------

func TestRunSet_CommandError_Continue(t *testing.T) {
	pauseCalled := false
	srv := setServer(t, map[string]http.HandlerFunc{
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
	if err := runSet("test", set, setCfg(nil), setClient(t, srv), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pauseCalled {
		t.Error("expected pause after next error with on_error:continue")
	}
}

func TestRunSet_CommandError_Fail(t *testing.T) {
	srv := setServer(t, map[string]http.HandlerFunc{
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
	err := runSet("test", set, setCfg(nil), setClient(t, srv), 0)
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("expected abort error, got %v", err)
	}
}

func TestRunSet_CommandError_SkipRemaining(t *testing.T) {
	pauseCalled := false
	srv := setServer(t, map[string]http.HandlerFunc{
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
	if err := runSet("test", set, setCfg(nil), setClient(t, srv), 0); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if pauseCalled {
		t.Error("pause should not run after skip_remaining")
	}
}

func TestRunSet_CommandOverridesSetDefault(t *testing.T) {
	srv := setServer(t, map[string]http.HandlerFunc{
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
	err := runSet("test", set, setCfg(nil), setClient(t, srv), 0)
	if err == nil {
		t.Fatal("expected error when command overrides with on_error:fail")
	}
}

// ----- runSet: sleep & composition -------------------------------------------

func TestRunSet_Sleep(t *testing.T) {
	set := config.Set{Commands: []config.Command{
		{Action: "sleep", Params: config.CommandParams{Duration: "20ms"}},
	}}
	start := time.Now()
	if err := runSet("test", set, setCfg(nil), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Error("sleep command did not sleep long enough")
	}
}

func TestRunSet_Composable(t *testing.T) {
	pauseCount := 0
	srv := setServer(t, map[string]http.HandlerFunc{
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
	if err := runSet("outer", outer, setCfg(map[string]config.Set{"inner": inner}), setClient(t, srv), 0); err != nil {
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
	cfg := setCfg(sets)
	err := runSet("self", cfg.Sets["self"], cfg, nil, maxSetDepth)
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
	err := runSet("outer", set, setCfg(nil), nil, 0)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// ----- handleCommandError ----------------------------------------------------

func TestHandleCommandError_Fail(t *testing.T) {
	err := handleCommandError("set", "cmd", errors.New("boom"), config.OnFailureFail)
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected abort error, got %v", err)
	}
}

func TestHandleCommandError_Continue(t *testing.T) {
	if err := handleCommandError("set", "cmd", errors.New("boom"), config.OnFailureContinue); err != nil {
		t.Errorf("expected nil for continue, got %v", err)
	}
}

func TestHandleCommandError_SkipRemaining(t *testing.T) {
	err := handleCommandError("set", "cmd", errors.New("boom"), config.OnFailureSkipRemaining)
	var skipErr *skipRemainingError
	if !errors.As(err, &skipErr) {
		t.Errorf("expected skipRemainingError, got %v", err)
	}
}
