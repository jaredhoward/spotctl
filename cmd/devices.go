package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/spf13/cobra"
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List available Spotify Connect devices",
	RunE:  runDevices,
}

func runDevices(cmd *cobra.Command, args []string) error {
	client, err := newClientFromConfig()
	if err != nil {
		return err
	}

	devices, err := client.GetDevices()
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	if len(devices) == 0 {
		fmt.Println("No active Spotify Connect devices found.")
		fmt.Println("Make sure your device is on and has been used recently.")
		return nil
	}

	update := cmd.Flags().Changed("update") && cmd.Flag("update").Value.String() == "true"

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.DeviceNames == nil {
		cfg.DeviceNames = map[string]string{}
	}

	changed := false

	fmt.Println("Available Spotify Connect devices:")
	fmt.Println()
	for _, d := range devices {
		active := ""
		if d.IsActive {
			active = " *"
		}

		displayName := d.Name
		missingName := displayName == "" || displayName == d.ID
		if missingName {
			if n, ok := cfg.DeviceNames[d.ID]; ok {
				displayName = n
			}
		} else if update {
			if displayName != d.ID && cfg.DeviceNames[d.ID] != displayName {
				cfg.DeviceNames[d.ID] = displayName
				changed = true
			}
		}

		fmt.Printf("  %-45s %-30s (%-10s) %d%%%s\n",
			d.ID, displayName, d.Type, d.VolumePercent, active)
	}
	fmt.Println()
	fmt.Println("* = currently active device")

	if update && changed {
		if err := config.Save(configPath, cfg); err != nil {
			return fmt.Errorf("failed to save config with device names: %w", err)
		}
		fmt.Println("Updated config with discovered device names.")
	}

	return nil
}

var updateDevicesFlag bool

func init() {
	devicesCmd.Flags().BoolVar(&updateDevicesFlag, "update", false, "update config device name mappings when available")
	rootCmd.AddCommand(devicesCmd)
}
