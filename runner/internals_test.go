package runner

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jaredhoward/spotctl/config"
)

// ----- handleCommandError ----------------------------------------------------

func TestHandleCommandError_Continue(t *testing.T) {
	err := handleCommandError("s", "c", errors.New("boom"), config.OnFailureContinue)
	if err != nil {
		t.Fatalf("expected nil for continue policy, got %v", err)
	}
}

func TestHandleCommandError_Fail(t *testing.T) {
	err := handleCommandError("s", "c", errors.New("boom"), config.OnFailureFail)
	if err == nil {
		t.Fatal("expected error for fail policy")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected 'aborted' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected original error wrapped, got: %v", err)
	}
}

func TestHandleCommandError_SkipRemaining(t *testing.T) {
	err := handleCommandError("s", "c", errors.New("boom"), config.OnFailureSkipRemaining)
	if err == nil {
		t.Fatal("expected skipRemainingError")
	}
	var skipErr *skipRemainingError
	if !errors.As(err, &skipErr) {
		t.Fatalf("expected *skipRemainingError, got %T: %v", err, err)
	}
}

// ----- skipRemainingError ----------------------------------------------------

func TestSkipRemainingError_Message(t *testing.T) {
	e := &skipRemainingError{}
	if e.Error() != "skip_remaining" {
		t.Errorf("unexpected message: %q", e.Error())
	}
}

// ----- DepthExceededError ----------------------------------------------------

func TestDepthExceededError_ContainsMaxDepth(t *testing.T) {
	e := &DepthExceededError{}
	msg := e.Error()
	if !strings.Contains(msg, fmt.Sprintf("%d", MaxSetDepth)) {
		t.Errorf("expected MaxSetDepth (%d) in error message, got: %q", MaxSetDepth, msg)
	}
}

// ----- CommandTimeoutError ---------------------------------------------------

func TestCommandTimeoutError_ContainsTimeoutAndAction(t *testing.T) {
	e := &CommandTimeoutError{Timeout: 3 * time.Second, Action: "pause"}
	msg := e.Error()
	if !strings.Contains(msg, "3s") {
		t.Errorf("expected timeout in message, got: %q", msg)
	}
	if !strings.Contains(msg, "pause") {
		t.Errorf("expected action in message, got: %q", msg)
	}
}
