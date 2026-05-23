package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

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

	level := volumeLevel
	return withPlayerClient(func(c *spotify.Client, id string) error {
		if err := c.SetVolume(id, level); err != nil {
			return fmt.Errorf("failed to set volume: %w", err)
		}
		fmt.Printf("Set volume to %d%% on device %s\n", level, id)
		return nil
	})
}

func init() {
	volumeCmd.Flags().IntVar(&volumeLevel, "level", 50, "volume level (0–100)")
	rootCmd.AddCommand(volumeCmd)
}
