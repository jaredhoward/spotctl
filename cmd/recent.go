package cmd

import (
	"fmt"
	"time"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var (
	recentLimit  int
	recentAfter  string
	recentBefore string
)

var recentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Show recently played tracks",
	RunE:  runRecent,
}

// recentTimeLayouts are the formats --after/--before accept, tried in
// order. All but time.RFC3339 have no zone offset and are parsed in Local
// time.
var recentTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseRecentTime(flag, s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range recentTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid --%s %q: expected RFC3339 (2026-08-04T03:10:08-06:00) or 2006-01-02[ T]15:04:05", flag, s)
}

func runRecent(cmd *cobra.Command, args []string) error {
	after, err := parseRecentTime("after", recentAfter)
	if err != nil {
		return err
	}
	before, err := parseRecentTime("before", recentBefore)
	if err != nil {
		return err
	}
	if !after.IsZero() && !before.IsZero() {
		return fmt.Errorf("--after and --before are mutually exclusive: only one may be set")
	}

	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}

	recent, err := client.GetRecentlyPlayed(spotify.WithReason(cmdCtx(cmd), "Requested Command"), recentLimit, after, before)
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
	recentCmd.Flags().StringVar(&recentAfter, "after", "", "only show tracks played after this time (RFC3339 or 2006-01-02 15:04:05, local time)")
	recentCmd.Flags().StringVar(&recentBefore, "before", "", "only show tracks played before this time (RFC3339 or 2006-01-02 15:04:05, local time); mutually exclusive with --after")
	rootCmd.AddCommand(recentCmd)
}
