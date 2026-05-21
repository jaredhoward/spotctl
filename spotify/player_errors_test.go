package spotify

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestShuffleErrorStatus covers the error branch in Shuffle (non-204/200 status).
func TestShuffleErrorStatus(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	APIBase = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Shuffle("device"); err == nil {
		t.Fatal("expected error for non-204/200 shuffle response")
	}
}

// TestShuffleOKStatus covers the 200 OK branch in Shuffle.
func TestShuffleOKStatus(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	APIBase = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Shuffle("device"); err != nil {
		t.Fatalf("unexpected error for 200 shuffle response: %v", err)
	}
}

// TestPlayErrorStatus covers the error branch in Play via doExpect204.
func TestPlayErrorStatus(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	APIBase = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Play("device", "spotify:track:abc"); err == nil {
		t.Fatal("expected error for forbidden play response")
	}
}

// TestTransferPlaybackErrorStatus covers the error branch in TransferPlayback.
func TestTransferPlaybackErrorStatus(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	APIBase = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.TransferPlayback([]string{"device"}, false); err == nil {
		t.Fatal("expected error for forbidden transfer response")
	}
}

// TestPauseErrorStatus covers the error branch in Pause.
func TestPauseErrorStatus(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	APIBase = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Pause("device"); err == nil {
		t.Fatal("expected error for forbidden pause response")
	}
}

// TestNextErrorStatus covers the error branch in Next.
func TestNextErrorStatus(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	APIBase = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Next("device"); err == nil {
		t.Fatal("expected error for forbidden next response")
	}
}

// TestPreviousErrorStatus covers the error branch in Previous.
func TestPreviousErrorStatus(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	APIBase = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Previous("device"); err == nil {
		t.Fatal("expected error for forbidden previous response")
	}
}

// TestSetVolumeErrorStatus covers the error branch in SetVolume.
func TestSetVolumeErrorStatus(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	APIBase = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.SetVolume("device", 50); err == nil {
		t.Fatal("expected error for forbidden set-volume response")
	}
}
