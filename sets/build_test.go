package sets

import (
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// minimalCfg returns a *config.Config with the given sets and no credentials.
func minimalCfg(sets map[string]config.Set) *config.Config {
	return &config.Config{Sets: sets}
}

// extractPlay walks rs to find the first *spotify.Play action, descending into
// nested *RunSet steps. Returns nil if none found.
func extractPlay(rs *RunSet) *spotify.Play {
	for _, s := range rs.Steps {
		switch a := s.action.(type) {
		case *spotify.Play:
			return a
		case *RunSet:
			if p := extractPlay(a); p != nil {
				return p
			}
		}
	}
	return nil
}

// ----- URI resolution: direct play ------------------------------------------

func TestResolveURI_Direct(t *testing.T) {
	cases := []struct {
		name    string
		params  config.CommandParams
		wantURI string
	}{
		{
			name:    "full URI passed through unchanged",
			params:  config.CommandParams{URI: "spotify:playlist:abc123"},
			wantURI: "spotify:playlist:abc123",
		},
		{
			name:    "playlist ID expanded to spotify:playlist URI",
			params:  config.CommandParams{PlaylistID: "abc123"},
			wantURI: "spotify:playlist:abc123",
		},
		{
			name:    "track ID expanded to spotify:track URI",
			params:  config.CommandParams{TrackID: "tr456"},
			wantURI: "spotify:track:tr456",
		},
		{
			name:    "album ID expanded to spotify:album URI",
			params:  config.CommandParams{AlbumID: "al789"},
			wantURI: "spotify:album:al789",
		},
		{
			name:    "artist ID expanded to spotify:artist URI",
			params:  config.CommandParams{ArtistID: "ar999"},
			wantURI: "spotify:artist:ar999",
		},
		{
			name:    "no URI fields yields empty context URI",
			params:  config.CommandParams{},
			wantURI: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set := config.Set{
				Commands: []config.Command{
					{Action: "play", Params: tc.params, Confirm: new(false)},
				},
			}
			rs, err := Build("test", set, minimalCfg(nil), 0, nil)
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}
			play := extractPlay(rs)
			if play == nil {
				t.Fatal("no play action found")
			}
			if play.ContextURI != tc.wantURI {
				t.Errorf("ContextURI: got %q, want %q", play.ContextURI, tc.wantURI)
			}
		})
	}
}

func TestResolveURI_MultipleFieldsError(t *testing.T) {
	set := config.Set{
		Commands: []config.Command{
			{Action: "play", Params: config.CommandParams{PlaylistID: "pl1", TrackID: "tr1"}},
		},
	}
	_, err := Build("test", set, minimalCfg(nil), 0, nil)
	if err == nil || !strings.Contains(err.Error(), "only one of") {
		t.Fatalf("expected multiple-URI error, got %v", err)
	}
}

// ----- URI resolution: via run_set params ------------------------------------

func TestResolveURI_ViaRunSet(t *testing.T) {
	cases := []struct {
		name       string
		paramName  string               // declared param name in inner set
		paramField string               // which CommandParams field uses the template
		forwarded  config.CommandParams // params on the run_set command
		wantURI    string
	}{
		{
			name:       "full URI forwarded via uri field",
			paramName:  "uri",
			paramField: "uri",
			forwarded:  config.CommandParams{Set: "inner", URI: "spotify:playlist:abc123"},
			wantURI:    "spotify:playlist:abc123",
		},
		{
			name:       "playlist ID forwarded and expanded",
			paramName:  "playlist",
			paramField: "playlist",
			forwarded:  config.CommandParams{Set: "inner", PlaylistID: "pl123"},
			wantURI:    "spotify:playlist:pl123",
		},
		{
			name:       "track ID forwarded and expanded",
			paramName:  "track",
			paramField: "track",
			forwarded:  config.CommandParams{Set: "inner", TrackID: "tr456"},
			wantURI:    "spotify:track:tr456",
		},
		{
			name:       "album ID forwarded and expanded",
			paramName:  "album",
			paramField: "album",
			forwarded:  config.CommandParams{Set: "inner", AlbumID: "al789"},
			wantURI:    "spotify:album:al789",
		},
		{
			name:       "artist ID forwarded and expanded",
			paramName:  "artist",
			paramField: "artist",
			forwarded:  config.CommandParams{Set: "inner", ArtistID: "ar999"},
			wantURI:    "spotify:artist:ar999",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Build the inner set's play command using the right CommandParams field.
			var innerParams config.CommandParams
			switch tc.paramField {
			case "uri":
				innerParams = config.CommandParams{URI: `{{ uri }}`}
			case "playlist":
				innerParams = config.CommandParams{PlaylistID: `{{ playlist }}`}
			case "track":
				innerParams = config.CommandParams{TrackID: `{{ track }}`}
			case "album":
				innerParams = config.CommandParams{AlbumID: `{{ album }}`}
			case "artist":
				innerParams = config.CommandParams{ArtistID: `{{ artist }}`}
			}

			inner := config.Set{
				Params: map[string]config.SetParam{
					tc.paramName: {Required: true},
				},
				Commands: []config.Command{
					{Action: "play", Params: innerParams, Confirm: new(false)},
				},
			}

			outer := config.Set{
				Commands: []config.Command{
					{Action: "run_set", Params: tc.forwarded},
				},
			}

			cfg := minimalCfg(map[string]config.Set{
				"outer": outer,
				"inner": inner,
			})

			rs, err := Build("outer", outer, cfg, 0, nil)
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}

			play := extractPlay(rs)
			if play == nil {
				t.Fatal("no play action found in nested RunSet")
			}
			if play.ContextURI != tc.wantURI {
				t.Errorf("ContextURI: got %q, want %q", play.ContextURI, tc.wantURI)
			}
		})
	}
}

// ----- params: default, missing required, default override -------------------

func TestBuildParams(t *testing.T) {
	makeSet := func(paramName, templateField string) config.Set {
		return config.Set{
			Params: map[string]config.SetParam{
				paramName: {Required: true},
				"volume":  {Default: "35"},
			},
			Commands: []config.Command{
				{Action: "play", Params: config.CommandParams{URI: `{{ ` + templateField + ` }}`}, Confirm: new(false)},
			},
		}
	}

	t.Run("default param used when arg absent", func(t *testing.T) {
		set := makeSet("uri", "uri")
		_, err := Build("test", set, minimalCfg(nil), 0, map[string]string{"uri": "spotify:playlist:x"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing required arg errors", func(t *testing.T) {
		set := makeSet("uri", "uri")
		_, err := Build("test", set, minimalCfg(nil), 0, nil)
		if err == nil || !strings.Contains(err.Error(), "uri") {
			t.Fatalf("expected missing-arg error, got %v", err)
		}
	})

	t.Run("caller arg overrides default", func(t *testing.T) {
		set := config.Set{
			Params: map[string]config.SetParam{
				"uri":    {Required: true},
				"volume": {Default: "35"},
			},
			Commands: []config.Command{
				{Action: "play", Params: config.CommandParams{URI: `{{ uri }}`}, Confirm: new(false)},
			},
		}
		rs, err := Build("test", set, minimalCfg(nil), 0, map[string]string{
			"uri": "spotify:track:override",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		play := extractPlay(rs)
		if play == nil || play.ContextURI != "spotify:track:override" {
			t.Errorf("expected overridden URI, got %v", play)
		}
	})
}

// ----- depth limit -----------------------------------------------------------

func TestBuild_MaxDepth(t *testing.T) {
	s := map[string]config.Set{
		"self": {Commands: []config.Command{
			{Action: "run_set", Params: config.CommandParams{Set: "self"}},
		}},
	}
	cfg := minimalCfg(s)
	_, err := Build("self", cfg.Sets["self"], cfg, MaxSetDepth, nil)
	if err == nil {
		t.Fatal("expected depth exceeded error")
	}
}

func TestBuild_UnknownNestedSet(t *testing.T) {
	set := config.Set{Commands: []config.Command{
		{Action: "run_set", Params: config.CommandParams{Set: "does-not-exist"}},
	}}
	_, err := Build("outer", set, minimalCfg(nil), 0, nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestBuild_UnknownAction(t *testing.T) {
	set := config.Set{Commands: []config.Command{{Action: "bogus"}}}
	_, err := Build("test", set, minimalCfg(nil), 0, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("expected unknown action error, got %v", err)
	}
}
