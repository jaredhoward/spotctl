package cmd

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jaredhoward/spotctl/config"
	"github.com/jaredhoward/spotctl/spotify"
)

func TestJoinArtists(t *testing.T) {
	artists := []spotify.Artist{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	if got := joinArtists(artists); got != "A, B, C" {
		t.Fatalf("expected joined artists, got %q", got)
	}
}

func TestDeviceActivity(t *testing.T) {
	if got := deviceActivity(true); got != "(active)" {
		t.Fatalf("expected active marker, got %q", got)
	}
	if got := deviceActivity(false); got != "" {
		t.Fatalf("expected empty marker, got %q", got)
	}
}

func TestFormatDurationMS(t *testing.T) {
	if got := formatDurationMS(123456); got != "02:03" {
		t.Fatalf("expected duration 2:03, got %q", got)
	}
}

func TestExistingValue(t *testing.T) {
	if got := existingValue(&config.Config{ClientID: "new"}, func(c *config.Config) string { return c.ClientID }); got != "new" {
		t.Fatalf("expected new value, got %q", got)
	}
	if got := existingValue(nil, func(c *config.Config) string { return c.ClientID }); got != "" {
		t.Fatalf("expected old value, got %q", got)
	}
}

func TestPrompt(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("hello\n"))
	if got := prompt(reader, "prompt"); got != "hello" {
		t.Fatalf("expected prompt response hello, got %q", got)
	}
}

func TestPromptWithDefault(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\n"))
	if got := promptWithDefault(reader, "prompt", "default"); got != "default" {
		t.Fatalf("expected default value, got %q", got)
	}

	reader = bufio.NewReader(strings.NewReader("custom\n"))
	if got := promptWithDefault(reader, "prompt", "default"); got != "custom" {
		t.Fatalf("expected custom value, got %q", got)
	}
}

func TestSetVersionAndExecuteVersion(t *testing.T) {
	oldVersion := appVersion
	oldArgs := rootCmd.Args
	defer func() {
		SetVersion(oldVersion)
		rootCmd.SetArgs([]string{})
		rootCmd.Args = oldArgs
	}()

	SetVersion("v1.2.3")
	rootCmd.SetArgs([]string{"version"})

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	w.Close()
	output, err := io.ReadAll(r)
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}

	if string(bytes.TrimSpace(output)) != "v1.2.3" {
		t.Fatalf("expected version output, got %q", string(output))
	}
}
