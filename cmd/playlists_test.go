package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
)

// setupPlaylistCmd points the commands at srv and restores flag and config
// globals afterwards.
func setupPlaylistCmd(t *testing.T, srv *httptest.Server) {
	t.Helper()
	oldConfigPath := configPath
	oldFlags := [4]int{playlistsLimit, playlistsOffset, playlistLimit, playlistOffset}
	t.Cleanup(func() {
		configPath = oldConfigPath
		playlistsLimit, playlistsOffset, playlistLimit, playlistOffset = oldFlags[0], oldFlags[1], oldFlags[2], oldFlags[3]
	})
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	wireClient(t, srv)
	t.Cleanup(srv.Close)
}

func TestRunPlaylists_PrintsPlaylists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/me/playlists" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"offset":0,"total":1,"items":[
			{"name":"Chill","uri":"spotify:playlist:a","owner":{"display_name":"Jared"},"items":{"total":7}}]}`))
	}))
	setupPlaylistCmd(t, srv)

	output := captureOutput(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"Chill", "Jared", "7", "spotify:playlist:a"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got: %q", want, output)
		}
	}
}

func TestRunPlaylists_NoPlaylists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[]}`))
	}))
	setupPlaylistCmd(t, srv)

	output := captureOutput(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(output, "No playlists found.") {
		t.Errorf("expected empty-state message, got: %q", output)
	}
}

func TestRunPlaylists_ForbiddenHintsAtSetup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	setupPlaylistCmd(t, srv)

	err := playlistsCmd.RunE(playlistsCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "spotctl setup") {
		t.Fatalf("expected 403 error with setup hint, got %v", err)
	}
}

func TestRunPlaylists_OtherErrorHasNoHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	setupPlaylistCmd(t, srv)

	err := playlistsCmd.RunE(playlistsCmd, nil)
	if err == nil || strings.Contains(err.Error(), "hint:") {
		t.Fatalf("expected plain error without hint, got %v", err)
	}
}

func TestRunPlaylists_ClientError(t *testing.T) {
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	configPath = t.TempDir() + "/missing.yaml"

	if err := playlistsCmd.RunE(playlistsCmd, nil); err == nil {
		t.Fatal("expected error when config is missing")
	}
}

func TestRunPlaylists_MoreHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"offset":0,"total":30,"items":[{"name":"A","uri":"spotify:playlist:a"}]}`))
	}))
	setupPlaylistCmd(t, srv)

	if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPlaylist_PrintsItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/playlists/abc123/items" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"offset":20,"total":22,"items":[
			{"item":{"uri":"spotify:track:t1","name":"Weightless","artists":[{"name":"Marconi Union"}]}},
			{"item":null},
			{"is_local":true,"item":{"name":"Local Song"}}]}`))
	}))
	setupPlaylistCmd(t, srv)

	output := captureOutput(t, func() {
		if err := playlistCmd.RunE(playlistCmd, []string{"spotify:playlist:abc123"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"21  Weightless", "Marconi Union", "[spotify:track:t1]", "22  (unavailable)", "23  Local Song"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got: %q", want, output)
		}
	}
	if strings.Contains(output, "Local Song") && strings.Contains(output, "Local Song  [") {
		t.Errorf("expected no URI bracket for item without uri, got: %q", output)
	}
}

func TestRunPlaylist_NoItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[]}`))
	}))
	setupPlaylistCmd(t, srv)

	output := captureOutput(t, func() {
		if err := playlistCmd.RunE(playlistCmd, []string{"abc123"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(output, "No items found.") {
		t.Errorf("expected empty-state message, got: %q", output)
	}
}

func TestRunPlaylist_InvalidArgSkipsNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s for an invalid playlist argument", r.URL.Path)
	}))
	setupPlaylistCmd(t, srv)

	if err := playlistCmd.RunE(playlistCmd, []string{"spotify:track:abc"}); err == nil {
		t.Fatal("expected error for non-playlist URI")
	}
}

func TestRunPlaylist_ForbiddenHintsAtSetup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	setupPlaylistCmd(t, srv)

	err := playlistCmd.RunE(playlistCmd, []string{"abc123"})
	if err == nil || !strings.Contains(err.Error(), "own or collaborate on") {
		t.Fatalf("expected 403 error with access hint, got %v", err)
	}
}

func TestRunPlaylist_ClientError(t *testing.T) {
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	configPath = t.TempDir() + "/missing.yaml"

	if err := playlistCmd.RunE(playlistCmd, []string{"abc123"}); err == nil {
		t.Fatal("expected error when config is missing")
	}
}
