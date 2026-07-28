package sets_test

import (
	"os"
	"testing"
	"time"

	"github.com/jaredhoward/spotctl/spotify"
)

// TestMain shrinks spotify.PlayWakeSettleDelay for the whole test binary so
// tests dispatching a device-targeted Play don't each pay the real-world
// default.
func TestMain(m *testing.M) {
	spotify.PlayWakeSettleDelay = time.Millisecond
	os.Exit(m.Run())
}
