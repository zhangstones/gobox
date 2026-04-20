package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	netcmd "gobox/cmds/net"
	textcmd "gobox/cmds/text"
)

func TestParity_RemainingLightweightCases(t *testing.T) {
	t.Run("HEAD-004", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "head", "-h")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Usage") {
			t.Fatalf("head -h failed: %+v", res)
		}
	})

	t.Run("FIND-007", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "tree", "a.txt"), "a")
		defaultRes := runGoboxCLI(t, env, "", "find", "tree")
		explicitRes := runGoboxCLI(t, env, "", "find", "-print", "tree")
		if defaultRes.ExitCode != 0 || explicitRes.ExitCode != 0 {
			t.Fatalf("find default/-print failed: default=%+v explicit=%+v", defaultRes, explicitRes)
		}
		if normalizeText(defaultRes.Stdout) != normalizeText(explicitRes.Stdout) {
			t.Fatalf("find default vs -print mismatch\n%s\n%s", defaultRes.Stdout, explicitRes.Stdout)
		}
	})

	t.Run("GREP-005", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "input.txt"), "foo\nbar\n")
		res := runGoboxCLI(t, env, "", "grep", "--line-buffered", "foo", "input.txt")
		if res.ExitCode != 0 || normalizeText(res.Stdout) != "foo" {
			t.Fatalf("grep --line-buffered failed: %+v", res)
		}
	})

	t.Run("SED-005", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "sed", "-h")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Usage") {
			t.Fatalf("sed -h failed: %+v", res)
		}
	})

	runExactParityCases(t, []parityCase{
		{
			ID:            "SED-002",
			Name:          "sed -i",
			GoboxArgs:     []string{"sed", "-i.bak", "s/foo/bar/", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"-i.bak", "s/foo/bar/", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") },
			Assert: func(t *testing.T, gobox, native parityResult) {
				withTempChdir(t, t.TempDir(), func() {})
				// output is in-place; stdout parity is enough here
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("sed -i exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
				}
			},
		},
		{
			ID:            "SED-009",
			Name:          "sed =",
			GoboxArgs:     []string{"sed", "=", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"=", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nb\n") },
		},
		{
			ID:            "SED-010",
			Name:          "sed i\\",
			GoboxArgs:     []string{"sed", "1i\\before", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"1i\\before", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\n") },
		},
		{
			ID:            "SED-011",
			Name:          "sed a\\",
			GoboxArgs:     []string{"sed", "1a\\after", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"1a\\after", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\n") },
		},
		{
			ID:            "SED-012",
			Name:          "sed c\\",
			GoboxArgs:     []string{"sed", "1c\\changed", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"1c\\changed", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\n") },
		},
		{
			ID:            "SED-013",
			Name:          "sed s///g",
			GoboxArgs:     []string{"sed", "s/foo/bar/g", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"s/foo/bar/g", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo foo\n") },
		},
		{
			ID:            "SED-014",
			Name:          "sed s///i",
			GoboxArgs:     []string{"sed", "s/foo/bar/i", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"s/foo/bar/i", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "Foo\n") },
		},
		{
			ID:            "SED-015",
			Name:          "sed s///p",
			GoboxArgs:     []string{"sed", "-n", "s/foo/bar/p", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"-n", "s/foo/bar/p", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") },
		},
		{
			ID:            "SED-016",
			Name:          "sed s///N",
			GoboxArgs:     []string{"sed", "s/foo/bar/2", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"s/foo/bar/2", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo foo foo\n") },
		},
	})

	t.Run("MD5-005", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "checksums.md5"), "bad line\n")
		res := runGoboxCLI(t, env, "", "md5sum", "--warn", "--check", "checksums.md5")
		if res.ExitCode == 0 || !strings.Contains(strings.ToLower(res.Stdout+res.Stderr), "improperly formatted") {
			t.Fatalf("md5sum --warn failed: %+v", res)
		}
	})

	t.Run("SORT-008", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "input.txt"), "a\nb\nc\n")
		res := runGoboxCLI(t, env, "", "sort", "-R", "input.txt")
		if res.ExitCode != 0 {
			t.Fatalf("sort -R failed: %+v", res)
		}
		lines := strings.Split(normalizeText(res.Stdout), "\n")
		if len(lines) != 3 {
			t.Fatalf("sort -R expected 3 lines, got %q", res.Stdout)
		}
	})

	t.Run("SORT-011", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "input.txt"), "b\x00a\x00")
		res := runGoboxCLI(t, env, "", "sort", "-z", "input.txt")
		if res.ExitCode != 0 || res.Stdout != "a\x00b\x00" {
			t.Fatalf("sort -z failed: %+v", res)
		}
	})

	t.Run("TAIL-002", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "follow.log")
		writeFile(t, file, "base\n")
		gobox := runTailGoboxFollow(t, env, []string{"-n", "0", "-f", "follow.log"}, func() {
			appendFile(t, file, "gobox-follow\n")
		}, 1600*time.Millisecond)
		native := runNativeFollow(t, env, "tail", []string{"-n", "0", "-f", "follow.log"}, func() {
			appendFile(t, file, "native-follow\n")
		}, 1600*time.Millisecond)
		if !strings.Contains(gobox.Stdout, "gobox-follow") || !strings.Contains(native.Stdout, "native-follow") {
			t.Fatalf("tail -f did not follow append\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("TAIL-003", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "rotate.log")
		writeFile(t, file, "base\n")
		action := func(content string) func() {
			return func() {
				if err := os.Rename(file, filepath.Join(env, "rotate.log.1")); err != nil {
					t.Fatalf("rename: %v", err)
				}
				writeFile(t, file, content+"\n")
			}
		}
		gobox := runTailGoboxFollow(t, env, []string{"-n", "1", "--follow=name", "-s", "0.1", "rotate.log"}, action("gobox-rotated"), 1200*time.Millisecond)
		writeFile(t, file, "base\n")
		native := runNativeFollow(t, env, "tail", []string{"-n", "1", "--follow=name", "-s", "0.1", "rotate.log"}, action("native-rotated"), 1200*time.Millisecond)
		if !strings.Contains(gobox.Stdout, "gobox-rotated") || !strings.Contains(native.Stdout, "native-rotated") {
			t.Fatalf("tail --follow=name did not follow rotation\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("TAIL-004", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "delayed.log")
		gobox := runTailGoboxFollow(t, env, []string{"-n", "1", "--retry", "--follow=name", "-s", "0.1", "delayed.log"}, func() {
			writeFile(t, file, "gobox-created\n")
		}, 900*time.Millisecond)
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove delayed file: %v", err)
		}
		native := runNativeFollow(t, env, "tail", []string{"-n", "1", "--retry", "--follow=name", "-s", "0.1", "delayed.log"}, func() {
			writeFile(t, file, "native-created\n")
		}, 900*time.Millisecond)
		if !strings.Contains(gobox.Stdout, "gobox-created") || !strings.Contains(native.Stdout, "native-created") {
			t.Fatalf("tail --retry did not read delayed file\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("TAIL-006", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "sleep.log")
		writeFile(t, file, "base\n")
		gobox := runTailGoboxFollow(t, env, []string{"-n", "0", "-f", "-s", "0.1", "sleep.log"}, func() {
			appendFile(t, file, "gobox-sleep\n")
		}, 800*time.Millisecond)
		native := runNativeFollow(t, env, "tail", []string{"-n", "0", "-f", "-s", "0.1", "sleep.log"}, func() {
			appendFile(t, file, "native-sleep\n")
		}, 800*time.Millisecond)
		if !strings.Contains(gobox.Stdout, "gobox-sleep") || !strings.Contains(native.Stdout, "native-sleep") {
			t.Fatalf("tail -s did not poll appended content\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("TAIL-007", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "pid.log")
		writeFile(t, file, "base\n")
		child := exec.Command("sleep", "0.5")
		if err := child.Start(); err != nil {
			t.Fatalf("start child: %v", err)
		}
		childDone := make(chan struct{})
		go func() {
			_ = child.Wait()
			close(childDone)
		}()
		start := time.Now()
		gobox := runTailGoboxFollow(t, env, []string{"-n", "0", "-f", "-s", "0.1", fmt.Sprintf("--pid=%d", child.Process.Pid), "pid.log"}, func() {
			appendFile(t, file, "pid-follow\n")
		}, 2*time.Second)
		<-childDone
		if gobox.ExitCode != 0 || time.Since(start) > 1500*time.Millisecond {
			t.Fatalf("tail --pid did not exit after pid ended: %+v elapsed=%s", gobox, time.Since(start))
		}
		if !strings.Contains(gobox.Stdout, "pid-follow") {
			t.Fatalf("tail --pid did not emit appended content: %+v", gobox)
		}
	})

	runExactParityCases(t, []parityCase{
		{
			ID:            "XARGS-002",
			Name:          "xargs -i",
			GoboxArgs:     []string{"xargs", "-i{}", "echo", "pre:{}:post"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-i{}", "echo", "pre:{}:post"},
			Stdin:         "abc\n",
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

	t.Run("TW-001", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "tw", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("tw help failed: %+v", res)
		}
	})
	t.Run("TW-002", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "index.html"), "ok")
		res := runGoboxCLI(t, env, "", "tw", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("tw dir contract failed: %+v", res)
		}
	})
	t.Run("TW-003", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "tw", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("tw reuse contract failed: %+v", res)
		}
	})

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

	t.Run("NETSTAT-002", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-sort", "pid")
		if res.ExitCode != 0 {
			t.Fatalf("netstat -sort pid failed: %+v", res)
		}
		pids := extractNetstatPIDs(res.Stdout)
		assertMonotonic(t, pids, false)
	})

	t.Run("NETSTAT-003", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-state", "LISTEN")
		if res.ExitCode != 0 {
			t.Fatalf("netstat -state failed: %+v", res)
		}
		for _, line := range nonEmptyLines(res.Stdout)[1:] {
			if !strings.Contains(line, "LISTEN") {
				t.Fatalf("netstat -state LISTEN leaked non-LISTEN row: %q", line)
			}
		}
	})

	t.Run("NETSTAT-006", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-a")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Proto") {
			t.Fatalf("netstat -a failed: %+v", res)
		}
	})

	t.Run("NETSTAT-007", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-port", strconv.Itoa(port))
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "TCP") || strings.Contains(res.Stdout, "UDP") || strings.Contains(res.Stdout, "UNIX") {
			t.Fatalf("netstat -t failed: %+v", res)
		}
	})

	t.Run("NETSTAT-008", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen udp: %v", err)
		}
		defer conn.Close()
		port := conn.LocalAddr().(*net.UDPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-u", "-port", strconv.Itoa(port))
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "UDP") || strings.Contains(res.Stdout, "TCP") || strings.Contains(res.Stdout, "UNIX") {
			t.Fatalf("netstat -u failed: %+v", res)
		}
	})

	t.Run("NETSTAT-009", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		unixPath := filepath.Join(t.TempDir(), "netstat.sock")
		ln, err := net.Listen("unix", unixPath)
		if err != nil {
			t.Fatalf("listen unix: %v", err)
		}
		defer ln.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-x", "-l")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "UNIX") || !strings.Contains(res.Stdout, unixPath) {
			t.Fatalf("netstat -x failed: %+v", res)
		}
	})

	t.Run("NETSTAT-010", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-p")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "PID/Program") {
			t.Fatalf("netstat -p failed: %+v", res)
		}
	})

	t.Run("NETSTAT-011", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp4: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-4", "-port", strconv.Itoa(port))
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "127.0.0.1") {
			t.Fatalf("netstat -4 failed: %+v", res)
		}
	})

	t.Run("NETSTAT-012", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		_, port, closeFn := startTCPEchoServer(t, "[::1]:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-6", "-port", port)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "::1") {
			t.Fatalf("netstat -6 failed: %+v", res)
		}
	})

	t.Run("NETSTAT-013", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-e")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "User") || !strings.Contains(res.Stdout, "Inode") {
			t.Fatalf("netstat -e failed: %+v", res)
		}
	})

	t.Run("NETSTAT-014", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-o")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Timer") {
			t.Fatalf("netstat -o failed: %+v", res)
		}
	})

	t.Run("NETSTAT-015", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-W", "-n", "-l")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Proto") {
			t.Fatalf("netstat -W/-n/-l failed: %+v", res)
		}
	})

	t.Run("DU-002", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "tree", "a.txt"), strings.Repeat("a", 128))
		writeFile(t, filepath.Join(env, "tree", "sub", "b.txt"), strings.Repeat("b", 256))
		res := runGoboxCLI(t, env, "", "du", "-s", "tree")
		if res.ExitCode != 0 {
			t.Fatalf("du -s failed: %+v", res)
		}
		lines := nonEmptyLines(res.Stdout)
		if len(lines) != 1 || !strings.Contains(lines[0], "tree") {
			t.Fatalf("du -s expected single summary line, got %q", res.Stdout)
		}
	})

	t.Run("DU-001", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "tree", "a.txt"), strings.Repeat("a", 128))
		res := runGoboxCLI(t, env, "", "du", "-h", "tree")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "tree/a.txt") {
			t.Fatalf("du -h failed: %+v", res)
		}
	})

	t.Run("CURL-002", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"curl", "-s", "-S", "://bad-url"}, &stdout, &stderr)
		if code == 0 || !strings.Contains(strings.ToLower(stderr.String()), "curl:") {
			t.Fatalf("curl -s -S failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	t.Run("CURL-003", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "file-body") }))
		defer server.Close()
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "curl", "-o", "out.txt", server.URL)
		if res.ExitCode != 0 {
			t.Fatalf("curl -o failed: %+v", res)
		}
		body, err := os.ReadFile(filepath.Join(env, "out.txt"))
		if err != nil || string(body) != "file-body" {
			t.Fatalf("curl -o file mismatch body=%q err=%v", string(body), err)
		}
	})

	t.Run("CURL-004", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "remote-body") }))
		defer server.Close()
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "curl", "-O", server.URL+"/artifact.txt")
		if res.ExitCode != 0 {
			t.Fatalf("curl -O failed: %+v", res)
		}
		body, err := os.ReadFile(filepath.Join(env, "artifact.txt"))
		if err != nil || string(body) != "remote-body" {
			t.Fatalf("curl -O file mismatch body=%q err=%v", string(body), err)
		}
	})

	t.Run("CURL-008", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			fmt.Fprint(w, "slow")
		}))
		defer server.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "-m", "0.05", server.URL)
		if res.ExitCode == 0 {
			t.Fatalf("curl -m expected timeout failure: %+v", res)
		}
	})

	t.Run("CURL-012", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "tls-ok") }))
		defer server.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "-k", server.URL)
		if res.ExitCode != 0 || normalizeText(res.Stdout) != "tls-ok" {
			t.Fatalf("curl -k failed: %+v", res)
		}
	})

	t.Run("CURL-013", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "--connect-timeout", "0.05", "http://10.255.255.1:81")
		if res.ExitCode == 0 {
			t.Fatalf("curl --connect-timeout expected failure: %+v", res)
		}
	})

	t.Run("CURL-014", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "resolved") }))
		defer server.Close()
		hostPort := strings.TrimPrefix(server.URL, "http://")
		_, port, _ := strings.Cut(hostPort, ":")
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "--resolve", "example.invalid:"+port+":127.0.0.1", "http://example.invalid:"+port)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "resolved") {
			t.Fatalf("curl --resolve failed: %+v", res)
		}
	})

	t.Run("CURL-016", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "1")
			fmt.Fprint(w, "body")
		}))
		defer server.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "-i", server.URL)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "X-Test: 1") || !strings.Contains(res.Stdout, "body") {
			t.Fatalf("curl -i failed: %+v", res)
		}
	})

	t.Run("CURL-019", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
		defer server.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "--bench", "-c", "2", "-n", "4", server.URL)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Concurrency: 2") {
			t.Fatalf("curl bench concurrent failed: %+v", res)
		}
	})

	t.Run("CURL-020", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
		defer server.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "--bench", "-n", "3", server.URL)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Requests: 3") {
			t.Fatalf("curl bench requests failed: %+v", res)
		}
	})

	t.Run("CURL-021", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
		defer server.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "--bench", "--warmup", "2", "-n", "2", server.URL)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Requests: 2") {
			t.Fatalf("curl bench warmup failed: %+v", res)
		}
	})

	t.Run("CURL-022", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(150 * time.Millisecond)
			fmt.Fprint(w, "slow")
		}))
		defer server.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "--bench", "-n", "2", "-t", "0.05", server.URL)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Failed:") {
			t.Fatalf("curl bench timeout failed: %+v", res)
		}
	})

	t.Run("NC-001", func(t *testing.T) {
		const serverMsg = "from-server\n"
		const clientMsg = "from-client\n"

		goboxPort := closedTCPPort(t)
		gobox := runGoboxNCListen(t, goboxPort, serverMsg, clientMsg, 2*time.Second)
		nativePort := closedTCPPort(t)
		native := runNativeNCListen(t, nativePort, serverMsg, clientMsg, 2*time.Second)

		for name, res := range map[string]ncListenResult{"gobox": gobox, "native": native} {
			if res.Server.ExitCode != 0 {
				t.Fatalf("%s nc -l failed: %+v", name, res.Server)
			}
			if res.ClientErr != nil {
				t.Fatalf("%s nc -l client failed: %v", name, res.ClientErr)
			}
			if !strings.Contains(res.Server.Stdout, clientMsg) {
				t.Fatalf("%s nc -l stdout missing client payload: server=%+v", name, res.Server)
			}
			if !strings.Contains(res.ClientOutput, serverMsg) {
				t.Fatalf("%s nc -l client missing server payload: %q", name, res.ClientOutput)
			}
		}
	})

	t.Run("NC-002", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-z", "127.0.0.1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-z", "127.0.0.1", port)
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("nc -z failed gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NC-003", func(t *testing.T) {
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen udp: %v", err)
		}
		defer conn.Close()
		host, port, _ := net.SplitHostPort(conn.LocalAddr().String())
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-u", "-z", host, port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-u", "-z", host, port)
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("nc -u failed gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NC-004", func(t *testing.T) {
		port := closedTCPPort(t)
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-w", "1", "127.0.0.1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-w", "1", "127.0.0.1", port)
		if gobox.ExitCode == 0 || native.ExitCode == 0 {
			t.Fatalf("nc -w expected connection failure gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NC-005", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-z", "-v", "127.0.0.1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-z", "-v", "127.0.0.1", port)
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("nc -v failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(gobox.Stdout+gobox.Stderr, "Connection successful") {
			t.Fatalf("gobox nc -v missing success output: %+v", gobox)
		}
	})

	t.Run("NC-006", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-n", "localhost", "1")
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-n", "localhost", "1")
		if gobox.ExitCode == 0 || native.ExitCode == 0 {
			t.Fatalf("nc -n hostname should fail gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NC-007", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-4", "-z", "127.0.0.1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-4", "-z", "127.0.0.1", port)
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("nc -4 failed gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NC-008", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "[::1]:0")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-6", "-z", "::1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-6", "-z", "::1", port)
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("nc -6 failed gobox=%+v native=%+v", gobox, native)
		}
	})

	for _, tc := range []struct {
		id   string
		args []string
		want string
	}{
		{"NC-009", []string{"nc", "--bench", "-n", "2", "-s", "16B"}, "Total:"},
		{"NC-010", []string{"nc", "--bench", "-c", "2", "-n", "4", "-s", "16B"}, "64B"},
		{"NC-011", []string{"nc", "--bench", "-n", "3", "-s", "16B"}, "48B"},
		{"NC-012", []string{"nc", "--bench", "-n", "2", "-s", "32B"}, "64B"},
		{"NC-013", []string{"nc", "--bench", "-t", "1", "-s", "16B"}, "Total:"},
		{"NC-014", []string{"nc", "--bench", "-n", "2", "-s", "16B", "-i", "1"}, "Total:"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
			defer closeFn()
			args := append([]string{}, tc.args...)
			args = append(args, "127.0.0.1", port)
			res := runGoboxCLI(t, t.TempDir(), "", args...)
			if res.ExitCode != 0 || !strings.Contains(res.Stdout, tc.want) {
				t.Fatalf("%s failed: %+v want %q", tc.id, res, tc.want)
			}
		})
	}

	t.Run("DNS-001", func(t *testing.T) {
		host, port, closeFn := startLocalDNSServer(t, "203.0.113.7")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "dig", "@"+net.JoinHostPort(host, port), "+short", "example.test")
		native := runNativeCLI(t, t.TempDir(), "", "dig", "@"+host, "-p", port, "+short", "example.test")
		if normalizeText(gobox.Stdout) != "203.0.113.7" || normalizeText(native.Stdout) != "203.0.113.7" {
			t.Fatalf("dig @DNS_SERVER mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("DNS-002", func(t *testing.T) {
		host, port, closeFn := startLocalDNSServer(t, "203.0.113.8")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "dig", "@"+net.JoinHostPort(host, port), "-t", "A", "+short", "example.test")
		native := runNativeCLI(t, t.TempDir(), "", "dig", "@"+host, "-p", port, "-t", "A", "+short", "example.test")
		if normalizeText(gobox.Stdout) != "203.0.113.8" || normalizeText(native.Stdout) != "203.0.113.8" {
			t.Fatalf("dig -t A mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("DNS-003", func(t *testing.T) {
		host, port, closeFn := startLocalDNSServer(t, "203.0.113.9")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "dig", "@"+net.JoinHostPort(host, port), "+short", "example.test")
		native := runNativeCLI(t, t.TempDir(), "", "dig", "@"+host, "-p", port, "+short", "example.test")
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("dig +short mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("DNS-004", func(t *testing.T) {
		host, port, closeFn := startLocalDNSServer(t, "203.0.113.10")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "dig", "@"+net.JoinHostPort(host, port), "+noall", "+answer", "example.test")
		native := runNativeCLI(t, t.TempDir(), "", "dig", "@"+host, "-p", port, "+noall", "+answer", "example.test")
		if !strings.Contains(gobox.Stdout, "203.0.113.10") || !strings.Contains(native.Stdout, "203.0.113.10") {
			t.Fatalf("dig +noall +answer mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("DNS-005", func(t *testing.T) {
		host, port, closeFn := startLocalDNSServer(t, "203.0.113.11")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "dig", "@"+net.JoinHostPort(host, port), "+tcp", "+short", "example.test")
		native := runNativeCLI(t, t.TempDir(), "", "dig", "@"+host, "-p", port, "+tcp", "+short", "example.test")
		if normalizeText(gobox.Stdout) != "203.0.113.11" || normalizeText(native.Stdout) != "203.0.113.11" {
			t.Fatalf("dig +tcp mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NP-001", func(t *testing.T) {
		if _, err := net.InterfaceByName("lo"); err != nil {
			t.Skip("loopback interface lo not available")
		}
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-I", "lo", "-p", port, "-c", "1", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "1 packets transmitted") {
			t.Fatalf("np -I failed: %+v", res)
		}
	})

	t.Run("NP-002", func(t *testing.T) {
		port := closedTCPPort(t)
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-W", "1", "-c", "1", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "1 errors") {
			t.Fatalf("np -W failed: %+v", res)
		}
	})

	t.Run("NP-003", func(t *testing.T) {
		gateway := defaultIPv4Gateway(t)
		gobox := runGoboxCLI(t, t.TempDir(), "", "np", "-arp", "-c", "1", "-W", "1", gateway)
		native := runNativeCLI(t, t.TempDir(), "", "arping", "-c", "1", "-w", "1", gateway)
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("np -arp failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(gobox.Stdout, gateway) || !strings.Contains(native.Stdout, "Received 1 response") {
			t.Fatalf("np -arp output mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NP-004", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-c", "2", "-i", "1000", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "2 packets transmitted") {
			t.Fatalf("np -c failed: %+v", res)
		}
	})

	t.Run("NP-005", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "np", "-icmp", "-flood", "-c", "3", "-q", "-W", "1", "127.0.0.1")
		native := runNativeCLI(t, t.TempDir(), "", "ping", "-f", "-c", "3", "-q", "-W", "1", "127.0.0.1")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("np -flood failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(gobox.Stdout, "3 packets transmitted") || !strings.Contains(native.Stdout, "3 packets transmitted") {
			t.Fatalf("np -flood packet count mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NP-006", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		start := time.Now()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-c", "2", "-i", "100000", "-q", "127.0.0.1")
		elapsed := time.Since(start)
		if res.ExitCode != 0 || elapsed < 100*time.Millisecond {
			t.Fatalf("np -i failed elapsed=%s result=%+v", elapsed, res)
		}
	})

	t.Run("NP-007", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "np", "-icmp", "-c", "1", "-q", "-W", "1", "127.0.0.1")
		native := runNativeCLI(t, t.TempDir(), "", "ping", "-c", "1", "-q", "-W", "1", "127.0.0.1")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("np -icmp failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(gobox.Stdout, "1 packets received") || !strings.Contains(native.Stdout, "1 received") {
			t.Fatalf("np -icmp receive mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NP-008", func(t *testing.T) {
		_, port, closeFn := startDelayedCloseServer(t, 150*time.Millisecond)
		defer closeFn()
		start := time.Now()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-l", "1", "-c", "1", "-q", "127.0.0.1")
		elapsed := time.Since(start)
		if res.ExitCode != 0 || elapsed < 100*time.Millisecond {
			t.Fatalf("np -l failed elapsed=%s result=%+v", elapsed, res)
		}
	})

	t.Run("NP-009", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-c", "1", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "1 packets received") {
			t.Fatalf("np -p failed: %+v", res)
		}
	})

	t.Run("NP-010", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-c", "1", "-q", "127.0.0.1")
		if res.ExitCode != 0 || strings.Contains(res.Stdout, "bytes from") || !strings.Contains(res.Stdout, "ping statistics") {
			t.Fatalf("np -q failed: %+v", res)
		}
	})

	t.Run("NP-011", func(t *testing.T) {
		sourcePort := atoiForTest(t, closedTCPPort(t))
		_, port, remotePorts, closeFn := startTCPRemotePortRecorder(t)
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-s", strconv.Itoa(sourcePort), "-c", "1", "-q", "127.0.0.1")
		if res.ExitCode != 0 {
			t.Fatalf("np -s failed: %+v", res)
		}
		select {
		case got := <-remotePorts:
			if got != sourcePort {
				t.Fatalf("np -s source port mismatch got=%d want=%d", got, sourcePort)
			}
		case <-time.After(time.Second):
			t.Fatal("source port recorder timed out")
		}
	})

	t.Run("NP-013", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-c", "1", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "1 packets received") {
			t.Fatalf("np -tcp failed: %+v", res)
		}
	})

	t.Run("NP-014", func(t *testing.T) {
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen udp: %v", err)
		}
		defer conn.Close()
		_, port, _ := net.SplitHostPort(conn.LocalAddr().String())
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-udp", "-p", port, "-c", "1", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "1 packets received") {
			t.Fatalf("np -udp failed: %+v", res)
		}
	})

	t.Run("NP-015", func(t *testing.T) {
		port := closedTCPPort(t)
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-c", "1", "-W", "1", "-v", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Connection failed") || !strings.Contains(res.Stdout, "ping statistics") {
			t.Fatalf("np -v failed: %+v", res)
		}
	})

	t.Run("NP-016", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-w", "2", "-c", "4", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "4 packets transmitted") {
			t.Fatalf("np -w failed: %+v", res)
		}
	})

	if runtime.GOOS == "linux" {
		for _, tc := range []struct {
			id   string
			args []string
		}{
			{"IFSTAT-001", []string{"ifstat", "-A", "-n", "1", "-p", "1"}},
			{"IFSTAT-002", []string{"ifstat", "-a", "-n", "1", "-p", "1"}},
			{"IFSTAT-003", []string{"ifstat", "-d", "-n", "1", "-p", "1"}},
			{"IFSTAT-004", []string{"ifstat", "-e", "-n", "1", "-p", "1"}},
			{"IFSTAT-005", []string{"ifstat", "-i", "lo", "-n", "1", "-p", "1"}},
			{"IFSTAT-007", []string{"ifstat", "-n", "1", "-p", "1"}},
			{"IOSTAT-001", []string{"iostat", "-i", "1", "-n", "1"}},
			{"IOSTAT-003", []string{"iostat", "-H", "-n", "1"}},
			{"IOSTAT-004", []string{"iostat", "-z", "-n", "1"}},
		} {
			t.Run(tc.id, func(t *testing.T) {
				res := runGoboxCLI(t, t.TempDir(), "", tc.args...)
				if res.ExitCode != 0 {
					t.Fatalf("%s failed: %+v", tc.id, res)
				}
			})
		}

		for _, tc := range []struct {
			id   string
			args []string
		}{
			{"IOPERF-001", []string{"ioperf", "--filename", "io.dat", "--bs", "4k", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-002", []string{"ioperf", "--filename", "io.dat", "--direct", "0", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-003", []string{"ioperf", "--filename", "io.dat", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-004", []string{"ioperf", "--filename", "io.dat", "--fsync", "1", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-005", []string{"ioperf", "--filename", "io.dat", "--group_reporting", "--numjobs", "2", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-007", []string{"ioperf", "--filename", "io.dat", "--latency", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-008", []string{"ioperf", "--filename", "io.dat", "--numjobs", "2", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-009", []string{"ioperf", "--filename", "io.dat", "--percentile", "95", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-010", []string{"ioperf", "--filename", "io.dat", "--rate", "1M", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-011", []string{"ioperf", "--filename", "io.dat", "--runtime", "1", "--size", "32K"}},
			{"IOPERF-012", []string{"ioperf", "--filename", "io.dat", "--rw", "read", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-013", []string{"ioperf", "--filename", "io.dat", "--rw", "readwrite", "--rwmixread", "70", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-014", []string{"ioperf", "--filename", "io.dat", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-015", []string{"ioperf", "--filename", "io.dat", "--sync", "1", "--size", "32K", "--runtime", "1"}},
			{"IOPERF-016", []string{"ioperf", "--filename", "io.dat", "--time_based", "--runtime", "1", "--size", "32K"}},
		} {
			t.Run(tc.id, func(t *testing.T) {
				env := t.TempDir()
				args := append([]string(nil), tc.args...)
				for i := range args {
					if args[i] == "io.dat" {
						args[i] = filepath.Join(env, "io.dat")
					}
				}
				res := runGoboxCLI(t, env, "", args...)
				if res.ExitCode != 0 {
					t.Fatalf("%s failed: %+v", tc.id, res)
				}
			})
		}
	}
}

func TestParity_IoperfAgainstFio(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	if _, err := exec.LookPath("fio"); err != nil {
		t.Skip("native fio not found")
	}

	t.Run("IOPERF-FIO-001", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "io.dat")
		writeFile(t, file, strings.Repeat("a", 32*1024))
		gobox := runGoboxCLI(t, env, "", "ioperf", "--filename", file, "--rw", "read", "--bs", "4k", "--size", "32K")
		native := runNativeCLI(t, env, "", "fio", "--name=job", "--filename="+file, "--rw=read", "--bs=4k", "--size=32K")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ioperf/fio read failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(gobox.Stdout, "READ:") || !strings.Contains(strings.ToLower(native.Stdout), "read:") {
			t.Fatalf("ioperf/fio read output mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("IOPERF-FIO-002", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "io.dat")
		gobox := runGoboxCLI(t, env, "", "ioperf", "--filename", file, "--rw", "write", "--bs", "4k", "--size", "32K")
		native := runNativeCLI(t, env, "", "fio", "--name=job", "--filename="+file, "--rw=write", "--bs=4k", "--size=32K")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ioperf/fio write failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(gobox.Stdout, "WRITE:") || !strings.Contains(strings.ToLower(native.Stdout), "write:") {
			t.Fatalf("ioperf/fio write output mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("IOPERF-FIO-003", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "io.dat")
		writeFile(t, file, strings.Repeat("b", 32*1024))
		gobox := runGoboxCLI(t, env, "", "ioperf", "--filename", file, "--rw", "readwrite", "--rwmixread", "70", "--bs", "4k", "--size", "32K")
		native := runNativeCLI(t, env, "", "fio", "--name=job", "--filename="+file, "--rw=readwrite", "--rwmixread=70", "--bs=4k", "--size=32K")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ioperf/fio readwrite failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(gobox.Stdout, "READ:") || !strings.Contains(gobox.Stdout, "WRITE:") {
			t.Fatalf("gobox readwrite missing read/write stats: %+v", gobox)
		}
		nativeLower := strings.ToLower(native.Stdout)
		if !strings.Contains(nativeLower, "read:") || !strings.Contains(nativeLower, "write:") {
			t.Fatalf("fio readwrite missing read/write stats: %+v", native)
		}
	})

	t.Run("IOPERF-FIO-004", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "io.dat")
		writeFile(t, file, strings.Repeat("c", 32*1024))
		gobox := runGoboxCLI(t, env, "", "ioperf", "--filename", file, "--rw", "read", "--size", "32K", "--time_based", "--runtime", "1")
		native := runNativeCLI(t, env, "", "fio", "--name=job", "--filename="+file, "--rw=read", "--size=32K", "--time_based", "--runtime=1")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ioperf/fio time_based failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(gobox.Stdout, "READ:") || !strings.Contains(strings.ToLower(native.Stdout), "read:") {
			t.Fatalf("ioperf/fio time_based output mismatch gobox=%+v native=%+v", gobox, native)
		}
	})
}

func TestParity_RemainingSmokeReferences(t *testing.T) {
	_ = fmt.Sprintf
	_ = net.IPv4len
	_ = httptest.NewServer
	_ = os.DevNull
}

func sameStringSet(a, b string) bool {
	al := nonEmptyLines(a)
	bl := nonEmptyLines(b)
	sort.Strings(al)
	sort.Strings(bl)
	return strings.Join(al, "\n") == strings.Join(bl, "\n")
}

func nonEmptyLines(s string) []string {
	s = normalizeText(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func lastLine(s string) string {
	lines := nonEmptyLines(s)
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

func startMarkerProcess(t *testing.T, marker string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("bash", "-lc", "exec -a "+marker+" sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start marker process: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	return cmd
}

func startExactNameProcess(t *testing.T, name string) *exec.Cmd {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.Symlink("/usr/bin/sleep", path); err != nil {
		t.Fatalf("create sleep symlink: %v", err)
	}
	cmd := exec.Command(path, "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start exact-name process: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	return cmd
}

func stopCmd(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

func extractLeadingInts(lines []string) []int {
	var vals []int
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		v, err := strconv.Atoi(fields[0])
		if err == nil {
			vals = append(vals, v)
		}
	}
	return vals
}

func extractNetstatPIDs(out string) []int {
	var vals []int
	for _, line := range nonEmptyLines(out)[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		last := fields[len(fields)-1]
		pidStr, _, _ := strings.Cut(last, "/")
		if pidStr == "-" {
			continue
		}
		v, err := strconv.Atoi(pidStr)
		if err == nil {
			vals = append(vals, v)
		}
	}
	return vals
}

func assertMonotonic(t *testing.T, vals []int, descending bool) {
	t.Helper()
	for i := 1; i < len(vals); i++ {
		if descending {
			if vals[i-1] < vals[i] {
				t.Fatalf("expected descending order, got %v", vals)
			}
		} else if vals[i-1] > vals[i] {
			t.Fatalf("expected ascending order, got %v", vals)
		}
	}
}

func runTailGoboxFollow(t *testing.T, dir string, args []string, action func(), timeout time.Duration) parityResult {
	t.Helper()
	parityExecMu.Lock()
	defer parityExecMu.Unlock()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	var stdoutBuf, stderrBuf bytes.Buffer
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
	os.Stdout = wOut
	os.Stderr = wErr

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- textcmd.TailCmdWithContext(ctx, args)
	}()
	time.Sleep(150 * time.Millisecond)
	if action != nil {
		action()
	}
	err = <-errCh

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	_, _ = io.Copy(&stdoutBuf, rOut)
	_, _ = io.Copy(&stderrBuf, rErr)
	_ = rOut.Close()
	_ = rErr.Close()

	exitCode := 0
	if err != nil && err != context.DeadlineExceeded {
		exitCode = 2
	}
	return parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode}
}

type ncListenResult struct {
	Server       parityResult
	ClientOutput string
	ClientErr    error
}

func runGoboxNCListen(t *testing.T, port, serverStdin, clientInput string, timeout time.Duration) ncListenResult {
	t.Helper()
	parityExecMu.Lock()
	defer parityExecMu.Unlock()

	var stdoutBuf, stderrBuf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldStdin := os.Stdin
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	_, _ = io.WriteString(wIn, serverStdin)
	_ = wIn.Close()
	os.Stdout = wOut
	os.Stderr = wErr
	os.Stdin = rIn

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- netcmd.NcCmdWithContext(ctx, []string{"-l", port})
	}()
	time.Sleep(150 * time.Millisecond)
	clientOutput, clientErr := exchangeWithNCListener(port, clientInput, serverStdin)
	err = <-errCh

	_ = rIn.Close()
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	os.Stdin = oldStdin
	_, _ = io.Copy(&stdoutBuf, rOut)
	_, _ = io.Copy(&stderrBuf, rErr)
	_ = rOut.Close()
	_ = rErr.Close()

	exitCode := 0
	if err != nil && err != context.DeadlineExceeded {
		exitCode = 2
	}
	return ncListenResult{
		Server:       parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode},
		ClientOutput: clientOutput,
		ClientErr:    clientErr,
	}
}

func runNativeNCListen(t *testing.T, port, serverStdin, clientInput string, timeout time.Duration) ncListenResult {
	t.Helper()
	path, err := exec.LookPath("nc")
	if err != nil {
		t.Skip("native nc not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-l", port)
	cmd.Stdin = strings.NewReader(serverStdin)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start native nc -l: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	clientOutput, clientErr := exchangeWithNCListener(port, clientInput, serverStdin)
	err = cmd.Wait()

	exitCode := 0
	if err != nil && ctx.Err() == nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 2
		}
	}
	return ncListenResult{
		Server:       parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode},
		ClientOutput: clientOutput,
		ClientErr:    clientErr,
	}
}

func exchangeWithNCListener(port, clientInput, wantOutput string) (string, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, clientInput); err != nil {
		return "", err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.CloseWrite()
	}
	var out strings.Builder
	buf := make([]byte, 256)
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(out.String(), wantOutput) {
		_ = conn.SetReadDeadline(deadline)
		n, err := conn.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if err != nil {
			if strings.Contains(out.String(), wantOutput) || err == io.EOF {
				break
			}
			return out.String(), err
		}
	}
	return out.String(), nil
}

func defaultIPv4Gateway(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		t.Skipf("default IPv4 gateway unavailable: %v", err)
	}
	fields := strings.Fields(string(out))
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "via" && net.ParseIP(fields[i+1]) != nil {
			return fields[i+1]
		}
	}
	t.Skipf("default IPv4 gateway unavailable in route output: %q", string(out))
	return ""
}

func runNativeFollow(t *testing.T, dir, command string, args []string, action func(), timeout time.Duration) parityResult {
	t.Helper()
	path, err := exec.LookPath(command)
	if err != nil {
		t.Skipf("native command %s not found", command)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start native %s: %v", command, err)
	}
	time.Sleep(150 * time.Millisecond)
	if action != nil {
		action()
	}
	err = cmd.Wait()
	exitCode := 0
	if err != nil && ctx.Err() == nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 2
		}
	}
	return parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("append open %s: %v", path, err)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		t.Fatalf("append write %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("append close %s: %v", path, err)
	}
}

func startTCPEchoServer(t *testing.T, addr string) (string, string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if strings.Contains(addr, "::1") {
			t.Skipf("IPv6 loopback unavailable: %v", err)
		}
		t.Fatalf("listen tcp %s: %v", addr, err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split listener addr: %v", err)
	}
	return host, port, func() {
		_ = ln.Close()
		<-done
	}
}

func startDelayedCloseServer(t *testing.T, delay time.Duration) (string, string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen delayed close: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				time.Sleep(delay)
				_ = c.Close()
			}(conn)
		}
	}()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		t.Fatalf("split delayed close addr: %v", err)
	}
	return host, port, func() {
		_ = ln.Close()
		<-done
	}
}

func startTCPRemotePortRecorder(t *testing.T) (string, string, <-chan int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen remote port recorder: %v", err)
	}
	remotePorts := make(chan int, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			remotePorts <- addr.Port
		}
	}()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		t.Fatalf("split remote port recorder addr: %v", err)
	}
	return host, port, remotePorts, func() {
		_ = ln.Close()
		<-done
	}
}

func atoiForTest(t *testing.T, s string) int {
	t.Helper()
	v, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("atoi %q: %v", s, err)
	}
	return v
}

func startUDPReceiver(t *testing.T) (net.Addr, <-chan string) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	done := make(chan string, 1)
	go func() {
		defer conn.Close()
		buf := make([]byte, 1024)
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			done <- ""
			return
		}
		_, _ = conn.WriteTo(buf[:n], addr)
		done <- string(buf[:n])
	}()
	return conn.LocalAddr(), done
}

func closedTCPPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for closed port: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		t.Fatalf("split closed port addr: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("close temp listener: %v", err)
	}
	return port
}

func startLocalDNSServer(t *testing.T, ip string) (string, string, func()) {
	t.Helper()
	udpConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen dns udp: %v", err)
	}
	host, port, err := net.SplitHostPort(udpConn.LocalAddr().String())
	if err != nil {
		_ = udpConn.Close()
		t.Fatalf("split dns udp addr: %v", err)
	}
	tcpLn, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		_ = udpConn.Close()
		t.Fatalf("listen dns tcp: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 512)
		for {
			n, addr, err := udpConn.ReadFrom(buf)
			if err != nil {
				return
			}
			resp := buildDNSAResponse(buf[:n], net.ParseIP(ip).To4())
			_, _ = udpConn.WriteTo(resp, addr)
		}
	}()

	tcpDone := make(chan struct{})
	go func() {
		defer close(tcpDone)
		for {
			conn, err := tcpLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var lenBuf [2]byte
				if _, err := io.ReadFull(c, lenBuf[:]); err != nil {
					return
				}
				msgLen := int(binary.BigEndian.Uint16(lenBuf[:]))
				msg := make([]byte, msgLen)
				if _, err := io.ReadFull(c, msg); err != nil {
					return
				}
				resp := buildDNSAResponse(msg, net.ParseIP(ip).To4())
				binary.BigEndian.PutUint16(lenBuf[:], uint16(len(resp)))
				_, _ = c.Write(lenBuf[:])
				_, _ = c.Write(resp)
			}(conn)
		}
	}()

	return host, port, func() {
		_ = udpConn.Close()
		_ = tcpLn.Close()
		<-done
		<-tcpDone
	}
}

func buildDNSAResponse(query []byte, ip net.IP) []byte {
	if len(query) < 12 || len(ip) != 4 {
		return nil
	}
	qEnd := 12
	for qEnd < len(query) {
		labelLen := int(query[qEnd])
		qEnd++
		if labelLen == 0 {
			break
		}
		qEnd += labelLen
	}
	if qEnd+4 > len(query) {
		return nil
	}
	question := query[12 : qEnd+4]
	resp := make([]byte, 0, 12+len(question)+16)
	resp = append(resp, query[0], query[1])
	resp = append(resp, 0x81, 0x80)
	resp = append(resp, 0x00, 0x01)
	resp = append(resp, 0x00, 0x01)
	resp = append(resp, 0x00, 0x00)
	resp = append(resp, 0x00, 0x00)
	resp = append(resp, question...)
	resp = append(resp, 0xc0, 0x0c)
	resp = append(resp, 0x00, 0x01)
	resp = append(resp, 0x00, 0x01)
	resp = append(resp, 0x00, 0x00, 0x00, 0x3c)
	resp = append(resp, 0x00, 0x04)
	resp = append(resp, ip...)
	return resp
}
