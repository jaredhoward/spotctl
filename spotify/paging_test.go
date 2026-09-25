package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

// pagedServer serves total generic items under path, honouring limit/offset
// the way Spotify does, and records the offsets it was asked for.
func pagedServer(t *testing.T, path string, total int, offsets *[]int, itemJSON func(i int) string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit != 50 {
			t.Errorf("expected limit=50 on every page, got %d", limit)
		}
		*offsets = append(*offsets, offset)
		items := []json.RawMessage{}
		for i := offset; i < offset+limit && i < total; i++ {
			items = append(items, json.RawMessage(itemJSON(i)))
		}
		json.NewEncoder(w).Encode(map[string]any{"total": total, "offset": offset, "items": items})
	}))
}

func TestGetAllUserPlaylistsWalksEveryPage(t *testing.T) {
	var offsets []int
	srv := pagedServer(t, "/v1/me/playlists", 120, &offsets, func(i int) string {
		return `{"id":"p` + strconv.Itoa(i) + `","name":"n"}`
	})
	defer srv.Close()

	got, err := newPlaylistTestClient(srv).GetAllUserPlaylists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 120 || got[0].ID != "p0" || got[119].ID != "p119" {
		t.Fatalf("expected 120 playlists in order, got %d", len(got))
	}
	if len(offsets) != 3 || offsets[0] != 0 || offsets[1] != 50 || offsets[2] != 100 {
		t.Errorf("expected offsets 0,50,100, got %v", offsets)
	}
}

func TestGetAllUserPlaylistsExactPageBoundaryDoesNotOverfetch(t *testing.T) {
	var offsets []int
	srv := pagedServer(t, "/v1/me/playlists", 50, &offsets, func(i int) string { return `{"id":"p"}` })
	defer srv.Close()

	got, err := newPlaylistTestClient(srv).GetAllUserPlaylists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 50 || len(offsets) != 1 {
		t.Errorf("expected 50 items in 1 request, got %d items in %d requests", len(got), len(offsets))
	}
}

func TestGetAllUserPlaylistsEmpty(t *testing.T) {
	var offsets []int
	srv := pagedServer(t, "/v1/me/playlists", 0, &offsets, func(i int) string { return `{}` })
	defer srv.Close()

	got, err := newPlaylistTestClient(srv).GetAllUserPlaylists(context.Background())
	if err != nil || len(got) != 0 || len(offsets) != 1 {
		t.Fatalf("expected no items in 1 request, got %d items, %d requests, err %v", len(got), len(offsets), err)
	}
}

func TestAllPagesStopsOnEmptyPageDespiteTotal(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		// Claims 500 items but never returns any.
		w.Write([]byte(`{"total":500,"items":[]}`))
	}))
	defer srv.Close()

	got, err := newPlaylistTestClient(srv).GetAllUserPlaylists(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("expected clean empty result, got %d items, err %v", len(got), err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("expected to stop after 1 request, made %d", n)
	}
}

func TestGetAllPlaylistItemsAndSavedTracks(t *testing.T) {
	var itemOffsets, savedOffsets []int
	items := pagedServer(t, "/v1/playlists/abc/items", 60, &itemOffsets, func(i int) string {
		return `{"item":{"uri":"spotify:track:t` + strconv.Itoa(i) + `","name":"n"}}`
	})
	defer items.Close()
	gotItems, err := newPlaylistTestClient(items).GetAllPlaylistItems(context.Background(), "spotify:playlist:abc")
	if err != nil || len(gotItems) != 60 || gotItems[59].Item.URI != "spotify:track:t59" {
		t.Fatalf("unexpected playlist items: %d, %v", len(gotItems), err)
	}

	saved := pagedServer(t, "/v1/me/tracks", 51, &savedOffsets, func(i int) string {
		return `{"track":{"uri":"spotify:track:s` + strconv.Itoa(i) + `"}}`
	})
	defer saved.Close()
	gotSaved, err := newPlaylistTestClient(saved).GetAllSavedTracks(context.Background())
	if err != nil || len(gotSaved) != 51 || gotSaved[50].Track.URI != "spotify:track:s50" {
		t.Fatalf("unexpected saved tracks: %d, %v", len(gotSaved), err)
	}
}

func TestGetAllPropagatesMidWalkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") == "50" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		items := make([]string, 50)
		for i := range items {
			items[i] = `{"id":"p"}`
		}
		w.Write([]byte(`{"total":120,"items":[` + join(items) + `]}`))
	}))
	defer srv.Close()

	_, err := newPlaylistTestClient(srv).GetAllUserPlaylists(context.Background())
	var httpErr *HTTPStatusError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected the second page's 403 to surface, got %v", err)
	}
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}

func TestGetCurrentUserID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/me" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"id":"user123","email":"x@example.com","display_name":"Me"}`))
	}))
	defer srv.Close()

	id, err := newPlaylistTestClient(srv).GetCurrentUserID(context.Background())
	if err != nil || id != "user123" {
		t.Fatalf("got %q, %v", id, err)
	}
}

func TestGetCurrentUserIDErrors(t *testing.T) {
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer empty.Close()
	if _, err := newPlaylistTestClient(empty).GetCurrentUserID(context.Background()); err == nil {
		t.Error("expected error when the response has no id")
	}

	denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer denied.Close()
	if _, err := newPlaylistTestClient(denied).GetCurrentUserID(context.Background()); err == nil {
		t.Error("expected error on 401")
	}
}
