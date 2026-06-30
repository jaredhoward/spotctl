package cmd

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
	"github.com/spf13/cobra"
)

const scopes = "user-modify-playback-state user-read-playback-state"

// setupStdin is the reader used by runSetup for interactive prompts and the
// OAuth redirect URL. Tests replace it to avoid reading from os.Stdin.
var setupStdin io.Reader = os.Stdin

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup to generate config file",
	RunE:  runSetup,
}

func runSetup(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(setupStdin)

	fmt.Println("=== setup ===")
	fmt.Println()

	// Load existing config if present to preserve sets and pre-fill prompts.
	existing, _ := config.Load(configPath)

	clientID := promptWithDefault(reader, "Spotify Client ID", existingValue(existing, func(c *config.Config) string { return c.ClientID }))
	clientSecret := promptWithDefault(reader, "Spotify Client Secret", existingValue(existing, func(c *config.Config) string { return c.ClientSecret }))
	redirectURI := promptWithDefault(reader, "Spotify Redirect URI", existingValue(existing, func(c *config.Config) string { return c.RedirectURI }))

	fmt.Println()
	fmt.Println("Starting OAuth flow...")
	fmt.Println()

	refreshToken, err := oauthFlow(cmdCtx(cmd), clientID, clientSecret, redirectURI, reader, setupTokenEndpoint)
	if err != nil {
		return fmt.Errorf("OAuth flow failed: %w", err)
	}

	sets := map[string]config.Set{}
	if existing != nil && existing.Sets != nil {
		sets = existing.Sets
	}

	deviceNames := map[string]string{}
	if existing != nil && existing.DeviceNames != nil {
		deviceNames = existing.DeviceNames
	}

	cfg := &config.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
		RedirectURI:  redirectURI,
		Sets:         sets,
		DeviceNames:  deviceNames,
	}

	if err := config.Save(configPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n✅ Config saved to %s\n", configPath)
	fmt.Println("Add your sets to the config file to get started.")
	return nil
}

// oauthHTTPClient is the HTTP client used for token exchange. Tests may
// replace it with a server-specific client to avoid mutating http.DefaultClient.
var oauthHTTPClient *http.Client

// setupTokenEndpoint overrides the Spotify token URL used by runSetup.
// Empty string means use the default. Tests set this to a local httptest server URL.
var setupTokenEndpoint string

func getOAuthClient() *http.Client {
	if oauthHTTPClient != nil {
		return oauthHTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func oauthFlow(ctx context.Context, clientID, clientSecret, redirectURI string, stdin io.Reader, opts ...string) (string, error) {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", scopes)

	authURL := spotify.URLAuth + "?" + params.Encode()
	fmt.Printf("Open this URL in your browser:\n\n%s\n\n", authURL)
	fmt.Printf("After authorizing, you'll be redirected to:\n%s?code=XXXXXX\n\n", redirectURI)

	reader := bufio.NewReader(stdin)
	redirectedURL := prompt(reader, "Paste the full redirect URL here")

	parsed, err := url.Parse(redirectedURL)
	if err != nil {
		return "", fmt.Errorf("could not parse redirect URL: %w", err)
	}

	code := parsed.Query().Get("code")
	if code == "" {
		return "", fmt.Errorf("no code found in redirect URL")
	}

	log.Println("Exchanging auth code for tokens...")

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	endpoint := spotify.URLToken
	if len(opts) > 0 && opts[0] != "" {
		endpoint = opts[0]
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+
		base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := getOAuthClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("could not decode token response: %w", err)
	}
	if result.RefreshToken == "" {
		return "", fmt.Errorf("no refresh token in response")
	}

	return result.RefreshToken, nil
}

func existingValue(cfg *config.Config, fn func(*config.Config) string) string {
	if cfg == nil {
		return ""
	}
	return fn(cfg)
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Printf("%s: ", label)
	val, _ := reader.ReadString('\n')
	return strings.TrimSpace(val)
}

func promptWithDefault(reader *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	val, _ := reader.ReadString('\n')
	val = strings.TrimSpace(val)
	if val == "" {
		return def
	}
	return val
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
