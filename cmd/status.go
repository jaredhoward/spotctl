package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current Spotify playback status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	accessToken, err := spotify.RefreshAccessToken(cfg.ClientB64(), cfg.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	client := newSpotifyClient(accessToken)
	playback, err := client.GetCurrentPlayback()
	if err != nil {
		return fmt.Errorf("failed to get current playback: %w", err)
	}

	if playback == nil {
		fmt.Println("No active playback found.")
		fmt.Println("Make sure Spotify is playing on a Connect device and try again.")
		return nil
	}

	playing := "paused"
	if playback.IsPlaying {
		playing = "playing"
	}

	fmt.Printf("Device: %s (%s) %s\n", playback.Device.Name, playback.Device.Type, deviceActivity(playback.Device.IsActive))
	fmt.Printf("Status: %s | shuffle: %t | repeat: %s | volume: %d%%\n",
		playing,
		playback.ShuffleState,
		playback.RepeatState,
		playback.Device.VolumePercent,
	)

	if playback.Item != nil {
		fmt.Printf("Track: %s\n", playback.Item.Name)
		fmt.Printf("Artists: %s\n", joinArtists(playback.Item.Artists))
		fmt.Printf("Progress: %s / %s\n", formatDurationMS(playback.ProgressMS), formatDurationMS(playback.Item.DurationMS))
	}

	if playback.Context != nil && playback.Context.URI != "" {
		fmt.Printf("Context: %s\n", playback.Context.URI)
	}

	return nil
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
