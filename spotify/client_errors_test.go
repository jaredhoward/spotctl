package spotify

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func errorServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
}

func TestShuffleErrorStatus(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := errorServer(t, http.StatusForbidden)
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Shuffle("device"); err == nil {
		t.Fatal("expected error for non-204/200 shuffle response")
	}
}

func TestShuffleOKStatus(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Shuffle("device"); err != nil {
		t.Fatalf("unexpected error for 200 shuffle response: %v", err)
	}
}

func TestPlayErrorStatus(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := errorServer(t, http.StatusForbidden)
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Play("device", "spotify:track:abc"); err == nil {
		t.Fatal("expected error for forbidden play response")
	}
}

func TestTransferPlaybackErrorStatus(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := errorServer(t, http.StatusForbidden)
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.TransferPlayback([]string{"device"}, false); err == nil {
		t.Fatal("expected error for forbidden transfer response")
	}
}

func TestPauseErrorStatus(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := errorServer(t, http.StatusForbidden)
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Pause("device"); err == nil {
		t.Fatal("expected error for forbidden pause response")
	}
}

func TestNextErrorStatus(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := errorServer(t, http.StatusForbidden)
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Next("device"); err == nil {
		t.Fatal("expected error for forbidden next response")
	}
}

func TestPreviousErrorStatus(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := errorServer(t, http.StatusForbidden)
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Previous("device"); err == nil {
		t.Fatal("expected error for forbidden previous response")
	}
}

func TestSetVolumeErrorStatus(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := errorServer(t, http.StatusForbidden)
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.SetVolume("device", 50); err == nil {
		t.Fatal("expected error for forbidden set-volume response")
	}
}
