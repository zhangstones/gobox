package proc

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureProcOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	defer rOut.Close()
	defer rErr.Close()

	os.Stdout = wOut
	os.Stderr = wErr
	runErr := fn()
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, rOut)
	_, _ = io.Copy(&buf, rErr)
	return buf.String(), runErr
}

func TestPsCmdFullFormatShowsExecutable(t *testing.T) {
	output, err := captureProcOutput(t, func() error {
		return PsCmd([]string{"-f", "-n", "3", "-i", "1"})
	})
	if err != nil {
		t.Fatalf("PsCmd failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and at least one process line, got %q", output)
	}
	if !strings.Contains(lines[0], "PPID") || !strings.Contains(lines[0], "EXE") {
		t.Fatalf("expected full-format header with PPID/EXE, got %q", lines[0])
	}
	if strings.Contains(lines[0], "CMD") {
		t.Fatalf("expected EXE column in full format, got %q", lines[0])
	}
}

func TestPsCmdLengthLimitAppliesWithoutTTY(t *testing.T) {
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	filter := filepath.Base(exePath)

	output, err := captureProcOutput(t, func() error {
		return PsCmd([]string{"-name", filter, "-n", "1", "-l", "8", "-i", "1", "-sort", "pid", "-r"})
	})
	if err != nil {
		t.Fatalf("PsCmd failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected truncated process output, got %q", output)
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		t.Fatalf("unexpected process row format: %q", lines[1])
	}
	cmd := strings.Join(fields[4:], " ")
	if len([]rune(cmd)) > 8 {
		t.Fatalf("expected command to respect -l 8, got %q", cmd)
	}
	if !strings.Contains(cmd, "...") {
		t.Fatalf("expected truncated command with ellipsis, got %q", cmd)
	}
}

func TestTopCmdNonTTYOutputDoesNotEmitClearScreen(t *testing.T) {
	output, err := captureProcOutput(t, func() error {
		return TopCmd([]string{"-n", "1", "-d", "0"})
	})
	if err != nil {
		t.Fatalf("TopCmd failed: %v", err)
	}
	if strings.Contains(output, "\x1b[H\x1b[2J") {
		t.Fatalf("expected no clear-screen escape in non-tty output, got %q", output)
	}
	if !strings.Contains(output, "PPID") || !strings.Contains(output, "EXE") {
		t.Fatalf("expected top output to include ps full-format header, got %q", output)
	}
}

func TestPsCmdNameFilterMatchesSubstring(t *testing.T) {
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	base := filepath.Base(exePath)
	if len(base) < 3 {
		t.Fatalf("unexpected executable name %q", base)
	}
	filter := base[:3]

	output, err := captureProcOutput(t, func() error {
		return PsCmd([]string{"-name", filter, "-n", "20", "-i", "1"})
	})
	if err != nil {
		t.Fatalf("PsCmd failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least one matching process, got %q", output)
	}
	found := false
	for _, line := range lines[1:] {
		if strings.Contains(line, filter) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected at least one row to contain substring %q, got %q", filter, output)
	}
}
