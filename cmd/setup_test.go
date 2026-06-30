package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
)

func TestOauthFlow_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		if r.FormValue("code") != "testcode" {
			t.Fatalf("expected code 'testcode', got %q", r.FormValue("code"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"refresh_token": "test-refresh-token"})
	}))

	oldClient := oauthHTTPClient
	oauthHTTPClient = ts.Client()

	stdin := strings.NewReader("http://localhost/callback?code=testcode\n")
	refreshToken, err := oauthFlow(context.Background(), "cid", "csecret", "http://localhost/callback", stdin, ts.URL)

	ts.Close()
	oauthHTTPClient = oldClient

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refreshToken != "test-refresh-token" {
		t.Fatalf("expected refresh token, got %q", refreshToken)
	}
}

func TestOauthFlow_ParseError(t *testing.T) {
	stdin := strings.NewReader("not a url\n")
	_, err := oauthFlow(context.Background(), "cid", "csecret", "http://localhost/callback", stdin, "http://localhost/token")
	if err == nil || (!strings.Contains(err.Error(), "could not parse redirect URL") && !strings.Contains(err.Error(), "no code found in redirect URL") && !strings.Contains(err.Error(), "does not match configured redirect URI")) {
		t.Fatalf("expected parse, no code, or mismatch error, got %v", err)
	}
}

func TestOauthFlow_RedirectMismatch(t *testing.T) {
	stdin := strings.NewReader("http://localhost:9999/callback?code=testcode\n")
	_, err := oauthFlow(context.Background(), "cid", "csecret", "http://localhost:8080/callback", stdin, "http://localhost/token")
	if err == nil || !strings.Contains(err.Error(), "does not match configured redirect URI") {
		t.Fatalf("expected redirect mismatch error, got %v", err)
	}
}

func TestOauthFlow_NoCode(t *testing.T) {
	stdin := strings.NewReader("http://localhost/callback?error=access_denied\n")
	_, err := oauthFlow(context.Background(), "cid", "csecret", "http://localhost/callback", stdin, "http://localhost/token")
	if err == nil || !strings.Contains(err.Error(), "no code found") {
		t.Fatalf("expected no code error, got %v", err)
	}
}

func TestOauthFlow_TokenExchangeError(t *testing.T) {
	oldClient := oauthHTTPClient
	oauthHTTPClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network fail")
		}),
	}
	defer func() { oauthHTTPClient = oldClient }()

	stdin := strings.NewReader("http://localhost/callback?code=testcode\n")
	_, err := oauthFlow(context.Background(), "cid", "csecret", "http://localhost/callback", stdin, "http://localhost/token")
	if err == nil || !strings.Contains(err.Error(), "token exchange failed") {
		t.Fatalf("expected token exchange error, got %v", err)
	}
}

func TestOauthFlow_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))

	oldClient := oauthHTTPClient
	oauthHTTPClient = ts.Client()

	stdin := strings.NewReader("http://localhost/callback?code=testcode\n")
	_, err := oauthFlow(context.Background(), "cid", "csecret", "http://localhost/callback", stdin, ts.URL)

	ts.Close()
	oauthHTTPClient = oldClient

	if err == nil || !strings.Contains(err.Error(), "could not decode token response") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestOauthFlow_NoRefreshToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"access_token": "only-access"})
	}))

	oldClient := oauthHTTPClient
	oauthHTTPClient = ts.Client()

	stdin := strings.NewReader("http://localhost/callback?code=testcode\n")
	_, err := oauthFlow(context.Background(), "cid", "csecret", "http://localhost/callback", stdin, ts.URL)

	ts.Close()
	oauthHTTPClient = oldClient

	if err == nil || !strings.Contains(err.Error(), "no refresh token") {
		t.Fatalf("expected no refresh token error, got %v", err)
	}
}

func TestRunSetup_SavesConfig(t *testing.T) {
	oldConfigPath := configPath
	oldStdin := setupStdin
	oldClient := oauthHTTPClient
	oldEndpoint := setupTokenEndpoint
	defer func() {
		configPath = oldConfigPath
		setupStdin = oldStdin
		oauthHTTPClient = oldClient
		setupTokenEndpoint = oldEndpoint
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"refresh_token": "saved-refresh-token"})
	}))
	defer ts.Close()

	configPath = t.TempDir() + "/config.yaml"
	oauthHTTPClient = ts.Client()
	setupTokenEndpoint = ts.URL
	// Prompts: clientID, clientSecret, redirectURI; then oauthFlow reads the redirect URL.
	setupStdin = strings.NewReader("myclientid\nmyclientsecret\nhttp://localhost/callback\nhttp://localhost/callback?code=testcode\n")

	if err := setupCmd.RunE(setupCmd, nil); err != nil {
		t.Fatalf("runSetup failed: %v", err)
	}

	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if loaded.ClientID != "myclientid" {
		t.Errorf("expected clientID myclientid, got %q", loaded.ClientID)
	}
	if loaded.RefreshToken != "saved-refresh-token" {
		t.Errorf("expected saved-refresh-token, got %q", loaded.RefreshToken)
	}
}

func TestRunSetup_PreservesExistingSets(t *testing.T) {
	oldConfigPath := configPath
	oldStdin := setupStdin
	oldClient := oauthHTTPClient
	oldEndpoint := setupTokenEndpoint
	defer func() {
		configPath = oldConfigPath
		setupStdin = oldStdin
		oauthHTTPClient = oldClient
		setupTokenEndpoint = oldEndpoint
	}()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"refresh_token": "new-token"})
	}))
	defer ts.Close()

	existing := &config.Config{
		ClientID:     "old-id",
		ClientSecret: "old-secret",
		RefreshToken: "old-token",
		RedirectURI:  "http://localhost/callback",
		Sets:         map[string]config.Set{"mySet": {}},
	}
	configPath = writeTempConfig(t, existing)
	oauthHTTPClient = ts.Client()
	setupTokenEndpoint = ts.URL
	// Empty lines accept the pre-filled defaults; last line is the OAuth redirect URL.
	setupStdin = strings.NewReader("\n\n\nhttp://localhost/callback?code=testcode\n")

	if err := setupCmd.RunE(setupCmd, nil); err != nil {
		t.Fatalf("runSetup failed: %v", err)
	}

	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if _, ok := loaded.Sets["mySet"]; !ok {
		t.Error("expected existing sets to be preserved")
	}
	if loaded.RefreshToken != "new-token" {
		t.Errorf("expected new-token, got %q", loaded.RefreshToken)
	}
}

func TestRunSetup_OAuthError(t *testing.T) {
	oldConfigPath := configPath
	oldStdin := setupStdin
	oldClient := oauthHTTPClient
	oldEndpoint := setupTokenEndpoint
	defer func() {
		configPath = oldConfigPath
		setupStdin = oldStdin
		oauthHTTPClient = oldClient
		setupTokenEndpoint = oldEndpoint
	}()

	configPath = t.TempDir() + "/config.yaml"
	setupTokenEndpoint = "http://localhost/token"
	oauthHTTPClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network fail")
		}),
	}
	setupStdin = strings.NewReader("cid\nsecret\nhttp://localhost/callback\nhttp://localhost/callback?code=testcode\n")

	err := setupCmd.RunE(setupCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "OAuth flow failed") {
		t.Fatalf("expected OAuth flow failed error, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
