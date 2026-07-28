package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jaredhoward/spotctl/config"
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
	contextURI, err := resolvePlayURI(cmd, uri, playlistID, trackID, albumID, artistID)
	if err != nil {
		return err
	}

	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}

	return dispatchAndPrintStatus(cmdCtx(cmd), &spotify.Play{DeviceID: playDeviceID, ContextURI: contextURI}, client, "play failed")
}

// resolvePlayURI validates that at most one URI-type flag was set and builds
// the full Spotify URI from whichever shorthand flag was used.
func resolvePlayURI(cmd *cobra.Command, uri, playlistID, trackID, albumID, artistID string) (string, error) {
	type flagVal struct {
		flag string
		val  string
	}
	var set []flagVal
	for _, fv := range []flagVal{
		{"--uri", uri},
		{"--playlist", playlistID},
		{"--track", trackID},
		{"--album", albumID},
		{"--artist", artistID},
	} {
		if cmd.Flags().Changed(fv.flag[2:]) {
			if fv.val == "" {
				return "", fmt.Errorf("%s requires a non-empty value", fv.flag)
			}
			set = append(set, fv)
		}
	}
	if len(set) > 1 {
		names := make([]string, len(set))
		for i, fv := range set {
			names[i] = fv.flag
		}
		return "", fmt.Errorf("only one of %v may be specified at a time", names)
	}
	p := config.CommandParams{
		URI:        uri,
		PlaylistID: playlistID,
		TrackID:    trackID,
		AlbumID:    albumID,
		ArtistID:   artistID,
	}
	return p.ResolveContextURI()
}

// ---- pause / next / previous ------------------------------------------------

var pauseDeviceID string
var nextDeviceID string
var previousDeviceID string

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause Spotify playback",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClientFromConfig(cmdCtx(cmd))
		if err != nil {
			return err
		}
		return dispatchAndPrintStatus(cmdCtx(cmd), &spotify.Pause{DeviceID: pauseDeviceID}, client, "pause failed")
	},
}

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Skip to the next track",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClientFromConfig(cmdCtx(cmd))
		if err != nil {
			return err
		}
		return dispatchAndPrintStatus(cmdCtx(cmd), &spotify.Next{DeviceID: nextDeviceID}, client, "next failed")
	},
}

var previousCmd = &cobra.Command{
	Use:   "previous",
	Short: "Return to the previous track",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClientFromConfig(cmdCtx(cmd))
		if err != nil {
			return err
		}
		return dispatchAndPrintStatus(cmdCtx(cmd), &spotify.Previous{DeviceID: previousDeviceID}, client, "previous failed")
	},
}

// ---- shuffle ----------------------------------------------------------------

var shuffleDeviceID string
var shuffleEnabled bool

var shuffleCmd = &cobra.Command{
	Use:   "shuffle",
	Short: "Enable or disable shuffle",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClientFromConfig(cmdCtx(cmd))
		if err != nil {
			return err
		}
		return dispatchAndPrintStatus(cmdCtx(cmd), &spotify.Shuffle{DeviceID: shuffleDeviceID, Enabled: shuffleEnabled}, client, "shuffle failed")
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
		client, err := newClientFromConfig(cmdCtx(cmd))
		if err != nil {
			return err
		}
		return dispatchAndPrintStatus(cmdCtx(cmd), &spotify.Repeat{DeviceID: repeatDeviceID, State: repeatState}, client, "repeat failed")
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

	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}

	return dispatchAndPrintStatus(cmdCtx(cmd), &spotify.Transfer{DeviceID: transferDeviceID, Play: transferPlay}, client, "transfer failed")
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

	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}

	return dispatchAndPrintStatus(cmdCtx(cmd), &spotify.Volume{DeviceID: volumeDeviceID, Level: volumeLevel}, client, "volume failed")
}

// ---- helper -----------------------------------------------------------------

// confirmStabilizeWindow is how long dispatchAndPrintStatus re-checks a
// confirmed command before trusting it, out of its 5s total budget. A
// package var (rather than a literal) so tests can shrink it. See
// sets.Execute for why this exists — some Spotify Connect devices confirm
// optimistically and then drop a moment later.
var confirmStabilizeWindow = 1 * time.Second

// dispatchAndPrintStatus dispatches a with confirmation polling (5s timeout),
// then fetches and prints the current playback status. A confirmation timeout
// is treated as a soft outcome — status is still printed so the user can see
// what the player is actually doing.
func dispatchAndPrintStatus(ctx context.Context, a spotify.Action, client *spotify.Client, errPrefix string) error {
	opts := sets.ExecuteOptions{
		Confirm:         true,
		Timeout:         5 * time.Second,
		PollInterval:    500 * time.Millisecond,
		StabilizeWindow: confirmStabilizeWindow,
	}
	if err := sets.Execute(ctx, a, client, opts); err != nil {
		var te *sets.TimeoutError
		if !errors.As(err, &te) {
			return fmt.Errorf("%s: %w", errPrefix, err)
		}
	}
	state, err := client.GetCurrentPlayback(ctx)
	if err != nil || state == nil {
		return nil
	}
	printStatus(state)
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
