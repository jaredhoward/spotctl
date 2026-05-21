package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var (
	transferDeviceID string
	transferPlay     bool
)

var transferCmd = &cobra.Command{
	Use:   "transfer",
	Short: "Transfer playback to a Spotify Connect device",
	RunE:  runTransfer,
}

func runTransfer(cmd *cobra.Command, args []string) error {
	if transferDeviceID == "" {
		return fmt.Errorf("device ID is required (use --device)")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	accessToken, err := spotify.RefreshAccessToken(cfg.ClientB64(), cfg.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	client := newSpotifyClient(accessToken)
	if err := client.TransferPlayback([]string{transferDeviceID}, transferPlay); err != nil {
		return fmt.Errorf("failed to transfer playback: %w", err)
	}

	message := "Transferred playback to device"
	if transferPlay {
		message = "Transferred and started playback to device"
	}

	fmt.Printf("%s %s\n", message, transferDeviceID)
	return nil
}

func init() {
	transferCmd.Flags().StringVar(&transferDeviceID, "device", "", "Spotify device ID")
	transferCmd.Flags().BoolVar(&transferPlay, "play", false, "start playback after transfer")
	rootCmd.AddCommand(transferCmd)
}
