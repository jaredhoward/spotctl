package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestExecute_CoversRootExecute(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	SetVersion("test-1.0")
	rootCmd.SetArgs([]string{"version"})
	err := Execute()

	w.Close()
	os.Stdout = oldStdout
	output, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(string(output), "test-1.0") {
		t.Fatalf("version not in output: %q", string(output))
	}
}
