package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Action is the interface satisfied by every Spotify command and orchestration
// primitive. Dispatch performs the operation; Confirmed reports whether the
// given playback state reflects the completed action; Label returns a short
// human-readable description for logging.
type Action interface {
	Dispatch(ctx context.Context, c *Client) error
	Confirmed(state *PlaybackState) bool
	Label() string
}

// Play starts or resumes playback, optionally on a specific device and/or
// with a specific context URI.
//
// Confirmation signal strength:
//   - ContextURI set: strong — verifies context URI matches.
//   - DeviceID set, no ContextURI: strong — verifies active device and IsPlaying.
//   - Neither set: weak — priorState is captured at Dispatch time to detect a
//     meaningful change (IsPlaying flipped, or active device changed). If the
//     snapshot fails or nothing was active, priorState is nil and Confirmed
//     returns false, causing polling to continue until timeout.
type Play struct {
	DeviceID   string
	ContextURI string
	priorState *PlaybackState // captured at Dispatch when no ContextURI is set
}

func (p *Play) Dispatch(ctx context.Context, c *Client) error {
	// Snapshot current state before dispatching when we have no ContextURI to
	// verify against. Used by Confirmed to detect a meaningful state change.
	if p.ContextURI == "" {
		if state, err := c.GetCurrentPlayback(ctx); err == nil {
			p.priorState = state
		}
	}

	var reqBody io.Reader
	if p.ContextURI != "" {
		body, err := json.Marshal(map[string]string{"context_uri": p.ContextURI})
		if err != nil {
			return fmt.Errorf("failed to marshal play request: %w", err)
		}
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, playerURL(c.urlPlayer, "/play", p.DeviceID), reqBody)
	if err != nil {
		return fmt.Errorf("failed to create play request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if p.ContextURI != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.doExpectSuccess(req, "play")
}

func (p *Play) Confirmed(state *PlaybackState) bool {
	if state == nil || !state.IsPlaying {
		return false
	}
	// Strong signal: verify the context URI landed.
	if p.ContextURI != "" {
		return state.Context != nil && state.Context.URI == p.ContextURI
	}
	// Strong signal: verify playback is active on the intended device.
	if p.DeviceID != "" {
		return state.Device.ID == p.DeviceID
	}
	// Weak signal: no constraints to verify against. Use priorState to detect
	// a meaningful change if available — either IsPlaying flipped from false,
	// or the active device changed. If priorState is nil (snapshot failed or
	// nothing was active), treat as unconfirmed — same conservative approach
	// as Next/Previous — and let polling continue until timeout.
	if p.priorState == nil {
		return false
	}
	wasPlaying := p.priorState.IsPlaying
	priorDevice := p.priorState.Device.ID
	return (!wasPlaying && state.IsPlaying) || (priorDevice != state.Device.ID)
}

func (p *Play) Label() string {
	if p.ContextURI != "" {
		return fmt.Sprintf("play uri=%s device=%s", p.ContextURI, p.DeviceID)
	}
	return fmt.Sprintf("play device=%s", p.DeviceID)
}

// Pause pauses playback.
type Pause struct {
	DeviceID string
}

func (p *Pause) Dispatch(ctx context.Context, c *Client) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, playerURL(c.urlPlayer, "/pause", p.DeviceID), nil)
	if err != nil {
		return fmt.Errorf("failed to create pause request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "pause")
}

func (p *Pause) Confirmed(state *PlaybackState) bool { return state != nil && !state.IsPlaying }
func (p *Pause) Label() string                       { return fmt.Sprintf("pause device=%s", p.DeviceID) }

// trackChanged returns true when the current track URI differs from the prior
// track URI. Returns false when either snapshot is missing or has no item —
// the caller should treat the result as unconfirmed and keep polling.
func trackChanged(prior, current *PlaybackState) bool {
	if current == nil || current.Item == nil {
		return false
	}
	if prior == nil || prior.Item == nil {
		return false
	}
	return current.Item.URI != prior.Item.URI
}

// Next skips to the next track.
//
// Confirmation signal: strong when priorState is captured at Dispatch time —
// verifies the track URI changed. If the snapshot fails or no track was active,
// priorState is nil and Confirmed returns false, causing polling to continue
// until timeout. This is intentional: an unverifiable confirmation is treated
// as unconfirmed rather than assumed successful.
type Next struct {
	DeviceID   string
	priorState *PlaybackState // captured at Dispatch time for track-change detection
}

func (n *Next) Dispatch(ctx context.Context, c *Client) error {
	if state, err := c.GetCurrentPlayback(ctx); err == nil {
		n.priorState = state
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, playerURL(c.urlPlayer, "/next", n.DeviceID), nil)
	if err != nil {
		return fmt.Errorf("failed to create next request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "next")
}

func (n *Next) Confirmed(state *PlaybackState) bool { return trackChanged(n.priorState, state) }

func (n *Next) Label() string { return fmt.Sprintf("next device=%s", n.DeviceID) }

// Previous returns to the previous track.
//
// Confirmation signal: same approach and caveats as Next.
type Previous struct {
	DeviceID   string
	priorState *PlaybackState // captured at Dispatch time for track-change detection
}

func (p *Previous) Dispatch(ctx context.Context, c *Client) error {
	if state, err := c.GetCurrentPlayback(ctx); err == nil {
		p.priorState = state
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, playerURL(c.urlPlayer, "/previous", p.DeviceID), nil)
	if err != nil {
		return fmt.Errorf("failed to create previous request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "previous")
}

func (p *Previous) Confirmed(state *PlaybackState) bool { return trackChanged(p.priorState, state) }

func (p *Previous) Label() string { return fmt.Sprintf("previous device=%s", p.DeviceID) }

// Shuffle enables or disables shuffle.
type Shuffle struct {
	DeviceID string
	Enabled  bool
}

func (s *Shuffle) Dispatch(ctx context.Context, c *Client) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		playerURL(c.urlPlayer, "/shuffle", s.DeviceID, url.Values{"state": {fmt.Sprintf("%t", s.Enabled)}}), nil)
	if err != nil {
		return fmt.Errorf("failed to create shuffle request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "shuffle")
}

func (s *Shuffle) Confirmed(state *PlaybackState) bool {
	return state != nil && state.ShuffleState == s.Enabled
}

func (s *Shuffle) Label() string {
	return fmt.Sprintf("shuffle enabled=%v device=%s", s.Enabled, s.DeviceID)
}

// Repeat sets the repeat mode.
type Repeat struct {
	DeviceID string
	State    string // "off" | "track" | "context"
}

func (r *Repeat) Dispatch(ctx context.Context, c *Client) error {
	switch r.State {
	case "off", "track", "context":
	default:
		return fmt.Errorf("invalid repeat state %q: must be off, track, or context", r.State)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		playerURL(c.urlPlayer, "/repeat", r.DeviceID, url.Values{"state": {r.State}}), nil)
	if err != nil {
		return fmt.Errorf("failed to create repeat request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "repeat")
}

func (r *Repeat) Confirmed(state *PlaybackState) bool {
	return state != nil && state.RepeatState == r.State
}

func (r *Repeat) Label() string {
	return fmt.Sprintf("repeat state=%s device=%s", r.State, r.DeviceID)
}

// Volume sets the playback volume.
type Volume struct {
	DeviceID string
	Level    int
}

func (v *Volume) Dispatch(ctx context.Context, c *Client) error {
	if v.Level < 0 || v.Level > 100 {
		return fmt.Errorf("invalid volume level %d: must be 0–100", v.Level)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		playerURL(c.urlPlayer, "/volume", v.DeviceID, url.Values{"volume_percent": {fmt.Sprintf("%d", v.Level)}}), nil)
	if err != nil {
		return fmt.Errorf("failed to create volume request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "set volume")
}

func (v *Volume) Confirmed(state *PlaybackState) bool {
	if state == nil {
		return false
	}
	// Spotify can report volume ±1% from the requested value due to device
	// rounding, so we treat a difference of 1 as confirmed.
	diff := state.Device.VolumePercent - v.Level
	if diff < 0 {
		diff = -diff
	}
	return diff <= 1
}

func (v *Volume) Label() string {
	return fmt.Sprintf("volume level=%d device=%s", v.Level, v.DeviceID)
}

// Transfer transfers playback to another device.
type Transfer struct {
	DeviceID string
	Play     bool
}

func (t *Transfer) Dispatch(ctx context.Context, c *Client) error {
	body, err := json.Marshal(map[string]interface{}{
		"device_ids": []string{t.DeviceID},
		"play":       t.Play,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal transfer request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.urlPlayer, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create transfer request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	return c.doExpectSuccess(req, "transfer playback")
}

func (t *Transfer) Confirmed(state *PlaybackState) bool {
	if state == nil || state.Device.ID != t.DeviceID {
		return false
	}
	if t.Play && !state.IsPlaying {
		return false
	}
	return true
}

func (t *Transfer) Label() string {
	return fmt.Sprintf("transfer device=%s play=%v", t.DeviceID, t.Play)
}
