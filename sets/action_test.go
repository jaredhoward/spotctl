package sets

import (
	"testing"

	"github.com/jaredhoward/spotctl/spotify"
)

func TestSessionReset(t *testing.T) {
	track := func(uri string) *spotify.Track {
		return &spotify.Track{URI: uri}
	}
	state := func(item *spotify.Track, progressMS int) *spotify.PlaybackState {
		return &spotify.PlaybackState{Item: item, ProgressMS: progressMS}
	}

	cases := []struct {
		name  string
		prior *spotify.PlaybackState
		curr  *spotify.PlaybackState
		want  bool
	}{
		{
			name:  "nil prior",
			prior: nil,
			curr:  state(track("a"), 0),
			want:  false,
		},
		{
			name:  "nil current",
			prior: state(track("a"), 1000),
			curr:  nil,
			want:  false,
		},
		{
			name:  "nil item on prior",
			prior: state(nil, 1000),
			curr:  state(track("a"), 0),
			want:  false,
		},
		{
			name:  "nil item on current",
			prior: state(track("a"), 1000),
			curr:  state(nil, 0),
			want:  false,
		},
		{
			name:  "different track — legitimate advance, not a reset",
			prior: state(track("a"), 5000),
			curr:  state(track("b"), 0),
			want:  false,
		},
		{
			name:  "same track, progress advanced",
			prior: state(track("a"), 1000),
			curr:  state(track("a"), 1500),
			want:  false,
		},
		{
			name:  "same track, regression exactly at tolerance — not a reset",
			prior: state(track("a"), 2000),
			curr:  state(track("a"), 2000-progressRegressionTolerance),
			want:  false,
		},
		{
			name:  "same track, regression one ms past tolerance — a reset",
			prior: state(track("a"), 2000),
			curr:  state(track("a"), 2000-progressRegressionTolerance-1),
			want:  true,
		},
		{
			name:  "same track, large regression near zero — a reset",
			prior: state(track("a"), 2709),
			curr:  state(track("a"), 200),
			want:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sessionReset(tc.prior, tc.curr); got != tc.want {
				t.Errorf("sessionReset() = %v, want %v", got, tc.want)
			}
		})
	}
}
