package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jaredhoward/spotctl/config"
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
			label := commandLabel(i+1, c)
			fmt.Printf("    %2d. %s\n", i+1, label)
		}
		fmt.Println()
	}

	return nil
}

// commandLabel returns a human-readable description of a command for display.
func commandLabel(n int, c config.Command) string {
	var parts []string
	parts = append(parts, c.Action)

	switch c.Action {
	case "volume":
		if c.Params.Level != nil {
			parts = append(parts, fmt.Sprintf("level=%d", *c.Params.Level))
		}
	case "shuffle":
		parts = append(parts, fmt.Sprintf("enabled=%v", c.Params.ShuffleEnabled()))
	case "repeat":
		if c.Params.RepeatState != "" {
			parts = append(parts, fmt.Sprintf("state=%s", c.Params.RepeatState))
		}
	case "sleep":
		if c.Params.Duration != "" {
			parts = append(parts, fmt.Sprintf("duration=%s", c.Params.Duration))
		}
	case "run_set":
		if c.Params.Set != "" {
			parts = append(parts, fmt.Sprintf("set=%s", c.Params.Set))
		}
	case "play":
		switch {
		case c.Params.URI != "":
			parts = append(parts, fmt.Sprintf("uri=%s", c.Params.URI))
		case c.Params.PlaylistID != "":
			parts = append(parts, fmt.Sprintf("playlist=%s", c.Params.PlaylistID))
		case c.Params.TrackID != "":
			parts = append(parts, fmt.Sprintf("track=%s", c.Params.TrackID))
		case c.Params.AlbumID != "":
			parts = append(parts, fmt.Sprintf("album=%s", c.Params.AlbumID))
		case c.Params.ArtistID != "":
			parts = append(parts, fmt.Sprintf("artist=%s", c.Params.ArtistID))
		}
	case "transfer":
		if c.Params.DeviceID != "" {
			parts = append(parts, fmt.Sprintf("device=%s", c.Params.DeviceID))
		}
	}

	if c.Params.DeviceID != "" && c.Action != "transfer" {
		parts = append(parts, fmt.Sprintf("device=%s", c.Params.DeviceID))
	}
	if c.Confirm {
		parts = append(parts, "confirm")
	}
	if c.Name != "" {
		return fmt.Sprintf("%s (%s)", strings.Join(parts, " "), c.Name)
	}
	return strings.Join(parts, " ")
}

func init() {
	rootCmd.AddCommand(setsCmd)
}
