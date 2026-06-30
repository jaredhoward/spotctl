package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetDevicesSuccess(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/devices" {
			t.Fatalf("expected GET /devices, got %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DevicesResponse{Devices: []Device{{ID: "device-1", Name: "Living Room", Type: "Speaker", IsActive: false, VolumePercent: 35}}})
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "token", httpClient: server.Client()}
	devices, err := client.GetDevices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].ID != "device-1" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
}

func TestGetDevicesError(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "token", httpClient: server.Client()}
	if _, err := client.GetDevices(context.Background()); err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestGetDevicesDecodeError(t *testing.T) {
	oldURLPlayer := URLPlayer
	t.Cleanup(func() { URLPlayer = oldURLPlayer })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	URLPlayer = server.URL
	client := &Client{accessToken: "token", httpClient: server.Client()}
	if _, err := client.GetDevices(context.Background()); err == nil {
		t.Fatal("expected decode error")
	}
}
