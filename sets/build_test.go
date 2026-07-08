package sets

import (
	"strings"
	"testing"
	"time"

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
			rs, err := Build("test", set, minimalCfg(nil), 0, nil, "")
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
	_, err := Build("test", set, minimalCfg(nil), 0, nil, "")
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

			rs, err := Build("outer", outer, cfg, 0, nil, "")
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
		_, err := Build("test", set, minimalCfg(nil), 0, map[string]string{"uri": "spotify:playlist:x"}, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing required arg errors", func(t *testing.T) {
		set := makeSet("uri", "uri")
		_, err := Build("test", set, minimalCfg(nil), 0, nil, "")
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
		}, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		play := extractPlay(rs)
		if play == nil || play.ContextURI != "spotify:track:override" {
			t.Errorf("expected overridden URI, got %v", play)
		}
	})
}

// ----- pool params ------------------------------------------------------------

func TestBuild_StepLabelIncludesResolvedActionDetail(t *testing.T) {
	set := config.Set{
		Commands: []config.Command{
			{Action: "play", Params: config.CommandParams{URI: "spotify:playlist:abc123"}, Confirm: new(false)},
		},
	}
	rs, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	label := rs.Steps[0].label
	if !strings.Contains(label, "uri=spotify:playlist:abc123") {
		t.Errorf("expected resolved uri in step label, got %q", label)
	}
}

func TestBuild_StepLabelWithNameStillIncludesResolvedActionDetail(t *testing.T) {
	set := config.Set{
		Commands: []config.Command{
			{Action: "play", Name: "nightly playlist", Params: config.CommandParams{URI: "spotify:playlist:abc123"}, Confirm: new(false)},
		},
	}
	rs, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	label := rs.Steps[0].label
	if !strings.Contains(label, "nightly playlist") || !strings.Contains(label, "uri=spotify:playlist:abc123") {
		t.Errorf("expected both custom name and resolved uri in step label, got %q", label)
	}
}

func TestBuild_StepLabelWithPoolShowsActualPick(t *testing.T) {
	oldNow := config.Now
	defer func() { config.Now = oldNow }()
	config.Now = func() time.Time { return time.Date(2026, 7, 6, 22, 0, 0, 0, time.UTC) }

	pool := []string{"spotify:playlist:a", "spotify:playlist:b", "spotify:playlist:c"}
	set := config.Set{
		Params: map[string]config.SetParam{"uri": {Pool: pool, Method: config.PoolMethodDate}},
		Commands: []config.Command{
			{Action: "play", Params: config.CommandParams{URI: "{{ uri }}"}, Confirm: new(false)},
		},
	}
	rs, err := Build("jareds_sleep", set, minimalCfg(nil), 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	label := rs.Steps[0].label
	found := false
	for _, p := range pool {
		if strings.Contains(label, "uri="+p) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected step label to contain the actual pool pick (not the raw {{ uri }} template), got %q", label)
	}
}

func TestBuildParams_PoolResolvesToPoolMember(t *testing.T) {
	oldNow := config.Now
	defer func() { config.Now = oldNow }()
	config.Now = func() time.Time { return time.Date(2026, 7, 6, 22, 0, 0, 0, time.UTC) }

	pool := []string{"spotify:playlist:a", "spotify:playlist:b", "spotify:playlist:c"}
	set := config.Set{
		Params: map[string]config.SetParam{
			"uri": {Pool: pool, Method: config.PoolMethodDate},
		},
		Commands: []config.Command{
			{Action: "play", Params: config.CommandParams{URI: `{{ uri }}`}, Confirm: new(false)},
		},
	}

	rs, err := Build("jareds_sleep", set, minimalCfg(nil), 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	play := extractPlay(rs)
	if play == nil {
		t.Fatal("no play action found")
	}
	found := false
	for _, p := range pool {
		if play.ContextURI == p {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ContextURI to be a pool member, got %q", play.ContextURI)
	}
}

func TestBuildParams_PoolViaNestedRunSetIsScopedToInnerSetName(t *testing.T) {
	oldNow := config.Now
	defer func() { config.Now = oldNow }()
	config.Now = func() time.Time { return time.Date(2026, 7, 6, 22, 0, 0, 0, time.UTC) }

	pool := []string{"spotify:playlist:a", "spotify:playlist:b", "spotify:playlist:c"}
	inner := config.Set{
		Params: map[string]config.SetParam{
			"uri": {Pool: pool, Method: config.PoolMethodDate},
		},
		Commands: []config.Command{
			{Action: "play", Params: config.CommandParams{URI: `{{ uri }}`}, Confirm: new(false)},
		},
	}
	outer := config.Set{
		Commands: []config.Command{
			{Action: "run_set", Params: config.CommandParams{Set: "inner"}},
		},
	}
	cfg := minimalCfg(map[string]config.Set{"outer": outer, "inner": inner})

	rsOuter, err := Build("outer", outer, cfg, 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error building via outer: %v", err)
	}
	viaOuter := extractPlay(rsOuter)
	if viaOuter == nil {
		t.Fatal("no play action found via outer")
	}

	// Resolving the inner set directly (as "inner") must produce the same pick
	// as resolving it through the outer set's run_set, since the pool's hash
	// seed is scoped to the *inner* set's own name either way.
	rsDirect, err := Build("inner", inner, cfg, 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error building inner directly: %v", err)
	}
	direct := extractPlay(rsDirect)
	if direct == nil {
		t.Fatal("no play action found via direct inner build")
	}

	if viaOuter.ContextURI != direct.ContextURI {
		t.Errorf("expected pool pick to be scoped to inner set name regardless of caller, got %q via outer vs %q direct",
			viaOuter.ContextURI, direct.ContextURI)
	}
}

// ----- depth limit -----------------------------------------------------------

func TestBuild_MaxDepth(t *testing.T) {
	s := map[string]config.Set{
		"self": {Commands: []config.Command{
			{Action: "run_set", Params: config.CommandParams{Set: "self"}},
		}},
	}
	cfg := minimalCfg(s)
	_, err := Build("self", cfg.Sets["self"], cfg, MaxSetDepth, nil, "")
	if err == nil {
		t.Fatal("expected depth exceeded error")
	}
}

func TestBuild_UnknownNestedSet(t *testing.T) {
	set := config.Set{Commands: []config.Command{
		{Action: "run_set", Params: config.CommandParams{Set: "does-not-exist"}},
	}}
	_, err := Build("outer", set, minimalCfg(nil), 0, nil, "")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestBuild_UnknownAction(t *testing.T) {
	set := config.Set{Commands: []config.Command{{Action: "bogus"}}}
	_, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("expected unknown action error, got %v", err)
	}
}

// buildAction is reachable for any action that passes Validate. These tests
// exercise the branches not covered by integration-style tests in sets_test.go.

func TestBuildAction_Previous(t *testing.T) {
	set := config.Set{Commands: []config.Command{{Action: "previous", DeviceID: "dev1", Confirm: new(false)}}}
	rs, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rs.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(rs.Steps))
	}
	prev, ok := rs.Steps[0].action.(*spotify.Previous)
	if !ok {
		t.Fatalf("expected *spotify.Previous, got %T", rs.Steps[0].action)
	}
	if prev.DeviceID != "dev1" {
		t.Errorf("expected DeviceID=dev1, got %q", prev.DeviceID)
	}
}

func TestBuildAction_Shuffle(t *testing.T) {
	enabled := true
	set := config.Set{Commands: []config.Command{
		{Action: "shuffle", DeviceID: "dev1", Params: config.CommandParams{Enabled: &enabled}, Confirm: new(false)},
	}}
	rs, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, ok := rs.Steps[0].action.(*spotify.Shuffle)
	if !ok {
		t.Fatalf("expected *spotify.Shuffle, got %T", rs.Steps[0].action)
	}
	if !a.Enabled {
		t.Error("expected Enabled=true")
	}
}

func TestBuildAction_Repeat(t *testing.T) {
	set := config.Set{Commands: []config.Command{
		{Action: "repeat", DeviceID: "dev1", Params: config.CommandParams{RepeatState: "track"}, Confirm: new(false)},
	}}
	rs, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, ok := rs.Steps[0].action.(*spotify.Repeat)
	if !ok {
		t.Fatalf("expected *spotify.Repeat, got %T", rs.Steps[0].action)
	}
	if a.State != "track" {
		t.Errorf("expected State=track, got %q", a.State)
	}
}

func TestBuildAction_Volume(t *testing.T) {
	level := config.IntOrTemplate{Value: 75}
	set := config.Set{Commands: []config.Command{
		{Action: "volume", DeviceID: "dev1", Params: config.CommandParams{Level: &level}, Confirm: new(false)},
	}}
	rs, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, ok := rs.Steps[0].action.(*spotify.Volume)
	if !ok {
		t.Fatalf("expected *spotify.Volume, got %T", rs.Steps[0].action)
	}
	if a.Level != 75 {
		t.Errorf("expected Level=75, got %d", a.Level)
	}
}

func TestBuildAction_VolumeUnresolvedExpr(t *testing.T) {
	// Call buildAction directly with an Expr that was not resolved to an int;
	// Resolved() must return an error.
	level := config.IntOrTemplate{Expr: "still-a-template"}
	cmd := config.Command{
		Action: "volume",
		Params: config.CommandParams{Level: &level},
	}
	_, err := buildAction(cmd, "", minimalCfg(nil), 0, "")
	if err == nil || !strings.Contains(err.Error(), "not resolved") {
		t.Fatalf("expected unresolved expression error from buildAction, got %v", err)
	}
}

func TestBuildAction_Transfer(t *testing.T) {
	play := true
	set := config.Set{Commands: []config.Command{
		{Action: "transfer", DeviceID: "target", Params: config.CommandParams{Play: &play}, Confirm: new(false)},
	}}
	rs, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, ok := rs.Steps[0].action.(*spotify.Transfer)
	if !ok {
		t.Fatalf("expected *spotify.Transfer, got %T", rs.Steps[0].action)
	}
	if !a.Play {
		t.Error("expected Play=true")
	}
}

func TestBuildAction_UnknownDirectly(t *testing.T) {
	cmd := config.Command{Action: "bogus"}
	_, err := buildAction(cmd, "", minimalCfg(nil), 0, "")
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("expected unknown action error from buildAction, got %v", err)
	}
}

// ----- device_id templating ---------------------------------------------------

func TestBuild_SetLevelDeviceIDTemplated(t *testing.T) {
	set := config.Set{
		DeviceID: "{{ device }}",
		Params: map[string]config.SetParam{
			"device": {Default: "dev-default"},
		},
		Commands: []config.Command{
			{Action: "pause", Confirm: new(false)},
		},
	}
	rs, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, ok := rs.Steps[0].action.(*spotify.Pause)
	if !ok {
		t.Fatalf("expected *spotify.Pause, got %T", rs.Steps[0].action)
	}
	if a.DeviceID != "dev-default" {
		t.Errorf("DeviceID: got %q, want dev-default", a.DeviceID)
	}
}

func TestBuild_SetLevelDeviceIDTemplated_ArgOverride(t *testing.T) {
	set := config.Set{
		DeviceID: "{{ device }}",
		Params: map[string]config.SetParam{
			"device": {Default: "dev-default"},
		},
		Commands: []config.Command{
			{Action: "pause", Confirm: new(false)},
		},
	}
	rs, err := Build("test", set, minimalCfg(nil), 0, map[string]string{"device": "dev-override"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := rs.Steps[0].action.(*spotify.Pause)
	if a.DeviceID != "dev-override" {
		t.Errorf("DeviceID: got %q, want dev-override", a.DeviceID)
	}
}

func TestBuild_CommandLevelDeviceIDOverridesSet(t *testing.T) {
	set := config.Set{
		DeviceID: "set-device",
		Params: map[string]config.SetParam{
			"device": {Default: "cmd-device"},
		},
		Commands: []config.Command{
			{Action: "pause", DeviceID: "{{ device }}", Confirm: new(false)},
		},
	}
	rs, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := rs.Steps[0].action.(*spotify.Pause)
	if a.DeviceID != "cmd-device" {
		t.Errorf("DeviceID: got %q, want cmd-device (command override)", a.DeviceID)
	}
}

func TestBuild_DeviceIDMissingPlaceholderErrors(t *testing.T) {
	set := config.Set{
		DeviceID: "{{ undeclared }}",
		Commands: []config.Command{
			{Action: "pause"},
		},
	}
	_, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err == nil || !strings.Contains(err.Error(), "undeclared") {
		t.Fatalf("expected undeclared placeholder error, got %v", err)
	}
}

func TestBuild_CommandDeviceIDMissingPlaceholderErrors(t *testing.T) {
	set := config.Set{
		Commands: []config.Command{
			{Action: "pause", DeviceID: "{{ undeclared }}"},
		},
	}
	_, err := Build("test", set, minimalCfg(nil), 0, nil, "")
	if err == nil || !strings.Contains(err.Error(), "undeclared") {
		t.Fatalf("expected undeclared placeholder error, got %v", err)
	}
}

func TestBuild_RunSet_ForwardsDeviceIDToNestedSet(t *testing.T) {
	inner := config.Set{
		Params: map[string]config.SetParam{
			"device": {Required: true},
		},
		Commands: []config.Command{
			{Action: "play", DeviceID: "{{ device }}", Confirm: new(false)},
		},
	}
	outer := config.Set{
		Params: map[string]config.SetParam{
			"device": {Default: "forwarded-device"},
		},
		Commands: []config.Command{
			{Action: "run_set", DeviceID: "{{ device }}", Params: config.CommandParams{Set: "inner"}},
		},
	}
	cfg := minimalCfg(map[string]config.Set{"outer": outer, "inner": inner})

	rs, err := Build("outer", outer, cfg, 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	play := extractPlay(rs)
	if play == nil {
		t.Fatal("no play action found in nested RunSet")
	}
	if play.DeviceID != "forwarded-device" {
		t.Errorf("DeviceID: got %q, want forwarded-device", play.DeviceID)
	}
}

func TestBuild_RunSet_NoDeviceOverridePreservesInnerDefault(t *testing.T) {
	inner := config.Set{
		DeviceID: "{{ device }}",
		Params: map[string]config.SetParam{
			"device": {Default: "inner-default-device"},
		},
		Commands: []config.Command{
			{Action: "play", Confirm: new(false)},
		},
	}
	outer := config.Set{
		// No device_id at all on the outer set or its run_set command — the
		// forwarded deviceID is empty, so the inner set's own default must
		// survive untouched (regression guard for the empty-string-overrides
		// default footgun in ResolveParams).
		Commands: []config.Command{
			{Action: "run_set", Params: config.CommandParams{Set: "inner"}},
		},
	}
	cfg := minimalCfg(map[string]config.Set{"outer": outer, "inner": inner})

	rs, err := Build("outer", outer, cfg, 0, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	play := extractPlay(rs)
	if play == nil {
		t.Fatal("no play action found in nested RunSet")
	}
	if play.DeviceID != "inner-default-device" {
		t.Errorf("DeviceID: got %q, want inner-default-device (inner default must survive)", play.DeviceID)
	}
}

// ----- deviceOverride (built-in --device flag) --------------------------------

func TestBuild_DeviceOverrideAppliesToAllCommands(t *testing.T) {
	set := config.Set{
		DeviceID: "configured-device",
		Commands: []config.Command{
			{Action: "pause", Confirm: new(false)},
			{Action: "play", DeviceID: "cmd-device", Params: config.CommandParams{PlaylistID: "pl1"}, Confirm: new(false)},
		},
	}
	rs, err := Build("test", set, minimalCfg(nil), 0, nil, "override-device")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pause := rs.Steps[0].action.(*spotify.Pause)
	if pause.DeviceID != "override-device" {
		t.Errorf("pause DeviceID: got %q, want override-device", pause.DeviceID)
	}
	play := rs.Steps[1].action.(*spotify.Play)
	if play.DeviceID != "override-device" {
		t.Errorf("play DeviceID: got %q, want override-device (must win over configured cmd-device)", play.DeviceID)
	}
}

func TestBuild_DeviceOverridePropagatesToNestedRunSet(t *testing.T) {
	inner := config.Set{
		DeviceID: "inner-configured-device",
		Commands: []config.Command{
			{Action: "play", Params: config.CommandParams{PlaylistID: "pl1"}, Confirm: new(false)},
		},
	}
	outer := config.Set{
		DeviceID: "outer-configured-device",
		Commands: []config.Command{
			{Action: "run_set", Params: config.CommandParams{Set: "inner"}},
		},
	}
	cfg := minimalCfg(map[string]config.Set{"outer": outer, "inner": inner})

	rs, err := Build("outer", outer, cfg, 0, nil, "override-device")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	play := extractPlay(rs)
	if play == nil {
		t.Fatal("no play action found in nested RunSet")
	}
	if play.DeviceID != "override-device" {
		t.Errorf("DeviceID: got %q, want override-device to propagate into nested run_set", play.DeviceID)
	}
}

func TestBuild_DeviceOverrideBypassesBrokenTemplate(t *testing.T) {
	// Mirrors the jareds_sleep-shaped bug: device_id templates {{ device }}
	// but the set never declares a "device" param, so normal resolution
	// would error. An active override must bypass that resolution entirely.
	set := config.Set{
		Commands: []config.Command{
			{Action: "run_set", DeviceID: "{{ device }}", Params: config.CommandParams{Set: "inner"}},
		},
	}
	inner := config.Set{
		Commands: []config.Command{
			{Action: "play", Params: config.CommandParams{PlaylistID: "pl1"}, Confirm: new(false)},
		},
	}
	cfg := minimalCfg(map[string]config.Set{"outer": set, "inner": inner})

	if _, err := Build("outer", set, cfg, 0, nil, ""); err == nil {
		t.Fatal("expected the broken template to error without an override")
	}

	rs, err := Build("outer", set, cfg, 0, nil, "override-device")
	if err != nil {
		t.Fatalf("expected override to bypass the broken template, got error: %v", err)
	}
	play := extractPlay(rs)
	if play == nil {
		t.Fatal("no play action found in nested RunSet")
	}
	if play.DeviceID != "override-device" {
		t.Errorf("DeviceID: got %q, want override-device", play.DeviceID)
	}
}
