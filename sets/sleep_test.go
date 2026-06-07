package sets

import (
	"testing"
	"time"
)

func TestSleep_LabelInternal(t *testing.T) {
	s := &Sleep{Duration: 30 * time.Millisecond}
	if got := s.Label(); got != "sleep duration=30ms" {
		t.Errorf("unexpected label: %q", got)
	}
}
