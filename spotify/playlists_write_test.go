package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func trackURIs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("spotify:track:T%04d", i)
	}
	return out
}

func TestParseTrackURI(t *testing.T) {
	got, err := ParseTrackURI("  spotify:track:21jGcNKet2qwijlDFuPiPb\n")
	if err != nil || got != "spotify:track:21jGcNKet2qwijlDFuPiPb" {
		t.Fatalf("ParseTrackURI = %q, %v", got, err)
	}
	for _, in := range []string{"", "spotify:track:", "spotify:album:abc", "21jGcNKet2qwijlDFuPiPb", "https://open.spotify.com/track/abc", "spotify:track:a b"} {
		if got, err := ParseTrackURI(in); err == nil {
			t.Errorf("ParseTrackURI(%q) = %q, nil; want error", in, got)
		}
	}
}

func TestCreatePlaylistSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/me/playlists" {
			t.Fatalf("expected POST /v1/me/playlists, got %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer t" {
			t.Fatalf("unexpected Authorization %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected Content-Type %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["name"] != "Chill: Test" || body["public"] != false || body["description"] != "desc" {
			t.Fatalf("unexpected body %#v", body)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"new1","uri":"spotify:playlist:new1","name":"Chill: Test"}`))
	}))
	defer srv.Close()

	pl, err := newPlaylistTestClient(srv).CreatePlaylist(context.Background(), "  Chill: Test ", "desc")
	if err != nil {
		t.Fatal(err)
	}
	if pl.ID != "new1" || pl.URI != "spotify:playlist:new1" {
		t.Fatalf("unexpected playlist %#v", pl)
	}
}

func TestCreatePlaylistOmitsEmptyDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["description"]; ok {
			t.Errorf("empty description should be omitted, got %#v", body)
		}
		w.Write([]byte(`{"id":"x","uri":"spotify:playlist:x","name":"n"}`))
	}))
	defer srv.Close()

	if _, err := newPlaylistTestClient(srv).CreatePlaylist(context.Background(), "n", ""); err != nil {
		t.Fatal(err)
	}
}

func TestCreatePlaylistValidation(t *testing.T) {
	c := &Client{accessToken: "t", httpClient: http.DefaultClient, apiBase: "http://127.0.0.1:0"}
	if _, err := c.CreatePlaylist(context.Background(), "   ", ""); err == nil || !strings.Contains(err.Error(), "name") {
		t.Errorf("expected name error, got %v", err)
	}
	long := strings.Repeat("é", MaxPlaylistDescription+1)
	if _, err := c.CreatePlaylist(context.Background(), "n", long); err == nil || !strings.Contains(err.Error(), "301") {
		t.Errorf("expected description length error, got %v", err)
	}
	// Exactly at the limit (counted in characters, not bytes) is fine to send.
	atLimit := strings.Repeat("é", MaxPlaylistDescription)
	if _, err := c.CreatePlaylist(context.Background(), "n", atLimit); err == nil || strings.Contains(err.Error(), "allows at most") {
		t.Errorf("300-character description should pass validation and fail only on the connection, got %v", err)
	}
}

func TestCreatePlaylistErrors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{"forbidden", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(403)
			w.Write([]byte("Insufficient client scope"))
		}, "403"},
		{"bad json", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("not json")) }, "decode"},
		{"no id", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }, "no id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()
			_, err := newPlaylistTestClient(srv).CreatePlaylist(context.Background(), "n", "")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
			if tc.name == "forbidden" {
				var httpErr *HTTPStatusError
				if !errors.As(err, &httpErr) || httpErr.StatusCode != 403 {
					t.Fatalf("expected *HTTPStatusError 403, got %v", err)
				}
			}
		})
	}
}

func TestCreatePlaylistTransportError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	c := newPlaylistTestClient(srv)
	srv.Close()
	if _, err := c.CreatePlaylist(context.Background(), "n", ""); err == nil || !strings.Contains(err.Error(), "create playlist request failed") {
		t.Fatalf("expected transport error, got %v", err)
	}
}

func TestAddPlaylistItemsBatches(t *testing.T) {
	var batches [][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/playlists/abc123/items" {
			t.Fatalf("expected POST /v1/playlists/abc123/items, got %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			URIs []string `json:"uris"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		batches = append(batches, body.URIs)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"snapshot_id":"s"}`))
	}))
	defer srv.Close()

	uris := trackURIs(250)
	if err := newPlaylistTestClient(srv).AddPlaylistItems(context.Background(), "spotify:playlist:abc123", uris); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 || len(batches[0]) != 100 || len(batches[1]) != 100 || len(batches[2]) != 50 {
		t.Fatalf("unexpected batch sizes: %d batches", len(batches))
	}
	var flat []string
	for _, b := range batches {
		flat = append(flat, b...)
	}
	if strings.Join(flat, ",") != strings.Join(uris, ",") {
		t.Fatal("tracks were reordered or dropped across batches")
	}
}

func TestAddPlaylistItemsNothingToAdd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no request expected for an empty list")
	}))
	defer srv.Close()
	if err := newPlaylistTestClient(srv).AddPlaylistItems(context.Background(), "abc", nil); err != nil {
		t.Fatal(err)
	}
}

func TestAddPlaylistItemsValidatesBeforeSending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no request expected when validation fails")
	}))
	defer srv.Close()
	c := newPlaylistTestClient(srv)

	if err := c.AddPlaylistItems(context.Background(), "not a playlist", trackURIs(1)); err == nil {
		t.Error("expected invalid playlist error")
	}
	// The bad URI is in the second batch; nothing may be sent for the first.
	uris := append(trackURIs(150), "spotify:album:nope")
	if err := c.AddPlaylistItems(context.Background(), "abc", uris); err == nil || !strings.Contains(err.Error(), "spotify:album:nope") {
		t.Errorf("expected invalid track error, got %v", err)
	}
}

func TestAddPlaylistItemsStopsOnFailure(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		io.Copy(io.Discard, r.Body)
		if calls == 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	err := newPlaylistTestClient(srv).AddPlaylistItems(context.Background(), "abc", trackURIs(250))
	if err == nil || !strings.Contains(err.Error(), "tracks 101–200 of 250") {
		t.Fatalf("expected failure naming the batch, got %v", err)
	}
	var httpErr *HTTPStatusError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != 502 {
		t.Fatalf("expected wrapped 502, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected to stop after the failed batch, made %d calls", calls)
	}
}

func TestSendJSONBuildFailures(t *testing.T) {
	c := &Client{accessToken: "t", httpClient: http.DefaultClient, apiBase: "http://127.0.0.1:0"}
	if err := c.sendJSON(context.Background(), http.MethodPost, "/x", make(chan int), "act", "Act", nil); err == nil || !strings.Contains(err.Error(), "encode") {
		t.Errorf("expected encode error, got %v", err)
	}
	if err := c.sendJSON(context.Background(), "bad method", "/x", map[string]int{}, "act", "Act", nil); err == nil || !strings.Contains(err.Error(), "create act request") {
		t.Errorf("expected request creation error, got %v", err)
	}
}
