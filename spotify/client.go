package spotify

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout = 10 * time.Second
)

// Verbose enables raw HTTP request/response tracing to stderr, set from the
// CLI --verbose flag. It is a runtime debugging aid, not persisted
// configuration, so it lives here as a package var rather than on Config.
//
// Deliberately not wired into the OAuth token exchange (auth.go uses its own
// http.Client, bypassing Client.doRequest) — that request/response carries
// the refresh and access tokens, which must never be printed.
var Verbose bool

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

// playerURL builds a player endpoint URL. device_id is appended when non-empty;
// extra holds any additional query params (e.g. state, volume_percent).
func playerURL(base, path, deviceID string, extra ...url.Values) string {
	u := base
	if path != "" {
		u = base + path
	}
	params := url.Values{}
	if deviceID != "" {
		params.Set("device_id", deviceID)
	}
	if len(extra) > 0 {
		for k, vs := range extra[0] {
			params[k] = vs
		}
	}
	if len(params) == 0 {
		return u
	}
	return u + "?" + params.Encode()
}

// doRequest sends req via the client's HTTP client. When Verbose is enabled
// it logs the request method/URL/body and the response status/body to
// stderr; the Authorization header is never logged. The response body is
// fully read and replaced with a fresh reader so callers can still consume
// it normally.
func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	if Verbose {
		fmt.Fprintf(os.Stderr, "[http] -> %s %s\n", req.Method, req.URL.String())
		if req.GetBody != nil {
			if rc, err := req.GetBody(); err == nil {
				if body, err := io.ReadAll(rc); err == nil && len(body) > 0 {
					fmt.Fprintf(os.Stderr, "[http]    body: %s\n", body)
				}
			}
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if Verbose {
			fmt.Fprintf(os.Stderr, "[http] <- %s %s: error: %v\n", req.Method, req.URL.Path, err)
		}
		return resp, err
	}

	if Verbose {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
		fmt.Fprintf(os.Stderr, "[http] <- %d %s %s\n", resp.StatusCode, req.Method, req.URL.Path)
		if len(body) > 0 {
			fmt.Fprintf(os.Stderr, "[http]    body: %s\n", body)
		}
	}

	return resp, nil
}

// doExpectSuccess executes req and returns nil on 2xx, or a descriptive error.
// A 429 response includes the Retry-After value in the error message.
func (c *Client) doExpectSuccess(req *http.Request, action string) error {
	resp, err := c.doRequest(req)
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
