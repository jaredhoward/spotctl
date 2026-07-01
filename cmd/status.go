package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current Spotify playback status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}

	playback, err := client.GetCurrentPlayback(cmdCtx(cmd))
	if err != nil {
		return fmt.Errorf("failed to get current playback: %w", err)
	}

	if playback == nil {
		fmt.Println("No active playback found.")
		fmt.Println("Make sure Spotify is playing on a Connect device and try again.")
		return nil
	}

	printStatus(playback)
	return nil
}

func printStatus(state *spotify.PlaybackState) {
	playing := "paused"
	if state.IsPlaying {
		playing = "playing"
	}

	fmt.Printf("Device: %s (%s) %s\n", state.Device.Name, state.Device.Type, deviceActivity(state.Device.IsActive))
	fmt.Printf("Status: %s | shuffle: %t | repeat: %s | volume: %d%%\n",
		playing,
		state.ShuffleState,
		state.RepeatState,
		state.Device.VolumePercent,
	)

	if state.Item != nil {
		fmt.Printf("Track: %s\n", state.Item.Name)
		fmt.Printf("Artists: %s\n", joinArtists(state.Item.Artists))
		fmt.Printf("Progress: %s / %s\n", formatDurationMS(state.ProgressMS), formatDurationMS(state.Item.DurationMS))
	}

	if state.Context != nil && state.Context.URI != "" {
		fmt.Printf("Context: %s\n", state.Context.URI)
	}
}

func joinArtists(artists []spotify.Artist) string {
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		names = append(names, artist.Name)
	}
	return strings.Join(names, ", ")
}

func deviceActivity(active bool) string {
	if active {
		return "(active)"
	}
	return ""
}

func formatDurationMS(ms int) string {
	d := time.Duration(ms) * time.Millisecond
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
