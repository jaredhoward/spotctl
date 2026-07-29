package spotify

import (
	"fmt"
	"net/http"
	"time"
)

// HTTPStatusError wraps a non-2xx response from doExpectSuccess so callers
// can distinguish retryable transient failures (502/503/429) from permanent
// ones (401/403/404/...) via errors.As, instead of parsing error strings.
type HTTPStatusError struct {
	Action     string
	StatusCode int
	Body       string
	// RetryAfter is the parsed Retry-After header value. Only set for 429
	// responses that included one; zero otherwise.
	RetryAfter time.Duration
}

func (e *HTTPStatusError) Error() string {
	if e.StatusCode == http.StatusTooManyRequests {
		if e.RetryAfter > 0 {
			return fmt.Sprintf("%s rate limited (429): retry after %s", e.Action, e.RetryAfter)
		}
		return fmt.Sprintf("%s rate limited (429)", e.Action)
	}
	return fmt.Sprintf("%s returned unexpected status %d: %s", e.Action, e.StatusCode, e.Body)
}

// Retryable reports whether this status is a transient failure worth
// redispatching: 429 (rate limited), 502 (bad gateway), or 503 (service
// unavailable).
func (e *HTTPStatusError) Retryable() bool {
	switch e.StatusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable:
		return true
	default:
		return false
	}
}
