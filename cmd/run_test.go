package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// ----- small helpers used across cmd tests -----------------------------------

// writeTempConfig saves cfg to a temp file and returns its path.
func writeTempConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()
	file, err := os.CreateTemp("", "spotctl-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	t.Cleanup(func() { os.Remove(file.Name()) })
	if err := config.Save(file.Name(), cfg); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}

// mockSpotifyServer creates an httptest.Server whose mux maps "METHOD /path"
// strings to handler functions. Unmatched requests are logged and return 404.
func mockSpotifyServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
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

// wireClient sets up newSpotifyClient to use srv's HTTP client and player URL,
// and installs a no-op token refresh. Cleanup is registered via t.Cleanup so
// it runs even if the test panics, before the server closes.
func wireClient(t *testing.T, srv *httptest.Server) (cleanup func()) {
	t.Helper()
	oldNewClient := newSpotifyClient
	oldRefresh := spotify.RefreshAccessToken

	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		c.SetPlayerURL(srv.URL)
		return c
	}
	spotify.RefreshAccessToken = func(_ context.Context, _, _, _ string) (spotify.RefreshResult, error) {
		return spotify.RefreshResult{AccessToken: "token"}, nil
	}

	restore := func() {
		newSpotifyClient = oldNewClient
		spotify.RefreshAccessToken = oldRefresh
	}
	t.Cleanup(restore)
	return restore
}

// ----- run command: integration ----------------------------------------------

func TestRunCmd_SetNotFound(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	configPath = writeTempConfig(t, &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
	})

	err := runCmd.RunE(runCmd, []string{"nonexistent"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRunCmd_Success(t *testing.T) {
	srv := mockSpotifyServer(t, map[string]http.HandlerFunc{
		"PUT /pause": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"GET /": func(w http.ResponseWriter, r *http.Request) {
			// Confirmation poll: return paused state so Pause.Confirmed resolves.
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"is_playing":false}`))
		},
	})
	defer srv.Close()

	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()
	cleanup := wireClient(t, srv)
	defer cleanup()

	cfg := &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
		Sets: map[string]config.Set{
			"mySet": {Commands: []config.Command{
				{Action: "pause"},
			}},
		},
	}
	configPath = writeTempConfig(t, cfg)

	if err := runCmd.RunE(runCmd, []string{"mySet"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
