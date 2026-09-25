package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var likedFlags listFlags

const likedHint = "run 'spotctl setup' to grant library access if you haven't since upgrading"

var likedCmd = &cobra.Command{
	Use:   "liked",
	Short: "Show your Liked Songs",
	Long: `Show your Liked Songs (saved tracks), most recently liked first.

Liked Songs is not a playlist, so it doesn't appear in 'spotctl playlists'
and can't be read with 'spotctl playlist'.`,
	Args: cobra.NoArgs,
	RunE: runLiked,
}

func runLiked(cmd *cobra.Command, args []string) error {
	f := likedFlags
	if f.all {
		if err := rejectPaging(cmd, "--all"); err != nil {
			return err
		}
	}

	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}
	ctx := spotify.WithReason(cmdCtx(cmd), "Requested Command")

	var entries []trackEntry
	var offset, total int
	if f.all {
		var saved []spotify.SavedTrack
		if saved, err = client.GetAllSavedTracks(ctx); err == nil {
			entries = savedEntries(saved)
			total = len(saved)
		}
	} else {
		var page *spotify.Page[spotify.SavedTrack]
		if page, err = client.GetSavedTracks(ctx, f.limit, f.offset); err == nil {
			entries, offset, total = savedEntries(page.Items), page.Offset, page.Total
		}
	}
	if err != nil {
		return fmt.Errorf("failed to get liked songs: %w", accessHint(err, likedHint))
	}
	return emitTracks(entries, offset, total, f, "No liked songs found.")
}

func savedEntries(saved []spotify.SavedTrack) []trackEntry {
	entries := make([]trackEntry, len(saved))
	for i, s := range saved {
		entries[i] = trackEntry{track: s.Track, addedAt: s.AddedAt}
	}
	return entries
}

func init() {
	likedFlags.register(likedCmd, "songs")
	rootCmd.AddCommand(likedCmd)
}
