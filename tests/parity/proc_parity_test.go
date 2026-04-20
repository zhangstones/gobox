package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestParity_ProcLightweightCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{
			ID:            "XARGS-001",
			Name:          "xargs -I",
			GoboxArgs:     []string{"xargs", "-I", "{}", "echo", "pre:{}:post"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-I", "{}", "echo", "pre:{}:post"},
			Stdin:         "abc\n",
		},
		{
			ID:            "XARGS-002",
			Name:          "xargs -i",
			GoboxArgs:     []string{"xargs", "-i{}", "echo", "pre:{}:post"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-i{}", "echo", "pre:{}:post"},
			Stdin:         "abc\n",
		},
		{
			ID:            "XARGS-003",
			Name:          "xargs -d",
			GoboxArgs:     []string{"xargs", "-d", ",", "echo"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-d", ",", "echo"},
			Stdin:         "a,b,c",
		},
		{
			ID:            "XARGS-004",
			Name:          "xargs -n",
			GoboxArgs:     []string{"xargs", "-n", "2", "echo"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-n", "2", "echo"},
			Stdin:         "a\nb\nc\n",
		},
		{
			ID:            "XARGS-006",
			Name:          "xargs -r",
			GoboxArgs:     []string{"xargs", "-r", "echo"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-r", "echo"},
			Stdin:         "",
		},
		{
			ID:            "XARGS-007",
			Name:          "xargs -t",
			GoboxArgs:     []string{"xargs", "-t", "echo"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-t", "echo"},
			Stdin:         "a\n",
			Normalize:     normalizeText,
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("xargs -t exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
				}
				if gobox.Stderr != native.Stderr {
					t.Fatalf("xargs -t stderr mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stderr, native.Stderr)
				}
			},
		},
	})

	t.Run("XARGS-005", func(t *testing.T) {
		env := t.TempDir()
		script := filepath.Join(env, "emit.sh")
		writeFile(t, script, "#!/bin/sh\nprintf '%s\\n' \"$1\"\n")
		if err := os.Chmod(script, 0o755); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		gobox := runGoboxCLI(t, env, "a\nb\nc\n", "xargs", "-P", "2", "-n", "1", "./emit.sh")
		native := runNativeCLI(t, env, "a\nb\nc\n", "xargs", "-P", "2", "-n", "1", "./emit.sh")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("xargs -P exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if !sameStringSet(gobox.Stdout, native.Stdout) {
			t.Fatalf("xargs -P output set mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("TOP-001", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "top", "-n", "1", "-d", "0")
		if res.ExitCode != 0 {
			t.Fatalf("top -d failed: %+v", res)
		}
	})
	for _, tc := range []struct {
		id   string
		args []string
	}{
		{"TOP-003", []string{"top", "-n", "1", "-d", "0", "-r"}},
		{"TOP-004", []string{"top", "-n", "1", "-d", "0", "-sort", "pid"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			res := runGoboxCLI(t, t.TempDir(), "", tc.args...)
			if res.ExitCode != 0 {
				t.Fatalf("%s failed: %+v", tc.id, res)
			}
		})
	}

	t.Run("PS-001", func(t *testing.T) {
		pid := os.Getpid()
		gobox := runGoboxCLI(t, t.TempDir(), "", "ps", "-e", "-n", "0", "-i", "1", "-ww")
		native := runNativeCLI(t, t.TempDir(), "", "ps", "-e")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps -e failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(gobox.Stdout, strconv.Itoa(pid)) || !strings.Contains(native.Stdout, strconv.Itoa(pid)) {
			t.Fatalf("ps -e missing current pid %d", pid)
		}
	})

	t.Run("PS-003", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "-i", "1", "-n", "2")
		if res.ExitCode != 0 {
			t.Fatalf("ps -i failed: %+v", res)
		}
	})

	t.Run("PS-004", func(t *testing.T) {
		markerCmd := startMarkerProcess(t, "parity-ps-trunc")
		defer stopCmd(markerCmd)
		shortRes := runGoboxCLI(t, t.TempDir(), "", "ps", "-full", "parity-ps-trunc", "-n", "1", "-l", "8", "-i", "1")
		wideRes := runGoboxCLI(t, t.TempDir(), "", "ps", "-full", "parity-ps-trunc", "-n", "1", "-ww", "-i", "1")
		if shortRes.ExitCode != 0 || wideRes.ExitCode != 0 {
			t.Fatalf("ps truncation failed short=%+v wide=%+v", shortRes, wideRes)
		}
		if len(lastLine(shortRes.Stdout)) >= len(lastLine(wideRes.Stdout)) {
			t.Fatalf("expected truncated output to be shorter\nshort=%q\nwide=%q", shortRes.Stdout, wideRes.Stdout)
		}
	})

	t.Run("PS-005", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "-n", "2", "-i", "1")
		if res.ExitCode != 0 {
			t.Fatalf("ps -n failed: %+v", res)
		}
		if lines := nonEmptyLines(res.Stdout); len(lines) > 3 {
			t.Fatalf("ps -n expected header + 2 rows max, got %d lines: %q", len(lines), res.Stdout)
		}
	})

	t.Run("PS-006", func(t *testing.T) {
		markerCmd := startMarkerProcess(t, "parity-ps-filter-123")
		defer stopCmd(markerCmd)
		pattern := "parity-ps-filter-[0-9]+"
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "-full", pattern, "-n", "5", "-ww", "-i", "1")
		native := runNativeCLI(t, t.TempDir(), "", "pgrep", "-f", pattern)
		if res.ExitCode != 0 || native.ExitCode != 0 || !strings.Contains(res.Stdout, "parity-ps-filter-123") {
			t.Fatalf("ps -full failed: %+v", res)
		}
	})

	t.Run("PS-007", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "-sort", "pid", "-r", "-n", "5", "-i", "1")
		if res.ExitCode != 0 {
			t.Fatalf("ps -r failed: %+v", res)
		}
		pids := extractLeadingInts(nonEmptyLines(res.Stdout)[1:])
		assertMonotonic(t, pids, true)
	})

	t.Run("PS-008", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "-sort", "pid", "-n", "5", "-i", "1")
		if res.ExitCode != 0 {
			t.Fatalf("ps -sort failed: %+v", res)
		}
		pids := extractLeadingInts(nonEmptyLines(res.Stdout)[1:])
		assertMonotonic(t, pids, false)
	})

	t.Run("PS-011", func(t *testing.T) {
		markerCmd := startExactNameProcess(t, "pscomm")
		defer stopCmd(markerCmd)
		pattern := "pscomm"
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "-comm", pattern, "-n", "5", "-ww", "-i", "1")
		native := runNativeCLI(t, t.TempDir(), "", "pgrep", "-x", pattern)
		if res.ExitCode != 0 || native.ExitCode != 0 || !strings.Contains(res.Stdout, pattern) {
			t.Fatalf("ps -comm failed: %+v", res)
		}
	})
}

func TestParity_ProcStructured(t *testing.T) {
	t.Run("PS-002", func(t *testing.T) {
		if _, err := exec.LookPath("ps"); err != nil {
			t.Skip("native ps not found")
		}
		env := &parityEnv{Dir: t.TempDir()}
		gobox := runGoboxCLI(t, env.Dir, "", "ps", "-f", "-n", "5", "-i", "1")
		native := runNativeCLI(t, env.Dir, "", "ps", "-f")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps command failed")
		}
		if !strings.Contains(gobox.Stdout, "PPID") || !strings.Contains(native.Stdout, "PPID") {
			t.Fatalf("ps -f missing PPID headers")
		}
	})

	t.Run("PS-009", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "ps", "-ww", "-n", "3", "-i", "1")
		if res.ExitCode != 0 {
			t.Fatalf("ps -ww failed: %+v", res)
		}
	})

	t.Run("PS-010", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "ps", "-o", "pid,ppid,cmd,pcpu,pmem", "-n", "3", "-i", "1")
		if res.ExitCode != 0 {
			t.Fatalf("ps -o failed: %+v", res)
		}
		for _, field := range []string{"PID", "PPID", "CMD", "%CPU", "%MEM"} {
			if !strings.Contains(res.Stdout, field) {
				t.Fatalf("ps -o missing %s: %q", field, res.Stdout)
			}
		}
	})

	t.Run("TOP-002", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "top", "-n", "1", "-d", "0")
		if res.ExitCode != 0 {
			t.Fatalf("top -n 1 failed: %+v", res)
		}
	})
}
