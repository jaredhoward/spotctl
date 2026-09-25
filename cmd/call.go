package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	callMethod string
	callRaw    bool
)

var callCmd = &cobra.Command{
	Use:   "call <path> [body]",
	Short: "Call an arbitrary Spotify Web API endpoint directly",
	Long: `Call issues a raw request to the Spotify Web API, bypassing every
action, confirmation, and polling abstraction spotctl otherwise wraps around
playback commands. Useful for debugging or exercising endpoints spotctl
doesn't have a dedicated command for.

<path> is resolved against https://api.spotify.com, e.g.:

  spotctl call /v1/me/player/play?device_id=xxx '{"context_uri":"spotify:playlist:abc"}'
  spotctl call -X PUT /v1/me/player/pause?device_id=xxx
  spotctl call /v1/me/player

[body], if given, is sent as the raw request body with
Content-Type: application/json. The response status and body are always
printed, even on a non-2xx response, so you can see exactly what the API
said.

With --raw, only the response body goes to stdout and the status line goes
to stderr, so the body can be piped straight into a tool like jq.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runCall,
}

func runCall(cmd *cobra.Command, args []string) error {
	path := args[0]
	var body string
	if len(args) > 1 {
		body = args[1]
	}

	client, err := newClientFromConfig(cmdCtx(cmd))
	if err != nil {
		return err
	}

	status, respBody, err := client.RawRequest(cmdCtx(cmd), strings.ToUpper(callMethod), path, body)
	if err != nil {
		return fmt.Errorf("call failed: %w", err)
	}

	if callRaw {
		fmt.Fprintf(os.Stderr, "Status: %d\n", status)
	} else {
		fmt.Printf("Status: %d\n", status)
	}
	if len(respBody) > 0 {
		fmt.Println(string(respBody))
	}

	if status < 200 || status > 299 {
		return fmt.Errorf("call returned non-2xx status %d", status)
	}
	return nil
}

func init() {
	callCmd.Flags().StringVarP(&callMethod, "method", "X", "GET", "HTTP method (GET, PUT, POST, DELETE, ...)")
	callCmd.Flags().BoolVar(&callRaw, "raw", false, "print only the response body on stdout (status goes to stderr), for piping")
	rootCmd.AddCommand(callCmd)
}
