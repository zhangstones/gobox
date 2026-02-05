package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunNoArgsShowsUsage(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer

	code := run([]string{}, &out, &err)

	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(out.String(), "Usage: gobox") {
		t.Fatalf("expected usage output, got %q", out.String())
	}
	if err.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", err.String())
	}
}

func TestRunHelp(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer

	code := run([]string{"--help"}, &out, &err)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Fatalf("expected commands list, got %q", out.String())
	}
	if err.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", err.String())
	}
}

func TestRunVersion(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer

	code := run([]string{"version"}, &out, &err)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out.String(), "gobox 0.1") {
		t.Fatalf("expected version output, got %q", out.String())
	}
	if err.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", err.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer

	code := run([]string{"unknown"}, &out, &err)

	if code != 127 {
		t.Fatalf("expected exit code 127, got %d", code)
	}
	if !strings.Contains(err.String(), "unknown command: unknown") {
		t.Fatalf("expected unknown command on stderr, got %q", err.String())
	}
	if !strings.Contains(out.String(), "Usage: gobox") {
		t.Fatalf("expected usage on stdout, got %q", out.String())
	}
}
