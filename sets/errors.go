package sets

import (
	"fmt"
	"time"
)

// TimeoutError is returned when confirmation polling exceeds the deadline.
type TimeoutError struct {
	Timeout     time.Duration
	ActionLabel string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("confirmation timed out after %s waiting for %q to reflect in playback state",
		e.Timeout, e.ActionLabel)
}

// DepthExceededError is returned when run_set nesting exceeds MaxSetDepth.
type DepthExceededError struct {
	Max int
}

func (e *DepthExceededError) Error() string {
	return fmt.Sprintf("set recursion depth exceeded (max %d)", e.Max)
}
