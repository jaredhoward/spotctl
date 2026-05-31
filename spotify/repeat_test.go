package spotify

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRepeat(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	cases := []struct {
		name     string
		state    string
		deviceID string
	}{
		{"off with device", "off", "dev1"},
		{"track with device", "track", "dev1"},
		{"context no device", "context", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotState, gotDevice string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut || r.URL.Path != "/repeat" {
					t.Fatalf("expected PUT /repeat, got %s %s", r.Method, r.URL.Path)
				}
				gotState = r.URL.Query().Get("state")
				gotDevice = r.URL.Query().Get("device_id")
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			URLPlayer = server.URL
			client := &Client{accessToken: "t", httpClient: server.Client()}
			if err := client.Repeat(tc.deviceID, tc.state); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotState != tc.state {
				t.Errorf("state: got %q, want %q", gotState, tc.state)
			}
			if gotDevice != tc.deviceID {
				t.Errorf("device_id: got %q, want %q", gotDevice, tc.deviceID)
			}
		})
	}
}
