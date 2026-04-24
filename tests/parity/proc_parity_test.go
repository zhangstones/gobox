package main

import (
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"gobox/cmds/proc"
)

func TestParity_XargsCases(t *testing.T) {
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
}

func TestParity_TopCases(t *testing.T) {
	t.Run("TOP-001", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "top", "-b", "-n", "1", "-d", "0")
		native := runNativeCLI(t, env, "", "top", "-b", "-n", "1", "-d", "0")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "PID") || !strings.Contains(native.Stdout, "PID") {
			t.Fatalf("top -d mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})
	for _, tc := range []struct {
		id         string
		args       []string
		nativeArgs []string
	}{
		{"TOP-003", []string{"top", "-b", "-n", "1", "-d", "0", "-r"}, []string{"-b", "-n", "1", "-d", "0"}},
		{"TOP-004", []string{"top", "-b", "-n", "1", "-d", "0", "-sort", "pid"}, []string{"-b", "-n", "1", "-d", "0"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			env := t.TempDir()
			res := runGoboxCLI(t, env, "", tc.args...)
			native := runNativeCLI(t, env, "", "top", tc.nativeArgs...)
			if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "PID") || !strings.Contains(native.Stdout, "PID") {
				t.Fatalf("%s mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", tc.id, res, native)
			}
		})
	}

	t.Run("TOP-002", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "top", "-b", "-n", "1", "-d", "0")
		native := runNativeCLI(t, env.Dir, "", "top", "-b", "-n", "1", "-d", "0")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "PID") || !strings.Contains(native.Stdout, "PID") {
			t.Fatalf("top -n 1 mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	for _, tc := range []struct {
		id       string
		args     []string
		contains []string
	}{
		{"TOP-005", []string{"top", "-b", "-n", "1", "-d", "0"}, []string{"PID"}},
		{"TOP-006", []string{"top", "-b", "-n", "1", "-d", "0", "-p", strconv.Itoa(os.Getpid())}, []string{strconv.Itoa(os.Getpid())}},
		{"TOP-007", []string{"top", "-b", "-n", "1", "-d", "0", "-u", strconv.Itoa(os.Getuid())}, []string{"PID"}},
		{"TOP-008", []string{"top", "-b", "-n", "1", "-d", "0", "-H"}, []string{"PID"}},
		{"TOP-009", []string{"top", "-b", "-n", "1", "-d", "0", "-i"}, []string{"PID"}},
		{"TOP-010", []string{"top", "-b", "-n", "1", "-d", "0", "-c"}, []string{"PID"}},
		{"TOP-011", []string{"top", "-b", "-n", "1", "-d", "0", "-o", "PID"}, []string{"PID"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			env := t.TempDir()
			res := runGoboxCLI(t, env, "", tc.args...)
			nativeArgs := append([]string{}, tc.args[1:]...)
			native := runNativeCLI(t, env, "", "top", nativeArgs...)
			if res.ExitCode != 0 {
				t.Fatalf("%s failed: %+v", tc.id, res)
			}
			if native.ExitCode != 0 {
				t.Fatalf("%s native top failed: %+v", tc.id, native)
			}
			if strings.Contains(res.Stdout, "\x1b[H\x1b[2J") {
				t.Fatalf("%s emitted clear-screen sequence in batch output: %q", tc.id, res.Stdout)
			}
			for _, want := range tc.contains {
				if !strings.Contains(res.Stdout, want) {
					t.Fatalf("%s missing %q in %q", tc.id, want, res.Stdout)
				}
			}
		})
	}
}

func TestParity_PsCases(t *testing.T) {
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
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "-sort", "pid", "-r", "-n", "5", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-e", "--sort", "-pid", "-o", "pid")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps -r failed gobox=%+v native=%+v", res, native)
		}
		pids := extractLeadingInts(nonEmptyLines(res.Stdout)[1:])
		assertMonotonic(t, pids, true)
		nativePIDs := extractLeadingInts(nonEmptyLines(native.Stdout)[1:])
		assertMonotonic(t, nativePIDs, true)
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

	t.Run("PS-012", func(t *testing.T) {
		pid := strconv.Itoa(os.Getpid())
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "-A", "-p", pid, "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-A")
		if res.ExitCode != 0 || native.ExitCode != 0 || !strings.Contains(res.Stdout, pid) || !strings.Contains(native.Stdout, pid) {
			t.Fatalf("ps -A mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("PS-013", func(t *testing.T) {
		pid := strconv.Itoa(os.Getpid())
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "-F", "-p", pid, "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-F", "-p", pid)
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "UID") || !strings.Contains(res.Stdout, "VSZ") || !strings.Contains(native.Stdout, "UID") || !strings.Contains(native.Stdout, "SZ") {
			t.Fatalf("ps -F mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("PS-014", func(t *testing.T) {
		uid := strconv.Itoa(os.Getuid())
		pid := strconv.Itoa(os.Getpid())
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "-u", uid, "-p", pid, "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-u", uid)
		if res.ExitCode != 0 || native.ExitCode != 0 || !strings.Contains(res.Stdout, pid) || !strings.Contains(native.Stdout, pid) {
			t.Fatalf("ps -u mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("PS-015", func(t *testing.T) {
		pid := strconv.Itoa(os.Getpid())
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "-p", pid, "-o", "pid", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-p", pid, "-o", "pid")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, pid) || !strings.Contains(native.Stdout, pid) {
			t.Fatalf("ps -p mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("PS-016", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("ps -C test reads /proc/self/comm")
		}
		data, err := os.ReadFile("/proc/self/comm")
		if err != nil {
			t.Skipf("cannot read comm: %v", err)
		}
		comm := strings.TrimSpace(string(data))
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "-C", comm, "-o", "comm", "-n", "20", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-C", comm, "-o", "comm")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, comm) || !strings.Contains(native.Stdout, comm) {
			t.Fatalf("ps -C mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("PS-017", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "--sort", "-pid", "-n", "5", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-e", "--sort", "-pid", "-o", "pid")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps --sort failed gobox=%+v native=%+v", res, native)
		}
		pids := extractLeadingInts(nonEmptyLines(res.Stdout)[1:])
		assertMonotonic(t, pids, true)
	})

	t.Run("PS-018", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "aux", "-n", "2", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "aux")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps aux failed gobox=%+v native=%+v", res, native)
		}
		for _, want := range []string{"USER", "PID", "%CPU", "%MEM"} {
			if !strings.Contains(res.Stdout, want) || !strings.Contains(native.Stdout, want) {
				t.Fatalf("ps aux missing %s\n--- gobox ---\n%s\n--- native ---\n%s", want, res.Stdout, native.Stdout)
			}
		}
		if !strings.Contains(res.Stdout, "CMD") || !(strings.Contains(native.Stdout, "CMD") || strings.Contains(native.Stdout, "COMMAND")) {
			t.Fatalf("ps aux missing command column\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
	})
	t.Run("PS-002", func(t *testing.T) {
		requireNativeCommand(t, "ps")
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
		native := runNativeCLI(t, env.Dir, "", "ps", "-ww")
		if res.ExitCode != 0 || native.ExitCode != 0 || !strings.Contains(res.Stdout, "CMD") {
			t.Fatalf("ps -ww mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("PS-010", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		fields := "pid,ppid,cmd,pcpu,pmem"
		res := runGoboxCLI(t, env.Dir, "", "ps", "-o", fields, "-n", "3", "-i", "1")
		native := runNativeCLI(t, env.Dir, "", "ps", "-o", fields)
		if res.ExitCode != native.ExitCode {
			t.Fatalf("ps -o exit mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		for _, field := range []string{"PID", "PPID", "CMD", "%CPU", "%MEM"} {
			if !strings.Contains(res.Stdout, field) || !strings.Contains(native.Stdout, field) {
				t.Fatalf("ps -o missing %s\n--- gobox ---\n%s\n--- native ---\n%s", field, res.Stdout, native.Stdout)
			}
		}
	})

}

func TestParity_LsofCases(t *testing.T) {
	requireNativeCommand(t, "lsof")

	t.Run("LSOF-001", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof")
		native := runNativeCLI(t, env, "", "lsof")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("lsof exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		pid := strconv.Itoa(os.Getpid())
		if !strings.Contains(gobox.Stdout, pid) || !strings.Contains(native.Stdout, pid) {
			t.Fatalf("lsof missing current pid\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("LSOF-002", func(t *testing.T) {
		env := t.TempDir()
		pid := strconv.Itoa(os.Getpid())
		gobox := runGoboxCLI(t, env, "", "lsof", "-p", pid)
		native := runNativeCLI(t, env, "", "lsof", "-p", pid)
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, pid) || !strings.Contains(native.Stdout, pid) {
			t.Fatalf("lsof -p mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("LSOF-003", func(t *testing.T) {
		cmd := startExactNameProcess(t, "lsofcmd")
		defer stopCmd(cmd)
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof", "-c", "lsofcmd")
		native := runNativeCLI(t, env, "", "lsof", "-c", "lsofcmd")
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "lsofcmd") || !strings.Contains(native.Stdout, "lsofcmd") {
			t.Fatalf("lsof -c mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("LSOF-004", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof", "-i")
		native := runNativeCLI(t, env, "", "lsof", "-i")
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "TCP") || !strings.Contains(native.Stdout, "TCP") {
			t.Fatalf("lsof -i mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("LSOF-005", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof", "-iTCP")
		native := runNativeCLI(t, env, "", "lsof", "-iTCP")
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "TCP") || !strings.Contains(native.Stdout, "TCP") {
			t.Fatalf("lsof -iTCP mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("LSOF-006", func(t *testing.T) {
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof", "-iUDP")
		native := runNativeCLI(t, env, "", "lsof", "-iUDP")
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "UDP") || !strings.Contains(native.Stdout, "UDP") {
			t.Fatalf("lsof -iUDP mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("LSOF-007", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof", "-i", ":"+port)
		native := runNativeCLI(t, env, "", "lsof", "-i", ":"+port)
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "TCP") || !strings.Contains(native.Stdout, ":"+port) {
			t.Fatalf("lsof -i :PORT mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("LSOF-008", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof", "-n", "-i")
		native := runNativeCLI(t, env, "", "lsof", "-n", "-i")
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "TCP") || !strings.Contains(native.Stdout, "TCP") {
			t.Fatalf("lsof -n mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("LSOF-009", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof", "-P", "-i", ":"+port)
		native := runNativeCLI(t, env, "", "lsof", "-P", "-i", ":"+port)
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, ":"+port) || !strings.Contains(native.Stdout, ":"+port) {
			t.Fatalf("lsof -P mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("LSOF-010", func(t *testing.T) {
		env := t.TempDir()
		pid := strconv.Itoa(os.Getpid())
		gobox := runGoboxCLI(t, env, "", "lsof", "-t", "-p", pid)
		native := runNativeCLI(t, env, "", "lsof", "-t", "-p", pid)
		if gobox.ExitCode != native.ExitCode || normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("lsof -t mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("LSOF-011", func(t *testing.T) {
		env := t.TempDir()
		f, err := os.Create(filepath.Join(env, "open.txt"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		gobox := runGoboxCLI(t, env, "", "lsof", "open.txt")
		native := runNativeCLI(t, env, "", "lsof", "open.txt")
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "open.txt") || !strings.Contains(native.Stdout, "open.txt") {
			t.Fatalf("lsof FILE mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

}

func TestParity_FreeCases(t *testing.T) {
	t.Run("FREE-001", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "free")
		native := runNativeCLI(t, env, "", "free")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("free exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if !strings.Contains(gobox.Stdout, "Mem:") || !strings.Contains(native.Stdout, "Mem:") {
			t.Fatalf("free output missing Mem row\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})
	t.Run("FREE-002", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "free", "-h")
		native := runNativeCLI(t, env, "", "free", "-h")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("free -h exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if !containsAny(gobox.Stdout, []string{"Ki", "Mi", "Gi", "Ti", "KB", "MB", "GB", "TB"}) || !containsAny(native.Stdout, []string{"Ki", "Mi", "Gi", "Ti", "KB", "MB", "GB", "TB"}) {
			t.Fatalf("free -h missing human units\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})
	t.Run("FREE-003", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "free", "-m")
		native := runNativeCLI(t, env, "", "free", "-m")
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "Mem:") || !strings.Contains(native.Stdout, "Mem:") {
			t.Fatalf("free -m mismatch gobox=%+v native=%+v", gobox, native)
		}
	})
	t.Run("FREE-004", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "free", "-g")
		native := runNativeCLI(t, env, "", "free", "-g")
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "Mem:") || !strings.Contains(native.Stdout, "Mem:") {
			t.Fatalf("free -g mismatch gobox=%+v native=%+v", gobox, native)
		}
	})
	t.Run("FREE-005", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "free", "-s", "1", "-c", "2")
		native := runNativeCLI(t, env, "", "free", "-s", "1", "-c", "2")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("free -s/-c exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if strings.Count(gobox.Stdout, "Mem:") < 2 || strings.Count(native.Stdout, "Mem:") < 2 {
			t.Fatalf("free -s/-c expected repeated samples\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})
}

func TestParity_TimeoutCases(t *testing.T) {
	t.Run("TIMEOUT-001", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "timeout", "0.1s", "sleep", "2")
		native := runNativeCLI(t, env, "", "timeout", "0.1s", "sleep", "2")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("timeout exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
	})
	t.Run("TIMEOUT-002", func(t *testing.T) {
		env := t.TempDir()
		goboxMarker := filepath.Join(env, "gobox-int")
		nativeMarker := filepath.Join(env, "native-int")
		script := "trap 'echo INT > \"$1\"; exit 0' INT; while true; do sleep 1; done"
		gobox := runGoboxCLI(t, env, "", "timeout", "-s", "INT", "0.1s", "sh", "-c", script, "sh", goboxMarker)
		native := runNativeCLI(t, env, "", "timeout", "-s", "INT", "0.1s", "sh", "-c", script, "sh", nativeMarker)
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("timeout -s exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		goboxData, err := os.ReadFile(goboxMarker)
		if err != nil {
			t.Fatalf("read gobox marker: %v", err)
		}
		nativeData, err := os.ReadFile(nativeMarker)
		if err != nil {
			t.Fatalf("read native marker: %v", err)
		}
		if strings.TrimSpace(string(goboxData)) != "INT" || strings.TrimSpace(string(nativeData)) != "INT" {
			t.Fatalf("timeout -s marker mismatch gobox=%q native=%q", string(goboxData), string(nativeData))
		}
	})
	t.Run("TIMEOUT-003", func(t *testing.T) {
		env := t.TempDir()
		start := time.Now()
		gobox := runGoboxCLI(t, env, "", "timeout", "-k", "0.1s", "0.1s", "sh", "-c", "trap '' TERM; while true; do sleep 1; done")
		goboxElapsed := time.Since(start)
		start = time.Now()
		native := runNativeCLI(t, env, "", "sh", "-c", "timeout -k 0.1s 0.1s sh -c 'trap \"\" TERM; while true; do sleep 1; done'; exit $?")
		nativeElapsed := time.Since(start)
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("timeout -k exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if goboxElapsed < 180*time.Millisecond || nativeElapsed < 180*time.Millisecond {
			t.Fatalf("timeout -k should honor grace period gobox=%s native=%s", goboxElapsed, nativeElapsed)
		}
	})
	t.Run("TIMEOUT-004", func(t *testing.T) {
		env := t.TempDir()
		script := "trap 'exit 7' TERM; while true; do sleep 1; done"
		gobox := runGoboxCLI(t, env, "", "timeout", "--preserve-status", "0.1s", "sh", "-c", script)
		native := runNativeCLI(t, env, "", "timeout", "--preserve-status", "0.1s", "sh", "-c", script)
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("timeout --preserve-status exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if gobox.ExitCode != 7 {
			t.Fatalf("timeout --preserve-status should keep child exit 7, got %+v", gobox)
		}
	})
	t.Run("TIMEOUT-005", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "timeout", "0.1s", "sleep", "1")
		native := runNativeCLI(t, env, "", "timeout", "0.1s", "sleep", "1")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("timeout suffix exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
	})
}

func TestParity_WatchCases(t *testing.T) {
	t.Run("WATCH-001", func(t *testing.T) {
		var out strings.Builder
		old := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = w
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
		defer cancel()
		err = proc.WatchCmdWithContext(ctx, []string{"-n", "0.05", "-t", "echo", "ok"})
		_ = w.Close()
		os.Stdout = old
		_, _ = io.Copy(&out, r)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "ok") {
			t.Fatalf("watch output missing command result: %q", out.String())
		}
	})
	t.Run("WATCH-002", func(t *testing.T) {
		runWatch := func(interval string, timeout time.Duration) int {
			var out strings.Builder
			old := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdout = w
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err = proc.WatchCmdWithContext(ctx, []string{"-n", interval, "-t", "echo", "tick"})
			_ = w.Close()
			os.Stdout = old
			_, _ = io.Copy(&out, r)
			if err != nil {
				t.Fatal(err)
			}
			return strings.Count(out.String(), "tick")
		}
		fast := runWatch("0.03", 220*time.Millisecond)
		slow := runWatch("0.09", 220*time.Millisecond)
		if fast <= slow {
			t.Fatalf("watch -n cadence mismatch fast=%d slow=%d", fast, slow)
		}
		if fast < 4 || slow < 2 {
			t.Fatalf("watch -n produced too few refreshes fast=%d slow=%d", fast, slow)
		}
	})
	t.Run("WATCH-003", func(t *testing.T) {
		var out strings.Builder
		old := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = w
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
		defer cancel()
		err = proc.WatchCmdWithContext(ctx, []string{"-n", "0.05", "-t", "echo", "ok"})
		_ = w.Close()
		os.Stdout = old
		_, _ = io.Copy(&out, r)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "ok") || strings.Contains(out.String(), "Every ") {
			t.Fatalf("watch -t title suppression mismatch: %q", out.String())
		}
	})
}

func TestParity_KillCases(t *testing.T) {
	t.Run("KILL-010", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "kill", "--dry-run", "-x", "sleep")
		if gobox.ExitCode != 0 {
			t.Fatalf("kill --dry-run failed: %+v", gobox)
		}
	})
	for _, tc := range []struct {
		id     string
		reason string
	}{
		{"KILL-001", ""},
		{"KILL-002", ""},
		{"KILL-003", ""},
		{"KILL-004", ""},
		{"KILL-005", ""},
		{"KILL-006", ""},
		{"KILL-007", ""},
		{"KILL-008", ""},
		{"KILL-009", ""},
	} {
		t.Run(tc.id, func(t *testing.T) {
			switch tc.id {
			case "KILL-001":
				cmd := exec.Command("sleep", "30")
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", strconv.Itoa(cmd.Process.Pid)); res.ExitCode != 0 {
					t.Fatalf("kill default TERM failed: %+v", res)
				}
				if err := waitForExit(t, cmd, time.Second); err != nil {
					_ = cmd.Process.Kill()
					t.Fatal(err)
				}
			case "KILL-002":
				gobox := runGoboxCLI(t, t.TempDir(), "", "kill", "-l")
				native := runNativeCLI(t, t.TempDir(), "", "kill", "-l")
				for _, want := range []string{"HUP", "INT", "KILL", "TERM"} {
					if !strings.Contains(gobox.Stdout, want) || !strings.Contains(native.Stdout, want) {
						t.Fatalf("kill -l missing %q\ngobox=%q\nnative=%q", want, gobox.Stdout, native.Stdout)
					}
				}
				if out := normalizeText(runGoboxCLI(t, t.TempDir(), "", "kill", "-l", "TERM").Stdout); out != "15" {
					t.Fatalf("kill -l TERM mismatch: %q", out)
				}
				if out := normalizeText(runGoboxCLI(t, t.TempDir(), "", "kill", "-l", "15").Stdout); out != "TERM" {
					t.Fatalf("kill -l 15 mismatch: %q", out)
				}
			case "KILL-003":
				cmd := exec.Command("sleep", "30")
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-s", "TERM", strconv.Itoa(cmd.Process.Pid)); res.ExitCode != 0 {
					t.Fatalf("kill -s TERM failed: %+v", res)
				}
				if err := waitForExit(t, cmd, time.Second); err != nil {
					_ = cmd.Process.Kill()
					t.Fatal(err)
				}
			case "KILL-004":
				cmd := exec.Command("sleep", "30")
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-KILL", strconv.Itoa(cmd.Process.Pid)); res.ExitCode != 0 {
					t.Fatalf("kill -KILL failed: %+v", res)
				}
				if err := waitForExit(t, cmd, time.Second); err != nil {
					_ = cmd.Process.Kill()
					t.Fatal(err)
				}
			case "KILL-005":
				marker := "pkfull-" + strconv.FormatInt(time.Now().UnixNano(), 10)
				cmd := exec.Command("sh", "-c", "sleep 30 & wait", marker)
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				time.Sleep(100 * time.Millisecond)
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-f", marker); res.ExitCode != 0 {
					stopCmd(cmd)
					t.Fatalf("kill -f failed: %+v", res)
				}
				if err := waitForExit(t, cmd, time.Second); err != nil {
					stopCmd(cmd)
					t.Fatal(err)
				}
			case "KILL-006":
				name := "pkx" + strconv.FormatInt(time.Now().UnixNano()%100000000, 10)
				cmd := startExactNameProcess(t, name)
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-x", name); res.ExitCode != 0 {
					stopCmd(cmd)
					t.Fatalf("kill -x failed: %+v", res)
				}
				if err := waitForExit(t, cmd, time.Second); err != nil {
					stopCmd(cmd)
					t.Fatal(err)
				}
			case "KILL-007":
				parent := exec.Command("sh", "-c", "sleep 30 & wait")
				if err := parent.Start(); err != nil {
					t.Fatal(err)
				}
				defer stopCmd(parent)
				time.Sleep(100 * time.Millisecond)
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-P", strconv.Itoa(parent.Process.Pid)); res.ExitCode != 0 {
					t.Fatalf("kill -P failed: %+v", res)
				}
			case "KILL-008":
				marker := "pknew-" + strconv.FormatInt(time.Now().UnixNano(), 10)
				oldest := exec.Command("sh", "-c", "sleep 30 & wait", marker+"-1")
				if err := oldest.Start(); err != nil {
					t.Fatal(err)
				}
				defer stopCmd(oldest)
				time.Sleep(1200 * time.Millisecond)
				newest := exec.Command("sh", "-c", "sleep 30 & wait", marker+"-2")
				if err := newest.Start(); err != nil {
					t.Fatal(err)
				}
				defer stopCmd(newest)
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-n", "-f", marker); res.ExitCode != 0 {
					t.Fatalf("kill -n failed: %+v", res)
				}
				if err := waitForExit(t, newest, time.Second); err != nil {
					t.Fatal(err)
				}
				requireAlive(t, oldest)
			case "KILL-009":
				marker := "pkold-" + strconv.FormatInt(time.Now().UnixNano(), 10)
				oldest := exec.Command("sh", "-c", "sleep 30 & wait", marker+"-1")
				if err := oldest.Start(); err != nil {
					t.Fatal(err)
				}
				defer stopCmd(oldest)
				time.Sleep(1200 * time.Millisecond)
				newest := exec.Command("sh", "-c", "sleep 30 & wait", marker+"-2")
				if err := newest.Start(); err != nil {
					t.Fatal(err)
				}
				defer stopCmd(newest)
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-o", "-f", marker); res.ExitCode != 0 {
					t.Fatalf("kill -o failed: %+v", res)
				}
				if err := waitForExit(t, oldest, time.Second); err != nil {
					t.Fatal(err)
				}
				requireAlive(t, newest)
			default:
				t.Fatalf("unexpected case %s", tc.id)
			}
		})
	}
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
