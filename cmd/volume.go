package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/spf13/cobra"
)

var volumeDeviceID string
var volumeLevel int

var volumeCmd = &cobra.Command{
	Use:   "volume",
	Short: "Set the volume on a Spotify Connect device",
	RunE:  runVolume,
}

func runVolume(cmd *cobra.Command, args []string) error {
	if volumeLevel < 0 || volumeLevel > 100 {
		return fmt.Errorf("volume must be between 0 and 100 (got %d)", volumeLevel)
	}

	client, err := verbClient()
	if err != nil {
		return err
	}

	p := config.CommandParams{
		DeviceID: volumeDeviceID,
		Level:    &volumeLevel,
	}

	if err := dispatchAction(p, "volume", client, nil, 0); err != nil {
		return fmt.Errorf("volume failed: %w", err)
	}
	return nil
}

func init() {
	volumeCmd.Flags().StringVar(&volumeDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	volumeCmd.Flags().IntVar(&volumeLevel, "level", 50, "volume level (0–100)")
	rootCmd.AddCommand(volumeCmd)
}
