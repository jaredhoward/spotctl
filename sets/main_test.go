package sets_test

import (
	"os"
	"testing"
	"time"

	"github.com/jaredhoward/spotctl/sets"
	"github.com/jaredhoward/spotctl/spotify"
)

// TestMain shrinks spotify.PlayWakeSettleDelay and sets.DispatchRetryBackoff
// for the whole test binary so tests dispatching a device-targeted Play or
// exercising the transient-error retry path don't each pay the real-world
// defaults.
func TestMain(m *testing.M) {
	spotify.PlayWakeSettleDelay = time.Millisecond
	sets.DispatchRetryBackoff = time.Millisecond
	os.Exit(m.Run())
}
