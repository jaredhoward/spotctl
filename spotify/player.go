package spotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var APIBase = "https://api.spotify.com/v1/me/player"

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

func (c *Client) Play(deviceID, playlistURI string) error {
	var reqBody io.Reader
	if playlistURI != "" {
		body, err := json.Marshal(map[string]string{"context_uri": playlistURI})
		if err != nil {
			return fmt.Errorf("could not marshal play request: %w", err)
		}
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/play?device_id=%s", APIBase, deviceID),
		reqBody,
	)
	if err != nil {
		return fmt.Errorf("could not create play request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if playlistURI != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.doExpect204(req, "play")
}

func (c *Client) TransferPlayback(deviceIDs []string, play bool) error {
	body, err := json.Marshal(map[string]interface{}{
		"device_ids": deviceIDs,
		"play":       play,
	})
	if err != nil {
		return fmt.Errorf("could not marshal transfer request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut,
		APIBase,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("could not create transfer request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	return c.doExpect204(req, "transfer playback")
}

func (c *Client) Shuffle(deviceID string) error {
	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/shuffle?state=true&device_id=%s", APIBase, deviceID),
		nil,
	)
	if err != nil {
		return fmt.Errorf("could not create shuffle request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("shuffle request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("shuffle returned unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

func (c *Client) Pause(deviceID string) error {
	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/pause?device_id=%s", APIBase, deviceID),
		nil,
	)
	if err != nil {
		return fmt.Errorf("could not create pause request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	return c.doExpect204(req, "pause")
}

func (c *Client) Next(deviceID string) error {
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/next?device_id=%s", APIBase, deviceID),
		nil,
	)
	if err != nil {
		return fmt.Errorf("could not create next request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	return c.doExpect204(req, "next")
}

func (c *Client) Previous(deviceID string) error {
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/previous?device_id=%s", APIBase, deviceID),
		nil,
	)
	if err != nil {
		return fmt.Errorf("could not create previous request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	return c.doExpect204(req, "previous")
}

func (c *Client) SetVolume(deviceID string, volumePercent int) error {
	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/volume?volume_percent=%d&device_id=%s", APIBase, volumePercent, deviceID),
		nil,
	)
	if err != nil {
		return fmt.Errorf("could not create volume request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	return c.doExpect204(req, "set volume")
}

func (c *Client) doExpect204(req *http.Request, action string) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s returned unexpected status %d: %s", action, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}
