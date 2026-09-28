package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func resetAddToPlaylist(t *testing.T) {
	t.Helper()
	reset := func() {
		addToPlaylistCmd.Flags().VisitAll(func(f *pflag.Flag) {
			f.Value.Set(f.DefValue)
			f.Changed = false
		})
		addToPlaylistCmd.SetIn(nil)
	}
	reset()
	t.Cleanup(reset)
}

// addServer serves a playlist "pl1" that already holds the given track URIs
// (plus one unavailable entry) and records what gets added.
type addServer struct {
	existing []string
	reads    int
	added    [][]string
}

func (as *addServer) handlers(addStatus int) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/playlists/pl1/items": func(w http.ResponseWriter, r *http.Request) {
			as.reads++
			pagedHandler(len(as.existing)+1, func(i int) string {
				if i == len(as.existing) {
					return `{"item":null}`
				}
				return fmt.Sprintf(`{"item":{"uri":%q}}`, as.existing[i])
			})(w, r)
		},
		"POST /v1/playlists/pl1/items": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				URIs []string `json:"uris"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			as.added = append(as.added, body.URIs)
			w.WriteHeader(addStatus)
			w.Write([]byte(`{"snapshot_id":"s"}`))
		},
	}
}

func TestRunAddToPlaylist_SkipsExisting(t *testing.T) {
	resetAddToPlaylist(t)
	as := &addServer{existing: []string{trackB}}
	serveCmdTest(t, as.handlers(201))

	var runErr error
	_, stderr := captureBoth(t, func() {
		runErr = addToPlaylistCmd.RunE(addToPlaylistCmd, []string{"spotify:playlist:pl1", trackA, trackB, trackC})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if len(as.added) != 1 || strings.Join(as.added[0], ",") != trackA+","+trackC {
		t.Errorf("expected only the new tracks, got %#v", as.added)
	}
	if !strings.Contains(stderr, "skipped 1 track(s) already in the playlist") || !strings.Contains(stderr, "Added 2 track(s)") {
		t.Errorf("unexpected stderr %q", stderr)
	}
}

func TestRunAddToPlaylist_AllowDuplicatesSkipsTheCheck(t *testing.T) {
	resetAddToPlaylist(t)
	as := &addServer{existing: []string{trackA}}
	serveCmdTest(t, as.handlers(201))
	setFlag(t, addToPlaylistCmd, "allow-duplicates", "true")

	var runErr error
	captureBoth(t, func() {
		runErr = addToPlaylistCmd.RunE(addToPlaylistCmd, []string{"pl1", trackA})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if as.reads != 0 {
		t.Error("--allow-duplicates should not read the playlist")
	}
	if len(as.added) != 1 || as.added[0][0] != trackA {
		t.Errorf("expected the duplicate to be added, got %#v", as.added)
	}
}

func TestRunAddToPlaylist_NothingToAdd(t *testing.T) {
	resetAddToPlaylist(t)
	as := &addServer{existing: []string{trackA, trackB}}
	serveCmdTest(t, as.handlers(201))

	var runErr error
	_, stderr := captureBoth(t, func() {
		runErr = addToPlaylistCmd.RunE(addToPlaylistCmd, []string{"pl1", trackA, trackB})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if len(as.added) != 0 || !strings.Contains(stderr, "Nothing to add") {
		t.Errorf("expected no add request and a note, got added=%#v stderr=%q", as.added, stderr)
	}
}

func TestRunAddToPlaylist_DryRunReadsButDoesNotWrite(t *testing.T) {
	resetAddToPlaylist(t)
	as := &addServer{existing: []string{trackB}}
	serveCmdTest(t, as.handlers(201))
	setFlag(t, addToPlaylistCmd, "dry-run", "true")

	var runErr error
	stdout := captureOutput(t, func() {
		runErr = addToPlaylistCmd.RunE(addToPlaylistCmd, []string{"pl1", trackA, trackB})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if as.reads == 0 || len(as.added) != 0 {
		t.Errorf("dry run should read but not write: reads=%d added=%#v", as.reads, as.added)
	}
	if !strings.Contains(stdout, "Would add 1 track(s) to pl1 (1 already there).") {
		t.Errorf("unexpected dry-run output %q", stdout)
	}
}

func TestRunAddToPlaylist_DryRunAllowDuplicatesContactsNothing(t *testing.T) {
	resetAddToPlaylist(t)
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	configPath = filepath.Join(t.TempDir(), "missing.yaml")
	setFlag(t, addToPlaylistCmd, "dry-run", "true")
	setFlag(t, addToPlaylistCmd, "allow-duplicates", "true")

	var runErr error
	stdout := captureOutput(t, func() {
		runErr = addToPlaylistCmd.RunE(addToPlaylistCmd, []string{"pl1", trackA, trackB})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if strings.TrimSpace(stdout) != "Would add 2 track(s) to pl1." {
		t.Errorf("unexpected dry-run output %q", stdout)
	}
}

func TestRunAddToPlaylist_FromFile(t *testing.T) {
	resetAddToPlaylist(t)
	as := &addServer{}
	serveCmdTest(t, as.handlers(201))
	file := filepath.Join(t.TempDir(), "tracks.txt")
	os.WriteFile(file, []byte("# tracks\n"+trackA+"\n"+trackB+"\n"), 0o600)
	setFlag(t, addToPlaylistCmd, "from-file", file)

	var runErr error
	captureBoth(t, func() { runErr = addToPlaylistCmd.RunE(addToPlaylistCmd, []string{"pl1"}) })
	if runErr != nil {
		t.Fatal(runErr)
	}
	if len(as.added) != 1 || len(as.added[0]) != 2 {
		t.Errorf("unexpected added tracks %#v", as.added)
	}
}

func TestRunAddToPlaylist_InputErrors(t *testing.T) {
	resetAddToPlaylist(t)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"bad playlist", []string{"not a playlist", trackA}, "invalid playlist"},
		{"no tracks", []string{"pl1"}, "no tracks given"},
		{"bad track", []string{"pl1", "spotify:album:zzz"}, "spotify:album:zzz"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := addToPlaylistCmd.RunE(addToPlaylistCmd, tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestRunAddToPlaylist_MissingFile(t *testing.T) {
	resetAddToPlaylist(t)
	setFlag(t, addToPlaylistCmd, "from-file", filepath.Join(t.TempDir(), "nope.txt"))
	if err := addToPlaylistCmd.RunE(addToPlaylistCmd, []string{"pl1"}); err == nil || !strings.Contains(err.Error(), "--from-file") {
		t.Fatalf("expected --from-file error, got %v", err)
	}
}

func TestRunAddToPlaylist_ReadForbiddenHint(t *testing.T) {
	resetAddToPlaylist(t)
	serveCmdTest(t, map[string]http.HandlerFunc{"GET /v1/playlists/pl1/items": statusHandler(403)})

	err := addToPlaylistCmd.RunE(addToPlaylistCmd, []string{"pl1", trackA})
	if err == nil || !strings.Contains(err.Error(), "check for duplicates") || !strings.Contains(err.Error(), "spotctl setup") {
		t.Fatalf("expected a 403 with the setup hint, got %v", err)
	}
}

func TestRunAddToPlaylist_AddFailure(t *testing.T) {
	resetAddToPlaylist(t)
	as := &addServer{}
	serveCmdTest(t, as.handlers(403))

	var runErr error
	captureBoth(t, func() { runErr = addToPlaylistCmd.RunE(addToPlaylistCmd, []string{"pl1", trackA}) })
	if runErr == nil || !strings.Contains(runErr.Error(), "partly updated") || !strings.Contains(runErr.Error(), "spotctl setup") {
		t.Fatalf("expected a partial-update error with the setup hint, got %v", runErr)
	}
}
