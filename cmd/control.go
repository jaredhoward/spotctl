package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/spf13/cobra"
)

var pauseDeviceID string
var nextDeviceID string
var previousDeviceID string

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause Spotify playback",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := verbClient()
		if err != nil {
			return err
		}
		if err := dispatchAction(config.CommandParams{DeviceID: pauseDeviceID}, "pause", client, nil, 0); err != nil {
			return fmt.Errorf("pause failed: %w", err)
		}
		return nil
	},
}

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Skip to the next track",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := verbClient()
		if err != nil {
			return err
		}
		if err := dispatchAction(config.CommandParams{DeviceID: nextDeviceID}, "next", client, nil, 0); err != nil {
			return fmt.Errorf("next failed: %w", err)
		}
		return nil
	},
}

var previousCmd = &cobra.Command{
	Use:   "previous",
	Short: "Return to the previous track",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := verbClient()
		if err != nil {
			return err
		}
		if err := dispatchAction(config.CommandParams{DeviceID: previousDeviceID}, "previous", client, nil, 0); err != nil {
			return fmt.Errorf("previous failed: %w", err)
		}
		return nil
	},
}

func init() {
	pauseCmd.Flags().StringVar(&pauseDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	nextCmd.Flags().StringVar(&nextDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	previousCmd.Flags().StringVar(&previousDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(previousCmd)
}
