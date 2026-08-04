package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var recentLimit int

var recentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Show recently played tracks",
	RunE:  runRecent,
}

func runRecent(cmd *cobra.Command, args []string) error {
	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}

	recent, err := client.GetRecentlyPlayed(spotify.WithReason(cmdCtx(cmd), "Requested Command"), recentLimit)
	if err != nil {
		return fmt.Errorf("failed to get recently played tracks: %w", err)
	}

	if len(recent.Items) == 0 {
		fmt.Println("No recently played tracks found.")
		return nil
	}

	for _, item := range recent.Items {
		line := fmt.Sprintf("%s  %-40s %s",
			item.PlayedAt.Local().Format("2006-01-02 15:04:05"),
			item.Track.Name,
			joinArtists(item.Track.Artists),
		)
		if item.Context != nil && item.Context.URI != "" {
			line += fmt.Sprintf("  [%s]", item.Context.URI)
		}
		fmt.Println(line)
	}
	return nil
}

func init() {
	recentCmd.Flags().IntVar(&recentLimit, "limit", 20, "number of tracks to show (1-50)")
	rootCmd.AddCommand(recentCmd)
}
