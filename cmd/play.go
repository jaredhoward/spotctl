package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/spf13/cobra"
)

var (
	playDeviceID string
	uri          string
	playlistID   string
	trackID      string
	albumID      string
	artistID     string
)

var playCmd = &cobra.Command{
	Use:   "play",
	Short: "Start or resume Spotify playback",
	RunE:  runPlay,
}

func runPlay(cmd *cobra.Command, args []string) error {
	contextURI, err := resolveURI(cmd, uri, playlistID, trackID, albumID, artistID)
	if err != nil {
		return err
	}

	client, err := newClientFromConfig()
	if err != nil {
		return err
	}

	p := config.CommandParams{
		DeviceID: playDeviceID,
		URI:      contextURI,
	}

	if err := dispatchAction(p, "play", client, nil, 0); err != nil {
		return fmt.Errorf("play failed: %w", err)
	}
	return nil
}

func resolveURI(cmd *cobra.Command, uri, playlistID, trackID, albumID, artistID string) (string, error) {
	set := []string{}
	if cmd.Flags().Changed("uri") {
		set = append(set, "--uri")
	}
	if cmd.Flags().Changed("playlist") {
		set = append(set, "--playlist")
	}
	if cmd.Flags().Changed("track") {
		set = append(set, "--track")
	}
	if cmd.Flags().Changed("album") {
		set = append(set, "--album")
	}
	if cmd.Flags().Changed("artist") {
		set = append(set, "--artist")
	}
	if len(set) > 1 {
		return "", fmt.Errorf("only one of %v may be specified at a time", set)
	}
	switch {
	case cmd.Flags().Changed("uri"):
		return uri, nil
	case cmd.Flags().Changed("playlist"):
		return "spotify:playlist:" + playlistID, nil
	case cmd.Flags().Changed("track"):
		return "spotify:track:" + trackID, nil
	case cmd.Flags().Changed("album"):
		return "spotify:album:" + albumID, nil
	case cmd.Flags().Changed("artist"):
		return "spotify:artist:" + artistID, nil
	}
	return "", nil
}

func init() {
	playCmd.Flags().StringVar(&playDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	playCmd.Flags().StringVar(&uri, "uri", "", "Spotify context URI (e.g. spotify:artist:xxx)")
	playCmd.Flags().StringVar(&playlistID, "playlist", "", "playlist ID (shorthand for --uri spotify:playlist:ID)")
	playCmd.Flags().StringVar(&trackID, "track", "", "track ID (shorthand for --uri spotify:track:ID)")
	playCmd.Flags().StringVar(&albumID, "album", "", "album ID (shorthand for --uri spotify:album:ID)")
	playCmd.Flags().StringVar(&artistID, "artist", "", "artist ID (shorthand for --uri spotify:artist:ID)")
	rootCmd.AddCommand(playCmd)
}
