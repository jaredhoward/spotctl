package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

// listFlags are the paging and output flags shared by the read-only list
// commands (playlists, playlist, liked).
type listFlags struct {
	limit  int
	offset int
	all    bool
	json   bool
}

func (f *listFlags) register(cmd *cobra.Command, noun string) {
	cmd.Flags().IntVar(&f.limit, "limit", 20, "number of "+noun+" to show (1-50)")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "index of the first of the "+noun+" to show")
	cmd.Flags().BoolVar(&f.all, "all", false, "show every one of the "+noun+", not just one page (not combinable with --limit/--offset)")
	cmd.Flags().BoolVar(&f.json, "json", false, "print machine-readable JSON to stdout instead of text")
}

// rejectPaging reports an error if --limit or --offset was given alongside a
// flag that fetches everything. what names that flag in the message.
func rejectPaging(cmd *cobra.Command, what string) error {
	if cmd.Flags().Changed("limit") || cmd.Flags().Changed("offset") {
		return fmt.Errorf("%s can't be combined with --limit or --offset", what)
	}
	return nil
}

// printJSON writes v to stdout as indented JSON. HTML escaping is off so
// names like "Mumford & Sons" stay readable.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// printMoreHint tells the user how to fetch the next page, on stderr so it
// doesn't pollute output that's being piped or parsed.
func printMoreHint(offset, shown, total int) {
	if next := offset + shown; shown > 0 && next < total {
		fmt.Fprintf(os.Stderr, "Showing %d–%d of %d. Use --offset %d for more (or --all).\n", offset+1, next, total, next)
	}
}

// trackEntry is one row of a track listing: the track (nil when Spotify
// returned null for it) plus what the source list knows about it.
type trackEntry struct {
	track   *spotify.Track
	addedAt time.Time
	local   bool
}

// trackJSON is the JSON shape of a listed track. Position is 1-based within
// the whole list, matching the number the text output prints.
type trackJSON struct {
	Position    int      `json:"position"`
	URI         string   `json:"uri,omitempty"`
	Name        string   `json:"name,omitempty"`
	Artists     []string `json:"artists,omitempty"`
	DurationMS  int      `json:"duration_ms,omitempty"`
	AddedAt     string   `json:"added_at,omitempty"`
	Local       bool     `json:"local,omitempty"`
	Unavailable bool     `json:"unavailable,omitempty"`
}

func toTrackJSON(n int, e trackEntry) trackJSON {
	row := trackJSON{Position: n, Local: e.local}
	if !e.addedAt.IsZero() {
		row.AddedAt = e.addedAt.UTC().Format(time.RFC3339)
	}
	if e.track == nil {
		row.Unavailable = true
		return row
	}
	row.URI = e.track.URI
	row.Name = e.track.Name
	row.DurationMS = e.track.DurationMS
	for _, a := range e.track.Artists {
		row.Artists = append(row.Artists, a.Name)
	}
	return row
}

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

// emitTracks prints a track listing as text or JSON. offset is the index of
// entries[0] within the full list and total the full list's size; the
// "more available" hint is skipped when everything was fetched. empty is the
// text-mode message for an empty list (JSON mode prints []).
func emitTracks(entries []trackEntry, offset, total int, f listFlags, empty string) error {
	if f.json {
		rows := make([]trackJSON, 0, len(entries))
		for i, e := range entries {
			rows = append(rows, toTrackJSON(offset+i+1, e))
		}
		if err := printJSON(rows); err != nil {
			return err
		}
	} else {
		if len(entries) == 0 {
			fmt.Println(empty)
			return nil
		}
		for i, e := range entries {
			printTrack(offset+i+1, e.track)
		}
	}
	if !f.all {
		printMoreHint(offset, len(entries), total)
	}
	return nil
}
