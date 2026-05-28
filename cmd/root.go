package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var configPath string
var appVersion string
var newSpotifyClient = spotify.NewClient

func SetVersion(v string) {
	appVersion = v
}

var rootCmd = &cobra.Command{
	Use:   "spotctl",
	Short: "Spotify Connect controller",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(appVersion)
	},
}

func Execute() error {
	return executeRoot()
}

func executeRoot() error {
	return rootCmd.Execute()
}

// newClientFromConfig loads config, refreshes the access token, and returns
// a ready-to-use Spotify client.
func newClientFromConfig() (*spotify.Client, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	accessToken, err := spotify.RefreshAccessToken(cfg.ClientB64(), cfg.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return newSpotifyClient(accessToken), nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", config.DefaultConfigPath, "path to config file")
	rootCmd.AddCommand(versionCmd)
}
