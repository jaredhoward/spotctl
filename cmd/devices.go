package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List available Spotify Connect devices",
	RunE:  runDevices,
}

func runDevices(cmd *cobra.Command, args []string) error {
	cfg, client, err := loadConfigWithClient(cmdCtx(cmd))
	if err != nil {
		return err
	}
	if cfg.DeviceNames == nil {
		cfg.DeviceNames = map[string]string{}
	}

	liveDevices, err := client.GetDevices(cmdCtx(cmd))
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	type displayDevice struct {
		spotify.Device
		displayName string
		offline     bool
	}

	seen := make(map[string]bool, len(liveDevices))
	all := make([]displayDevice, 0, len(liveDevices)+len(cfg.DeviceNames))

	for _, d := range liveDevices {
		seen[d.ID] = true
		name := d.Name
		if name == "" || name == d.ID {
			if n, ok := cfg.DeviceNames[d.ID]; ok {
				name = n
			}
		}
		all = append(all, displayDevice{Device: d, displayName: name})
	}

	for id, name := range cfg.DeviceNames {
		if !seen[id] {
			all = append(all, displayDevice{
				Device:      spotify.Device{ID: id, Name: name},
				displayName: name,
				offline:     true,
			})
		}
	}

	if len(all) == 0 {
		fmt.Println("No Spotify Connect devices found (live or configured).")
		return nil
	}

	// Sort by display name (case-insensitive), ID as tiebreak.
	sort.SliceStable(all, func(i, j int) bool {
		ni := strings.ToLower(all[i].displayName)
		nj := strings.ToLower(all[j].displayName)
		if ni != nj {
			return ni < nj
		}
		return all[i].ID < all[j].ID
	})

	update := updateDevicesFlag
	configChanged := false

	fmt.Println("Spotify Connect devices:")
	fmt.Println()
	for _, d := range all {
		suffix := ""
		if d.IsActive {
			suffix = " *"
		} else if d.offline {
			suffix = " (offline)"
		}

		if update && !d.offline {
			if d.displayName != "" && d.displayName != d.ID && cfg.DeviceNames[d.ID] != d.displayName {
				cfg.DeviceNames[d.ID] = d.displayName
				configChanged = true
			}
		}

		typeField := d.Type
		if typeField == "" {
			typeField = "unknown"
		}

		fmt.Printf("  %-45s %-30s (%-10s) %d%%%s\n",
			d.ID, d.displayName, typeField, d.VolumePercent, suffix)
	}
	fmt.Println()
	fmt.Println("* = currently active device")

	if update && configChanged {
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
