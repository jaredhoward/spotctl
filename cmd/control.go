package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/spf13/cobra"
)

var pauseDeviceID string
var nextDeviceID string
var previousDeviceID string
var shuffleDeviceID string
var shuffleEnabled bool
var repeatDeviceID string
var repeatState string

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause Spotify playback",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClientFromConfig()
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
		client, err := newClientFromConfig()
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
		client, err := newClientFromConfig()
		if err != nil {
			return err
		}
		if err := dispatchAction(config.CommandParams{DeviceID: previousDeviceID}, "previous", client, nil, 0); err != nil {
			return fmt.Errorf("previous failed: %w", err)
		}
		return nil
	},
}

var shuffleCmd = &cobra.Command{
	Use:   "shuffle",
	Short: "Enable or disable shuffle",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClientFromConfig()
		if err != nil {
			return err
		}
		p := config.CommandParams{
			DeviceID: shuffleDeviceID,
			Enabled:  &shuffleEnabled,
		}
		if err := dispatchAction(p, "shuffle", client, nil, 0); err != nil {
			return fmt.Errorf("shuffle failed: %w", err)
		}
		return nil
	},
}

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
		p := config.CommandParams{
			DeviceID:    repeatDeviceID,
			RepeatState: repeatState,
		}
		if err := dispatchAction(p, "repeat", client, nil, 0); err != nil {
			return fmt.Errorf("repeat failed: %w", err)
		}
		return nil
	},
}

func init() {
	pauseCmd.Flags().StringVar(&pauseDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	nextCmd.Flags().StringVar(&nextDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	previousCmd.Flags().StringVar(&previousDeviceID, "device", "", "Spotify device ID (omit to target active device)")

	shuffleCmd.Flags().StringVar(&shuffleDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	shuffleCmd.Flags().BoolVar(&shuffleEnabled, "enabled", true, "enable shuffle (use --enabled=false to disable)")

	repeatCmd.Flags().StringVar(&repeatDeviceID, "device", "", "Spotify device ID (omit to target active device)")
	repeatCmd.Flags().StringVar(&repeatState, "state", "context", "repeat mode: off, track, context")

	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(previousCmd)
	rootCmd.AddCommand(shuffleCmd)
	rootCmd.AddCommand(repeatCmd)
}
