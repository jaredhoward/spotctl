package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// ----- spotctl sets ----------------------------------------------------------

func TestSetsCmd_NoSets(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	configPath = writeTempConfig(t, &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
	})

	output := captureOutput(t, func() {
		if err := setsCmd.RunE(setsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "No sets configured.") {
		t.Errorf("expected no-sets message, got: %q", output)
	}
}

func TestSetsCmd_WithSets(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	level := 50
	cfg := &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
		Sets: map[string]config.Set{
			"morning": {
				DeviceID: "dev-abc",
				Commands: []config.Command{
					{Action: "play"},
					{Action: "shuffle"},
				},
			},
			"sleep": {
				Commands: []config.Command{
					{Action: "play"},
					{Action: "volume", Params: config.CommandParams{Level: &level}},
					{Action: "pause"},
				},
			},
		},
	}
	configPath = writeTempConfig(t, cfg)

	output := captureOutput(t, func() {
		if err := setsCmd.RunE(setsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "morning") {
		t.Errorf("expected 'morning' in output, got: %q", output)
	}
	if !strings.Contains(output, "2 command(s)") {
		t.Errorf("expected '2 command(s)' for morning, got: %q", output)
	}
	if !strings.Contains(output, "sleep") {
		t.Errorf("expected 'sleep' in output, got: %q", output)
	}
	if !strings.Contains(output, "3 command(s)") {
		t.Errorf("expected '3 command(s)' for sleep, got: %q", output)
	}
	if !strings.Contains(output, "dev-abc") {
		t.Errorf("expected device ID in output, got: %q", output)
	}
	if !strings.Contains(output, "(active device)") {
		t.Errorf("expected '(active device)' for set without device_id, got: %q", output)
	}
}

func TestSetsCmd_OutputIsSorted(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	cfg := &config.Config{
		ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh",
		Sets: map[string]config.Set{
			"zzz": {Commands: []config.Command{{Action: "pause"}}},
			"aaa": {Commands: []config.Command{{Action: "play"}}},
			"mmm": {Commands: []config.Command{{Action: "next"}}},
		},
	}
	configPath = writeTempConfig(t, cfg)

	output := captureOutput(t, func() {
		if err := setsCmd.RunE(setsCmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	aPos := strings.Index(output, "aaa")
	mPos := strings.Index(output, "mmm")
	zPos := strings.Index(output, "zzz")
	if aPos == -1 || mPos == -1 || zPos == -1 {
		t.Fatalf("not all set names found in output: %q", output)
	}
	if !(aPos < mPos && mPos < zPos) {
		t.Errorf("expected sorted output (aaa < mmm < zzz), positions %d %d %d", aPos, mPos, zPos)
	}
}

func TestSetsCmd_BadConfig(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	configPath = "/nonexistent/config.yaml"
	err := setsCmd.RunE(setsCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("expected config load error, got %v", err)
	}
}

// ----- shuffle verb ----------------------------------------------------------

func TestShuffleCmd_Enabled(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldShuffleDeviceID := shuffleDeviceID
	oldShuffleEnabled := shuffleEnabled
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		shuffleDeviceID = oldShuffleDeviceID
		shuffleEnabled = oldShuffleEnabled
	}()

	shuffleDeviceID = "dev-1"
	shuffleEnabled = true
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	var gotState string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotState = r.URL.Query().Get("state")
		if r.URL.Query().Get("device_id") != "dev-1" {
			t.Errorf("expected device_id=dev-1, got %q", r.URL.Query().Get("device_id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}

	if err := shuffleCmd.RunE(shuffleCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "true" {
		t.Errorf("expected state=true, got %q", gotState)
	}
}

func TestShuffleCmd_Disabled(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldShuffleDeviceID := shuffleDeviceID
	oldShuffleEnabled := shuffleEnabled
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		shuffleDeviceID = oldShuffleDeviceID
		shuffleEnabled = oldShuffleEnabled
	}()

	shuffleDeviceID = ""
	shuffleEnabled = false
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	var gotState string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotState = r.URL.Query().Get("state")
		if r.URL.Query().Get("device_id") != "" {
			t.Errorf("expected no device_id, got %q", r.URL.Query().Get("device_id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}

	if err := shuffleCmd.RunE(shuffleCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "false" {
		t.Errorf("expected state=false, got %q", gotState)
	}
}

// ----- repeat verb -----------------------------------------------------------

func TestRepeatCmd_InvalidState(t *testing.T) {
	oldRepeatState := repeatState
	defer func() { repeatState = oldRepeatState }()

	repeatState = "loop"
	err := repeatCmd.RunE(repeatCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--state must be one of") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestRepeatCmd_Off(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldRepeatDeviceID := repeatDeviceID
	oldRepeatState := repeatState
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		repeatDeviceID = oldRepeatDeviceID
		repeatState = oldRepeatState
	}()

	repeatDeviceID = "dev-1"
	repeatState = "off"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	var gotState, gotDevice string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotState = r.URL.Query().Get("state")
		gotDevice = r.URL.Query().Get("device_id")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}

	if err := repeatCmd.RunE(repeatCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "off" {
		t.Errorf("expected state=off, got %q", gotState)
	}
	if gotDevice != "dev-1" {
		t.Errorf("expected device_id=dev-1, got %q", gotDevice)
	}
}

func TestRepeatCmd_Context_NoDevice(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	oldNewClient := newSpotifyClient
	oldURLPlayer := spotify.URLPlayer
	oldRepeatDeviceID := repeatDeviceID
	oldRepeatState := repeatState
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
		newSpotifyClient = oldNewClient
		spotify.URLPlayer = oldURLPlayer
		repeatDeviceID = oldRepeatDeviceID
		repeatState = oldRepeatState
	}()

	repeatDeviceID = ""
	repeatState = "context"
	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) { return "token", nil }

	var gotState, gotDevice string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotState = r.URL.Query().Get("state")
		gotDevice = r.URL.Query().Get("device_id")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	spotify.URLPlayer = srv.URL
	newSpotifyClient = func(token string) *spotify.Client {
		c := spotify.NewClient(token)
		c.SetHTTPClient(srv.Client())
		return c
	}

	if err := repeatCmd.RunE(repeatCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "context" {
		t.Errorf("expected state=context, got %q", gotState)
	}
	if gotDevice != "" {
		t.Errorf("expected no device_id, got %q", gotDevice)
	}
}

// ----- confirmed: repeat -----------------------------------------------------

func TestConfirmed_Repeat(t *testing.T) {
	cmd := config.Command{Action: "repeat", Params: config.CommandParams{RepeatState: "context"}}
	if !confirmed(cmd, &spotify.PlaybackState{RepeatState: "context"}, "") {
		t.Error("expected confirmed when repeat_state matches")
	}
	if confirmed(cmd, &spotify.PlaybackState{RepeatState: "off"}, "") {
		t.Error("expected not confirmed when repeat_state differs")
	}
}

// ----- dispatchAction: repeat ------------------------------------------------

func TestDispatchAction_Repeat(t *testing.T) {
	var gotState, gotDevice string
	srv := setServer(t, map[string]http.HandlerFunc{
		"PUT /repeat": func(w http.ResponseWriter, r *http.Request) {
			gotState = r.URL.Query().Get("state")
			gotDevice = r.URL.Query().Get("device_id")
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	useTestServer(t, srv)

	p := config.CommandParams{DeviceID: "d1", RepeatState: "track"}
	if err := dispatchAction(p, "repeat", setClient(t, srv), nil, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotState != "track" {
		t.Errorf("expected state=track, got %q", gotState)
	}
	if gotDevice != "d1" {
		t.Errorf("expected device_id=d1, got %q", gotDevice)
	}
}

// ----- appVersion fallback ---------------------------------------------------

func TestAppVersionDefault(t *testing.T) {
	if appVersion != "dev" {
		t.Errorf("expected default appVersion to be 'dev', got %q", appVersion)
	}
}

func TestSetVersion(t *testing.T) {
	old := appVersion
	defer func() { appVersion = old }()

	SetVersion("1.2.3")
	if appVersion != "1.2.3" {
		t.Errorf("expected appVersion to be '1.2.3', got %q", appVersion)
	}
}
