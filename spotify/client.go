package spotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	accessToken string
	httpClient  *http.Client
}

func NewClient(accessToken string) *Client {
	return &Client{
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

// playerURL builds a player endpoint URL, appending device_id only when non-empty.
func playerURL(base, path, deviceID string) string {
	u := base
	if path != "" {
		u = base + path
	}
	if deviceID == "" {
		return u
	}
	return u + "?" + url.Values{"device_id": {deviceID}}.Encode()
}

func (c *Client) Play(deviceID, contextURI string) error {
	var reqBody io.Reader
	if contextURI != "" {
		body, err := json.Marshal(map[string]string{"context_uri": contextURI})
		if err != nil {
			return fmt.Errorf("could not marshal play request: %w", err)
		}
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(http.MethodPut, playerURL(URLPlayer, "/play", deviceID), reqBody)
	if err != nil {
		return fmt.Errorf("could not create play request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if contextURI != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.doExpectSuccess(req, "play")
}

func (c *Client) TransferPlayback(deviceIDs []string, play bool) error {
	body, err := json.Marshal(map[string]interface{}{
		"device_ids": deviceIDs,
		"play":       play,
	})
	if err != nil {
		return fmt.Errorf("could not marshal transfer request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, URLPlayer, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("could not create transfer request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	return c.doExpectSuccess(req, "transfer playback")
}

// Shuffle enables shuffle on a device. Kept for backward compatibility.
func (c *Client) Shuffle(deviceID string) error {
	return c.SetShuffle(deviceID, true)
}

// SetShuffle enables or disables shuffle. device_id is optional.
func (c *Client) SetShuffle(deviceID string, enabled bool) error {
	params := url.Values{"state": {fmt.Sprintf("%t", enabled)}}
	if deviceID != "" {
		params.Set("device_id", deviceID)
	}
	req, err := http.NewRequest(http.MethodPut, URLPlayer+"/shuffle?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("could not create shuffle request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "shuffle")
}

func (c *Client) Pause(deviceID string) error {
	req, err := http.NewRequest(http.MethodPut, playerURL(URLPlayer, "/pause", deviceID), nil)
	if err != nil {
		return fmt.Errorf("could not create pause request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "pause")
}

func (c *Client) Next(deviceID string) error {
	req, err := http.NewRequest(http.MethodPost, playerURL(URLPlayer, "/next", deviceID), nil)
	if err != nil {
		return fmt.Errorf("could not create next request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "next")
}

func (c *Client) Previous(deviceID string) error {
	req, err := http.NewRequest(http.MethodPost, playerURL(URLPlayer, "/previous", deviceID), nil)
	if err != nil {
		return fmt.Errorf("could not create previous request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "previous")
}

func (c *Client) SetVolume(deviceID string, volumePercent int) error {
	params := url.Values{"volume_percent": {fmt.Sprintf("%d", volumePercent)}}
	if deviceID != "" {
		params.Set("device_id", deviceID)
	}
	req, err := http.NewRequest(http.MethodPut, URLPlayer+"/volume?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("could not create volume request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	return c.doExpectSuccess(req, "set volume")
}

// doExpectSuccess executes req and returns nil on 2xx, or a descriptive error.
func (c *Client) doExpectSuccess(req *http.Request, action string) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s returned unexpected status %d: %s", action, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}
