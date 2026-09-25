package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// resetListCmds restores every list command's flags (values and the
// "Changed" state cobra tracks) so tests that set flags don't leak.
func resetListCmds(t *testing.T) {
	t.Helper()
	reset := func() {
		for _, c := range []*cobra.Command{playlistsCmd, playlistCmd, likedCmd} {
			c.Flags().VisitAll(func(f *pflag.Flag) {
				f.Value.Set(f.DefValue)
				f.Changed = false
			})
		}
	}
	reset()
	t.Cleanup(reset)
}

func setFlag(t *testing.T, c *cobra.Command, name, value string) {
	t.Helper()
	if err := c.Flags().Set(name, value); err != nil {
		t.Fatal(err)
	}
}

// captureBoth runs fn and returns what it wrote to stdout and stderr.
func captureBoth(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	oldErr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	stdout = captureOutput(t, fn)
	w.Close()
	b, _ := io.ReadAll(r)
	os.Stderr = oldErr
	return stdout, string(b)
}

// serveCmdTest starts a mock Spotify API, points the commands at it, and
// resets flags and config afterwards.
func serveCmdTest(t *testing.T, handlers map[string]http.HandlerFunc) {
	t.Helper()
	resetListCmds(t)
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	srv := mockSpotifyServer(t, handlers)
	wireClient(t, srv)
	t.Cleanup(srv.Close)
}

func jsonHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }
}

func statusHandler(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

// pagedHandler serves total items, item(i) each, honouring limit/offset.
func pagedHandler(total int, item func(i int) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit == 0 {
			limit = 20
		}
		var items []string
		for i := offset; i < offset+limit && i < total; i++ {
			items = append(items, item(i))
		}
		fmt.Fprintf(w, `{"offset":%d,"total":%d,"items":[%s]}`, offset, total, strings.Join(items, ","))
	}
}

const meHandlerBody = `{"id":"me","email":"never-used@example.com"}`

// Three playlists: one mine, one followed, one someone else's collaborative.
const mixedPlaylists = `{"offset":0,"total":3,"items":[
	{"name":"Mine","uri":"spotify:playlist:a","id":"a","owner":{"id":"me","display_name":"Me"},"items":{"total":7}},
	{"name":"Theirs","uri":"spotify:playlist:b","id":"b","owner":{"id":"other","display_name":"Other"},"items":{"total":9}},
	{"name":"Shared","uri":"spotify:playlist:c","id":"c","collaborative":true,"owner":{"id":"other","display_name":"Other"},"items":{"total":3}}]}`

func TestRunPlaylists_PrintsAndMarksFollowed(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/playlists": jsonHandler(mixedPlaylists),
		"GET /v1/me":           jsonHandler(meHandlerBody),
	})

	out := captureOutput(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %q", out)
	}
	if strings.Contains(lines[0], "(") {
		t.Errorf("own playlist should carry no marker: %q", lines[0])
	}
	if !strings.Contains(lines[1], "Theirs") || !strings.HasSuffix(lines[1], "(followed)") {
		t.Errorf("expected followed marker: %q", lines[1])
	}
	if !strings.HasSuffix(lines[2], "(collaborative)") {
		t.Errorf("expected collaborative marker: %q", lines[2])
	}
	if !strings.Contains(lines[0], "spotify:playlist:a") || !strings.Contains(lines[0], "7") {
		t.Errorf("expected URI and count: %q", lines[0])
	}
}

func TestRunPlaylists_NoPlaylists(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/playlists": jsonHandler(`{"items":[]}`),
		"GET /v1/me":           jsonHandler(meHandlerBody),
	})
	out := captureOutput(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "No playlists found.") {
		t.Errorf("expected empty-state message, got: %q", out)
	}
}

func TestRunPlaylists_AllFetchesEveryPage(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/playlists": pagedHandler(120, func(i int) string {
			return fmt.Sprintf(`{"name":"P%03d","uri":"spotify:playlist:p%d","owner":{"id":"me"}}`, i, i)
		}),
		"GET /v1/me": jsonHandler(meHandlerBody),
	})
	setFlag(t, playlistsCmd, "all", "true")

	stdout, stderr := captureBoth(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if n := len(strings.Split(strings.TrimSpace(stdout), "\n")); n != 120 {
		t.Errorf("expected 120 lines, got %d", n)
	}
	if strings.Contains(stderr, "Showing") {
		t.Errorf("--all should not print a paging hint, got %q", stderr)
	}
}

func TestRunPlaylists_PagedPrintsMoreHint(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/playlists": pagedHandler(30, func(i int) string {
			return fmt.Sprintf(`{"name":"P%d","owner":{"id":"me"}}`, i)
		}),
		"GET /v1/me": jsonHandler(meHandlerBody),
	})
	_, stderr := captureBoth(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(stderr, "Showing 1–20 of 30") || !strings.Contains(stderr, "--offset 20") {
		t.Errorf("expected paging hint, got %q", stderr)
	}
}

func TestRunPlaylists_ReadableKeepsOwnedAndCollaborative(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/playlists": jsonHandler(mixedPlaylists),
		"GET /v1/me":           jsonHandler(meHandlerBody),
	})
	setFlag(t, playlistsCmd, "readable", "true")

	out := captureOutput(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Mine") || !strings.Contains(out, "Shared") || strings.Contains(out, "Theirs") {
		t.Errorf("expected Mine and Shared but not Theirs, got %q", out)
	}
}

func TestRunPlaylists_ReadableWithNothingReadable(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/playlists": jsonHandler(`{"total":1,"items":[{"name":"Theirs","owner":{"id":"other"}}]}`),
		"GET /v1/me":           jsonHandler(meHandlerBody),
	})
	setFlag(t, playlistsCmd, "readable", "true")

	out := captureOutput(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "No playlists found.") {
		t.Errorf("expected empty-state message, got %q", out)
	}
}

func TestRunPlaylists_FlagConflicts(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{})
	for _, tc := range []struct {
		name  string
		flags [][2]string
		want  string
	}{
		{"all with limit", [][2]string{{"all", "true"}, {"limit", "5"}}, "--all can't be combined"},
		{"all with offset", [][2]string{{"all", "true"}, {"offset", "5"}}, "--all can't be combined"},
		{"readable with limit", [][2]string{{"readable", "true"}, {"limit", "5"}}, "--readable can't be combined"},
		{"all with readable", [][2]string{{"all", "true"}, {"readable", "true"}}, "--all and --readable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetListCmds(t)
			for _, f := range tc.flags {
				setFlag(t, playlistsCmd, f[0], f[1])
			}
			err := playlistsCmd.RunE(playlistsCmd, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestRunPlaylists_JSON(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/playlists": jsonHandler(mixedPlaylists),
		"GET /v1/me":           jsonHandler(meHandlerBody),
	})
	setFlag(t, playlistsCmd, "json", "true")

	out := captureOutput(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	var rows []playlistJSON
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("output isn't valid JSON: %v\n%s", err, out)
	}
	if len(rows) != 3 || rows[0].Name != "Mine" || rows[0].OwnerID != "me" || rows[0].ItemCount != 7 {
		t.Fatalf("unexpected rows: %#v", rows)
	}
	if rows[0].Owned == nil || !*rows[0].Owned || rows[1].Owned == nil || *rows[1].Owned {
		t.Errorf("expected owned true/false, got %v %v", rows[0].Owned, rows[1].Owned)
	}
	if !rows[2].Collaborative {
		t.Errorf("expected collaborative flag on the third playlist")
	}
	if strings.Contains(out, "never-used@example.com") {
		t.Errorf("the profile's email must never be printed")
	}
}

func TestRunPlaylists_JSONEmptyIsAnEmptyArray(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/playlists": jsonHandler(`{"items":[]}`),
		"GET /v1/me":           jsonHandler(meHandlerBody),
	})
	setFlag(t, playlistsCmd, "json", "true")
	out := captureOutput(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("expected [], got %q", out)
	}
}

func TestRunPlaylists_JSONDoesNotEscapeAmpersands(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/playlists": jsonHandler(`{"total":1,"items":[{"name":"Mumford & Sons","owner":{"id":"me"}}]}`),
		"GET /v1/me":           jsonHandler(meHandlerBody),
	})
	setFlag(t, playlistsCmd, "json", "true")
	out := captureOutput(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Mumford & Sons") {
		t.Errorf("expected a literal ampersand, got %q", out)
	}
}

func TestRunPlaylists_UserLookupFailureDegrades(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/playlists": jsonHandler(mixedPlaylists),
		"GET /v1/me":           statusHandler(http.StatusInternalServerError),
	})

	stdout, stderr := captureBoth(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatalf("a failed user lookup must not fail the command: %v", err)
		}
	})
	if strings.Contains(stdout, "(followed)") {
		t.Errorf("no markers expected without an ID, got %q", stdout)
	}
	if !strings.Contains(stderr, "warning: could not look up your user ID") {
		t.Errorf("expected a warning on stderr, got %q", stderr)
	}

	// JSON: "owned" must be absent, not false.
	setFlag(t, playlistsCmd, "json", "true")
	out := captureOutput(t, func() {
		if err := playlistsCmd.RunE(playlistsCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, `"owned"`) {
		t.Errorf("owned should be omitted when unknown, got %s", out)
	}
}

func TestRunPlaylists_ReadableFailsWithoutUserID(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/me/playlists": jsonHandler(mixedPlaylists),
		"GET /v1/me":           statusHandler(http.StatusInternalServerError),
	})
	setFlag(t, playlistsCmd, "readable", "true")
	err := playlistsCmd.RunE(playlistsCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--readable") {
		t.Fatalf("expected --readable to fail without a user ID, got %v", err)
	}
}

func TestRunPlaylists_ForbiddenHintsAtSetup(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{"GET /v1/me/playlists": statusHandler(http.StatusForbidden)})
	err := playlistsCmd.RunE(playlistsCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "spotctl setup") {
		t.Fatalf("expected 403 error with setup hint, got %v", err)
	}
}

func TestRunPlaylists_OtherErrorHasNoHint(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{"GET /v1/me/playlists": statusHandler(http.StatusInternalServerError)})
	err := playlistsCmd.RunE(playlistsCmd, nil)
	if err == nil || strings.Contains(err.Error(), "hint:") {
		t.Fatalf("expected plain error without hint, got %v", err)
	}
}

func TestRunPlaylists_ClientError(t *testing.T) {
	resetListCmds(t)
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	configPath = t.TempDir() + "/missing.yaml"
	if err := playlistsCmd.RunE(playlistsCmd, nil); err == nil {
		t.Fatal("expected error when config is missing")
	}
}

func TestPlaylistMarker(t *testing.T) {
	pl := func(owner string, collab bool) spotify.Playlist {
		return spotify.Playlist{Owner: spotify.PlaylistOwner{ID: owner}, Collaborative: collab}
	}
	for _, tc := range []struct {
		name string
		p    spotify.Playlist
		me   string
		want string
	}{
		{"own", pl("me", false), "me", ""},
		{"own and collaborative", pl("me", true), "me", ""},
		{"followed", pl("other", false), "me", "(followed)"},
		{"collaborative", pl("other", true), "me", "(collaborative)"},
		{"unknown user never marks", pl("other", false), "", ""},
	} {
		if got := playlistMarker(tc.p, tc.me); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// ---- playlist (items) ----

const itemsPage = `{"offset":20,"total":23,"items":[
	{"added_at":"2026-08-04T03:10:00Z","item":{"uri":"spotify:track:t1","name":"Weightless","duration_ms":480000,"artists":[{"name":"Marconi Union"}]}},
	{"item":null},
	{"is_local":true,"item":{"name":"Local Song"}}]}`

func TestRunPlaylist_PrintsItems(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/playlists/abc123/items": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(itemsPage)) },
	})
	out := captureOutput(t, func() {
		if err := playlistCmd.RunE(playlistCmd, []string{"spotify:playlist:abc123"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"21  Weightless", "Marconi Union", "[spotify:track:t1]", "22  (unavailable)", "23  Local Song"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %q", want, out)
		}
	}
	if strings.Contains(out, "Local Song  [") {
		t.Errorf("expected no URI bracket for an item without a uri, got: %q", out)
	}
}

func TestRunPlaylist_JSON(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/playlists/abc123/items": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(itemsPage)) },
	})
	setFlag(t, playlistCmd, "json", "true")

	out := captureOutput(t, func() {
		if err := playlistCmd.RunE(playlistCmd, []string{"abc123"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	var rows []trackJSON
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %#v", rows)
	}
	first := rows[0]
	if first.Position != 21 || first.URI != "spotify:track:t1" || first.Name != "Weightless" || first.DurationMS != 480000 ||
		len(first.Artists) != 1 || first.Artists[0] != "Marconi Union" || first.AddedAt != "2026-08-04T03:10:00Z" {
		t.Errorf("unexpected first row: %#v", first)
	}
	if !rows[1].Unavailable || rows[1].Position != 22 {
		t.Errorf("expected unavailable row at position 22, got %#v", rows[1])
	}
	if !rows[2].Local {
		t.Errorf("expected local flag, got %#v", rows[2])
	}
}

func TestRunPlaylist_AllFetchesEveryPage(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{
		"GET /v1/playlists/abc/items": pagedHandler(70, func(i int) string {
			return fmt.Sprintf(`{"item":{"uri":"spotify:track:t%d","name":"Track %d"}}`, i, i)
		}),
	})
	setFlag(t, playlistCmd, "all", "true")

	stdout, stderr := captureBoth(t, func() {
		if err := playlistCmd.RunE(playlistCmd, []string{"abc"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(stdout, "  70  Track 69") || len(strings.Split(strings.TrimSpace(stdout), "\n")) != 70 {
		t.Errorf("expected all 70 numbered rows, got %d lines", len(strings.Split(strings.TrimSpace(stdout), "\n")))
	}
	if strings.Contains(stderr, "Showing") {
		t.Errorf("--all should not print a paging hint, got %q", stderr)
	}
}

func TestRunPlaylist_NoItems(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{"GET /v1/playlists/abc123/items": jsonHandler(`{"items":[]}`)})
	out := captureOutput(t, func() {
		if err := playlistCmd.RunE(playlistCmd, []string{"abc123"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "No items found.") {
		t.Errorf("expected empty-state message, got: %q", out)
	}
}

func TestRunPlaylist_InvalidArgSkipsNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s for an invalid playlist argument", r.URL.Path)
	}))
	resetListCmds(t)
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	wireClient(t, srv)
	t.Cleanup(srv.Close)

	if err := playlistCmd.RunE(playlistCmd, []string{"spotify:track:abc"}); err == nil {
		t.Fatal("expected error for non-playlist URI")
	}
}

func TestRunPlaylist_AllWithOffsetIsRejected(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{})
	setFlag(t, playlistCmd, "all", "true")
	setFlag(t, playlistCmd, "offset", "5")
	if err := playlistCmd.RunE(playlistCmd, []string{"abc"}); err == nil || !strings.Contains(err.Error(), "--all can't be combined") {
		t.Fatalf("expected flag conflict error, got %v", err)
	}
}

func TestRunPlaylist_ForbiddenHintsAtAccess(t *testing.T) {
	serveCmdTest(t, map[string]http.HandlerFunc{"GET /v1/playlists/abc123/items": statusHandler(http.StatusForbidden)})
	err := playlistCmd.RunE(playlistCmd, []string{"abc123"})
	if err == nil || !strings.Contains(err.Error(), "own or collaborate on") {
		t.Fatalf("expected 403 error with access hint, got %v", err)
	}
}

func TestRunPlaylist_ClientError(t *testing.T) {
	resetListCmds(t)
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	configPath = t.TempDir() + "/missing.yaml"
	if err := playlistCmd.RunE(playlistCmd, []string{"abc123"}); err == nil {
		t.Fatal("expected error when config is missing")
	}
}
