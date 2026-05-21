package cmd

import (
	"fmt"
	"log"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var (
	preset     string
	deviceID   string
	uri        string
	playlistID string
	trackID    string
	albumID    string
	shuffle    bool
)

var playCmd = &cobra.Command{
	Use:     "play",
	Aliases: []string{"run"},
	Short:   "Start Spotify playback",
	RunE:    runPlay,
}

func runPlay(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var p config.Preset

	if preset != "" {
		found, ok := cfg.Presets[preset]
		if !ok {
			return fmt.Errorf("preset %q not found in config", preset)
		}
		p = found
	}

	// Resolve context URI from convenience flags
	contextURI, err := resolveURI(cmd, uri, playlistID, trackID, albumID)
	if err != nil {
		return err
	}

	// Flags override preset values
	if deviceID != "" {
		p.DeviceID = deviceID
	}
	if contextURI != "" {
		p.ContextURI = contextURI
	}
	if cmd.Flags().Changed("shuffle") {
		p.Shuffle = shuffle
	}

	if p.DeviceID == "" {
		return fmt.Errorf("device ID is required (use --device or set in preset)")
	}

	log.Println("Refreshing Spotify access token...")
	accessToken, err := spotify.RefreshAccessToken(cfg.ClientB64(), cfg.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	client := newSpotifyClient(accessToken)

	if p.ContextURI != "" {
		log.Printf("Starting playback of %s on device %s...", p.ContextURI, p.DeviceID)
	} else {
		log.Printf("Resuming playback on device %s...", p.DeviceID)
	}
	if err := client.Play(p.DeviceID, p.ContextURI); err != nil {
		return fmt.Errorf("failed to start playback: %w", err)
	}

	if p.Shuffle {
		log.Println("Waiting for playback to start...")
		if err := waitForPlayback(client, 15*time.Second, cfg.PlaybackPollIntervalDuration()); err != nil {
			log.Printf("Warning: %v; attempting shuffle anyway...", err)
		}

		log.Println("Enabling shuffle...")
		if err := client.Shuffle(p.DeviceID); err != nil {
			log.Printf("Warning: failed to enable shuffle: %v", err)
		}
	}

	log.Println("Done.")
	return nil
}

func waitForPlayback(client *spotify.Client, timeout, pollInterval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, err := client.GetCurrentPlayback()
		if err == nil && state != nil && state.IsPlaying {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("timed out waiting for playback to start")
}

func resolveURI(cmd *cobra.Command, uri, playlistID, trackID, albumID string) (string, error) {
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
	}

	return "", nil
}

func init() {
	playCmd.Flags().StringVar(&preset, "preset", "", "name of the preset to run")
	playCmd.Flags().StringVar(&deviceID, "device", "", "Spotify device ID (overrides preset)")
	playCmd.Flags().StringVar(&uri, "uri", "", "Spotify context URI (e.g. spotify:artist:xxx)")
	playCmd.Flags().StringVar(&playlistID, "playlist", "", "Spotify playlist ID (convenience for --uri spotify:playlist:ID)")
	playCmd.Flags().StringVar(&trackID, "track", "", "Spotify track ID (convenience for --uri spotify:track:ID)")
	playCmd.Flags().StringVar(&albumID, "album", "", "Spotify album ID (convenience for --uri spotify:album:ID)")
	playCmd.Flags().BoolVar(&shuffle, "shuffle", false, "enable shuffle (overrides preset)")
	rootCmd.AddCommand(playCmd)
}
