package cmd

import (
	"fmt"
	"strings"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause Spotify playback",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executePlaybackAction("pause", func(c *spotify.Client, deviceID string) error {
			return c.Pause(deviceID)
		})
	},
}

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Skip to the next track",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executePlaybackAction("skip to next track", func(c *spotify.Client, deviceID string) error {
			return c.Next(deviceID)
		})
	},
}

var previousCmd = &cobra.Command{
	Use:   "previous",
	Short: "Return to the previous track",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executePlaybackAction("return to previous track", func(c *spotify.Client, deviceID string) error {
			return c.Previous(deviceID)
		})
	},
}

func executePlaybackAction(action string, fn func(*spotify.Client, string) error) error {
	if deviceID == "" {
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

	client := spotify.NewClient(accessToken)
	if err := fn(client, deviceID); err != nil {
		return fmt.Errorf("failed to %s: %w", action, err)
	}

	message := strings.Title(action)
	fmt.Printf("%s on device %s\n", message, deviceID)
	return nil
}

func init() {
	pauseCmd.Flags().StringVar(&deviceID, "device", "", "Spotify device ID")
	nextCmd.Flags().StringVar(&deviceID, "device", "", "Spotify device ID")
	previousCmd.Flags().StringVar(&deviceID, "device", "", "Spotify device ID")
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(previousCmd)
}
