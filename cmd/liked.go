package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var (
	likedLimit  int
	likedOffset int
)

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
	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}

	page, err := client.GetSavedTracks(spotify.WithReason(cmdCtx(cmd), "Requested Command"), likedLimit, likedOffset)
	if err != nil {
		return fmt.Errorf("failed to get liked songs: %w", accessHint(err, likedHint))
	}

	if len(page.Items) == 0 {
		fmt.Println("No liked songs found.")
		return nil
	}

	for i, entry := range page.Items {
		printTrack(page.Offset+i+1, entry.Track)
	}
	printMoreHint(page.Offset, len(page.Items), page.Total)
	return nil
}

func init() {
	likedCmd.Flags().IntVar(&likedLimit, "limit", 20, "number of songs to show (1-50)")
	likedCmd.Flags().IntVar(&likedOffset, "offset", 0, "index of the first song to show")
	rootCmd.AddCommand(likedCmd)
}
