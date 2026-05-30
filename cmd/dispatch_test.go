package cmd

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// ----- newClientFromConfig ---------------------------------------------------

func TestNewClientFromConfig_TokenRefreshFailure(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
	}()

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) {
		return "", errors.New("token refresh failed")
	}

	_, err := newClientFromConfig()
	if err == nil || !strings.Contains(err.Error(), "failed to refresh token") {
		t.Fatalf("expected token refresh error, got %v", err)
	}
}

func TestNewClientFromConfig_ConfigLoadFailure(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	configPath = "/nonexistent/path/config.yaml"
	_, err := newClientFromConfig()
	if err == nil || !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("expected config load error, got %v", err)
	}
}

// ----- dispatchAction: sleep -------------------------------------------------

func TestDispatchAction_Sleep(t *testing.T) {
	p := config.CommandParams{Duration: "20ms"}
	start := time.Now()
	if err := dispatchAction(p, "sleep", nil, setCfg(nil), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Error("sleep did not sleep long enough")
	}
}

// ----- dispatchAction: run_set -----------------------------------------------

func TestDispatchAction_RunSet_Success(t *testing.T) {
	pauseCount := 0
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { pauseCount++; w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	inner := config.Set{
		Commands: []config.Command{
			{Action: "pause", Params: config.CommandParams{DeviceID: "d1"}},
		},
	}
	cfg := setCfg(map[string]config.Set{"inner": inner})
	p := config.CommandParams{Set: "inner"}

	if err := dispatchAction(p, "run_set", setClient(t, srv), cfg, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pauseCount != 1 {
		t.Errorf("expected inner set to run once, got pause count=%d", pauseCount)
	}
}

func TestDispatchAction_RunSet_NotFound(t *testing.T) {
	p := config.CommandParams{Set: "does-not-exist"}
	err := dispatchAction(p, "run_set", nil, setCfg(nil), 0)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestDispatchAction_UnknownAction(t *testing.T) {
	err := dispatchAction(config.CommandParams{}, "bogus", nil, setCfg(nil), 0)
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("expected unknown action error, got %v", err)
	}
}

// ----- dispatchAction: play variants -----------------------------------------

func TestDispatchAction_Play_WithPlaylist(t *testing.T) {
	var gotBody string
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			b, _ := readBody(r)
			gotBody = b
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	p := config.CommandParams{DeviceID: "d1", PlaylistID: "pl123"}
	if err := dispatchAction(p, "play", setClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:playlist:pl123") {
		t.Errorf("expected playlist URI in body, got: %q", gotBody)
	}
}

func TestDispatchAction_Play_WithTrack(t *testing.T) {
	var gotBody string
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			b, _ := readBody(r)
			gotBody = b
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	p := config.CommandParams{DeviceID: "d1", TrackID: "tr456"}
	if err := dispatchAction(p, "play", setClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:track:tr456") {
		t.Errorf("expected track URI in body, got: %q", gotBody)
	}
}

func TestDispatchAction_Play_WithAlbum(t *testing.T) {
	var gotBody string
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			b, _ := readBody(r)
			gotBody = b
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	p := config.CommandParams{DeviceID: "d1", AlbumID: "al789"}
	if err := dispatchAction(p, "play", setClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:album:al789") {
		t.Errorf("expected album URI in body, got: %q", gotBody)
	}
}

func TestDispatchAction_Play_WithArtist(t *testing.T) {
	var gotBody string
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /play": func(w http.ResponseWriter, r *http.Request) {
			b, _ := readBody(r)
			gotBody = b
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	p := config.CommandParams{DeviceID: "d1", ArtistID: "ar999"}
	if err := dispatchAction(p, "play", setClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "spotify:artist:ar999") {
		t.Errorf("expected artist URI in body, got: %q", gotBody)
	}
}

func TestDispatchAction_Play_MultipleURIParamsError(t *testing.T) {
	p := config.CommandParams{PlaylistID: "pl1", TrackID: "tr1"}
	err := dispatchAction(p, "play", nil, nil, 0)
	if err == nil || !strings.Contains(err.Error(), "only one of") {
		t.Fatalf("expected multiple URI error, got %v", err)
	}
}

// ----- dispatchAction: transfer ----------------------------------------------

func TestDispatchAction_Transfer_WithPlay(t *testing.T) {
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
	})
	defer srv.Close()
	useTestServer(t, srv)

	play := true
	p := config.CommandParams{DeviceID: "d1", Play: &play}
	if err := dispatchAction(p, "transfer", setClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ----- dispatchAction: shuffle -----------------------------------------------

func TestDispatchAction_Shuffle_Disabled(t *testing.T) {
	var gotState string
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /shuffle": func(w http.ResponseWriter, r *http.Request) {
			gotState = r.URL.Query().Get("state")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	enabled := false
	p := config.CommandParams{DeviceID: "d1", Enabled: &enabled}
	if err := dispatchAction(p, "shuffle", setClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "false" {
		t.Errorf("expected state=false, got %q", gotState)
	}
}

// ----- helper ----------------------------------------------------------------

func readBody(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	b, err := io.ReadAll(r.Body)
	return string(b), err
}
