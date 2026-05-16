package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List available Spotify Connect devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		accessToken, err := spotify.RefreshAccessToken(cfg.ClientB64(), cfg.RefreshToken)
		if err != nil {
			return fmt.Errorf("failed to refresh token: %w", err)
		}

		client := spotify.NewClient(accessToken)

		devices, err := client.GetDevices()
		if err != nil {
			return fmt.Errorf("failed to get devices: %w", err)
		}

		if len(devices) == 0 {
			fmt.Println("No active Spotify Connect devices found.")
			fmt.Println("Make sure your device is on and has been used recently.")
			return nil
		}

		fmt.Println("Available Spotify Connect devices:")
		fmt.Println()
		for _, d := range devices {
			active := ""
			if d.IsActive {
				active = " *"
			}
			fmt.Printf("  %-45s %-30s (%-10s) %d%%%s\n",
				d.ID, d.Name, d.Type, d.VolumePercent, active)
		}
		fmt.Println()
		fmt.Println("* = currently active device")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(devicesCmd)
}
