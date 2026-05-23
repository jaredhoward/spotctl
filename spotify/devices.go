package spotify

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Device struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	IsActive      bool   `json:"is_active"`
	VolumePercent int    `json:"volume_percent"`
}

type DevicesResponse struct {
	Devices []Device `json:"devices"`
}

func (c *Client) GetDevices() ([]Device, error) {
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/devices", URLPlayer),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create devices request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("devices request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("devices request returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var devicesResp DevicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&devicesResp); err != nil {
		return nil, fmt.Errorf("could not decode devices response: %w", err)
	}

	return devicesResp.Devices, nil
}
