package sets

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jaredhoward/spotctl/config"
)

// paramLabel replaces Go template expressions like `{{ index . "key" }}` with
// the more readable `<key>` for display in the sets listing.
var templateExpr = regexp.MustCompile(`\{\{\s*index\s+\.\s+"(\w+)"\s*\}\}`)

func paramLabel(s string) string {
	return templateExpr.ReplaceAllString(s, "<$1>")
}

// CommandLabel returns a human-readable description of a command for logging
// and the sets listing.
func CommandLabel(n int, c config.Command) string {
	var parts []string
	parts = append(parts, c.Action)

	switch c.Action {
	case "play":
		switch {
		case c.Params.URI != "":
			parts = append(parts, fmt.Sprintf("uri=%s", paramLabel(c.Params.URI)))
		case c.Params.PlaylistID != "":
			parts = append(parts, fmt.Sprintf("playlist=%s", paramLabel(c.Params.PlaylistID)))
		case c.Params.TrackID != "":
			parts = append(parts, fmt.Sprintf("track=%s", paramLabel(c.Params.TrackID)))
		case c.Params.AlbumID != "":
			parts = append(parts, fmt.Sprintf("album=%s", paramLabel(c.Params.AlbumID)))
		case c.Params.ArtistID != "":
			parts = append(parts, fmt.Sprintf("artist=%s", paramLabel(c.Params.ArtistID)))
		}
	case "volume":
		if c.Params.Level != nil {
			parts = append(parts, fmt.Sprintf("level=%d", *c.Params.Level))
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
		for k, v := range c.Params.Args {
			parts = append(parts, fmt.Sprintf("arg:%s=%s", k, paramLabel(v)))
		}
	}

	if c.DeviceID != "" {
		parts = append(parts, fmt.Sprintf("device=%s", c.DeviceID))
	}
	parts = append(parts, fmt.Sprintf("confirm=%v", c.EffectiveConfirm(nil)))
	if c.Name != "" {
		return fmt.Sprintf("%s (%s)", strings.Join(parts, " "), c.Name)
	}
	return strings.Join(parts, " ")
}
