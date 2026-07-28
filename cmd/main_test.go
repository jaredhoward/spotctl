package cmd

import (
	"os"
	"testing"
	"time"
)

// TestMain shrinks confirmStabilizeWindow for the whole test binary so tests
// exercising a successful confirm don't each pay the real-world default.
func TestMain(m *testing.M) {
	confirmStabilizeWindow = 5 * time.Millisecond
	os.Exit(m.Run())
}
