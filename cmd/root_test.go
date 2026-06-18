package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

func TestExecute_CoversRootExecute(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	SetVersion("test-1.0")
	rootCmd.SetArgs([]string{"version"})
	err := Execute()

	w.Close()
	os.Stdout = oldStdout
	output, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(string(output), "test-1.0") {
		t.Fatalf("version not in output: %q", string(output))
	}
}

func TestNewClientFromConfig_TokenRefreshFailure(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
	}()

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) {
		return "", errors.New("token refresh failed")
	}

	_, err := newClientFromConfig()
	if err == nil || !strings.Contains(err.Error(), "failed to refresh token") {
		t.Fatalf("expected token refresh error, got %v", err)
	}
}

func TestNewClientFromConfig_ConfigLoadFailure(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	configPath = "/nonexistent/path/config.yaml"
	_, err := newClientFromConfig()
	if err == nil || !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("expected config load error, got %v", err)
	}
}

// TestNewClientFromConfig_InvalidGrantDiscardsToken covers Spotify's new
// refresh-token-expiry behavior: when the refresh fails with invalid_grant,
// spotctl must not retry, must clear the stored refresh token from disk, and
// must tell the user to re-run `spotctl setup` instead of surfacing a raw
// API error.
func TestNewClientFromConfig_InvalidGrantDiscardsToken(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
	}()

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "stale-refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (string, error) {
		return "", fmt.Errorf("%w: token revoked", spotify.ErrInvalidGrant)
	}

	_, err := newClientFromConfig()
	if err == nil || !strings.Contains(err.Error(), "spotctl setup") {
		t.Fatalf("expected reauthorization error mentioning 'spotctl setup', got %v", err)
	}

	reloaded, loadErr := config.Load(configPath)
	if loadErr != nil {
		// A missing refresh_token will make validate() fail to load; that's
		// fine as long as it's because the token was cleared, not some other
		// error.
		if !strings.Contains(loadErr.Error(), "refresh_token") {
			t.Fatalf("unexpected error reloading config: %v", loadErr)
		}
		return
	}
	if reloaded.RefreshToken != "" {
		t.Fatalf("expected stale refresh token to be discarded, got %q", reloaded.RefreshToken)
	}
}
