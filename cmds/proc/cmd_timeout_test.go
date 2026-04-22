package proc

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestTimeoutTerminatesCommand(t *testing.T) {
	err := TimeoutCmd([]string{"0.1s", "sleep", "2"})
	if exitErr, ok := err.(timeoutExitError); !ok || exitErr.ExitCode() != 124 {
		t.Fatalf("expected timeout exit 124, got %T %v", err, err)
	}
}

func TestTimeoutCmdOptionsCommandExitsBeforeTimeout(t *testing.T) {

	if err := TimeoutCmd([]string{"1s", "sh", "-c", "exit 7"}); err == nil {
		t.Fatal("expected child exit error")
	} else if exitErr, ok := err.(timeoutExitError); !ok || exitErr.ExitCode() != 7 {
		t.Fatalf("expected child exit 7, got %T %v", err, err)
	}

}

func TestTimeoutCmdOptionsCustomSignal(t *testing.T) {

	err := TimeoutCmd([]string{"-s", "KILL", "0.1s", "sleep", "2"})
	if exitErr, ok := err.(timeoutExitError); !ok || exitErr.ExitCode() != 124 {
		t.Fatalf("expected timeout exit 124, got %T %v", err, err)
	}

}

func TestTimeoutCmdOptionsNumericSignal(t *testing.T) {

	err := TimeoutCmd([]string{"-s", "15", "0.1s", "sleep", "2"})
	if exitErr, ok := err.(timeoutExitError); !ok || exitErr.ExitCode() != 124 {
		t.Fatalf("expected timeout exit 124, got %T %v", err, err)
	}

}

func TestTimeoutCmdOptionsKillAfter(t *testing.T) {

	start := time.Now()
	err := TimeoutCmd([]string{"-k", "0.1s", "0.1s", "sh", "-c", "trap '' TERM; while true; do sleep 1; done"})
	if exitErr, ok := err.(timeoutExitError); !ok || exitErr.ExitCode() != 124 {
		t.Fatalf("expected timeout exit 124, got %T %v", err, err)
	}
	if elapsed := time.Since(start); elapsed < 180*time.Millisecond {
		t.Fatalf("expected kill-after grace period, elapsed=%v", elapsed)
	}

}

func TestTimeoutCmdOptionsInvalidKillAfterDuration(t *testing.T) {

	if err := TimeoutCmd([]string{"-k", "bad", "0.1s", "sleep", "1"}); err == nil {
		t.Fatal("expected invalid kill-after duration error")
	}

}

func TestTimeoutCmdOptionsInvalidDuration(t *testing.T) {

	if err := TimeoutCmd([]string{"bad", "sleep", "1"}); err == nil {
		t.Fatal("expected invalid duration error")
	}

}

func TestTimeoutCmdOptionsPreserveStatusAccepted(t *testing.T) {

	err := TimeoutCmd([]string{"--preserve-status", "0.1s", "sleep", "2"})
	if exitErr, ok := err.(timeoutExitError); !ok || exitErr.ExitCode() != 124 {
		t.Fatalf("expected timeout exit 124, got %T %v", err, err)
	}

}

func TestTimeoutCmdOptionsUnsupportedSignal(t *testing.T) {

	if err := TimeoutCmd([]string{"-s", "USR1", "0.1s", "sleep", "1"}); err == nil {
		t.Fatal("expected unsupported signal error")
	}

}

func TestTimeoutCmdOptionsMissingCommand(t *testing.T) {

	if err := TimeoutCmd([]string{"1s"}); err == nil {
		t.Fatal("expected missing command error")
	}

}

func TestTimeoutCmdOptionsNumericDurationSeconds(t *testing.T) {

	if err := TimeoutCmd([]string{"1", "true"}); err != nil {
		t.Fatal(err)
	}

}

func TestTimeoutCmdOptionsCustomSignalIsDeliveredBeforeKillAfter(t *testing.T) {

	dir := t.TempDir()
	marker := dir + "/term"
	script := "trap 'echo term > \"$1\"; exit 0' TERM; while true; do sleep 0.01; done"
	err := TimeoutCmd([]string{"-s", "TERM", "-k", "1s", "0.1s", "sh", "-c", script, "sh", marker})
	if exitErr, ok := err.(timeoutExitError); !ok || exitErr.ExitCode() != 124 {
		t.Fatalf("expected timeout exit 124, got %T %v", err, err)
	}
	data, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.TrimSpace(string(data)) != "term" {
		t.Fatalf("expected TERM trap marker, got %q", data)
	}

}
