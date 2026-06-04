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

// ----- TestBuildWithParams ---------------------------------------------------

func TestBuildWithParams(t *testing.T) {
	playSet := config.Set{
		Params: map[string]config.SetParam{
			"uri":    {Required: true},
			"volume": {Default: "35"},
		},
		Commands: []config.Command{
			{
				Action: "play",
				Params: config.CommandParams{URI: `{{ index . "uri" }}`},
			},
		},
	}
	cfg := minimalCfg(map[string]config.Set{"play_set": playSet})

	rs, err := Build("play_set", playSet, cfg, 0, map[string]string{
		"uri": "spotify:playlist:abc123",
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(rs.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(rs.Steps))
	}

	play, ok := rs.Steps[0].action.(*spotify.Play)
	if !ok {
		t.Fatalf("expected *spotify.Play action, got %T", rs.Steps[0].action)
	}
	if play.ContextURI != "spotify:playlist:abc123" {
		t.Errorf("ContextURI: got %q, want spotify:playlist:abc123", play.ContextURI)
	}
}

// ----- TestBuildDefaultParam -------------------------------------------------

func TestBuildDefaultParam(t *testing.T) {
	// volume param has a default; no arg supplied — should not error.
	playSet := config.Set{
		Params: map[string]config.SetParam{
			"uri":    {Required: true},
			"volume": {Default: "35"},
		},
		Commands: []config.Command{
			{
				Action: "play",
				Params: config.CommandParams{URI: `{{ index . "uri" }}`},
			},
		},
	}
	cfg := minimalCfg(map[string]config.Set{"play_set": playSet})

	_, err := Build("play_set", playSet, cfg, 0, map[string]string{
		"uri": "spotify:playlist:xyz",
		// volume intentionally omitted — default should apply
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ----- TestBuildMissingRequiredParam -----------------------------------------

func TestBuildMissingRequiredParam(t *testing.T) {
	playSet := config.Set{
		Params: map[string]config.SetParam{
			"uri": {Required: true},
		},
		Commands: []config.Command{
			{
				Action: "play",
				Params: config.CommandParams{URI: `{{ index . "uri" }}`},
			},
		},
	}
	cfg := minimalCfg(map[string]config.Set{"play_set": playSet})

	_, err := Build("play_set", playSet, cfg, 0, nil)
	if err == nil {
		t.Fatal("expected error for missing required param")
	}
	if !strings.Contains(err.Error(), "uri") {
		t.Errorf("expected param name in error, got: %v", err)
	}
}

// ----- TestBuildRunSetPassesArgs ---------------------------------------------

func TestBuildRunSetPassesArgs(t *testing.T) {
	// inner set expects a required uri param
	inner := config.Set{
		Params: map[string]config.SetParam{
			"uri": {Required: true},
		},
		Commands: []config.Command{
			{
				Action: "play",
				Params: config.CommandParams{URI: `{{ index . "uri" }}`},
			},
		},
	}

	// outer set calls inner via run_set with args
	outer := config.Set{
		Commands: []config.Command{
			{
				Action: "run_set",
				Params: config.CommandParams{
					Set:  "inner",
					Args: map[string]string{"uri": "spotify:playlist:passed"},
				},
			},
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

	if len(rs.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(rs.Steps))
	}

	// The run_set step is itself a *RunSet
	inner_rs, ok := rs.Steps[0].action.(*RunSet)
	if !ok {
		t.Fatalf("expected nested *RunSet, got %T", rs.Steps[0].action)
	}
	if len(inner_rs.Steps) != 1 {
		t.Fatalf("expected 1 inner step, got %d", len(inner_rs.Steps))
	}

	play, ok := inner_rs.Steps[0].action.(*spotify.Play)
	if !ok {
		t.Fatalf("expected *spotify.Play in inner set, got %T", inner_rs.Steps[0].action)
	}
	if play.ContextURI != "spotify:playlist:passed" {
		t.Errorf("ContextURI: got %q, want spotify:playlist:passed", play.ContextURI)
	}
}
