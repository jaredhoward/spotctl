package cmd

import (
	"testing"

	"github.com/jaredhoward/spotctl/spotify"
)

func TestJoinArtists(t *testing.T) {
	artists := []spotify.Artist{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	if got := joinArtists(artists); got != "A, B, C" {
		t.Fatalf("expected joined artists, got %q", got)
	}
}

func TestDeviceActivity(t *testing.T) {
	if got := deviceActivity(true); got != "(active)" {
		t.Fatalf("expected active marker, got %q", got)
	}
	if got := deviceActivity(false); got != "" {
		t.Fatalf("expected empty marker, got %q", got)
	}
}

func TestFormatDurationMS(t *testing.T) {
	if got := formatDurationMS(123456); got != "02:03" {
		t.Fatalf("expected duration 2:03, got %q", got)
	}
}
