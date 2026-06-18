package cmd

import (
	"errors"
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

var configPath string
var appVersion = "dev"
var newSpotifyClient = spotify.NewClient

func SetVersion(v string) {
	appVersion = v
}

var rootCmd = &cobra.Command{
	Use:          "spotctl",
	Short:        "Spotify Connect controller",
	SilenceUsage: true,
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
		if errors.Is(err, spotify.ErrInvalidGrant) {
			// The refresh token itself is no longer valid (e.g. it has hit
			// Spotify's expiry window). Do not retry the refresh — discard
			// the stored token and send the user back through sign-in.
			cfg.RefreshToken = ""
			if saveErr := config.Save(configPath, cfg); saveErr != nil {
				return nil, fmt.Errorf("refresh token expired and could not be cleared from config: %w", saveErr)
			}
			return nil, fmt.Errorf("your Spotify sign-in has expired, please run 'spotctl setup' to reauthorize: %w", err)
		}
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return newSpotifyClient(accessToken), nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", config.DefaultConfigPath, "path to config file")
	rootCmd.AddCommand(versionCmd)
}
