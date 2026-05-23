package cmd

import (
	"fmt"
	"unicode"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause Spotify playback",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executePlaybackAction("pause", func(c *spotify.Client, id string) error {
			return c.Pause(id)
		})
	},
}

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Skip to the next track",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executePlaybackAction("skip to next track", func(c *spotify.Client, id string) error {
			return c.Next(id)
		})
	},
}

var previousCmd = &cobra.Command{
	Use:   "previous",
	Short: "Return to the previous track",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executePlaybackAction("return to previous track", func(c *spotify.Client, id string) error {
			return c.Previous(id)
		})
	},
}

// withPlayerClient validates that a device ID is set, builds an authenticated
// Spotify client, and calls fn. It is the shared foundation for all player
// commands (pause, next, previous, volume, transfer).
func withPlayerClient(fn func(*spotify.Client, string) error) error {
	if deviceID == "" {
		return fmt.Errorf("device ID is required (use --device)")
	}

	client, err := newClientFromConfig()
	if err != nil {
		return err
	}

	return fn(client, deviceID)
}

// executePlaybackAction wraps withPlayerClient and prints a standard success
// line. Use it for simple fire-and-forget commands (pause, next, previous).
// Commands that need custom output (transfer, volume) call withPlayerClient
// directly.
func executePlaybackAction(action string, fn func(*spotify.Client, string) error) error {
	err := withPlayerClient(func(c *spotify.Client, id string) error {
		return fn(c, id)
	})
	if err != nil {
		return fmt.Errorf("failed to %s: %w", action, err)
	}
	fmt.Printf("%s on device %s\n", ucFirst(action), deviceID)
	return nil
}

func ucFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func init() {
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(previousCmd)
}
