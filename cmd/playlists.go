package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var (
	playlistsLimit  int
	playlistsOffset int
	playlistLimit   int
	playlistOffset  int
)

var playlistsCmd = &cobra.Command{
	Use:   "playlists",
	Short: "List the playlists in your library (owned and followed)",
	Long: `List the playlists in your Spotify library: ones you own plus ones you
follow from other people. The owner column shows which is which.

Only playlists you own or collaborate on have readable items (see 'spotctl
playlist'); a followed playlist is listed here but its items return a 403.`,
	Args: cobra.NoArgs,
	RunE: runPlaylists,
}

var playlistCmd = &cobra.Command{
	Use:   "playlist <id|uri|url>",
	Short: "Show the items in a playlist",
	Long: `Show the items in a playlist. The playlist may be given as a bare ID, a
spotify:playlist:<id> URI, or an open.spotify.com/playlist/<id> link.

Spotify only returns items for playlists you own or collaborate on.`,
	Args: cobra.ExactArgs(1),
	RunE: runPlaylist,
}

// accessHint appends hint to a 403, the status Spotify returns both for a
// token issued before spotctl asked for a scope (refresh tokens keep the
// scopes they were issued with) and for content the user can't access. Any
// other error passes through unchanged.
func accessHint(err error, hint string) error {
	var httpErr *spotify.HTTPStatusError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%w\nhint: %s", err, hint)
	}
	return err
}

const playlistHint = "run 'spotctl setup' to grant playlist access if you haven't since upgrading; playlist items are also only readable for playlists you own or collaborate on"

// printTrack prints one numbered track line: position, name, artists, and
// the track URI when there is one. A nil track (Spotify returned null, e.g.
// the track was removed from the catalogue) prints as unavailable.
func printTrack(n int, t *spotify.Track) {
	if t == nil {
		fmt.Printf("%4d  (unavailable)\n", n)
		return
	}
	line := fmt.Sprintf("%4d  %-40s %s", n, t.Name, joinArtists(t.Artists))
	if t.URI != "" {
		line += fmt.Sprintf("  [%s]", t.URI)
	}
	fmt.Println(line)
}

// printMoreHint tells the user how to fetch the next page, on stderr so it
// doesn't pollute output that's being piped or parsed.
func printMoreHint(offset, shown, total int) {
	if next := offset + shown; shown > 0 && next < total {
		fmt.Fprintf(os.Stderr, "Showing %d–%d of %d. Use --offset %d for more.\n", offset+1, next, total, next)
	}
}

func runPlaylists(cmd *cobra.Command, args []string) error {
	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}

	page, err := client.GetUserPlaylists(spotify.WithReason(cmdCtx(cmd), "Requested Command"), playlistsLimit, playlistsOffset)
	if err != nil {
		return fmt.Errorf("failed to get playlists: %w", accessHint(err, playlistHint))
	}

	if len(page.Items) == 0 {
		fmt.Println("No playlists found.")
		return nil
	}

	for _, p := range page.Items {
		fmt.Printf("%-40s %-20s %5d  %s\n", p.Name, p.Owner.DisplayName, p.TotalItems(), p.URI)
	}
	printMoreHint(page.Offset, len(page.Items), page.Total)
	return nil
}

func runPlaylist(cmd *cobra.Command, args []string) error {
	// Validate the argument before spending a token refresh on it.
	if _, err := spotify.ParsePlaylistID(args[0]); err != nil {
		return err
	}

	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}

	page, err := client.GetPlaylistItems(spotify.WithReason(cmdCtx(cmd), "Requested Command"), args[0], playlistLimit, playlistOffset)
	if err != nil {
		return fmt.Errorf("failed to get playlist items: %w", accessHint(err, playlistHint))
	}

	if len(page.Items) == 0 {
		fmt.Println("No items found.")
		return nil
	}

	for i, entry := range page.Items {
		printTrack(page.Offset+i+1, entry.Item)
	}
	printMoreHint(page.Offset, len(page.Items), page.Total)
	return nil
}

func init() {
	playlistsCmd.Flags().IntVar(&playlistsLimit, "limit", 20, "number of playlists to show (1-50)")
	playlistsCmd.Flags().IntVar(&playlistsOffset, "offset", 0, "index of the first playlist to show")
	playlistCmd.Flags().IntVar(&playlistLimit, "limit", 20, "number of items to show (1-50)")
	playlistCmd.Flags().IntVar(&playlistOffset, "offset", 0, "index of the first item to show")
	rootCmd.AddCommand(playlistsCmd, playlistCmd)
}
