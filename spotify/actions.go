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
//     snapshot fails or nothing was active, falls back to bare IsPlaying check,
//     which cannot distinguish a no-op from a successful play.
type Play struct {
	DeviceID   string
	ContextURI string
	priorState *PlaybackState // captured at Dispatch when no ContextURI is set
}

func (p *Play) Dispatch(ctx context.Context, c *Client) error {
	// Snapshot current state before dispatching when we have no ContextURI to
	// verify against. Used by Confirmed to detect a meaningful state change.
	if p.ContextURI == "" {
		if state, err := c.GetCurrentPlayback(); err == nil {
			p.priorState = state
		}
	}

	var reqBody io.Reader
	if p.ContextURI != "" {
		body, err := json.Marshal(map[string]string{"context_uri": p.ContextURI})
		if err != nil {
			return fmt.Errorf("could not marshal play request: %w", err)
		}
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, playerURL(URLPlayer, "/play", p.DeviceID), reqBody)
	if err != nil {
		return fmt.Errorf("could not create play request: %w", err)
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
	// nothing was active), fall back to bare IsPlaying, which cannot distinguish
	// a no-op from a successful play.
	if p.priorState != nil {
		wasPlaying := p.priorState.IsPlaying
		priorDevice := p.priorState.Device.ID
		return (!wasPlaying && state.IsPlaying) || (priorDevice != state.Device.ID)
	}
	return true
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, playerURL(URLPlayer, "/pause", p.DeviceID), nil)
	if err != nil {
		return fmt.Errorf("could not create pause request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "pause")
}

func (p *Pause) Confirmed(state *PlaybackState) bool { return state != nil && !state.IsPlaying }
func (p *Pause) Label() string                       { return fmt.Sprintf("pause device=%s", p.DeviceID) }

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
	if state, err := c.GetCurrentPlayback(); err == nil {
		n.priorState = state
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, playerURL(URLPlayer, "/next", n.DeviceID), nil)
	if err != nil {
		return fmt.Errorf("could not create next request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "next")
}

func (n *Next) Confirmed(state *PlaybackState) bool {
	if state == nil || state.Item == nil {
		return false
	}
	if n.priorState == nil || n.priorState.Item == nil {
		// Snapshot unavailable; cannot verify track changed. Polling will
		// continue until timeout — see type-level doc comment.
		return false
	}
	return state.Item.URI != n.priorState.Item.URI
}

func (n *Next) Label() string { return fmt.Sprintf("next device=%s", n.DeviceID) }

// Previous returns to the previous track.
//
// Confirmation signal: same approach and caveats as Next.
type Previous struct {
	DeviceID   string
	priorState *PlaybackState // captured at Dispatch time for track-change detection
}

func (p *Previous) Dispatch(ctx context.Context, c *Client) error {
	if state, err := c.GetCurrentPlayback(); err == nil {
		p.priorState = state
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, playerURL(URLPlayer, "/previous", p.DeviceID), nil)
	if err != nil {
		return fmt.Errorf("could not create previous request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "previous")
}

func (p *Previous) Confirmed(state *PlaybackState) bool {
	if state == nil || state.Item == nil {
		return false
	}
	if p.priorState == nil || p.priorState.Item == nil {
		// Snapshot unavailable; cannot verify track changed. Polling will
		// continue until timeout — see type-level doc comment.
		return false
	}
	return state.Item.URI != p.priorState.Item.URI
}

func (p *Previous) Label() string { return fmt.Sprintf("previous device=%s", p.DeviceID) }

// Shuffle enables or disables shuffle.
type Shuffle struct {
	DeviceID string
	Enabled  bool
}

func (s *Shuffle) Dispatch(ctx context.Context, c *Client) error {
	params := url.Values{"state": {fmt.Sprintf("%t", s.Enabled)}}
	if s.DeviceID != "" {
		params.Set("device_id", s.DeviceID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, URLPlayer+"/shuffle?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("could not create shuffle request: %w", err)
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
	params := url.Values{"state": {r.State}}
	if r.DeviceID != "" {
		params.Set("device_id", r.DeviceID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, URLPlayer+"/repeat?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("could not create repeat request: %w", err)
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
	params := url.Values{"volume_percent": {fmt.Sprintf("%d", v.Level)}}
	if v.DeviceID != "" {
		params.Set("device_id", v.DeviceID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, URLPlayer+"/volume?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("could not create volume request: %w", err)
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
		return fmt.Errorf("could not marshal transfer request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, URLPlayer, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("could not create transfer request: %w", err)
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
