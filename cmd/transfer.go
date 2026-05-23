package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var transferPlay bool

var transferCmd = &cobra.Command{
	Use:   "transfer",
	Short: "Transfer playback to a Spotify Connect device",
	RunE:  runTransfer,
}

func runTransfer(cmd *cobra.Command, args []string) error {
	play := transferPlay
	return withPlayerClient(func(c *spotify.Client, id string) error {
		if err := c.TransferPlayback([]string{id}, play); err != nil {
			return fmt.Errorf("failed to transfer playback: %w", err)
		}
		if play {
			fmt.Printf("Transferred and started playback to device %s\n", id)
		} else {
			fmt.Printf("Transferred playback to device %s\n", id)
		}
		return nil
	})
}

func init() {
	transferCmd.Flags().BoolVar(&transferPlay, "play", false, "start playback after transfer")
	rootCmd.AddCommand(transferCmd)
}
