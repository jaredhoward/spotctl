package config

import (
    "os"
    "path/filepath"
    "testing"
)

func TestSaveLoadRoundtrip(t *testing.T) {
    tmpDir := t.TempDir()
    path := filepath.Join(tmpDir, "config.yaml")

    original := &Config{
        ClientID:     "test-client",
        ClientSecret: "test-secret",
        RefreshToken: "refresh-token",
        RedirectURI:  "https://example.com/callback",
        Presets: map[string]Preset{
            "sleep": {DeviceID: "device-1", ContextURI: "spotify:playlist:abc", Shuffle: true},
        },
        DeviceNames: map[string]string{"device-1": "Living Room"},
    }

    if err := Save(path, original); err != nil {
        t.Fatal(err)
    }

    loaded, err := Load(path)
    if err != nil {
        t.Fatal(err)
    }

    if loaded.ClientID != original.ClientID {
        t.Fatalf("expected client id %q, got %q", original.ClientID, loaded.ClientID)
    }
    if loaded.ClientSecret != original.ClientSecret {
        t.Fatalf("expected client secret %q, got %q", original.ClientSecret, loaded.ClientSecret)
    }
    if loaded.RefreshToken != original.RefreshToken {
        t.Fatalf("expected refresh token %q, got %q", original.RefreshToken, loaded.RefreshToken)
    }
    if loaded.RedirectURI != original.RedirectURI {
        t.Fatalf("expected redirect uri %q, got %q", original.RedirectURI, loaded.RedirectURI)
    }
    if want, got := original.Presets["sleep"].DeviceID, loaded.Presets["sleep"].DeviceID; want != got {
        t.Fatalf("expected preset device id %q, got %q", want, got)
    }
    if want, got := original.DeviceNames["device-1"], loaded.DeviceNames["device-1"]; want != got {
        t.Fatalf("expected device name %q, got %q", want, got)
    }
}

func TestLoadMissingFields(t *testing.T) {
    tmpDir := t.TempDir()
    path := filepath.Join(tmpDir, "config.yaml")
    if err := os.WriteFile(path, []byte("client_id: test-client\n"), 0600); err != nil {
        t.Fatal(err)
    }

    if _, err := Load(path); err == nil {
        t.Fatal("expected validation error for missing fields")
    }
}

func TestClientB64(t *testing.T) {
    cfg := &Config{
        ClientID:     "id",
        ClientSecret: "secret",
    }
    encoder := cfg.ClientB64()
    if encoder == "" {
        t.Fatal("expected non-empty base64 client string")
    }
}
