package sets

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jaredhoward/spotctl/config"
)

// ParamLabel replaces {{ name }} placeholders with the more readable <name>
// for display in the sets listing.
var templateExpr = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

func ParamLabel(s string) string {
	return templateExpr.ReplaceAllString(s, "<$1>")
}

// actionDetail returns the action-specific portion of a command's label
// (e.g. "uri=..." for play, "level=..." for volume). By the time this is
// called from Build, {{ }} placeholders in c.Params have already been
// resolved to their run-time values, so this reports what will actually
// happen — e.g. which pool-picked playlist is about to play.
func actionDetail(c config.Command) []string {
	var parts []string
	switch c.Action {
	case "play":
		switch {
		case c.Params.URI != "":
			parts = append(parts, fmt.Sprintf("uri=%s", ParamLabel(c.Params.URI)))
		case c.Params.PlaylistID != "":
			parts = append(parts, fmt.Sprintf("playlist=%s", ParamLabel(c.Params.PlaylistID)))
		case c.Params.TrackID != "":
			parts = append(parts, fmt.Sprintf("track=%s", ParamLabel(c.Params.TrackID)))
		case c.Params.AlbumID != "":
			parts = append(parts, fmt.Sprintf("album=%s", ParamLabel(c.Params.AlbumID)))
		case c.Params.ArtistID != "":
			parts = append(parts, fmt.Sprintf("artist=%s", ParamLabel(c.Params.ArtistID)))
		}
	case "volume":
		if c.Params.Level != nil {
			if c.Params.Level.Expr != "" {
				parts = append(parts, fmt.Sprintf("level=%s", ParamLabel(c.Params.Level.Expr)))
			} else {
				parts = append(parts, fmt.Sprintf("level=%d", c.Params.Level.Value))
			}
		}
	case "shuffle":
		parts = append(parts, fmt.Sprintf("enabled=%v", c.Params.ShuffleEnabled()))
	case "repeat":
		if c.Params.RepeatState != "" {
			parts = append(parts, fmt.Sprintf("state=%s", c.Params.RepeatState))
		}
	case "sleep":
		if c.Params.Duration != "" {
			parts = append(parts, fmt.Sprintf("duration=%s", c.Params.Duration))
		}
	case "run_set":
		if c.Params.Set != "" {
			parts = append(parts, fmt.Sprintf("set=%s", c.Params.Set))
		}
		for k, v := range c.Params.ForwardedArgs() {
			parts = append(parts, fmt.Sprintf("%s=%s", k, ParamLabel(v)))
		}
	}
	return parts
}

// CommandLabel returns a human-readable description of a command for the
// sets listing (spotctl sets).
func CommandLabel(n int, c config.Command) string {
	parts := append([]string{c.Action}, actionDetail(c)...)

	if c.DeviceID != "" {
		parts = append(parts, fmt.Sprintf("device=%s", ParamLabel(c.DeviceID)))
	}
	parts = append(parts, fmt.Sprintf("confirm=%v", c.EffectiveConfirm(nil)))
	if c.Name != "" {
		return fmt.Sprintf("%s (%s)", strings.Join(parts, " "), c.Name)
	}
	return strings.Join(parts, " ")
}

// ActionDetail returns the resolved action-specific and device details for c
// (e.g. "uri=... device=..."), without the confirm/name suffix CommandLabel
// adds. Used for RunSet's dispatch log, which already logs the authoritative
// confirm value sourced from ExecuteOptions rather than recomputing one from
// a nil set-level default the way CommandLabel does for the static listing.
func ActionDetail(c config.Command) string {
	parts := actionDetail(c)
	if c.DeviceID != "" {
		parts = append(parts, fmt.Sprintf("device=%s", ParamLabel(c.DeviceID)))
	}
	return strings.Join(parts, " ")
}
