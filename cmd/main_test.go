package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/jaredhoward/spotctl/spotify"
)

// TestMain shrinks spotify.PlayWakeSettleDelay for the whole test binary so
// tests dispatching a device-targeted play don't each pay the real-world
// default. The stabilize window itself comes from each test's own
// config.Config (see the ConfirmStabilizeWindow field set on relevant
// configs in commands_test.go etc.) rather than a package var.
func TestMain(m *testing.M) {
	spotify.PlayWakeSettleDelay = time.Millisecond
	os.Exit(m.Run())
}
