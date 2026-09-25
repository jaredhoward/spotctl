package spotify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newPlaylistTestClient(srv *httptest.Server) *Client {
	return &Client{accessToken: "t", httpClient: srv.Client(), apiBase: srv.URL}
}

func TestParsePlaylistID(t *testing.T) {
	const id = "37i9dQZF1DXcBWIGoYBM5M"
	valid := []string{
		id,
		"  " + id + "\n",
		"spotify:playlist:" + id,
		"https://open.spotify.com/playlist/" + id,
		"https://open.spotify.com/playlist/" + id + "?si=abc123",
	}
	for _, in := range valid {
		got, err := ParsePlaylistID(in)
		if err != nil || got != id {
			t.Errorf("ParsePlaylistID(%q) = %q, %v; want %q", in, got, err, id)
		}
	}

	invalid := []string{
		"",
		"spotify:playlist:",
		"spotify:track:" + id,
		"https://open.spotify.com/track/" + id,
		"https://open.spotify.com/playlist/",
		id + "/../me",
		id + "?x=1",
		"not an id",
	}
	for _, in := range invalid {
		if got, err := ParsePlaylistID(in); err == nil {
			t.Errorf("ParsePlaylistID(%q) = %q, nil; want error", in, got)
		}
	}
}

func TestGetUserPlaylistsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/me/playlists" {
			t.Fatalf("expected GET /v1/me/playlists, got %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer t" {
			t.Fatalf("unexpected Authorization header %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Fatalf("expected limit=5, got %q", got)
		}
		if got := r.URL.Query().Get("offset"); got != "10" {
			t.Fatalf("expected offset=10, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"limit": 5, "offset": 10, "total": 12, "next": null, "previous": "https://x",
			"items": [
				{"id":"a","uri":"spotify:playlist:a","name":"Chill","collaborative":true,
				 "owner":{"id":"u1","display_name":"Jared"},"items":{"total":7}},
				{"id":"c","uri":"spotify:playlist:c","name":"No Count","owner":{"id":"u2"}}
			]
		}`))
	}))
	defer srv.Close()

	page, err := newPlaylistTestClient(srv).GetUserPlaylists(context.Background(), 5, 10)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 12 || page.Offset != 10 || page.Next != "" || len(page.Items) != 2 {
		t.Fatalf("unexpected page: %#v", page)
	}
	if p := page.Items[0]; p.Name != "Chill" || p.Owner.DisplayName != "Jared" || !p.Collaborative || p.TotalItems() != 7 {
		t.Errorf("unexpected first playlist: %#v", p)
	}
	if got := page.Items[1].TotalItems(); got != 0 {
		t.Errorf("expected 0 with no count stub, got %d", got)
	}
}

func TestGetUserPlaylistsDefaultsOmitParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Fatalf("expected no query params, got %q", r.URL.RawQuery)
		}
		w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	if _, err := newPlaylistTestClient(srv).GetUserPlaylists(context.Background(), 0, 0); err != nil {
		t.Fatal(err)
	}
}

func TestGetUserPlaylistsInvalidPaging(t *testing.T) {
	client := &Client{accessToken: "t", httpClient: http.DefaultClient, apiBase: "http://example.invalid"}
	for _, tc := range []struct{ limit, offset int }{{51, 0}, {-1, 0}, {0, -1}} {
		if _, err := client.GetUserPlaylists(context.Background(), tc.limit, tc.offset); err == nil {
			t.Errorf("expected error for limit=%d offset=%d", tc.limit, tc.offset)
		}
	}
}

func TestGetPlaylistItemsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/playlists/abc123/items" {
			t.Fatalf("expected GET /v1/playlists/abc123/items, got %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"limit": 20, "offset": 0, "total": 3, "next": "https://next",
			"items": [
				{"added_at":"2026-08-04T03:10:00Z","is_local":false,
				 "item":{"uri":"spotify:track:t1","name":"Weightless","duration_ms":480000,"artists":[{"name":"Marconi Union"}]}},
				{"added_at":null,"is_local":true,"item":null},
				{"added_at":"2026-08-05T00:00:00Z","item":{"uri":"spotify:track:t3","name":"Third"}}
			]
		}`))
	}))
	defer srv.Close()

	page, err := newPlaylistTestClient(srv).GetPlaylistItems(context.Background(), "spotify:playlist:abc123", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || page.Next != "https://next" || len(page.Items) != 3 {
		t.Fatalf("unexpected page: %#v", page)
	}
	first := page.Items[0]
	if first.Item == nil || first.Item.Name != "Weightless" || first.Item.Artists[0].Name != "Marconi Union" || first.AddedAt.IsZero() {
		t.Errorf("unexpected first item: %#v", first)
	}
	if page.Items[1].Item != nil || !page.Items[1].IsLocal || !page.Items[1].AddedAt.IsZero() {
		t.Errorf("expected null item and null added_at to decode to nil/zero, got %#v", page.Items[1])
	}
}

func TestGetPlaylistItemsInvalidPlaylist(t *testing.T) {
	client := &Client{accessToken: "t", httpClient: http.DefaultClient, apiBase: "http://example.invalid"}
	if _, err := client.GetPlaylistItems(context.Background(), "spotify:track:abc", 0, 0); err == nil {
		t.Fatal("expected error for non-playlist URI")
	}
	if _, err := client.GetPlaylistItems(context.Background(), "abc", 51, 0); err == nil {
		t.Fatal("expected error for limit > 50")
	}
}

func TestGetPlaylistItemsForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(" Forbidden \n"))
	}))
	defer srv.Close()

	_, err := newPlaylistTestClient(srv).GetPlaylistItems(context.Background(), "abc", 0, 0)
	var httpErr *HTTPStatusError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected *HTTPStatusError, got %v", err)
	}
	if httpErr.StatusCode != http.StatusForbidden || httpErr.Body != "Forbidden" {
		t.Errorf("unexpected error: %#v", httpErr)
	}
}

func TestGetUserPlaylistsDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer srv.Close()

	if _, err := newPlaylistTestClient(srv).GetUserPlaylists(context.Background(), 0, 0); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestGetUserPlaylistsRequestError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	client := newPlaylistTestClient(srv)
	srv.Close()

	if _, err := client.GetUserPlaylists(context.Background(), 0, 0); err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestGetUserPlaylistsBadURL(t *testing.T) {
	client := &Client{accessToken: "t", httpClient: http.DefaultClient, apiBase: "http://bad host"}
	if _, err := client.GetUserPlaylists(context.Background(), 0, 0); err == nil {
		t.Fatal("expected error for malformed API base")
	}
}

func TestGetSavedTracksSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/me/tracks" {
			t.Fatalf("expected GET /v1/me/tracks, got %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "2" {
			t.Fatalf("expected limit=2, got %q", got)
		}
		w.Write([]byte(`{"offset":0,"total":900,"next":"https://next","items":[
			{"added_at":"2026-09-01T12:00:00Z","track":{"uri":"spotify:track:t1","name":"Hazey","artists":[{"name":"Glass Animals"}]}},
			{"added_at":"2026-08-01T12:00:00Z","track":null}]}`))
	}))
	defer srv.Close()

	page, err := newPlaylistTestClient(srv).GetSavedTracks(context.Background(), 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 900 || len(page.Items) != 2 {
		t.Fatalf("unexpected page: %#v", page)
	}
	if first := page.Items[0]; first.Track == nil || first.Track.Name != "Hazey" || first.AddedAt.IsZero() {
		t.Errorf("unexpected first entry: %#v", first)
	}
	if page.Items[1].Track != nil {
		t.Errorf("expected null track to decode to nil, got %#v", page.Items[1].Track)
	}
}

func TestGetSavedTracksErrors(t *testing.T) {
	client := &Client{accessToken: "t", httpClient: http.DefaultClient, apiBase: "http://example.invalid"}
	if _, err := client.GetSavedTracks(context.Background(), 51, 0); err == nil {
		t.Fatal("expected error for limit > 50")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := newPlaylistTestClient(srv).GetSavedTracks(context.Background(), 0, 0)
	var httpErr *HTTPStatusError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 *HTTPStatusError, got %v", err)
	}
}
