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
	playlistsFlags    listFlags
	playlistsReadable bool
	playlistFlags     listFlags
)

var playlistsCmd = &cobra.Command{
	Use:   "playlists",
	Short: "List the playlists in your library (owned and followed)",
	Long: `List the playlists in your Spotify library: ones you own plus ones you
follow from other people. Followed playlists are marked "(followed)".

Only playlists you own or collaborate on have readable items (see 'spotctl
playlist'); a followed playlist is listed here but its items return a 403.
Use --readable to list only the ones you can read.`,
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

// playlistJSON is the JSON shape of a listed playlist. Owned is omitted when
// the current user couldn't be looked up, so "unknown" isn't reported as
// "not yours".
type playlistJSON struct {
	ID            string `json:"id"`
	URI           string `json:"uri"`
	Name          string `json:"name"`
	OwnerID       string `json:"owner_id"`
	OwnerName     string `json:"owner_name,omitempty"`
	Owned         *bool  `json:"owned,omitempty"`
	Collaborative bool   `json:"collaborative"`
	ItemCount     int    `json:"item_count"`
}

// playlistMarker labels a playlist that isn't the user's own, or "" for
// their own. A collaborative playlist someone else owns is labelled as such
// because, unlike a merely followed one, its items are readable.
func playlistMarker(p spotify.Playlist, me string) string {
	switch {
	case me == "" || p.Owner.ID == me:
		return ""
	case p.Collaborative:
		return "(collaborative)"
	default:
		return "(followed)"
	}
}

func runPlaylists(cmd *cobra.Command, args []string) error {
	f := playlistsFlags
	fetchAll := f.all || playlistsReadable
	if f.all && playlistsReadable {
		return errors.New("--all and --readable can't be combined: --readable already lists everything you can read")
	}
	if fetchAll {
		what := "--all"
		if playlistsReadable {
			what = "--readable"
		}
		if err := rejectPaging(cmd, what); err != nil {
			return err
		}
	}

	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}
	ctx := spotify.WithReason(cmdCtx(cmd), "Requested Command")

	var items []spotify.Playlist
	var offset, total int
	if fetchAll {
		items, err = client.GetAllUserPlaylists(ctx)
		total = len(items)
	} else {
		var page *spotify.Page[spotify.Playlist]
		if page, err = client.GetUserPlaylists(ctx, f.limit, f.offset); err == nil {
			items, offset, total = page.Items, page.Offset, page.Total
		}
	}
	if err != nil {
		return fmt.Errorf("failed to get playlists: %w", accessHint(err, playlistHint))
	}

	// Ownership needs one extra request. Markers are a convenience, so a
	// failure only costs the markers, unless --readable needs the answer.
	me, meErr := client.GetCurrentUserID(ctx)
	if meErr != nil {
		if playlistsReadable {
			return fmt.Errorf("failed to look up your user ID for --readable: %w", meErr)
		}
		fmt.Fprintf(os.Stderr, "warning: could not look up your user ID, so followed playlists aren't marked: %v\n", meErr)
	}

	if playlistsReadable {
		kept := items[:0:0]
		for _, p := range items {
			if p.Owner.ID == me || p.Collaborative {
				kept = append(kept, p)
			}
		}
		items = kept
	}

	if f.json {
		rows := make([]playlistJSON, 0, len(items))
		for _, p := range items {
			row := playlistJSON{ID: p.ID, URI: p.URI, Name: p.Name, OwnerID: p.Owner.ID, OwnerName: p.Owner.DisplayName, Collaborative: p.Collaborative, ItemCount: p.TotalItems()}
			if meErr == nil {
				owned := p.Owner.ID == me
				row.Owned = &owned
			}
			rows = append(rows, row)
		}
		if err := printJSON(rows); err != nil {
			return err
		}
	} else {
		if len(items) == 0 {
			fmt.Println("No playlists found.")
			return nil
		}
		for _, p := range items {
			line := fmt.Sprintf("%-40s %-20s %5d  %s", p.Name, p.Owner.DisplayName, p.TotalItems(), p.URI)
			if marker := playlistMarker(p, me); marker != "" {
				line += "  " + marker
			}
			fmt.Println(line)
		}
	}
	if !fetchAll {
		printMoreHint(offset, len(items), total)
	}
	return nil
}

func runPlaylist(cmd *cobra.Command, args []string) error {
	f := playlistFlags
	if f.all {
		if err := rejectPaging(cmd, "--all"); err != nil {
			return err
		}
	}
	// Validate the argument before spending a token refresh on it.
	if _, err := spotify.ParsePlaylistID(args[0]); err != nil {
		return err
	}

	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}
	ctx := spotify.WithReason(cmdCtx(cmd), "Requested Command")

	var entries []trackEntry
	var offset, total int
	if f.all {
		var items []spotify.PlaylistItem
		if items, err = client.GetAllPlaylistItems(ctx, args[0]); err == nil {
			entries = playlistEntries(items)
			total = len(items)
		}
	} else {
		var page *spotify.Page[spotify.PlaylistItem]
		if page, err = client.GetPlaylistItems(ctx, args[0], f.limit, f.offset); err == nil {
			entries, offset, total = playlistEntries(page.Items), page.Offset, page.Total
		}
	}
	if err != nil {
		return fmt.Errorf("failed to get playlist items: %w", accessHint(err, playlistHint))
	}
	return emitTracks(entries, offset, total, f, "No items found.")
}

func playlistEntries(items []spotify.PlaylistItem) []trackEntry {
	entries := make([]trackEntry, len(items))
	for i, it := range items {
		entries[i] = trackEntry{track: it.Item, addedAt: it.AddedAt, local: it.IsLocal}
	}
	return entries
}

func init() {
	playlistsFlags.register(playlistsCmd, "playlists")
	playlistsCmd.Flags().BoolVar(&playlistsReadable, "readable", false, "only playlists whose items you can read (yours or collaborative); fetches all of them, so not combinable with --limit/--offset")
	playlistFlags.register(playlistCmd, "items")
	rootCmd.AddCommand(playlistsCmd, playlistCmd)
}
