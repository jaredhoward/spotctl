package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/runner"
	"github.com/spf13/cobra"
)

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

	p := config.CommandParams{
		DeviceID: transferDeviceID,
		Play:     &transferPlay,
	}

	if err := runner.DispatchAction(p, "transfer", client, nil, 0); err != nil {
		return fmt.Errorf("transfer failed: %w", err)
	}
	return nil
}

func init() {
	transferCmd.Flags().StringVar(&transferDeviceID, "device", "", "Spotify device ID to transfer playback to (required)")
	transferCmd.Flags().BoolVar(&transferPlay, "play", false, "start playback after transfer")
	rootCmd.AddCommand(transferCmd)
}
