package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const likedPage = `{"offset":0,"total":40,"items":[
	{"added_at":"2026-09-01T12:00:00Z","track":{"uri":"spotify:track:t1","name":"Hazey","artists":[{"name":"Glass Animals"}]}},
	{"track":null}]}`

func TestRunLiked_PrintsTracks(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{"GET /v1/me/tracks": jsonHandler(likedPage)})
	stdout, stderr := captureBoth(t, func() {
		if err := likedCmd.RunE(likedCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"1  Hazey", "Glass Animals", "[spotify:track:t1]", "2  (unavailable)"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in output, got: %q", want, stdout)
		}
	}
	if !strings.Contains(stderr, "Showing 1–2 of 40") || !strings.Contains(stderr, "--all") {
		t.Errorf("expected a paging hint mentioning --all, got %q", stderr)
	}
}

func TestRunLiked_JSON(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{"GET /v1/me/tracks": jsonHandler(likedPage)})
	setFlag(t, likedCmd, "json", "true")

	out := captureOutput(t, func() {
		if err := likedCmd.RunE(likedCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	var rows []trackJSON
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(rows) != 2 || rows[0].Name != "Hazey" || rows[0].AddedAt != "2026-09-01T12:00:00Z" || rows[0].Local {
		t.Errorf("unexpected rows: %#v", rows)
	}
	if !rows[1].Unavailable {
		t.Errorf("expected the null track to be marked unavailable, got %#v", rows[1])
	}
}

func TestRunLiked_AllFetchesEveryPage(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/tracks": pagedHandler(101, func(i int) string {
			return fmt.Sprintf(`{"track":{"uri":"spotify:track:t%d","name":"Song %d"}}`, i, i)
		}),
	})
	setFlag(t, likedCmd, "all", "true")

	stdout, stderr := captureBoth(t, func() {
		if err := likedCmd.RunE(likedCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if n := len(strings.Split(strings.TrimSpace(stdout), "\n")); n != 101 {
		t.Errorf("expected 101 lines, got %d", n)
	}
	if strings.Contains(stderr, "Showing") {
		t.Errorf("--all should not print a paging hint, got %q", stderr)
	}
}

func TestRunLiked_AllWithLimitIsRejected(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{})
	setFlag(t, likedCmd, "all", "true")
	setFlag(t, likedCmd, "limit", "5")
	if err := likedCmd.RunE(likedCmd, nil); err == nil || !strings.Contains(err.Error(), "--all can't be combined") {
		t.Fatalf("expected flag conflict error, got %v", err)
	}
}

func TestRunLiked_NoTracks(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{"GET /v1/me/tracks": jsonHandler(`{"items":[]}`)})
	out := captureOutput(t, func() {
		if err := likedCmd.RunE(likedCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "No liked songs found.") {
		t.Errorf("expected empty-state message, got: %q", out)
	}
}

func TestRunLiked_ForbiddenHintsAtSetup(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{"GET /v1/me/tracks": statusHandler(http.StatusForbidden)})
	err := likedCmd.RunE(likedCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "library access") {
		t.Fatalf("expected 403 error with library hint, got %v", err)
	}
}

func TestRunLiked_ClientError(t *testing.T) {
	resetListCmds(t)
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	configPath = t.TempDir() + "/missing.yaml"
	if err := likedCmd.RunE(likedCmd, nil); err == nil {
		t.Fatal("expected error when config is missing")
	}
}
