package cmd

import (
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

// wireClient points URLPlayer at srv, sets up newSpotifyClient to use srv's
// HTTP client, and installs a no-op token refresh. Returns cleanup.
func wireClient(t *testing.T, srv *httptest.Server) (cleanup func()) {
	t.Helper()
	oldURLPlayer := spotify.URLPlayer
	oldNewClient := newSpotifyClient
	oldRefresh := spotify.RefreshAccessToken

	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	return func() {
		spotify.URLPlayer = oldURLPlayer
		newSpotifyClient = oldNewClient
		spotify.RefreshAccessToken = oldRefresh
	}
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
