package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/spf13/pflag"
)

const (
	trackA = "spotify:track:aaa111"
	trackB = "spotify:track:bbb222"
	trackC = "spotify:track:ccc333"
)

func resetCreatePlaylist(t *testing.T) {
	t.Helper()
	reset := func() {
		createPlaylistCmd.Flags().VisitAll(func(f *pflag.Flag) {
			f.Value.Set(f.DefValue)
			f.Changed = false
		})
		createPlaylistCmd.SetIn(nil)
	}
	reset()
	t.Cleanup(reset)
}

// createServer serves the two endpoints create-playlist calls and records the
// bodies it received.
type createServer struct {
	created map[string]any
	added   [][]string
}

func (cs *createServer) handlers(createStatus, addStatus int) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /v1/me/playlists": func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&cs.created)
			w.WriteHeader(createStatus)
			w.Write([]byte(`{"id":"new1","uri":"spotify:playlist:new1","name":"Chill: Test"}`))
		},
		"POST /v1/playlists/new1/items": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				URIs []string `json:"uris"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			cs.added = append(cs.added, body.URIs)
			w.WriteHeader(addStatus)
			w.Write([]byte(`{"snapshot_id":"s"}`))
		},
	}
}

func TestRunCreatePlaylist_FromArgs(t *testing.T) {
	resetCreatePlaylist(t)
	cs := &createServer{}
	serveCmdTest(t, cs.handlers(201, 201))
	setFlag(t, createPlaylistCmd, "description", "quiet songs")

	var runErr error
	stdout, stderr := captureBoth(t, func() {
		runErr = createPlaylistCmd.RunE(createPlaylistCmd, []string{"Chill: Test", trackA, trackB})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if strings.TrimSpace(stdout) != "spotify:playlist:new1" {
		t.Errorf("stdout should be just the playlist URI, got %q", stdout)
	}
	if !strings.Contains(stderr, "Created private playlist") || !strings.Contains(stderr, "2 track(s)") {
		t.Errorf("unexpected stderr %q", stderr)
	}
	if cs.created["name"] != "Chill: Test" || cs.created["public"] != false || cs.created["description"] != "quiet songs" {
		t.Errorf("unexpected create body %#v", cs.created)
	}
	if len(cs.added) != 1 || strings.Join(cs.added[0], ",") != trackA+","+trackB {
		t.Errorf("unexpected added tracks %#v", cs.added)
	}
}

func TestRunCreatePlaylist_FromFileAndArgs(t *testing.T) {
	resetCreatePlaylist(t)
	cs := &createServer{}
	serveCmdTest(t, cs.handlers(201, 201))

	file := filepath.Join(t.TempDir(), "tracks.txt")
	os.WriteFile(file, []byte("# comment\n"+trackB+"\n\n  "+trackC+"  \n"+trackA+"\n"), 0o600)
	setFlag(t, createPlaylistCmd, "from-file", file)

	var runErr error
	_, stderr := captureBoth(t, func() {
		runErr = createPlaylistCmd.RunE(createPlaylistCmd, []string{"Chill: Test", trackA})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	// trackA from the args comes first; the repeat in the file is dropped.
	if len(cs.added) != 1 || strings.Join(cs.added[0], ",") != strings.Join([]string{trackA, trackB, trackC}, ",") {
		t.Errorf("unexpected added tracks %#v", cs.added)
	}
	if !strings.Contains(stderr, "skipped 1 repeated") {
		t.Errorf("expected a note about the repeat, got %q", stderr)
	}
}

func TestRunCreatePlaylist_FromStdin(t *testing.T) {
	resetCreatePlaylist(t)
	cs := &createServer{}
	serveCmdTest(t, cs.handlers(201, 201))
	setFlag(t, createPlaylistCmd, "from-file", "-")
	createPlaylistCmd.SetIn(strings.NewReader(trackA + "\n" + trackB + "\n"))

	var runErr error
	captureBoth(t, func() { runErr = createPlaylistCmd.RunE(createPlaylistCmd, []string{"Chill: Test"}) })
	if runErr != nil {
		t.Fatal(runErr)
	}
	if len(cs.added) != 1 || len(cs.added[0]) != 2 {
		t.Errorf("unexpected added tracks %#v", cs.added)
	}
}

func TestRunCreatePlaylist_DryRunContactsNothing(t *testing.T) {
	resetCreatePlaylist(t)
	// No server and no config: a dry run must not need either.
	oldConfigPath := configPath
	t.Cleanup(func() { configPath = oldConfigPath })
	configPath = filepath.Join(t.TempDir(), "missing.yaml")
	setFlag(t, createPlaylistCmd, "dry-run", "true")
	setFlag(t, createPlaylistCmd, "description", "quiet songs")

	var runErr error
	stdout := captureOutput(t, func() {
		runErr = createPlaylistCmd.RunE(createPlaylistCmd, []string{"Chill: Test", trackA, trackB})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if !strings.Contains(stdout, `"Chill: Test"`) || !strings.Contains(stdout, "2 track(s)") || !strings.Contains(stdout, "quiet songs") {
		t.Errorf("unexpected dry-run output %q", stdout)
	}
}

func TestRunCreatePlaylist_DryRunWithoutDescription(t *testing.T) {
	resetCreatePlaylist(t)
	setFlag(t, createPlaylistCmd, "dry-run", "true")
	stdout := captureOutput(t, func() {
		if err := createPlaylistCmd.RunE(createPlaylistCmd, []string{"n", trackA}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(stdout, "Description:") {
		t.Errorf("no description was given, got %q", stdout)
	}
}

func TestRunCreatePlaylist_InputErrors(t *testing.T) {
	resetCreatePlaylist(t)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"blank name", []string{"   ", trackA}, "name must not be empty"},
		{"no tracks", []string{"Chill: Test"}, "no tracks given"},
		{"bad track", []string{"Chill: Test", trackA, "spotify:album:zzz"}, "spotify:album:zzz"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := createPlaylistCmd.RunE(createPlaylistCmd, tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestRunCreatePlaylist_MissingFile(t *testing.T) {
	resetCreatePlaylist(t)
	setFlag(t, createPlaylistCmd, "from-file", filepath.Join(t.TempDir(), "nope.txt"))
	err := createPlaylistCmd.RunE(createPlaylistCmd, []string{"n"})
	if err == nil || !strings.Contains(err.Error(), "--from-file") {
		t.Fatalf("expected --from-file error, got %v", err)
	}
}

func TestRunCreatePlaylist_CreateForbiddenHint(t *testing.T) {
	resetCreatePlaylist(t)
	cs := &createServer{}
	serveCmdTest(t, cs.handlers(403, 201))

	var runErr error
	captureBoth(t, func() { runErr = createPlaylistCmd.RunE(createPlaylistCmd, []string{"Chill: Test", trackA}) })
	if runErr == nil || !strings.Contains(runErr.Error(), "spotctl setup") || !strings.Contains(runErr.Error(), "failed to create playlist") {
		t.Fatalf("expected a 403 with the setup hint, got %v", runErr)
	}
	if len(cs.added) != 0 {
		t.Error("no tracks should be added when creation fails")
	}
}

func TestRunCreatePlaylist_AddFailureNamesThePlaylist(t *testing.T) {
	resetCreatePlaylist(t)
	cs := &createServer{}
	serveCmdTest(t, cs.handlers(201, 500))

	var runErr error
	captureBoth(t, func() { runErr = createPlaylistCmd.RunE(createPlaylistCmd, []string{"Chill: Test", trackA}) })
	if runErr == nil || !strings.Contains(runErr.Error(), "spotify:playlist:new1") || !strings.Contains(runErr.Error(), "partly filled") {
		t.Fatalf("expected the error to name the created playlist, got %v", runErr)
	}
}

func TestReadURILines_ReadError(t *testing.T) {
	boom := errors.New("boom")
	if _, err := readURILines(iotest.ErrReader(boom), "-"); !errors.Is(err, boom) {
		t.Fatalf("expected the read error, got %v", err)
	}
}

func TestRunCreatePlaylist_Public(t *testing.T) {
	resetCreatePlaylist(t)
	cs := &createServer{}
	serveCmdTest(t, cs.handlers(201, 201))
	setFlag(t, createPlaylistCmd, "public", "true")

	var runErr error
	_, stderr := captureBoth(t, func() {
		runErr = createPlaylistCmd.RunE(createPlaylistCmd, []string{"Chill: Test", trackA})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if cs.created["public"] != true {
		t.Errorf("expected public=true in the create body, got %#v", cs.created)
	}
	if !strings.Contains(stderr, "Created public playlist") {
		t.Errorf("unexpected stderr %q", stderr)
	}
	if strings.Contains(stderr, "warning") {
		t.Errorf("no warning expected when visibility matches, got %q", stderr)
	}
}

func TestRunCreatePlaylist_DryRunShowsVisibility(t *testing.T) {
	resetCreatePlaylist(t)
	setFlag(t, createPlaylistCmd, "dry-run", "true")
	setFlag(t, createPlaylistCmd, "public", "true")
	stdout := captureOutput(t, func() {
		if err := createPlaylistCmd.RunE(createPlaylistCmd, []string{"n", trackA}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(stdout, "Would create public playlist") {
		t.Errorf("unexpected dry-run output %q", stdout)
	}
}

func TestRunCreatePlaylist_WarnsWhenSpotifyIgnoresVisibility(t *testing.T) {
	resetCreatePlaylist(t)
	cs := &createServer{}
	handlers := cs.handlers(201, 201)
	handlers["POST /v1/me/playlists"] = jsonHandler(`{"id":"new1","uri":"spotify:playlist:new1","name":"Chill: Test","public":true}`)
	serveCmdTest(t, handlers)

	var runErr error
	_, stderr := captureBoth(t, func() {
		runErr = createPlaylistCmd.RunE(createPlaylistCmd, []string{"Chill: Test", trackA})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if !strings.Contains(stderr, "warning: Spotify reports this playlist as public, not private") {
		t.Errorf("expected a visibility warning, got %q", stderr)
	}
}
