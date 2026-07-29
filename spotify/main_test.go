package spotify

import (
	"os"
	"testing"
	"time"
)

// TestMain shrinks PlayWakeSettleDelay for the whole test binary so tests
// dispatching a device-targeted Play don't each pay the real-world default.
func TestMain(m *testing.M) {
	PlayWakeSettleDelay = time.Millisecond
	os.Exit(m.Run())
}
