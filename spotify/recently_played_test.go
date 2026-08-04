package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetRecentlyPlayedSuccess(t *testing.T) {
	played := time.Date(2026, 8, 4, 3, 10, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/recently-played" {
			t.Fatalf("expected GET /recently-played, got %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Fatalf("expected limit=5, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RecentlyPlayedResponse{
			Items: []RecentlyPlayedItem{
				{
					Track:    Track{Name: "Weightless", Artists: []Artist{{Name: "Marconi Union"}}},
					PlayedAt: played,
					Context:  &PlaybackContext{URI: "spotify:playlist:2SNUuYsP7S4K3V4xANjA46", Type: "playlist"},
				},
			},
		})
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	recent, err := client.GetRecentlyPlayed(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent.Items) != 1 || recent.Items[0].Track.Name != "Weightless" {
		t.Fatalf("unexpected response: %#v", recent)
	}
	if !recent.Items[0].PlayedAt.Equal(played) {
		t.Fatalf("expected played_at %v, got %v", played, recent.Items[0].PlayedAt)
	}
}

func TestGetRecentlyPlayedNoLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("limit") {
			t.Fatalf("expected no limit param, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RecentlyPlayedResponse{})
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	if _, err := client.GetRecentlyPlayed(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
}

func TestGetRecentlyPlayedInvalidLimit(t *testing.T) {
	client := &Client{accessToken: "t", httpClient: http.DefaultClient, urlPlayer: "http://example.invalid"}
	if _, err := client.GetRecentlyPlayed(context.Background(), 51); err == nil {
		t.Fatal("expected error for limit > 50")
	}
	if _, err := client.GetRecentlyPlayed(context.Background(), -1); err == nil {
		t.Fatal("expected error for negative limit")
	}
}

func TestGetRecentlyPlayedErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	if _, err := client.GetRecentlyPlayed(context.Background(), 0); err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestGetRecentlyPlayedDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	if _, err := client.GetRecentlyPlayed(context.Background(), 0); err == nil {
		t.Fatal("expected decode error")
	}
}
