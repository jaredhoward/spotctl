package runner

import (
	"fmt"
	"log"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// DispatchAction is the single execution point for every Spotify command,
// whether invoked from a CLI verb or a set. deviceID is already resolved
// (command-level wins over set-level; empty means active device).
func DispatchAction(p config.CommandParams, action string, deviceID string, client *spotify.Client, cfg *config.Config, depth int) error {
	switch action {
	case "play":
		contextURI, err := ResolveURIFromParams(p)
		if err != nil {
			return err
		}
		return client.Play(deviceID, contextURI)

	case "pause":
		return client.Pause(deviceID)

	case "next":
		return client.Next(deviceID)

	case "previous":
		return client.Previous(deviceID)

	case "shuffle":
		return client.Shuffle(deviceID, p.ShuffleEnabled())

	case "repeat":
		return client.Repeat(deviceID, p.RepeatState)

	case "volume":
		return client.Volume(deviceID, *p.Level)

	case "transfer":
		return client.TransferPlayback([]string{deviceID}, p.TransferPlay())

	case "sleep":
		d, _ := time.ParseDuration(p.Duration) // already validated
		log.Printf("sleeping %s", d)
		time.Sleep(d)
		return nil

	case "run_set":
		sub, ok := cfg.Sets[p.Set]
		if !ok {
			return fmt.Errorf("set %q not found", p.Set)
		}
		return RunSet(p.Set, sub, cfg, client, depth+1)

	default:
		return fmt.Errorf("unknown action %q", action)
	}
}

// ResolveURIFromParams builds a Spotify context URI from the convenience
// fields in CommandParams. Returns an error if more than one is set.
func ResolveURIFromParams(p config.CommandParams) (string, error) {
	count := 0
	if p.URI != "" {
		count++
	}
	if p.PlaylistID != "" {
		count++
	}
	if p.TrackID != "" {
		count++
	}
	if p.AlbumID != "" {
		count++
	}
	if p.ArtistID != "" {
		count++
	}
	if count > 1 {
		return "", fmt.Errorf("only one of uri/playlist/track/album/artist may be set in params")
	}
	switch {
	case p.URI != "":
		return p.URI, nil
	case p.PlaylistID != "":
		return "spotify:playlist:" + p.PlaylistID, nil
	case p.TrackID != "":
		return "spotify:track:" + p.TrackID, nil
	case p.AlbumID != "":
		return "spotify:album:" + p.AlbumID, nil
	case p.ArtistID != "":
		return "spotify:artist:" + p.ArtistID, nil
	}
	return "", nil
}
