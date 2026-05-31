package runner

import (
	"fmt"
	"strings"

	"github.com/jaredhoward/spotctl/config"
)

// CommandLabel returns a human-readable description of a command for display.
func CommandLabel(n int, c config.Command) string {
	var parts []string
	parts = append(parts, c.Action)

	switch c.Action {
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
	case "play":
		switch {
		case c.Params.URI != "":
			parts = append(parts, fmt.Sprintf("uri=%s", c.Params.URI))
		case c.Params.PlaylistID != "":
			parts = append(parts, fmt.Sprintf("playlist=%s", c.Params.PlaylistID))
		case c.Params.TrackID != "":
			parts = append(parts, fmt.Sprintf("track=%s", c.Params.TrackID))
		case c.Params.AlbumID != "":
			parts = append(parts, fmt.Sprintf("album=%s", c.Params.AlbumID))
		case c.Params.ArtistID != "":
			parts = append(parts, fmt.Sprintf("artist=%s", c.Params.ArtistID))
		}
	case "transfer":
		if c.DeviceID != "" {
			parts = append(parts, fmt.Sprintf("device=%s", c.DeviceID))
		}
	}

	if c.DeviceID != "" && c.Action != "transfer" {
		parts = append(parts, fmt.Sprintf("device=%s", c.DeviceID))
	}
	if c.Confirm {
		parts = append(parts, "confirm")
	}
	if c.Name != "" {
		return fmt.Sprintf("%s (%s)", strings.Join(parts, " "), c.Name)
	}
	return strings.Join(parts, " ")
}
