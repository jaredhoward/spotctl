package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
)

func setupLikedCmd(t *testing.T, srv *httptest.Server) {
	t.Helper()
	oldConfigPath := configPath
	oldLimit, oldOffset := likedLimit, likedOffset
	t.Cleanup(func() {
		configPath = oldConfigPath
		likedLimit, likedOffset = oldLimit, oldOffset
	})
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	wireClient(t, srv)
	t.Cleanup(srv.Close)
}

func TestRunLiked_PrintsTracks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/me/tracks" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"offset":0,"total":40,"items":[
			{"track":{"uri":"spotify:track:t1","name":"Hazey","artists":[{"name":"Glass Animals"}]}},
			{"track":null}]}`))
	}))
	setupLikedCmd(t, srv)

	output := captureOutput(t, func() {
		if err := likedCmd.RunE(likedCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"1  Hazey", "Glass Animals", "[spotify:track:t1]", "2  (unavailable)"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got: %q", want, output)
		}
	}
}

func TestRunLiked_NoTracks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[]}`))
	}))
	setupLikedCmd(t, srv)

	output := captureOutput(t, func() {
		if err := likedCmd.RunE(likedCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(output, "No liked songs found.") {
		t.Errorf("expected empty-state message, got: %q", output)
	}
}

func TestRunLiked_ForbiddenHintsAtSetup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	setupLikedCmd(t, srv)

	err := likedCmd.RunE(likedCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "library access") {
		t.Fatalf("expected 403 error with library hint, got %v", err)
	}
}

func TestRunLiked_ClientError(t *testing.T) {
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	configPath = t.TempDir() + "/missing.yaml"

	if err := likedCmd.RunE(likedCmd, nil); err == nil {
		t.Fatal("expected error when config is missing")
	}
}
