package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var (
	createPlaylistDescription string
	createPlaylistFromFile    string
	createPlaylistDryRun      bool
	createPlaylistPublic      bool
)

var createPlaylistCmd = &cobra.Command{
	Use:   "create-playlist <name> [track-uri...]",
	Short: "Create a new playlist and fill it with tracks",
	Long: `Create a new playlist and add tracks to it, in the order given. The
playlist is private unless you pass --public.

Tracks are spotify:track:<id> URIs, given as arguments and/or read from a
file with --from-file (one URI per line; blank lines and lines starting with
# are ignored; "-" reads standard input). A URI listed more than once is added
once. At least one track is required.

This only ever creates a new playlist; to add to an existing one, use
'spotctl add-to-playlist'. Use --dry-run to validate the input and see what
would be created without contacting Spotify.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCreatePlaylist,
}

const createPlaylistHint = "run 'spotctl setup' to grant playlist-modify access if you haven't since upgrading"

func runCreatePlaylist(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	if name == "" {
		return errors.New("playlist name must not be empty")
	}

	uris, err := collectTrackURIs(cmd, args[1:], createPlaylistFromFile)
	if err != nil {
		return err
	}

	if createPlaylistDryRun {
		fmt.Printf("Would create %s playlist %q with %d track(s).\n", visibility(createPlaylistPublic), name, len(uris))
		if createPlaylistDescription != "" {
			fmt.Printf("Description: %s\n", createPlaylistDescription)
		}
		return nil
	}

	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}
	ctx := spotify.WithReason(cmdCtx(cmd), "Requested Command")

	pl, err := client.CreatePlaylist(ctx, name, createPlaylistDescription, createPlaylistPublic)
	if err != nil {
		return fmt.Errorf("failed to create playlist: %w", accessHint(err, createPlaylistHint))
	}
	if err := client.AddPlaylistItems(ctx, pl.ID, uris); err != nil {
		return fmt.Errorf("playlist %q was created (%s) but adding tracks failed, so it may be partly filled: %w",
			pl.Name, pl.URI, accessHint(err, createPlaylistHint))
	}

	fmt.Fprintf(os.Stderr, "Created %s playlist %q with %d track(s).\n", visibility(createPlaylistPublic), pl.Name, len(uris))
	if pl.Public != nil && *pl.Public != createPlaylistPublic {
		fmt.Fprintf(os.Stderr, "warning: Spotify reports this playlist as %s, not %s as requested; check it in the Spotify app\n",
			visibility(*pl.Public), visibility(createPlaylistPublic))
	}
	fmt.Println(pl.URI)
	return nil
}

func visibility(public bool) string {
	if public {
		return "public"
	}
	return "private"
}

// collectTrackURIs gathers track URIs from args and, if fromFile is set, from
// that file ("-" for stdin), validating each and dropping repeats. It returns
// an error if none remain, and notes dropped repeats on stderr.
func collectTrackURIs(cmd *cobra.Command, args []string, fromFile string) ([]string, error) {
	raw := append([]string(nil), args...)
	if fromFile != "" {
		lines, err := readURILines(cmd.InOrStdin(), fromFile)
		if err != nil {
			return nil, err
		}
		raw = append(raw, lines...)
	}
	uris, dupes, err := cleanTrackURIs(raw)
	if err != nil {
		return nil, err
	}
	if len(uris) == 0 {
		return nil, errors.New("no tracks given: pass spotify:track:<id> URIs as arguments or with --from-file")
	}
	if dupes > 0 {
		fmt.Fprintf(os.Stderr, "note: skipped %d repeated track URI(s)\n", dupes)
	}
	return uris, nil
}

// readURILines reads one value per line from path ("-" means stdin), skipping
// blank lines and # comments.
func readURILines(stdin io.Reader, path string) ([]string, error) {
	var r io.Reader = stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open --from-file: %w", err)
		}
		defer f.Close()
		r = f
	}
	var out []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("failed to read tracks: %w", err)
	}
	return out, nil
}

// cleanTrackURIs validates every entry as a track URI and drops repeats,
// keeping first-seen order. It returns how many repeats were dropped.
func cleanTrackURIs(raw []string) (uris []string, dupes int, err error) {
	seen := map[string]bool{}
	for _, s := range raw {
		uri, err := spotify.ParseTrackURI(s)
		if err != nil {
			return nil, 0, err
		}
		if seen[uri] {
			dupes++
			continue
		}
		seen[uri] = true
		uris = append(uris, uri)
	}
	return uris, dupes, nil
}

func init() {
	createPlaylistCmd.Flags().StringVar(&createPlaylistDescription, "description", "", "playlist description (max 300 characters)")
	createPlaylistCmd.Flags().StringVar(&createPlaylistFromFile, "from-file", "", "read track URIs from a file, one per line (\"-\" for stdin)")
	createPlaylistCmd.Flags().BoolVar(&createPlaylistPublic, "public", false, "make the playlist public (default: private)")
	createPlaylistCmd.Flags().BoolVar(&createPlaylistDryRun, "dry-run", false, "validate and show what would be created, without contacting Spotify")
	rootCmd.AddCommand(createPlaylistCmd)
}
