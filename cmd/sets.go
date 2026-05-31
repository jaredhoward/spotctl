package cmd

import (
	"fmt"
	"sort"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/runner"
	"github.com/spf13/cobra"
)

var setsCmd = &cobra.Command{
	Use:   "sets",
	Short: "List configured sets",
	RunE:  runSets,
}

func runSets(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Sets) == 0 {
		fmt.Println("No sets configured.")
		fmt.Printf("Add sets to %s to get started.\n", configPath)
		return nil
	}

	// Sort names for stable, predictable output.
	names := make([]string, 0, len(cfg.Sets))
	for name := range cfg.Sets {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		set := cfg.Sets[name]
		device := set.DeviceID
		if device == "" {
			device = "(active device)"
		}
		fmt.Printf("%s  —  %d command(s)  device: %s\n", name, len(set.Commands), device)

		for i, c := range set.Commands {
			fmt.Printf("    %2d. %s\n", i+1, runner.CommandLabel(i+1, c))
		}
		fmt.Println()
	}

	return nil
}

func init() {
	rootCmd.AddCommand(setsCmd)
}
