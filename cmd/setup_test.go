package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestOauthFlow_Success(t *testing.T) {
	// Setup test server to simulate Spotify token endpoint
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
	defer ts.Close()

	// Patch http.DefaultClient.Do to redirect to our test server
	oldDefaultClient := http.DefaultClient
	http.DefaultClient = ts.Client()
	defer func() { http.DefaultClient = oldDefaultClient }()

	clientID := "cid"
	clientSecret := "csecret"
	redirectURI := "http://localhost/callback"

	// Simulate user input for redirect URL
	redirected := redirectURI + "?code=testcode"
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()
	go func() {
		w.Write([]byte(redirected + "\n"))
		w.Close()
	}()

	// Patch prompt to just read from stdin
	refreshToken, err := oauthFlow(clientID, clientSecret, redirectURI, ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refreshToken != "test-refresh-token" {
		t.Fatalf("expected refresh token, got %q", refreshToken)
	}
}

func TestOauthFlow_ParseError(t *testing.T) {
	clientID := "cid"
	clientSecret := "csecret"
	redirectURI := "http://localhost/callback"

	// Simulate bad URL input
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = os.NewFile(0, "/dev/stdin") }()
	go func() {
		w.Write([]byte("not a url\n"))
		w.Close()
	}()

	_, err := oauthFlow(clientID, clientSecret, redirectURI, "http://localhost/token")
	if err == nil || (!strings.Contains(err.Error(), "could not parse redirect URL") && !strings.Contains(err.Error(), "no code found in redirect URL")) {
		t.Fatalf("expected parse or no code error, got %v", err)
	}
}

func TestOauthFlow_NoCode(t *testing.T) {
	clientID := "cid"
	clientSecret := "csecret"
	redirectURI := "http://localhost/callback"

	// Simulate URL with no code
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = os.NewFile(0, "/dev/stdin") }()
	go func() {
		w.Write([]byte(redirectURI + "?error=access_denied\n"))
		w.Close()
	}()

	_, err := oauthFlow(clientID, clientSecret, redirectURI, "http://localhost/token")
	if err == nil || !strings.Contains(err.Error(), "no code found") {
		t.Fatalf("expected no code error, got %v", err)
	}
}

func TestOauthFlow_TokenExchangeError(t *testing.T) {
	clientID := "cid"
	clientSecret := "csecret"
	redirectURI := "http://localhost/callback"

	// Patch http.DefaultClient.Do to simulate network error
	oldDefaultClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network fail")
		}),
	}
	defer func() { http.DefaultClient = oldDefaultClient }()

	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = os.NewFile(0, "/dev/stdin") }()
	go func() {
		w.Write([]byte(redirectURI + "?code=testcode\n"))
		w.Close()
	}()

	_, err := oauthFlow(clientID, clientSecret, redirectURI, "http://localhost/token")
	if err == nil || !strings.Contains(err.Error(), "token exchange failed") {
		t.Fatalf("expected token exchange error, got %v", err)
	}
}

func TestOauthFlow_DecodeError(t *testing.T) {
	clientID := "cid"
	clientSecret := "csecret"
	redirectURI := "http://localhost/callback"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer ts.Close()

	oldDefaultClient := http.DefaultClient
	http.DefaultClient = ts.Client()
	defer func() { http.DefaultClient = oldDefaultClient }()

	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = os.NewFile(0, "/dev/stdin") }()
	go func() {
		w.Write([]byte(redirectURI + "?code=testcode\n"))
		w.Close()
	}()

	_, err := oauthFlow(clientID, clientSecret, redirectURI, ts.URL)
	if err == nil || !strings.Contains(err.Error(), "could not decode token response") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
