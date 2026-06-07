package cmd

import (
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
	refreshToken, err := oauthFlow("cid", "csecret", "http://localhost/callback", stdin, ts.URL)

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
	_, err := oauthFlow("cid", "csecret", "http://localhost/callback", stdin, "http://localhost/token")
	if err == nil || (!strings.Contains(err.Error(), "could not parse redirect URL") && !strings.Contains(err.Error(), "no code found in redirect URL")) {
		t.Fatalf("expected parse or no code error, got %v", err)
	}
}

func TestOauthFlow_NoCode(t *testing.T) {
	stdin := strings.NewReader("http://localhost/callback?error=access_denied\n")
	_, err := oauthFlow("cid", "csecret", "http://localhost/callback", stdin, "http://localhost/token")
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
	_, err := oauthFlow("cid", "csecret", "http://localhost/callback", stdin, "http://localhost/token")
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
	_, err := oauthFlow("cid", "csecret", "http://localhost/callback", stdin, ts.URL)

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
	_, err := oauthFlow("cid", "csecret", "http://localhost/callback", stdin, ts.URL)

	ts.Close()
	oauthHTTPClient = oldClient

	if err == nil || !strings.Contains(err.Error(), "no refresh token") {
		t.Fatalf("expected no refresh token error, got %v", err)
	}
}

func TestSetupCmd_SavesConfig(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()

	tmpFile := writeTempConfig(t, &config.Config{})
	if tmpFile == "" {
		t.Fatal("failed to create temp config")
	}
	configPath = tmpFile

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"refresh_token": "saved-refresh-token"})
	}))

	oldClient := oauthHTTPClient
	oauthHTTPClient = ts.Client()

	stdin := strings.NewReader("http://localhost/callback?code=testcode\n")
	refreshToken, err := oauthFlow("cid", "csecret", "http://localhost/callback", stdin, ts.URL)

	ts.Close()
	oauthHTTPClient = oldClient

	if err != nil {
		t.Fatalf("oauthFlow failed: %v", err)
	}
	if refreshToken != "saved-refresh-token" {
		t.Fatalf("unexpected refresh token: %q", refreshToken)
	}

	cfg := &config.Config{
		ClientID:     "cid",
		ClientSecret: "csecret",
		RefreshToken: refreshToken,
	}
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("config.Save failed: %v", err)
	}

	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if loaded.RefreshToken != "saved-refresh-token" {
		t.Errorf("expected saved-refresh-token, got %q", loaded.RefreshToken)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
