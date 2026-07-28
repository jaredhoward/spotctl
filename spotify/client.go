package spotify

import (
	"bytes"
	"context"
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
	defaultAPIBase     = "https://api.spotify.com"
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
	apiBase     string
}

func NewClient(accessToken string) *Client {
	return &Client{
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: defaultHTTPTimeout},
		urlPlayer:   "https://api.spotify.com/v1/me/player",
		apiBase:     defaultAPIBase,
	}
}

func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

func (c *Client) SetPlayerURL(url string) {
	c.urlPlayer = url
}

// SetAPIBase overrides the base URL RawRequest resolves paths against
// (defaults to https://api.spotify.com). Exists for tests.
func (c *Client) SetAPIBase(base string) {
	c.apiBase = base
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

// httpTimestampFormat gives verbose HTTP trace lines millisecond-precision
// timestamps so elapsed time between requests (e.g. play to confirm) can be
// read directly off --verbose output instead of measured separately.
const httpTimestampFormat = "2006-01-02 15:04:05.000"

// logHTTP writes a debug line to stderr, prefixed with the current time.
// Callers only invoke this when Verbose is already true.
func logHTTP(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[%s] "+format+"\n", append([]any{time.Now().Format(httpTimestampFormat)}, args...)...)
}

// doRequest sends req via the client's HTTP client. When Verbose is enabled
// it logs the request method/URL/body and the response status/body to
// stderr; the Authorization header is never logged. The response body is
// fully read and replaced with a fresh reader so callers can still consume
// it normally.
func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	if Verbose {
		logHTTP("[http] -> %s %s", req.Method, req.URL.String())
		if req.GetBody != nil {
			if rc, err := req.GetBody(); err == nil {
				if body, err := io.ReadAll(rc); err == nil && len(body) > 0 {
					logHTTP("[http]    body: %s", body)
				}
			}
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if Verbose {
			logHTTP("[http] <- %s %s: error: %v", req.Method, req.URL.Path, err)
		}
		return resp, err
	}

	if Verbose {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
		logHTTP("[http] <- %d %s %s", resp.StatusCode, req.Method, req.URL.Path)
		if len(body) > 0 {
			logHTTP("[http]    body: %s", body)
		}
	}

	return resp, nil
}

// RawRequest issues an arbitrary Spotify Web API request, bypassing every
// action/confirm abstraction entirely. path is resolved against the
// client's API base (https://api.spotify.com by default; a leading slash is
// added if missing) — e.g. "/v1/me/player/play?device_id=xxx". body, if
// non-empty, is sent as-is with Content-Type: application/json. Returns the
// response status code and raw body regardless of status, so callers can
// inspect error responses too.
func (c *Client) RawRequest(ctx context.Context, method, path, body string) (int, []byte, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.apiBase+path, reqBody)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return resp.StatusCode, respBody, nil
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
