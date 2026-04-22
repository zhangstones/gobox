package proc

import (
	"context"
	"strings"
	"testing"
	"time"
)

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
