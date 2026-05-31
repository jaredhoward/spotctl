package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/runner"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <set>",
	Short: "Run a named set of Spotify commands",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		set, ok := cfg.Sets[name]
		if !ok {
			return fmt.Errorf("set %q not found in config", name)
		}

		client, err := newClientFromConfig()
		if err != nil {
			return err
		}

		return runner.RunSet(name, set, cfg, client, 0)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
