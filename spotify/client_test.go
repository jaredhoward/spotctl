package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// captureStderr runs fn with os.Stderr redirected and returns what it wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	w.Close()
	out, err := io.ReadAll(r)
	os.Stderr = old
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestDoExpectSuccess_NonTwoXX_IncludesBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request body"))
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/play?device_id=device", server.URL), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer t")
	err = client.doExpectSuccess(req, "play")
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
	if !strings.Contains(err.Error(), "bad request body") {
		t.Errorf("expected response body in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected status code in error, got: %v", err)
	}
}

func TestGetCurrentPlayback_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	state, err := client.GetCurrentPlayback(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if state != nil {
		t.Error("expected nil state on error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected status in error, got: %v", err)
	}
}

func TestGetCurrentPlayback_NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	state, err := client.GetCurrentPlayback(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != nil {
		t.Error("expected nil state for 204 No Content")
	}
}

func TestGetCurrentPlayback_WithState(t *testing.T) {
	expected := PlaybackState{
		IsPlaying:    true,
		ShuffleState: true,
		RepeatState:  "context",
		ProgressMS:   30000,
		Device:       Device{ID: "dev1", Name: "Speaker", VolumePercent: 50, IsActive: true},
		Item: &Track{
			URI:        "spotify:track:abc",
			Name:       "Song",
			DurationMS: 200000,
			Artists:    []Artist{{Name: "Artist"}},
		},
		Context: &PlaybackContext{URI: "spotify:playlist:xyz", Type: "playlist"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	state, err := client.GetCurrentPlayback(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if state.Device.ID != "dev1" {
		t.Errorf("device ID: got %q", state.Device.ID)
	}
	if state.Item == nil || state.Item.URI != "spotify:track:abc" {
		t.Errorf("item URI: got %v", state.Item)
	}
	if state.Context == nil || state.Context.URI != "spotify:playlist:xyz" {
		t.Errorf("context URI: got %v", state.Context)
	}
	if !state.IsPlaying {
		t.Error("expected IsPlaying=true")
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("token")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.accessToken != "token" {
		t.Fatalf("expected access token token, got %q", c.accessToken)
	}
	if c.httpClient == nil {
		t.Fatal("expected a configured HTTP client")
	}
	if c.httpClient.Timeout != 10*time.Second {
		t.Fatalf("expected timeout 10s, got %v", c.httpClient.Timeout)
	}
}

func TestRawRequest_GET_NoBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/me" {
			t.Errorf("expected path /v1/me, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer t" {
			t.Errorf("expected bearer token header, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"me"}`))
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client()}
	client.SetAPIBase(server.URL)

	status, body, err := client.RawRequest(context.Background(), "GET", "/v1/me", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Errorf("expected status 200, got %d", status)
	}
	if string(body) != `{"id":"me"}` {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestRawRequest_PUT_WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected json content-type, got %q", r.Header.Get("Content-Type"))
		}
		got, _ := io.ReadAll(r.Body)
		if string(got) != `{"context_uri":"spotify:playlist:abc"}` {
			t.Errorf("unexpected request body: %s", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client()}
	client.SetAPIBase(server.URL)

	status, body, err := client.RawRequest(context.Background(), "PUT", "/v1/me/player/play?device_id=xxx", `{"context_uri":"spotify:playlist:abc"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", status)
	}
	if len(body) != 0 {
		t.Errorf("expected empty body for 204, got %s", body)
	}
}

func TestRawRequest_NonSuccessStatus_StillReturnsBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client()}
	client.SetAPIBase(server.URL)

	status, body, err := client.RawRequest(context.Background(), "GET", "/v1/whatever", "")
	if err != nil {
		t.Fatalf("expected no error for a non-2xx response, got %v", err)
	}
	if status != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", status)
	}
	if !strings.Contains(string(body), "bad request") {
		t.Errorf("expected error body to be returned, got %s", body)
	}
}

func TestRawRequest_AddsLeadingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/me" {
			t.Errorf("expected leading slash normalized to /v1/me, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client()}
	client.SetAPIBase(server.URL)

	if _, _, err := client.RawRequest(context.Background(), "GET", "v1/me", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRawRequest_TransportError(t *testing.T) {
	client := &Client{accessToken: "t", httpClient: &http.Client{Timeout: time.Millisecond}}
	client.SetAPIBase("http://127.0.0.1:1")

	_, _, err := client.RawRequest(context.Background(), "GET", "/v1/me", "")
	if err == nil {
		t.Fatal("expected error from unreachable server")
	}
}

func TestRawRequest_InvalidMethod(t *testing.T) {
	client := &Client{accessToken: "t", httpClient: http.DefaultClient}
	client.SetAPIBase("http://example.com")

	_, _, err := client.RawRequest(context.Background(), "BAD METHOD", "/v1/me", "")
	if err == nil {
		t.Fatal("expected error for invalid method")
	}
}

func TestPlayerURL_WithAndWithoutDevice(t *testing.T) {
	cases := []struct {
		base     string
		path     string
		deviceID string
		want     string
	}{
		{"http://base", "/play", "dev1", "http://base/play?device_id=dev1"},
		{"http://base", "/play", "", "http://base/play"},
		{"http://base", "", "dev1", "http://base?device_id=dev1"},
		{"http://base", "", "", "http://base"},
	}
	for _, tc := range cases {
		got := playerURL(tc.base, tc.path, tc.deviceID)
		if got != tc.want {
			t.Errorf("playerURL(%q, %q, %q) = %q, want %q", tc.base, tc.path, tc.deviceID, got, tc.want)
		}
	}
}

func TestDoRequest_VerboseLogsAndPreservesBody(t *testing.T) {
	old := Verbose
	Verbose = true
	defer func() { Verbose = old }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"is_playing":true}`))
	}))
	defer server.Close()

	client := &Client{accessToken: "secret-token", httpClient: server.Client(), urlPlayer: server.URL}

	var state *PlaybackState
	var err error
	logs := captureStderr(t, func() {
		state, err = client.GetCurrentPlayback(context.Background())
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil || !state.IsPlaying {
		t.Fatalf("expected body to still be readable after verbose logging, got %+v", state)
	}
	if !strings.Contains(logs, "[http]") {
		t.Errorf("expected verbose HTTP trace, got: %q", logs)
	}
	if strings.Contains(logs, "secret-token") {
		t.Errorf("verbose logging must never include the access token, got: %q", logs)
	}
}

func TestDoRequest_VerboseLogsRequestBody(t *testing.T) {
	old := Verbose
	Verbose = true
	defer func() { Verbose = old }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{accessToken: "t", httpClient: server.Client(), urlPlayer: server.URL}
	p := &Play{ContextURI: "spotify:playlist:abc"}

	logs := captureStderr(t, func() {
		if err := p.Dispatch(context.Background(), client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(logs, "spotify:playlist:abc") {
		t.Errorf("expected request body in verbose trace, got: %q", logs)
	}
}

func TestDoRequest_VerboseLogsTransportError(t *testing.T) {
	old := Verbose
	Verbose = true
	defer func() { Verbose = old }()

	client := &Client{accessToken: "t", httpClient: &http.Client{Timeout: time.Millisecond}, urlPlayer: "http://127.0.0.1:1"}

	var err error
	logs := captureStderr(t, func() {
		_, err = client.GetCurrentPlayback(context.Background())
	})
	if err == nil {
		t.Fatal("expected error from unreachable server")
	}
	if !strings.Contains(logs, "[http]") {
		t.Errorf("expected verbose error trace, got: %q", logs)
	}
}

func TestPlayerURL_WithExtraParams(t *testing.T) {
	got := playerURL("http://base", "/shuffle", "dev1", url.Values{"state": {"true"}})
	if got != "http://base/shuffle?device_id=dev1&state=true" {
		t.Errorf("unexpected URL: %q", got)
	}

	got = playerURL("http://base", "/shuffle", "", url.Values{"state": {"false"}})
	if got != "http://base/shuffle?state=false" {
		t.Errorf("unexpected URL: %q", got)
	}
}
