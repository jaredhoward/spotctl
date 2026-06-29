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
	spotify.RefreshAccessToken = func(_, _ string) (spotify.RefreshResult, error) {
		return spotify.RefreshResult{}, errors.New("token refresh failed")
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
	spotify.RefreshAccessToken = func(_, _ string) (spotify.RefreshResult, error) {
		return spotify.RefreshResult{}, fmt.Errorf("%w: token revoked", spotify.ErrInvalidGrant)
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

// TestLoadConfigWithClient_RotatesRefreshToken covers the branch where
// Spotify returns a new refresh_token alongside the access token: the
// rotated token must be persisted to config so future refreshes use it.
func TestLoadConfigWithClient_RotatesRefreshToken(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
	}()

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "old-refresh"})
	spotify.RefreshAccessToken = func(_, _ string) (spotify.RefreshResult, error) {
		return spotify.RefreshResult{AccessToken: "new-access", NewRefreshToken: "new-refresh"}, nil
	}

	cfg, client, err := loadConfigWithClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected a non-nil client")
	}
	if cfg.RefreshToken != "new-refresh" {
		t.Fatalf("expected in-memory cfg to reflect rotated token, got %q", cfg.RefreshToken)
	}

	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.RefreshToken != "new-refresh" {
		t.Fatalf("expected rotated refresh token to be persisted, got %q", reloaded.RefreshToken)
	}
}

func TestNewClientFromCfg_Success(t *testing.T) {
	oldRefresh := spotify.RefreshAccessToken
	defer func() { spotify.RefreshAccessToken = oldRefresh }()

	spotify.RefreshAccessToken = func(_, _ string) (spotify.RefreshResult, error) {
		return spotify.RefreshResult{AccessToken: "access-token"}, nil
	}

	cfg := &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"}
	client, err := newClientFromCfg(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected a non-nil client")
	}
}

func TestNewClientFromCfg_TokenRefreshFailure(t *testing.T) {
	oldRefresh := spotify.RefreshAccessToken
	defer func() { spotify.RefreshAccessToken = oldRefresh }()

	spotify.RefreshAccessToken = func(_, _ string) (spotify.RefreshResult, error) {
		return spotify.RefreshResult{}, errors.New("token refresh failed")
	}

	cfg := &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"}
	_, err := newClientFromCfg(cfg)
	if err == nil || !strings.Contains(err.Error(), "failed to refresh token") {
		t.Fatalf("expected token refresh error, got %v", err)
	}
}

// TestNewClientFromCfg_InvalidGrantDiscardsToken mirrors
// TestNewClientFromConfig_InvalidGrantDiscardsToken but exercises the
// already-loaded-config path used by the run command.
func TestNewClientFromCfg_InvalidGrantDiscardsToken(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
	}()

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "stale-refresh"})
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	spotify.RefreshAccessToken = func(_, _ string) (spotify.RefreshResult, error) {
		return spotify.RefreshResult{}, fmt.Errorf("%w: token revoked", spotify.ErrInvalidGrant)
	}

	_, err = newClientFromCfg(cfg)
	if err == nil || !strings.Contains(err.Error(), "spotctl setup") {
		t.Fatalf("expected reauthorization error mentioning 'spotctl setup', got %v", err)
	}

	reloaded, loadErr := config.Load(configPath)
	if loadErr != nil {
		if !strings.Contains(loadErr.Error(), "refresh_token") {
			t.Fatalf("unexpected error reloading config: %v", loadErr)
		}
		return
	}
	if reloaded.RefreshToken != "" {
		t.Fatalf("expected stale refresh token to be discarded, got %q", reloaded.RefreshToken)
	}
}

// TestNewClientFromCfg_RotatesRefreshToken covers the rotation-save branch
// specific to newClientFromCfg (used by the run command, which has already
// loaded its own config object before calling this).
func TestNewClientFromCfg_RotatesRefreshToken(t *testing.T) {
	oldConfigPath := configPath
	oldRefresh := spotify.RefreshAccessToken
	defer func() {
		configPath = oldConfigPath
		spotify.RefreshAccessToken = oldRefresh
	}()

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "old-refresh"})
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	spotify.RefreshAccessToken = func(_, _ string) (spotify.RefreshResult, error) {
		return spotify.RefreshResult{AccessToken: "new-access", NewRefreshToken: "new-refresh"}, nil
	}

	if _, err := newClientFromCfg(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RefreshToken != "new-refresh" {
		t.Fatalf("expected in-memory cfg to reflect rotated token, got %q", cfg.RefreshToken)
	}

	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.RefreshToken != "new-refresh" {
		t.Fatalf("expected rotated refresh token to be persisted, got %q", reloaded.RefreshToken)
	}
}
