package spotify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlayWithAndWithoutContext(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}

		switch called {
		case 1:
			if r.URL.Path != "/play" {
				t.Fatalf("expected /play path, got %s", r.URL.Path)
			}
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload["context_uri"] != "spotify:track:abc" {
				t.Fatalf("expected context_uri set, got %v", payload)
			}
			w.WriteHeader(http.StatusNoContent)
		case 2:
			if r.URL.Path != "/play" {
				t.Fatalf("expected /play path, got %s", r.URL.Path)
			}
			body := make([]byte, 1)
			n, _ := r.Body.Read(body)
			if n != 0 {
				t.Fatalf("expected empty body, got %d bytes", n)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatal("unexpected call")
		}
	}))
	defer server.Close()

	APIBase = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.Play("device", "spotify:track:abc"); err != nil {
		t.Fatal(err)
	}
	if err := client.Play("device", ""); err != nil {
		t.Fatal(err)
	}
}

func TestTransferPlayback(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/" {
			t.Fatalf("expected PUT /, got %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["play"] != true {
			t.Fatalf("expected play true, got %#v", payload)
		}
		deviceIDs, ok := payload["device_ids"].([]interface{})
		if !ok || len(deviceIDs) != 1 || deviceIDs[0] != "device" {
			t.Fatalf("unexpected device_ids payload: %v", payload["device_ids"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	APIBase = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	if err := client.TransferPlayback([]string{"device"}, true); err != nil {
		t.Fatal(err)
	}
}

func TestVolumePauseNextPreviousShuffle(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	cases := []struct {
		name    string
		path    string
		method  string
		query   string
		handler func(*http.Request) bool
		invoke  func(*Client) error
	}{
		{
			name:   "set volume",
			path:   "/volume",
			method: http.MethodPut,
			handler: func(r *http.Request) bool {
				return r.URL.Query().Get("volume_percent") == "42" && r.URL.Query().Get("device_id") == "device"
			},
			invoke: func(c *Client) error { return c.SetVolume("device", 42) },
		},
		{
			name:    "pause",
			path:    "/pause",
			method:  http.MethodPut,
			handler: func(r *http.Request) bool { return r.URL.Query().Get("device_id") == "device" },
			invoke:  func(c *Client) error { return c.Pause("device") },
		},
		{
			name:    "next",
			path:    "/next",
			method:  http.MethodPost,
			handler: func(r *http.Request) bool { return r.URL.Query().Get("device_id") == "device" },
			invoke:  func(c *Client) error { return c.Next("device") },
		},
		{
			name:    "previous",
			path:    "/previous",
			method:  http.MethodPost,
			handler: func(r *http.Request) bool { return r.URL.Query().Get("device_id") == "device" },
			invoke:  func(c *Client) error { return c.Previous("device") },
		},
		{
			name:   "shuffle",
			path:   "/shuffle",
			method: http.MethodPut,
			handler: func(r *http.Request) bool {
				return r.URL.Query().Get("state") == "true" && r.URL.Query().Get("device_id") == "device"
			},
			invoke: func(c *Client) error { return c.Shuffle("device") },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Fatalf("expected %s %s, got %s %s", tc.method, tc.path, r.Method, r.URL.Path)
				}
				if !tc.handler(r) {
					t.Fatalf("unexpected request: %v", r.URL)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			APIBase = server.URL
			client := &Client{accessToken: "t", httpClient: server.Client()}
			if err := tc.invoke(client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDoExpect204Error(t *testing.T) {
	oldAPIBase := APIBase
	t.Cleanup(func() { APIBase = oldAPIBase })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	APIBase = server.URL
	client := &Client{accessToken: "t", httpClient: server.Client()}
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/play?device_id=device", APIBase), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer t")
	if err := client.doExpect204(req, "play"); err == nil {
		t.Fatal("expected error for non-204 response")
	}
}
