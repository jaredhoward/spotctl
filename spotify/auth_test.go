package spotify

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRefreshAccessTokenSuccess(t *testing.T) {
	oldURL := URLToken
	t.Cleanup(func() { URLToken = oldURL })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", r.Method)
		}
		if !strings.Contains(r.Header.Get("Authorization"), "Basic") {
			t.Fatal("expected Authorization header")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{AccessToken: "access-token", TokenType: "Bearer", ExpiresIn: 3600})
	}))
	defer server.Close()

	URLToken = server.URL
	result, err := RefreshAccessToken(base64.StdEncoding.EncodeToString([]byte("id:secret")), "refresh-token")
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken != "access-token" {
		t.Fatalf("expected access token %q, got %q", "access-token", result.AccessToken)
	}
	if result.NewRefreshToken != "" {
		t.Fatalf("expected no rotated refresh token, got %q", result.NewRefreshToken)
	}
}

func TestRefreshAccessTokenRotation(t *testing.T) {
	oldURL := URLToken
	t.Cleanup(func() { URLToken = oldURL })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "access-token",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
			RefreshToken: "new-refresh-token",
		})
	}))
	defer server.Close()

	URLToken = server.URL
	result, err := RefreshAccessToken("clientb64", "old-refresh-token")
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken != "access-token" {
		t.Fatalf("expected access token %q, got %q", "access-token", result.AccessToken)
	}
	if result.NewRefreshToken != "new-refresh-token" {
		t.Fatalf("expected rotated refresh token %q, got %q", "new-refresh-token", result.NewRefreshToken)
	}
}

func TestRefreshAccessTokenBadResponse(t *testing.T) {
	oldURL := URLToken
	t.Cleanup(func() { URLToken = oldURL })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	URLToken = server.URL
	if _, err := RefreshAccessToken("foo", "bar"); err == nil {
		t.Fatal("expected error for bad token response")
	}
}

func TestRefreshAccessTokenEmptyToken(t *testing.T) {
	oldURL := URLToken
	t.Cleanup(func() { URLToken = oldURL })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{AccessToken: "", TokenType: "Bearer", ExpiresIn: 3600})
	}))
	defer server.Close()

	URLToken = server.URL
	if _, err := RefreshAccessToken("foo", "bar"); err == nil {
		t.Fatal("expected error for empty access token")
	}
}

func TestRefreshAccessTokenDecodeError(t *testing.T) {
	oldURL := URLToken
	t.Cleanup(func() { URLToken = oldURL })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	URLToken = server.URL
	if _, err := RefreshAccessToken("foo", "bar"); err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

// TestRefreshAccessTokenNetworkError covers the branch where the HTTP request
// itself fails (connection refused / network error), not just a bad status.
func TestRefreshAccessTokenNetworkError(t *testing.T) {
	oldURL := URLToken
	t.Cleanup(func() { URLToken = oldURL })

	// Start a server just to get a valid address, then close it immediately so
	// any connection attempt fails with a network error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := server.URL
	server.Close()

	URLToken = addr
	if _, err := RefreshAccessToken("clientb64", "refresh"); err == nil {
		t.Fatal("expected network error when server is closed")
	}
}

// TestRefreshAccessTokenInvalidGrant covers Spotify's documented response for
// an expired/revoked refresh token: a 400 with an invalid_grant error body.
// Callers rely on errors.Is(err, ErrInvalidGrant) to decide whether to
// discard the token and force re-authorization rather than retry.
func TestRefreshAccessTokenInvalidGrant(t *testing.T) {
	oldURL := URLToken
	t.Cleanup(func() { URLToken = oldURL })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "Refresh token revoked",
		})
	}))
	defer server.Close()

	URLToken = server.URL
	_, err := RefreshAccessToken("foo", "bar")
	if err == nil {
		t.Fatal("expected invalid_grant error")
	}
	if !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("expected errors.Is(err, ErrInvalidGrant) to be true, got %v", err)
	}
}

// TestRefreshAccessTokenOtherBadRequest ensures a 400 that isn't an
// invalid_grant (e.g. a malformed request) does NOT get classified as
// ErrInvalidGrant, so callers won't discard a token unnecessarily.
func TestRefreshAccessTokenOtherBadRequest(t *testing.T) {
	oldURL := URLToken
	t.Cleanup(func() { URLToken = oldURL })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_request",
			"error_description": "missing parameter",
		})
	}))
	defer server.Close()

	URLToken = server.URL
	_, err := RefreshAccessToken("foo", "bar")
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("did not expect ErrInvalidGrant for invalid_request, got %v", err)
	}
}
