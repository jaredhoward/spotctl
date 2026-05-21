package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var (
	volumeDeviceID string
	volumeLevel    int
)

var volumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "Set Spotify playback volume on a device",
	RunE:  runVolume,
}

func runVolume(cmd *cobra.Command, args []string) error {
	if volumeDeviceID == "" {
		return fmt.Errorf("device ID is required (use --device)")
	}
	if volumeLevel < 0 || volumeLevel > 100 {
		return fmt.Errorf("volume must be between 0 and 100")
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
	if err := client.SetVolume(volumeDeviceID, volumeLevel); err != nil {
		return fmt.Errorf("failed to set volume: %w", err)
	}

	fmt.Printf("Set volume to %d%% on device %s\n", volumeLevel, volumeDeviceID)
	return nil
}

func init() {
	volumeCmd.Flags().StringVar(&volumeDeviceID, "device", "", "Spotify device ID")
	volumeCmd.Flags().IntVar(&volumeLevel, "level", 0, "Volume level 0-100")
	_ = volumeCmd.MarkFlagRequired("level")
	rootCmd.AddCommand(volumeCmd)
}
