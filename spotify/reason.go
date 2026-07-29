package spotify

import "context"

// reasonKey is the context key WithReason/reasonFromContext use to carry a
// plain-language explanation of why an HTTP call is being made. Several
// internal call sites (wake diagnostics, confirm polling, stabilize checks)
// all hit the same endpoints (e.g. GetCurrentPlayback) for different
// reasons, so the "what" alone (see actionLabels) doesn't tell --verbose
// readers why a given call happened.
type reasonKey struct{}

// WithReason attaches reason to ctx so the next HTTP call made with the
// returned context logs it as a verbose label prefix, e.g.
// "[Polling for Confirmation: Get Playback State]". Unset (or an empty
// string) falls back to just the operation label.
func WithReason(ctx context.Context, reason string) context.Context {
	return context.WithValue(ctx, reasonKey{}, reason)
}

func reasonFromContext(ctx context.Context) string {
	reason, _ := ctx.Value(reasonKey{}).(string)
	return reason
}
