package sets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaredhoward/spotctl/spotify"
)

type fakeAction struct {
	called bool
}

func (f *fakeAction) Dispatch(_ context.Context, _ *spotify.Client) error {
	f.called = true
	return nil
}

func (f *fakeAction) Confirmed(_ *spotify.PlaybackState) bool { return false }
func (f *fakeAction) Label() string                           { return "fake" }

func TestSnapshotAction_DispatchConfirmedAndLabel(t *testing.T) {
	oldURLPlayer := spotify.URLPlayer
	t.Cleanup(func() { spotify.URLPlayer = oldURLPlayer })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(spotify.PlaybackState{Item: &spotify.Track{URI: "spotify:track:old"}})
	}))
	defer srv.Close()

	spotify.URLPlayer = srv.URL
	client := spotify.NewClient("token")
	client.SetHTTPClient(srv.Client())

	inner := &fakeAction{}
	s := &snapshotAction{inner: inner}
	if err := s.Dispatch(context.Background(), client); err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if !inner.called {
		t.Fatal("expected inner action to be dispatched")
	}
	if got := s.Label(); got != "fake" {
		t.Fatalf("expected label to forward to inner action, got %q", got)
	}
	if !s.Confirmed(&spotify.PlaybackState{Item: &spotify.Track{URI: "spotify:track:new"}}) {
		t.Fatal("expected Confirmed to be true when current URI differs from prior URI")
	}
	if s.Confirmed(&spotify.PlaybackState{Item: &spotify.Track{URI: "spotify:track:old"}}) {
		t.Fatal("expected Confirmed to be false when current URI matches prior URI")
	}
}
