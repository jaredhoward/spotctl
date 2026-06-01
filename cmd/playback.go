package cmd

import (
	"context"
	"fmt"

	"github.com/jaredhoward/spotctl/sets"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

// ---- play -------------------------------------------------------------------

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

	a := &spotify.Play{DeviceID: playDeviceID, ContextURI: contextURI}
	if err := sets.Execute(context.Background(), a, client, sets.ExecuteOptions{}); err != nil {
		return fmt.Errorf("play failed: %w", err)
	}
	return nil
}

// resolveURI validates that at most one URI-type flag was set and builds the
// full Spotify URI string from whichever shorthand flag was used.
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

// ---- pause / next / previous ------------------------------------------------

var pauseDeviceID string
var nextDeviceID string
var previousDeviceID string

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause Spotify playback",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClientFromConfig()
		if err != nil {
			return err
		}
		if err := sets.Execute(context.Background(), &spotify.Pause{DeviceID: pauseDeviceID}, client, sets.ExecuteOptions{}); err != nil {
			return fmt.Errorf("pause failed: %w", err)
		}
		return nil
	},
}

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Skip to the next track",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClientFromConfig()
		if err != nil {
			return err
		}
		if err := sets.Execute(context.Background(), &spotify.Next{DeviceID: nextDeviceID}, client, sets.ExecuteOptions{}); err != nil {
			return fmt.Errorf("next failed: %w", err)
		}
		return nil
	},
}

var previousCmd = &cobra.Command{
	Use:   "previous",
	Short: "Return to the previous track",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClientFromConfig()
		if err != nil {
			return err
		}
		if err := sets.Execute(context.Background(), &spotify.Previous{DeviceID: previousDeviceID}, client, sets.ExecuteOptions{}); err != nil {
			return fmt.Errorf("previous failed: %w", err)
		}
		return nil
	},
}

// ---- shuffle ----------------------------------------------------------------

var shuffleDeviceID string
var shuffleEnabled bool

var shuffleCmd = &cobra.Command{
	Use:   "shuffle",
	Short: "Enable or disable shuffle",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClientFromConfig()
		if err != nil {
			return err
		}
		if err := sets.Execute(context.Background(), &spotify.Shuffle{DeviceID: shuffleDeviceID, Enabled: shuffleEnabled}, client, sets.ExecuteOptions{}); err != nil {
			return fmt.Errorf("shuffle failed: %w", err)
		}
		return nil
	},
}

// ---- repeat -----------------------------------------------------------------

var repeatDeviceID string
var repeatState string

var repeatCmd = &cobra.Command{
	Use:   "repeat",
	Short: "Set repeat mode (off, track, context)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if repeatState != "off" && repeatState != "track" && repeatState != "context" {
			return fmt.Errorf("--state must be one of: off, track, context (got %q)", repeatState)
		}
		client, err := newClientFromConfig()
		if err != nil {
			return err
		}
		if err := sets.Execute(context.Background(), &spotify.Repeat{DeviceID: repeatDeviceID, State: repeatState}, client, sets.ExecuteOptions{}); err != nil {
			return fmt.Errorf("repeat failed: %w", err)
		}
		return nil
	},
}

// ---- transfer ---------------------------------------------------------------

var transferDeviceID string
var transferPlay bool

var transferCmd = &cobra.Command{
	Use:   "transfer",
	Short: "Transfer playback to a Spotify Connect device",
	RunE:  runTransfer,
}

func runTransfer(cmd *cobra.Command, args []string) error {
	if transferDeviceID == "" {
		return fmt.Errorf("transfer requires --device")
	}

	client, err := newClientFromConfig()
	if err != nil {
		return err
	}

	a := &spotify.Transfer{DeviceID: transferDeviceID, Play: transferPlay}
	if err := sets.Execute(context.Background(), a, client, sets.ExecuteOptions{}); err != nil {
		return fmt.Errorf("transfer failed: %w", err)
	}
	return nil
}

// ---- volume -----------------------------------------------------------------

var volumeDeviceID string
var volumeLevel int

var volumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "Set the volume on a Spotify Connect device",
	RunE:  runVolume,
}

func runVolume(cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Changed("level") {
		return fmt.Errorf("volume requires --level (0–100)")
	}
	if volumeLevel < 0 || volumeLevel > 100 {
		return fmt.Errorf("volume must be between 0 and 100 (got %d)", volumeLevel)
	}

	client, err := newClientFromConfig()
	if err != nil {
		return err
	}

	a := &spotify.Volume{DeviceID: volumeDeviceID, Level: volumeLevel}
	if err := sets.Execute(context.Background(), a, client, sets.ExecuteOptions{}); err != nil {
		return fmt.Errorf("volume failed: %w", err)
	}
	return nil
}

// ---- init -------------------------------------------------------------------

func init() {
	// play
	playCmd.Flags().StringVar(&playDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	playCmd.Flags().StringVar(&uri, "uri", "", "Spotify context URI (e.g. spotify:artist:xxx)")
	playCmd.Flags().StringVar(&playlistID, "playlist", "", "playlist ID (shorthand for --uri spotify:playlist:ID)")
	playCmd.Flags().StringVar(&trackID, "track", "", "track ID (shorthand for --uri spotify:track:ID)")
	playCmd.Flags().StringVar(&albumID, "album", "", "album ID (shorthand for --uri spotify:album:ID)")
	playCmd.Flags().StringVar(&artistID, "artist", "", "artist ID (shorthand for --uri spotify:artist:ID)")

	// pause / next / previous
	pauseCmd.Flags().StringVar(&pauseDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	nextCmd.Flags().StringVar(&nextDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	previousCmd.Flags().StringVar(&previousDeviceID, "device", "", "Spotify device ID (omit to target active device)")

	// shuffle
	shuffleCmd.Flags().StringVar(&shuffleDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	shuffleCmd.Flags().BoolVar(&shuffleEnabled, "enabled", true, "enable shuffle (use --enabled=false to disable)")

	// repeat
	repeatCmd.Flags().StringVar(&repeatDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	repeatCmd.Flags().StringVar(&repeatState, "state", "context", "repeat mode: off, track, context")

	// transfer
	transferCmd.Flags().StringVar(&transferDeviceID, "device", "", "Spotify device ID to transfer playback to (required)")
	transferCmd.Flags().BoolVar(&transferPlay, "play", false, "start playback after transfer")

	// volume
	volumeCmd.Flags().StringVar(&volumeDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	volumeCmd.Flags().IntVar(&volumeLevel, "level", 0, "volume level (0–100, required)")

	rootCmd.AddCommand(playCmd, pauseCmd, nextCmd, previousCmd, shuffleCmd, repeatCmd, transferCmd, volumeCmd)
}
