package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveURI(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("uri", "", "")
	cmd.Flags().String("playlist", "", "")
	cmd.Flags().String("track", "", "")
	cmd.Flags().String("album", "", "")
	cmd.Flags().String("artist", "", "")

	if err := cmd.Flags().Set("uri", "spotify:artist:abc"); err != nil {
		t.Fatal(err)
	}
	uri, err := resolvePlayURI(cmd, "spotify:artist:abc", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if uri != "spotify:artist:abc" {
		t.Fatalf("expected uri to be spotify:artist:abc, got %q", uri)
	}

	cmd = &cobra.Command{}
	cmd.Flags().String("uri", "", "")
	cmd.Flags().String("playlist", "", "")
	cmd.Flags().String("track", "", "")
	cmd.Flags().String("album", "", "")
	cmd.Flags().String("artist", "", "")
	if err := cmd.Flags().Set("playlist", "playlistid"); err != nil {
		t.Fatal(err)
	}
	uri, err = resolvePlayURI(cmd, "", "playlistid", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if uri != "spotify:playlist:playlistid" {
		t.Fatalf("expected playlist uri, got %q", uri)
	}

	cmd = &cobra.Command{}
	cmd.Flags().String("uri", "", "")
	cmd.Flags().String("playlist", "", "")
	cmd.Flags().String("track", "", "")
	cmd.Flags().String("album", "", "")
	cmd.Flags().String("artist", "", "")
	if err := cmd.Flags().Set("artist", "artistid"); err != nil {
		t.Fatal(err)
	}
	uri, err = resolvePlayURI(cmd, "", "", "", "", "artistid")
	if err != nil {
		t.Fatal(err)
	}
	if uri != "spotify:artist:artistid" {
		t.Fatalf("expected artist uri, got %q", uri)
	}
}

func TestResolveURIEmptyValue(t *testing.T) {
	for _, flag := range []string{"uri", "playlist", "track", "album", "artist"} {
		cmd := &cobra.Command{}
		cmd.Flags().String("uri", "", "")
		cmd.Flags().String("playlist", "", "")
		cmd.Flags().String("track", "", "")
		cmd.Flags().String("album", "", "")
		cmd.Flags().String("artist", "", "")
		if err := cmd.Flags().Set(flag, ""); err != nil {
			t.Fatalf("flag %s: %v", flag, err)
		}
		_, err := resolvePlayURI(cmd, "", "", "", "", "")
		if err == nil {
			t.Errorf("expected error for --%s with empty value, got nil", flag)
		}
	}
}

func TestResolveURIMultipleFlags(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("uri", "", "")
	cmd.Flags().String("playlist", "", "")
	cmd.Flags().String("track", "", "")
	cmd.Flags().String("album", "", "")
	cmd.Flags().String("artist", "", "")

	if err := cmd.Flags().Set("uri", "spotify:artist:abc"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("track", "trackid"); err != nil {
		t.Fatal(err)
	}
	_, err := resolvePlayURI(cmd, "spotify:artist:abc", "", "trackid", "", "")
	if err == nil {
		t.Fatal("expected error when multiple uri flags are set")
	}
}
