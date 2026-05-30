package cmd

import (
	"fmt"
	"log"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

// dispatchAction is the single execution point for every Spotify command,
// whether invoked from a CLI verb or a set. The device_id in p is already
// resolved (command-level wins over set-level; empty means active device).
func dispatchAction(p config.CommandParams, action string, client *spotify.Client, cfg *config.Config, depth int) error {
	switch action {
	case "play":
		contextURI, err := resolveURIFromParams(p)
		if err != nil {
			return err
		}
		return client.Play(p.DeviceID, contextURI)

	case "pause":
		return client.Pause(p.DeviceID)

	case "next":
		return client.Next(p.DeviceID)

	case "previous":
		return client.Previous(p.DeviceID)

	case "shuffle":
		return client.SetShuffle(p.DeviceID, p.ShuffleEnabled())

	case "repeat":
		return client.SetRepeat(p.DeviceID, p.RepeatState)

	case "volume":
		return client.SetVolume(p.DeviceID, *p.Level)

	case "transfer":
		return client.TransferPlayback([]string{p.DeviceID}, p.TransferPlay())

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
		return runSet(p.Set, sub, cfg, client, depth+1)

	default:
		return fmt.Errorf("unknown action %q", action)
	}
}

// resolveURIFromParams builds a Spotify context URI from the convenience
// fields in CommandParams. Returns an error if more than one is set.
func resolveURIFromParams(p config.CommandParams) (string, error) {
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
