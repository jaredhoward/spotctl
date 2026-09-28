package cmd

import (
	"fmt"
	"os"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var (
	addToPlaylistFromFile        string
	addToPlaylistDryRun          bool
	addToPlaylistAllowDuplicates bool
)

var addToPlaylistCmd = &cobra.Command{
	Use:   "add-to-playlist <playlist> [track-uri...]",
	Short: "Append tracks to an existing playlist",
	Long: `Append tracks to the end of a playlist you own or collaborate on. The playlist
may be a bare ID, a spotify:playlist:<id> URI, or an open.spotify.com link.

Tracks are spotify:track:<id> URIs, given as arguments and/or read from a
file with --from-file (one URI per line; blank lines and lines starting with
# are ignored; "-" reads standard input). At least one track is required.

Tracks already in the playlist are skipped (matched by URI, so a remaster or
other version of the same song counts as different). Pass --allow-duplicates
to add them anyway and skip that check.

This only ever appends: it never removes or reorders anything. --dry-run does
the duplicate check (a read-only request) and reports what would be added,
without changing the playlist.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAddToPlaylist,
}

const addToPlaylistHint = "run 'spotctl setup' to grant playlist-modify access if you haven't since upgrading; playlists you don't own or collaborate on can't be changed"

func runAddToPlaylist(cmd *cobra.Command, args []string) error {
	// Validate the arguments before spending a token refresh on them.
	if _, err := spotify.ParsePlaylistID(args[0]); err != nil {
		return err
	}
	uris, err := collectTrackURIs(cmd, args[1:], addToPlaylistFromFile)
	if err != nil {
		return err
	}

	var client *spotify.Client
	var ctx = cmdCtx(cmd)
	if !(addToPlaylistDryRun && addToPlaylistAllowDuplicates) {
		if client, err = newClientFromConfig(ctx); err != nil {
			return err
		}
		ctx = spotify.WithReason(ctx, "Requested Command")
	}

	skipped := 0
	if !addToPlaylistAllowDuplicates {
		items, err := client.GetAllPlaylistItems(ctx, args[0])
		if err != nil {
			return fmt.Errorf("failed to read the playlist to check for duplicates: %w", accessHint(err, addToPlaylistHint))
		}
		have := make(map[string]bool, len(items))
		for _, it := range items {
			if it.Item != nil {
				have[it.Item.URI] = true
			}
		}
		kept := uris[:0:0]
		for _, u := range uris {
			if have[u] {
				skipped++
				continue
			}
			kept = append(kept, u)
		}
		uris = kept
	}

	if addToPlaylistDryRun {
		fmt.Printf("Would add %d track(s) to %s", len(uris), args[0])
		if skipped > 0 {
			fmt.Printf(" (%d already there)", skipped)
		}
		fmt.Println(".")
		return nil
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "note: skipped %d track(s) already in the playlist\n", skipped)
	}
	if len(uris) == 0 {
		fmt.Fprintln(os.Stderr, "Nothing to add: every track is already in the playlist.")
		return nil
	}

	if err := client.AddPlaylistItems(ctx, args[0], uris); err != nil {
		return fmt.Errorf("adding tracks failed, so the playlist may be partly updated: %w", accessHint(err, addToPlaylistHint))
	}
	fmt.Fprintf(os.Stderr, "Added %d track(s) to %s.\n", len(uris), args[0])
	return nil
}

func init() {
	addToPlaylistCmd.Flags().StringVar(&addToPlaylistFromFile, "from-file", "", "read track URIs from a file, one per line (\"-\" for stdin)")
	addToPlaylistCmd.Flags().BoolVar(&addToPlaylistDryRun, "dry-run", false, "check for duplicates and show what would be added, without changing the playlist")
	addToPlaylistCmd.Flags().BoolVar(&addToPlaylistAllowDuplicates, "allow-duplicates", false, "add tracks even if already in the playlist (skips the duplicate check)")
	rootCmd.AddCommand(addToPlaylistCmd)
}
