package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/pflag"
	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/sets"
	"github.com/spf13/cobra"
)

// ---- sets -------------------------------------------------------------------

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
			fmt.Printf("    %2d. %s\n", i+1, sets.CommandLabel(i+1, c))
		}
		fmt.Println()
	}

	return nil
}

// ---- run --------------------------------------------------------------------

var runCmd = &cobra.Command{
	Use:   "run <set>",
	Short: "Run a named set of Spotify commands",
	Long: `Run a named set of Spotify commands.

Sets that declare params accept values as flags named after the param:

  spotctl run my_set --uri spotify:playlist:abc123 --volume 50

Run 'spotctl sets' to see declared params for each set.`,
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// DisableFlagParsing means cobra gives us all tokens raw, including
		// persistent flags like --config. We do a first pass with a temporary
		// flagset to extract --config and the set name, then a second pass
		// with the dynamic param flags registered.
		if len(args) == 0 {
			return fmt.Errorf("requires a set name argument")
		}

		// First pass: strip --config <path> from args and update configPath.
		remaining := make([]string, 0, len(args))
		for i := 0; i < len(args); i++ {
			switch {
			case args[i] == "--config" && i+1 < len(args):
				configPath = args[i+1]
				i++
			case len(args[i]) > 9 && args[i][:9] == "--config=":
				configPath = args[i][9:]
			default:
				remaining = append(remaining, args[i])
			}
		}

		if len(remaining) == 0 {
			return fmt.Errorf("requires a set name argument")
		}
		name := remaining[0]
		flagTokens := remaining[1:]

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		set, ok := cfg.Sets[name]
		if !ok {
			return fmt.Errorf("set %q not found in config", name)
		}

		// Second pass: register dynamic flags for declared params and parse.
		fs := pflag.NewFlagSet("run-params", pflag.ContinueOnError)
		setArgs := make(map[string]*string, len(set.Params))
		for paramName := range set.Params {
			v := new(string)
			setArgs[paramName] = v
			fs.StringVar(v, paramName, "", fmt.Sprintf("value for set param %q", paramName))
		}

		if err := fs.Parse(flagTokens); err != nil {
			return err
		}

		// Collect only flags that were explicitly provided.
		resolvedArgs := make(map[string]string, len(setArgs))
		for k, v := range setArgs {
			if *v != "" {
				resolvedArgs[k] = *v
			}
		}

		rs, err := sets.Build(name, set, cfg, 0, resolvedArgs)
		if err != nil {
			return err
		}

		client, err := newClientFromConfig()
		if err != nil {
			return err
		}

		return rs.Dispatch(context.Background(), client)
	},
}

// ---- init -------------------------------------------------------------------

func init() {
	rootCmd.AddCommand(setsCmd, runCmd)
}
