package cmd

import (
	"fmt"

	"github.com/jaredhoward/spotctl/config"
	"github.com/spf13/cobra"
)

var configPath string
var appVersion string

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
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", config.DefaultConfigPath, "path to config file")
	rootCmd.AddCommand(versionCmd)
}
