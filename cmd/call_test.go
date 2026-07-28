package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
)

func TestCallCmdRunE_DefaultMethodGET(t *testing.T) {
	oldConfigPath := configPath
	oldCallMethod := callMethod
	defer func() {
		configPath = oldConfigPath
		callMethod = oldCallMethod
	}()
	callMethod = "GET"

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/me" {
			t.Errorf("expected /v1/me, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"me"}`))
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	out := captureOutput(t, func() {
		if err := callCmd.RunE(callCmd, []string{"/v1/me"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Status: 200") || !strings.Contains(out, `{"id":"me"}`) {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestCallCmdRunE_PUTWithBody(t *testing.T) {
	oldConfigPath := configPath
	oldCallMethod := callMethod
	defer func() {
		configPath = oldConfigPath
		callMethod = oldCallMethod
	}()
	callMethod = "PUT"

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/v1/me/player/play" || r.URL.Query().Get("device_id") != "dev1" {
			t.Errorf("unexpected URL: %s", r.URL.String())
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"context_uri":"spotify:playlist:abc"}` {
			t.Errorf("unexpected body: %s", body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	out := captureOutput(t, func() {
		err := callCmd.RunE(callCmd, []string{
			"/v1/me/player/play?device_id=dev1",
			`{"context_uri":"spotify:playlist:abc"}`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Status: 204") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestCallCmdRunE_NonSuccessStatus_ReturnsErrorButPrintsBody(t *testing.T) {
	oldConfigPath := configPath
	oldCallMethod := callMethod
	defer func() {
		configPath = oldConfigPath
		callMethod = oldCallMethod
	}()
	callMethod = "GET"

	configPath = writeTempConfig(t, &config.Config{ClientID: "id", ClientSecret: "secret", RefreshToken: "refresh"})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer srv.Close()
	cleanup := wireClient(t, srv)
	defer cleanup()

	var runErr error
	out := captureOutput(t, func() {
		runErr = callCmd.RunE(callCmd, []string{"/v1/whatever"})
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "400") {
		t.Fatalf("expected an error mentioning the 400 status, got %v", runErr)
	}
	if !strings.Contains(out, "Status: 400") || !strings.Contains(out, "bad request") {
		t.Errorf("expected status/body to still be printed, got %q", out)
	}
}

func TestCallCmdRunE_ClientError(t *testing.T) {
	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()
	configPath = "/nonexistent/config.yaml"

	if err := callCmd.RunE(callCmd, []string{"/v1/me"}); err == nil {
		t.Fatal("expected error for missing config")
	}
}
