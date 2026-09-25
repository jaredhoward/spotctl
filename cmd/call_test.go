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

	stdout, stderr := captureBoth(t, func() {
		if err := callCmd.RunE(callCmd, []string{"/v1/me"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	// stdout is only the body, so it pipes cleanly; the status goes to stderr.
	if strings.TrimSpace(stdout) != `{"id":"me"}` {
		t.Errorf("stdout should be only the response body, got %q", stdout)
	}
	if !strings.Contains(stderr, "Status: 200") {
		t.Errorf("expected the status on stderr, got %q", stderr)
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

	stdout, stderr := captureBoth(t, func() {
		err := callCmd.RunE(callCmd, []string{
			"/v1/me/player/play?device_id=dev1",
			`{"context_uri":"spotify:playlist:abc"}`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(stderr, "Status: 204") {
		t.Errorf("expected the status on stderr, got %q", stderr)
	}
	if stdout != "" {
		t.Errorf("an empty response body should print nothing on stdout, got %q", stdout)
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
	stdout, stderr := captureBoth(t, func() {
		runErr = callCmd.RunE(callCmd, []string{"/v1/whatever"})
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "400") {
		t.Fatalf("expected an error mentioning the 400 status, got %v", runErr)
	}
	if !strings.Contains(stderr, "Status: 400") || !strings.Contains(stdout, "bad request") {
		t.Errorf("expected the status on stderr and the body on stdout, got stderr=%q stdout=%q", stderr, stdout)
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
