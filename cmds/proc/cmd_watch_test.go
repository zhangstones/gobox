package proc

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWatchCmdHelpMentionsDefaultAndAppendModes(t *testing.T) {
	out, err := captureProcOutput(t, func() error {
		return WatchCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("WatchCmd help failed: %v", err)
	}
	for _, want := range []string{
		"Usage: gobox watch [-n SEC] [-t] [--append] COMMAND [ARG]...",
		"refresh in-place by clearing the screen",
		"--append           append output instead of clearing the screen",
		"gobox watch --append -n 1 date",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
}

func TestWatchWithContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 220*time.Millisecond)
	defer cancel()
	out, err := captureProcCmd(t, func() error {
		return WatchCmdWithContext(ctx, []string{"-n", "0.05", "-t", "echo", "ok"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("unexpected watch output %q", out)
	}
	if strings.Count(out, "ok") < 2 {
		t.Fatalf("expected watch to run multiple iterations, got %q", out)
	}
	if strings.Contains(out, "\x1b[H\x1b[J") {
		t.Fatalf("expected non-tty watch output to avoid clear-screen sequences, got %q", out)
	}
}

func TestWatchCmdOptionsTitleShownByDefault(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	out, err := captureProcCmd(t, func() error {
		return WatchCmdWithContext(ctx, []string{"-n", "0.05", "echo", "ok"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Every 0.1s") || strings.Count(out, "ok") < 1 {
		t.Fatalf("unexpected watch output %q", out)
	}

}

func TestWatchCmdOptionsMissingCommand(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := WatchCmdWithContext(ctx, nil); err == nil {
		t.Fatal("expected missing command error")
	}

}

func TestWatchCmdOptionsNegativeIntervalCoercesToOneSecond(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	out, err := captureProcCmd(t, func() error {
		return WatchCmdWithContext(ctx, []string{"-n", "-1", "-t", "echo", "ok"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("unexpected watch output %q", out)
	}

}

func TestWatchCmdOptionsInvalidIntervalFlag(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := WatchCmdWithContext(ctx, []string{"-n", "bad", "echo", "ok"}); err == nil {
		t.Fatal("expected invalid interval flag error")
	}

}

func TestWatchCmdOptionsCommandFailureStillPrintsNextIterationsUntilContextStops(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), 220*time.Millisecond)
	defer cancel()
	out, err := captureProcCmd(t, func() error {
		return WatchCmdWithContext(ctx, []string{"-n", "0.05", "-t", "sh", "-c", "echo tick; exit 7"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "tick") < 2 {
		t.Fatalf("expected failed command output to be shown, got %q", out)
	}

}

func TestWatchCmdAppendModeSkipsClearScreen(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 220*time.Millisecond)
	defer cancel()
	out, err := captureProcCmd(t, func() error {
		return WatchCmdWithContext(ctx, []string{"-n", "0.05", "-t", "--append", "echo", "ok"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "[H[J") {
		t.Fatalf("expected --append mode to avoid clear-screen output, got %q", out)
	}
	if strings.Count(out, "ok") < 2 {
		t.Fatalf("expected append mode to keep repeated payload output, got %q", out)
	}
}

func TestWatchCmdDefaultModeSkipsClearScreenWhenStdoutIsNotTTY(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	out, err := captureProcCmd(t, func() error {
		return WatchCmdWithContext(ctx, []string{"-n", "0.05", "-t", "echo", "ok"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\x1b[H\x1b[J") {
		t.Fatalf("expected non-tty watch output to avoid clear-screen sequences, got %q", out)
	}
}
