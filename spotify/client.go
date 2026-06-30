package spotify

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout = 10 * time.Second
)

type Client struct {
	accessToken string
	httpClient  *http.Client
	urlPlayer   string
}

func NewClient(accessToken string) *Client {
	return &Client{
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: defaultHTTPTimeout},
		urlPlayer:   "https://api.spotify.com/v1/me/player",
	}
}

func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

func (c *Client) SetPlayerURL(url string) {
	c.urlPlayer = url
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

// doExpectSuccess executes req and returns nil on 2xx, or a descriptive error.
// A 429 response includes the Retry-After value in the error message.
func (c *Client) doExpectSuccess(req *http.Request, action string) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				return fmt.Errorf("%s rate limited (429): retry after %s seconds", action, retryAfter)
			}
			return fmt.Errorf("%s rate limited (429)", action)
		}
		return fmt.Errorf("%s returned unexpected status %d: %s", action, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}
