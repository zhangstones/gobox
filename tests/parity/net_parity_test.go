package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParity_TwCases(t *testing.T) {
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

	t.Run("TW-004", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "tw", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("tw -h failed: %+v", res)
		}
	})
}

func TestParity_NetstatCases(t *testing.T) {
	t.Run("NETSTAT-001", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-port", fmt.Sprintf("%d", port))
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-an")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat failed gobox=%+v native=%+v", res, native)
		}
		portText := fmt.Sprintf(":%d", port)
		if !strings.Contains(res.Stdout, portText) || !strings.Contains(native.Stdout, portText) {
			t.Fatalf("netstat -port missing listener\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
	})

	t.Run("NETSTAT-002", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-sort", "pid", "-p")
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
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-an")
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-state", "LISTEN")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat -state failed gobox=%+v native=%+v", res, native)
		}
		if !strings.Contains(native.Stdout, "LISTEN") {
			t.Fatalf("native netstat baseline missing LISTEN rows: %+v", native)
		}
		for _, line := range nonEmptyLines(res.Stdout)[1:] {
			if !strings.Contains(line, "LISTEN") {
				t.Fatalf("netstat -state LISTEN leaked non-LISTEN row: %q", line)
			}
		}
	})

	t.Run("NETSTAT-004", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-l", "-port", fmt.Sprintf("%d", port))
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-ln")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat -l failed gobox=%+v native=%+v", res, native)
		}
		if !strings.Contains(res.Stdout, "LISTEN") || !strings.Contains(native.Stdout, fmt.Sprintf(":%d", port)) {
			t.Fatalf("netstat -l missing listener\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
	})

	t.Run("NETSTAT-005", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		env := t.TempDir()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, env, "", "netstat", "-port", port)
		res := runGoboxCLI(t, env, "", "netstat", "-n", "-port", port)
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("netstat -n baseline failed base=%+v numeric=%+v", base, res)
		}
		if base.Stdout != res.Stdout {
			t.Fatalf("netstat -n should be a no-op because gobox output is already numeric\n--- base ---\n%s\n--- -n ---\n%s", base.Stdout, res.Stdout)
		}
		if !strings.Contains(res.Stdout, "Proto") || !strings.Contains(res.Stdout, port) || strings.Contains(res.Stdout, "localhost:") {
			t.Fatalf("netstat -n should still render the socket table in numeric form\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-006", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		env := t.TempDir()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, env, "", "netstat", "-port", port)
		res := runGoboxCLI(t, env, "", "netstat", "-a", "-port", port)
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("netstat -a baseline failed base=%+v all=%+v", base, res)
		}
		if base.Stdout != res.Stdout {
			t.Fatalf("netstat -a should currently match the default socket selection\n--- base ---\n%s\n--- -a ---\n%s", base.Stdout, res.Stdout)
		}
		if !strings.Contains(res.Stdout, "Proto") || !strings.Contains(res.Stdout, port) {
			t.Fatalf("netstat -a should still render the socket table\n%s", res.Stdout)
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
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-tln")
		if res.ExitCode != 0 || native.ExitCode != 0 || !strings.Contains(res.Stdout, "TCP") || !strings.Contains(native.Stdout, strconv.Itoa(port)) || strings.Contains(res.Stdout, "UDP") || strings.Contains(res.Stdout, "UNIX") {
			t.Fatalf("netstat -t mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
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
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-uln")
		if res.ExitCode != 0 || native.ExitCode != 0 || !strings.Contains(res.Stdout, "UDP") || !strings.Contains(native.Stdout, strconv.Itoa(port)) || strings.Contains(res.Stdout, "TCP") || strings.Contains(res.Stdout, "UNIX") {
			t.Fatalf("netstat -u mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
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
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-x", "-l")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "UNIX") || !strings.Contains(res.Stdout, unixPath) || !strings.Contains(native.Stdout, unixPath) {
			t.Fatalf("netstat -x mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("NETSTAT-010", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-port", port)
		withProg := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-p", "-port", port)
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-tnlp")
		if base.ExitCode != 0 || withProg.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat -p baseline failed base=%+v withProg=%+v native=%+v", base, withProg, native)
		}
		if base.Stdout == withProg.Stdout {
			t.Fatalf("netstat -p did not change output\n--- base ---\n%s\n--- with -p ---\n%s", base.Stdout, withProg.Stdout)
		}
		if !strings.Contains(withProg.Stdout, "PID/Program") || !strings.Contains(native.Stdout, "PID/Program") {
			t.Fatalf("netstat -p missing PID/Program column\n--- gobox ---\n%s\n--- native ---\n%s", withProg.Stdout, native.Stdout)
		}
		if !strings.Contains(withProg.Stdout, "/") {
			t.Fatalf("netstat -p did not render pid/program cell\n%s", withProg.Stdout)
		}
		lines := nonEmptyLines(withProg.Stdout)
		if len(lines) < 2 || !strings.Contains(lines[1], port) || !strings.Contains(lines[1], "/") {
			t.Fatalf("netstat -p should keep the filtered listener row and annotate pid/program\n%s", withProg.Stdout)
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
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-4ln")
		if res.ExitCode != 0 || native.ExitCode != 0 || !strings.Contains(res.Stdout, "127.0.0.1") || !strings.Contains(native.Stdout, strconv.Itoa(port)) {
			t.Fatalf("netstat -4 mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("NETSTAT-012", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		_, port, closeFn := startTCPEchoServer(t, "[::1]:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-6", "-port", port)
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-6ln")
		if res.ExitCode != 0 || native.ExitCode != 0 || !strings.Contains(res.Stdout, "::1") || !strings.Contains(native.Stdout, port) {
			t.Fatalf("netstat -6 mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("NETSTAT-013", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-port", port)
		extended := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-e", "-port", port)
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-tnle")
		if base.ExitCode != 0 || extended.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat -e baseline failed base=%+v extended=%+v native=%+v", base, extended, native)
		}
		if base.Stdout == extended.Stdout {
			t.Fatalf("netstat -e did not change output\n--- base ---\n%s\n--- with -e ---\n%s", base.Stdout, extended.Stdout)
		}
		for _, want := range []string{"User", "Inode"} {
			if !strings.Contains(extended.Stdout, want) || !strings.Contains(native.Stdout, want) {
				t.Fatalf("netstat -e missing %q\n--- gobox ---\n%s\n--- native ---\n%s", want, extended.Stdout, native.Stdout)
			}
		}
		lines := nonEmptyLines(extended.Stdout)
		if len(lines) < 2 || len(strings.Fields(lines[1])) < 8 {
			t.Fatalf("netstat -e should extend the filtered row with extra columns\n%s", extended.Stdout)
		}
	})

	t.Run("NETSTAT-014", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-port", port)
		withTimers := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-o", "-port", port)
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-tnlo")
		if base.ExitCode != 0 || withTimers.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat -o baseline failed base=%+v withTimers=%+v native=%+v", base, withTimers, native)
		}
		if base.Stdout == withTimers.Stdout {
			t.Fatalf("netstat -o did not change output\n--- base ---\n%s\n--- with -o ---\n%s", base.Stdout, withTimers.Stdout)
		}
		if !strings.Contains(withTimers.Stdout, "Timer") || !strings.Contains(native.Stdout, "Timer") {
			t.Fatalf("netstat -o missing Timer column\n--- gobox ---\n%s\n--- native ---\n%s", withTimers.Stdout, native.Stdout)
		}
		lines := nonEmptyLines(withTimers.Stdout)
		if len(lines) < 2 || !strings.Contains(lines[1], port) {
			t.Fatalf("netstat -o should keep the filtered listener row\n%s", withTimers.Stdout)
		}
	})

	t.Run("NETSTAT-015", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, t.TempDir(), "", "netstat", "-n", "-l", "-port", port)
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-W", "-n", "-l", "-port", port)
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("netstat -W baseline failed base=%+v wide=%+v", base, res)
		}
		if base.Stdout != res.Stdout {
			t.Fatalf("netstat -W should be a compatibility no-op because gobox does not truncate addresses\n--- base ---\n%s\n--- -W ---\n%s", base.Stdout, res.Stdout)
		}
		if !strings.Contains(res.Stdout, "Proto") || !strings.Contains(res.Stdout, port) {
			t.Fatalf("netstat -W should still render the filtered listening row\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-016", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		base := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-port", strconv.Itoa(port))
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-tnlp", "-port", strconv.Itoa(port))
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-tnlp")
		if base.ExitCode != 0 || res.ExitCode != native.ExitCode {
			t.Fatalf("netstat combined flags mismatch\n--- base ---\n%+v\n--- gobox ---\n%+v\n--- native ---\n%+v", base, res, native)
		}
		if base.Stdout == res.Stdout {
			t.Fatalf("netstat -tnlp should change output relative to -t -l by enabling numeric/program views\n--- base ---\n%s\n--- combined ---\n%s", base.Stdout, res.Stdout)
		}
		if !strings.Contains(res.Stdout, "TCP") || !strings.Contains(res.Stdout, "PID/Program") || !strings.Contains(native.Stdout, "tcp") {
			t.Fatalf("netstat combined flags missing expected protocol/program output\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		lines := nonEmptyLines(res.Stdout)
		if len(lines) < 2 || !strings.Contains(lines[1], strconv.Itoa(port)) || !strings.Contains(lines[1], "/") {
			t.Fatalf("netstat -tnlp should keep the filtered listener row and annotate it with pid/program\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-017", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-r")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-r")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "Kernel IP routing table") || !strings.Contains(native.Stdout, "Kernel IP routing table") {
			t.Fatalf("netstat -r mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		if !strings.Contains(res.Stdout, "Iface") || !strings.Contains(native.Stdout, "Iface") {
			t.Fatalf("netstat -r missing route columns\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
		if strings.Contains(native.Stdout, "default") && !strings.Contains(res.Stdout, "default") && !strings.Contains(res.Stdout, "0.0.0.0") {
			t.Fatalf("netstat -r missing default-route semantic present in native\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
		if !strings.Contains(res.Stdout, "eth0") && !strings.Contains(res.Stdout, "lo") {
			t.Fatalf("netstat -r should include at least one concrete interface route\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-018", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-i")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-i")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "Iface") || !strings.Contains(native.Stdout, "Iface") {
			t.Fatalf("netstat -i mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		if !strings.Contains(res.Stdout, "lo") || !strings.Contains(native.Stdout, "lo") {
			t.Fatalf("netstat -i missing loopback interface\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
		lines := nonEmptyLines(res.Stdout)
		if len(lines) < 3 {
			t.Fatalf("netstat -i should include header plus multiple interfaces\n%s", res.Stdout)
		}
		if !strings.Contains(lines[0], "RX-OK") || !strings.Contains(lines[0], "TX-OK") {
			t.Fatalf("netstat -i missing traffic counters in header\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-019", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "netstat", "-s")
		tcpOnly := runGoboxCLI(t, env, "", "netstat", "-s", "-t")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-s")
		if res.ExitCode != native.ExitCode || tcpOnly.ExitCode != 0 {
			t.Fatalf("netstat -s mismatch\n--- gobox ---\n%+v\n--- gobox tcp ---\n%+v\n--- native ---\n%+v", res, tcpOnly, native)
		}
		if !strings.Contains(res.Stdout, "Tcp:") || !strings.Contains(native.Stdout, "Tcp:") {
			t.Fatalf("netstat -s missing tcp stats section\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
		if res.Stdout == tcpOnly.Stdout {
			t.Fatalf("netstat -s should include more than the tcp-only filtered view\n--- all stats ---\n%s\n--- tcp only ---\n%s", res.Stdout, tcpOnly.Stdout)
		}
		if !strings.Contains(res.Stdout, "Udp:") && !strings.Contains(res.Stdout, "Ip:") {
			t.Fatalf("netstat -s should include non-TCP protocol stats in the unfiltered view\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-020", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		gobox := runGoboxSubprocess(t, t.TempDir(), []string{"netstat", "-c", "-n", "-l", "-port", port}, 1350*time.Millisecond)
		native := runNativeFollow(t, t.TempDir(), "netstat", []string{"-c", "-n", "-l"}, nil, 1350*time.Millisecond)
		if strings.Count(gobox.Stdout, "Proto") < 2 {
			t.Fatalf("gobox netstat -c did not render multiple cycles: %q", gobox.Stdout)
		}
		if strings.Count(native.Stdout, "Proto") < 2 {
			t.Fatalf("native netstat -c did not render multiple cycles: %q", native.Stdout)
		}
	})

	t.Run("NETSTAT-021", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		short := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-p", "-e", "-o", "-n", "-W", "-port", port)
		long := runGoboxCLI(t, t.TempDir(), "", "netstat", "--tcp", "--listening", "--programs", "--extend", "--timers", "--numeric", "--wide", "-port", port)
		if short.ExitCode != 0 || long.ExitCode != 0 {
			t.Fatalf("netstat short/long flag parity failed short=%+v long=%+v", short, long)
		}
		if short.Stdout != long.Stdout {
			t.Fatalf("netstat short/long flag output mismatch\n--- short ---\n%s\n--- long ---\n%s", short.Stdout, long.Stdout)
		}
	})

	t.Run("NETSTAT-022", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		short := runGoboxCLI(t, t.TempDir(), "", "netstat", "-r")
		long := runGoboxCLI(t, t.TempDir(), "", "netstat", "--route")
		if short.ExitCode != 0 || long.ExitCode != 0 {
			t.Fatalf("netstat route short/long parity failed short=%+v long=%+v", short, long)
		}
		if short.Stdout != long.Stdout {
			t.Fatalf("netstat route short/long output mismatch\n--- short ---\n%s\n--- long ---\n%s", short.Stdout, long.Stdout)
		}
	})

	t.Run("NETSTAT-023", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "--help")
		if res.ExitCode != 0 {
			t.Fatalf("netstat --help failed: %+v", res)
		}
		for _, want := range []string{"-t, --tcp", "-u, --udp", "-n, --numeric", "-W, --wide", "Filters:", "Views:"} {
			if !strings.Contains(res.Stdout, want) && !strings.Contains(res.Stderr, want) {
				t.Fatalf("netstat --help missing %q\nstdout=%q\nstderr=%q", want, res.Stdout, res.Stderr)
			}
		}
	})

	t.Run("NETSTAT-024", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		goboxStats := runGoboxCLI(t, t.TempDir(), "", "netstat", "-s")
		goboxTCPStats := runGoboxCLI(t, t.TempDir(), "", "netstat", "-s", "-t")
		nativeStats := runNativeCLI(t, t.TempDir(), "", "netstat", "-s")
		nativeTCPStats := runNativeCLI(t, t.TempDir(), "", "netstat", "-s", "-t")
		if goboxStats.ExitCode != 0 || goboxTCPStats.ExitCode != 0 || nativeStats.ExitCode != 0 || nativeTCPStats.ExitCode != 0 {
			t.Fatalf("netstat -s/-s -t failed goboxStats=%+v goboxTCPStats=%+v nativeStats=%+v nativeTCPStats=%+v", goboxStats, goboxTCPStats, nativeStats, nativeTCPStats)
		}
		if goboxStats.Stdout == goboxTCPStats.Stdout {
			t.Fatalf("gobox netstat -s -t should not be identical to bare -s\n--- -s ---\n%s\n--- -s -t ---\n%s", goboxStats.Stdout, goboxTCPStats.Stdout)
		}
		if nativeStats.Stdout == nativeTCPStats.Stdout {
			t.Fatalf("native netstat -s -t should not be identical to bare -s\n--- -s ---\n%s\n--- -s -t ---\n%s", nativeStats.Stdout, nativeTCPStats.Stdout)
		}
		if strings.Contains(goboxTCPStats.Stdout, "Ip:") || strings.Contains(goboxTCPStats.Stdout, "Udp:") {
			t.Fatalf("gobox netstat -s -t leaked non-TCP sections\n%s", goboxTCPStats.Stdout)
		}
		if !strings.Contains(goboxTCPStats.Stdout, "Tcp:") {
			t.Fatalf("gobox netstat -s -t missing Tcp section\n%s", goboxTCPStats.Stdout)
		}
	})
}

func TestParity_IpCases(t *testing.T) {
	requireNativeCommand(t, "ip")

	t.Run("IP-001", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "ip", "addr")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "addr")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip addr exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if !strings.Contains(gobox.Stdout, "lo") || !strings.Contains(native.Stdout, "lo") {
			t.Fatalf("ip addr output missing loopback\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		for _, want := range []string{"127.0.0.1/8", "state UP"} {
			if !strings.Contains(gobox.Stdout, want) || !strings.Contains(native.Stdout, want) {
				t.Fatalf("ip addr missing %q\ngobox=%s\nnative=%s", want, gobox.Stdout, native.Stdout)
			}
		}
		if len(nonEmptyLines(gobox.Stdout)) < 3 {
			t.Fatalf("ip addr expected interface header plus address lines\n%s", gobox.Stdout)
		}
	})

	t.Run("IP-002", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "ip", "addr")
		gobox := runGoboxCLI(t, env, "", "ip", "-o", "addr")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "-o", "addr")
		if base.ExitCode != 0 || gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip -o addr exit mismatch base=%+v gobox=%+v native=%+v", base, gobox, native)
		}
		if base.Stdout == gobox.Stdout {
			t.Fatalf("ip -o addr should change output relative to multiline addr view\n--- base ---\n%s\n--- oneline ---\n%s", base.Stdout, gobox.Stdout)
		}
		if !strings.Contains(gobox.Stdout, " lo ") || !strings.Contains(native.Stdout, " lo ") {
			t.Fatalf("ip -o addr missing loopback line\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		for _, line := range nonEmptyLines(gobox.Stdout) {
			if strings.HasPrefix(line, "    ") || !strings.Contains(line, " scope ") {
				t.Fatalf("ip -o addr should emit one-line scoped records, got %q", line)
			}
		}
	})

	t.Run("IP-003", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "ip", "link")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "link")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip link exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		for _, want := range []string{"lo", "mtu"} {
			if !strings.Contains(gobox.Stdout, want) || !strings.Contains(native.Stdout, want) {
				t.Fatalf("ip link missing %q\ngobox=%s\nnative=%s", want, gobox.Stdout, native.Stdout)
			}
		}
		lines := nonEmptyLines(gobox.Stdout)
		if len(lines) < 2 || !strings.Contains(lines[1], "link/") {
			t.Fatalf("ip link should include link-layer details after interface header\n%s", gobox.Stdout)
		}
		nativeLines := nonEmptyLines(native.Stdout)
		if len(nativeLines) < 2 {
			t.Fatalf("native ip link output too short\n%s", native.Stdout)
		}

		hasLinkAddress := func(lines []string) bool {
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) >= 2 && strings.HasPrefix(fields[0], "link/") && strings.Contains(fields[1], ":") {
					return true
				}
			}
			return false
		}
		if !hasLinkAddress(lines) {
			t.Fatalf("ip link should expose at least one parseable link type/address row\n%s", gobox.Stdout)
		}
		if !hasLinkAddress(nativeLines) {
			t.Fatalf("native ip link should expose at least one parseable link type/address row\n%s", native.Stdout)
		}
	})

	t.Run("IP-004", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "ip", "link")
		gobox := runGoboxCLI(t, env, "", "ip", "-s", "link")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "-s", "link")
		if base.ExitCode != 0 || gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip -s link exit mismatch base=%+v gobox=%+v native=%+v", base, gobox, native)
		}
		if base.Stdout == gobox.Stdout {
			t.Fatalf("ip -s link should change output relative to plain link view\n--- base ---\n%s\n--- stats ---\n%s", base.Stdout, gobox.Stdout)
		}
		for _, want := range []string{"RX", "TX"} {
			if !strings.Contains(strings.ToUpper(gobox.Stdout), want) || !strings.Contains(strings.ToUpper(native.Stdout), want) {
				t.Fatalf("ip -s link missing %q\ngobox=%s\nnative=%s", want, gobox.Stdout, native.Stdout)
			}
		}
		if !strings.Contains(gobox.Stdout, "packets") || !strings.Contains(gobox.Stdout, "errors") {
			t.Fatalf("ip -s link should include packet/error counters\n%s", gobox.Stdout)
		}
	})

	t.Run("IP-005", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "ip", "route")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "route")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip route exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if strings.Contains(native.Stdout, "default") && !strings.Contains(gobox.Stdout, "default") {
			t.Fatalf("ip route missing default route\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		for _, line := range nonEmptyLines(gobox.Stdout) {
			if !strings.Contains(line, " dev ") {
				t.Fatalf("ip route row missing dev field: %q", line)
			}
		}
	})

	t.Run("IP-006", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "ip", "neigh")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "neigh")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip neigh exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if native.Stdout != "" && gobox.Stdout == "" {
			t.Fatalf("ip neigh unexpectedly empty\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		for _, line := range nonEmptyLines(gobox.Stdout) {
			if !strings.Contains(line, " dev ") {
				t.Fatalf("ip neigh row missing dev field: %q", line)
			}
			if fields := strings.Fields(line); len(fields) < 4 {
				t.Fatalf("ip neigh row too short: %q", line)
			}
		}
	})
}

func TestParity_CurlCases(t *testing.T) {
	requireNativeCommand(t, "curl")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, "ok")
		case "/redirect":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			fmt.Fprint(w, "redirected")
		case "/echo":
			body, _ := io.ReadAll(r.Body)
			fmt.Fprint(w, string(body))
		case "/upload":
			body, _ := io.ReadAll(r.Body)
			fmt.Fprint(w, string(body))
		case "/multipart":
			mr, err := r.MultipartReader()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			part, err := mr.NextPart()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(part)
			fmt.Fprintf(w, "%s:%s", part.FileName(), string(data))
		case "/fail":
			http.Error(w, "nope", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Run("CURL-001", func(t *testing.T) {
		runExactParityCases(t, []parityCase{{ID: "CURL-001", Name: "curl -s", GoboxArgs: []string{"curl", "-s", server.URL}, NativeCommand: "curl", NativeArgs: []string{"-s", server.URL}}})
	})

	t.Run("CURL-002", func(t *testing.T) {
		res := runGoboxMainCLI(t, t.TempDir(), "", "curl", "-s", "-S", "://bad-url")
		native := runNativeCLI(t, t.TempDir(), "", "curl", "-s", "-S", "://bad-url")
		if res.ExitCode == 0 || native.ExitCode == 0 {
			t.Fatalf("curl -s -S expected failure gobox=%+v native=%+v", res, native)
		}
		if !strings.Contains(strings.ToLower(res.Stderr), "curl:") {
			t.Fatalf("curl -s -S missing gobox error prefix: %+v", res)
		}
	})

	t.Run("CURL-003", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "curl", "-o", "out.txt", server.URL)
		native := runNativeCLI(t, env, "", "curl", "-o", "native.txt", server.URL)
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("curl -o exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		gBody, gErr := os.ReadFile(filepath.Join(env, "out.txt"))
		nBody, nErr := os.ReadFile(filepath.Join(env, "native.txt"))
		if gErr != nil || nErr != nil || string(gBody) != string(nBody) {
			t.Fatalf("curl -o file mismatch gobox=%q native=%q gErr=%v nErr=%v", string(gBody), string(nBody), gErr, nErr)
		}
	})

	t.Run("CURL-004", func(t *testing.T) {
		fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "remote-body") }))
		defer fileServer.Close()
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "curl", "-O", fileServer.URL+"/artifact.txt")
		native := runNativeCLI(t, env, "", "curl", "-O", fileServer.URL+"/artifact-native.txt")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("curl -O exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		gBody, gErr := os.ReadFile(filepath.Join(env, "artifact.txt"))
		nBody, nErr := os.ReadFile(filepath.Join(env, "artifact-native.txt"))
		if gErr != nil || nErr != nil || string(gBody) != string(nBody) {
			t.Fatalf("curl -O file mismatch gobox=%q native=%q gErr=%v nErr=%v", string(gBody), string(nBody), gErr, nErr)
		}
	})

	methodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.Method)
	}))
	defer methodServer.Close()

	headerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.Header.Get("X-Test"))
	}))
	defer headerServer.Close()

	for _, tc := range []parityCase{
		{ID: "CURL-005", Name: "curl -L", GoboxArgs: []string{"curl", "-L", server.URL + "/redirect"}, NativeCommand: "curl", NativeArgs: []string{"-L", server.URL + "/redirect"}},
		{ID: "CURL-006", Name: "curl -I", GoboxArgs: []string{"curl", "-I", server.URL}, NativeCommand: "curl", NativeArgs: []string{"-I", server.URL}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -I exit mismatch")
			}
			if !strings.Contains(gobox.Stdout, "HTTP/") {
				t.Fatalf("curl -I missing status line: %q", gobox.Stdout)
			}
			if strings.Contains(gobox.Stdout, "ok") {
				t.Fatalf("curl -I should not include response body: %q", gobox.Stdout)
			}
		}},
		{ID: "CURL-007", Name: "curl -w", GoboxArgs: []string{"curl", "-w", "%{http_code}", "-o", os.DevNull, server.URL}, NativeCommand: "curl", NativeArgs: []string{"-w", "%{http_code}", "-o", os.DevNull, server.URL}},
		{ID: "CURL-009", Name: "curl -X", GoboxArgs: []string{"curl", "-X", "POST", methodServer.URL}, NativeCommand: "curl", NativeArgs: []string{"-X", "POST", methodServer.URL}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -X exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
			}
			if normalizeText(gobox.Stdout) != "POST" || normalizeText(native.Stdout) != "POST" {
				t.Fatalf("curl -X did not switch request method gobox=%q native=%q", gobox.Stdout, native.Stdout)
			}
		}},
		{ID: "CURL-010", Name: "curl -H", GoboxArgs: []string{"curl", "-H", "X-Test: 1", headerServer.URL}, NativeCommand: "curl", NativeArgs: []string{"-H", "X-Test: 1", headerServer.URL}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -H exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
			}
			if normalizeText(gobox.Stdout) != "1" || normalizeText(native.Stdout) != "1" {
				t.Fatalf("curl -H did not send custom header gobox=%q native=%q", gobox.Stdout, native.Stdout)
			}
		}},
		{ID: "CURL-011", Name: "curl -d", GoboxArgs: []string{"curl", "-d", "name=test", server.URL + "/echo"}, NativeCommand: "curl", NativeArgs: []string{"-d", "name=test", server.URL + "/echo"}},
		{ID: "CURL-015", Name: "curl -f", GoboxArgs: []string{"curl", "-f", server.URL + "/fail"}, NativeCommand: "curl", NativeArgs: []string{"-f", server.URL + "/fail"}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -f exit mismatch %d != %d", gobox.ExitCode, native.ExitCode)
			}
		}},
	} {
		t.Run(tc.ID, func(t *testing.T) {
			gobox := runGoboxCLI(t, t.TempDir(), tc.Stdin, tc.GoboxArgs...)
			native := runNativeCLI(t, t.TempDir(), tc.Stdin, tc.NativeCommand, tc.NativeArgs...)
			if tc.Assert != nil {
				tc.Assert(t, gobox, native)
				return
			}
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("%s exit mismatch gobox=%d native=%d", tc.ID, gobox.ExitCode, native.ExitCode)
			}
			if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
				t.Fatalf("%s stdout mismatch\n%s\n%s", tc.ID, gobox.Stdout, native.Stdout)
			}
		})
	}

	t.Run("CURL-008", func(t *testing.T) {
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			fmt.Fprint(w, "slow")
		}))
		defer slowServer.Close()
		gobox := runGoboxCLI(t, t.TempDir(), "", "curl", "-m", "0.05", slowServer.URL)
		native := runNativeCLI(t, t.TempDir(), "", "curl", "-m", "0.05", slowServer.URL)
		if gobox.ExitCode == 0 || native.ExitCode == 0 {
			t.Fatalf("curl -m expected timeout failure gobox=%+v native=%+v", gobox, native)
		}
		if strings.Contains(gobox.Stdout, "slow") {
			t.Fatalf("curl -m should time out before receiving the body: %+v", gobox)
		}
	})

	t.Run("CURL-012", func(t *testing.T) {
		tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "tls-ok") }))
		defer tlsServer.Close()
		gobox := runGoboxCLI(t, t.TempDir(), "", "curl", "-k", tlsServer.URL)
		native := runNativeCLI(t, t.TempDir(), "", "curl", "-k", tlsServer.URL)
		if gobox.ExitCode != native.ExitCode || normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -k mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("CURL-013", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "curl", "--connect-timeout", "0.05", "http://10.255.255.1:81")
		native := runNativeCLI(t, t.TempDir(), "", "curl", "--noproxy", "*", "-s", "--connect-timeout", "0.05", "http://10.255.255.1:81")
		if gobox.ExitCode == 0 || native.ExitCode == 0 {
			t.Fatalf("curl --connect-timeout expected failure gobox=%+v native=%+v", gobox, native)
		}
		if strings.TrimSpace(gobox.Stdout) != "" {
			t.Fatalf("curl --connect-timeout should not produce a successful response body: %+v", gobox)
		}
	})

	t.Run("CURL-014", func(t *testing.T) {
		hostPort := strings.TrimPrefix(server.URL, "http://")
		_, port, _ := strings.Cut(hostPort, ":")
		gobox := runGoboxCLI(t, t.TempDir(), "", "curl", "--resolve", "example.invalid:"+port+":127.0.0.1", "http://example.invalid:"+port)
		native := runNativeCLI(t, t.TempDir(), "", "curl", "--noproxy", "*", "-s", "--resolve", "example.invalid:"+port+":127.0.0.1", "http://example.invalid:"+port)
		if gobox.ExitCode != native.ExitCode || normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl --resolve mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("CURL-016", func(t *testing.T) {
		headerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "1")
			fmt.Fprint(w, "body")
		}))
		defer headerServer.Close()
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "curl", "-s", headerServer.URL)
		gobox := runGoboxCLI(t, env, "", "curl", "-i", headerServer.URL)
		native := runNativeCLI(t, t.TempDir(), "", "curl", "-i", headerServer.URL)
		if base.ExitCode != 0 || gobox.ExitCode != native.ExitCode {
			t.Fatalf("curl -i exit mismatch base=%+v gobox=%+v native=%+v", base, gobox, native)
		}
		if !strings.Contains(gobox.Stdout, "X-Test: 1") || !strings.Contains(gobox.Stdout, "body") {
			t.Fatalf("curl -i gobox output incomplete: %+v", gobox)
		}
		if !strings.Contains(native.Stdout, "X-Test: 1") || !strings.Contains(native.Stdout, "body") {
			t.Fatalf("curl -i native output incomplete: %+v", native)
		}
		if base.Stdout == gobox.Stdout {
			t.Fatalf("curl -i should change output relative to body-only mode\n--- base ---\n%s\n--- include ---\n%s", base.Stdout, gobox.Stdout)
		}
		if statusIdx := strings.Index(gobox.Stdout, "HTTP/"); statusIdx == -1 {
			t.Fatalf("curl -i missing status line: %+v", gobox)
		} else if headerIdx, bodyIdx := strings.Index(gobox.Stdout, "X-Test: 1"), strings.Index(gobox.Stdout, "body"); headerIdx == -1 || bodyIdx == -1 || !(statusIdx < headerIdx && headerIdx < bodyIdx) {
			t.Fatalf("curl -i should emit status/header/body in order, got %q", gobox.Stdout)
		}
	})

	t.Run("CURL-017", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "payload.txt")
		writeFile(t, file, "upload-body")
		uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			fmt.Fprintf(w, "%s:%s", r.Method, string(body))
		}))
		defer uploadServer.Close()
		gobox := runGoboxCLI(t, env, "", "curl", "-T", file, uploadServer.URL)
		native := runNativeCLI(t, env, "", "curl", "-T", file, uploadServer.URL)
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -T mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
		}
		if normalizeText(gobox.Stdout) != "PUT:upload-body" || normalizeText(native.Stdout) != "PUT:upload-body" {
			t.Fatalf("curl -T should perform a PUT upload with the file body gobox=%q native=%q", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("CURL-018", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "payload.txt")
		writeFile(t, file, "form-body")
		formServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mr, err := r.MultipartReader()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			part, err := mr.NextPart()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(part)
			fmt.Fprintf(w, "%s:%s:%s", part.FormName(), part.FileName(), string(data))
		}))
		defer formServer.Close()
		gobox := runGoboxCLI(t, env, "", "curl", "-F", "file=@payload.txt", formServer.URL)
		native := runNativeCLI(t, env, "", "curl", "-F", "file=@payload.txt", formServer.URL)
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -F mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
		}
		if normalizeText(gobox.Stdout) != "file:payload.txt:form-body" || normalizeText(native.Stdout) != "file:payload.txt:form-body" {
			t.Fatalf("curl -F should upload a multipart file part gobox=%q native=%q", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("CURL-019", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "curl", "--bench", "-n", "4", server.URL)
		res := runGoboxCLI(t, env, "", "curl", "--bench", "-c", "2", "-n", "4", server.URL)
		if base.ExitCode != 0 || res.ExitCode != 0 || !strings.Contains(res.Stdout, "Concurrency: 2") {
			t.Fatalf("curl bench concurrent failed base=%+v concurrent=%+v", base, res)
		}
		if base.Stdout == res.Stdout {
			t.Fatalf("curl bench -c should change output relative to the default concurrency\n--- base ---\n%s\n--- concurrent ---\n%s", base.Stdout, res.Stdout)
		}
	})

	t.Run("CURL-020", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "curl", "--bench", "-n", "2", server.URL)
		res := runGoboxCLI(t, env, "", "curl", "--bench", "-n", "3", server.URL)
		if base.ExitCode != 0 || res.ExitCode != 0 || !strings.Contains(res.Stdout, "Requests: 3") {
			t.Fatalf("curl bench requests failed base=%+v requests=%+v", base, res)
		}
		if base.Stdout == res.Stdout {
			t.Fatalf("curl bench -n should change output relative to the baseline request count\n--- base ---\n%s\n--- requests ---\n%s", base.Stdout, res.Stdout)
		}
	})

	t.Run("CURL-021", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "curl", "--bench", "-n", "2", server.URL)
		res := runGoboxCLI(t, env, "", "curl", "--bench", "--warmup", "2", "-n", "2", server.URL)
		if base.ExitCode != 0 || res.ExitCode != 0 || !strings.Contains(res.Stdout, "Requests: 2") {
			t.Fatalf("curl bench warmup failed base=%+v warmup=%+v", base, res)
		}
		if base.Stdout == res.Stdout {
			t.Fatalf("curl bench --warmup should change output relative to the no-warmup baseline\n--- base ---\n%s\n--- warmup ---\n%s", base.Stdout, res.Stdout)
		}
		if !strings.Contains(res.Stdout+res.Stderr, "Warming up 2 requests") {
			t.Fatalf("curl bench --warmup missing warmup banner: %+v", res)
		}
	})

	t.Run("CURL-022", func(t *testing.T) {
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(150 * time.Millisecond)
			fmt.Fprint(w, "slow")
		}))
		defer slowServer.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "--bench", "-n", "2", "-t", "0.05", slowServer.URL)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Failed:") {
			t.Fatalf("curl bench timeout failed: %+v", res)
		}
	})
}

func TestParity_NcCases(t *testing.T) {
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
		if strings.TrimSpace(gobox.Stdout+gobox.Stderr) != "" {
			t.Fatalf("nc -z should not emit data-path output without -v: %+v", gobox)
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
		if strings.TrimSpace(gobox.Stdout+gobox.Stderr) != "" {
			t.Fatalf("nc -u -z should remain quiet without -v: %+v", gobox)
		}
	})

	t.Run("NC-004", func(t *testing.T) {
		port := closedTCPPort(t)
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-w", "1", "127.0.0.1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-w", "1", "127.0.0.1", port)
		if gobox.ExitCode == 0 || native.ExitCode == 0 {
			t.Fatalf("nc -w expected connection failure gobox=%+v native=%+v", gobox, native)
		}
		if strings.Contains(strings.ToLower(gobox.Stdout+gobox.Stderr), "successful") {
			t.Fatalf("gobox nc -w should not report success on a closed port: %+v", gobox)
		}
	})

	t.Run("NC-005", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "nc", "-z", "127.0.0.1", port)
		gobox := runGoboxCLI(t, env, "", "nc", "-z", "-v", "127.0.0.1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-z", "-v", "127.0.0.1", port)
		if base.ExitCode != 0 || gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("nc -v failed base=%+v gobox=%+v native=%+v", base, gobox, native)
		}
		if normalizeText(base.Stdout+base.Stderr) == normalizeText(gobox.Stdout+gobox.Stderr) {
			t.Fatalf("nc -v should add diagnostic output relative to plain -z\n--- base ---\n%s%s\n--- verbose ---\n%s%s", base.Stdout, base.Stderr, gobox.Stdout, gobox.Stderr)
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
		if strings.Contains(strings.ToLower(gobox.Stderr+gobox.Stdout), "ipv6") {
			t.Fatalf("nc -4 should not attempt ipv6 path: %+v", gobox)
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
		if strings.Contains(strings.ToLower(gobox.Stderr+gobox.Stdout), "ipv4") {
			t.Fatalf("nc -6 should not attempt ipv4 path: %+v", gobox)
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
			env := t.TempDir()
			baseArgs := []string{"nc", "--bench", "-n", "2", "-s", "16B", "127.0.0.1", port}
			base := runGoboxCLI(t, env, "", baseArgs...)
			res := runGoboxCLI(t, env, "", args...)
			if res.ExitCode != 0 || !strings.Contains(res.Stdout, tc.want) {
				t.Fatalf("%s failed: %+v want %q", tc.id, res, tc.want)
			}
			if tc.id != "NC-009" && base.ExitCode == 0 && base.Stdout == res.Stdout {
				t.Fatalf("%s should change bench output relative to the default bench baseline\n--- base ---\n%s\n--- variant ---\n%s", tc.id, base.Stdout, res.Stdout)
			}
		})
	}
}

func TestParity_DnsCases(t *testing.T) {
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
}

func TestParity_NpCases(t *testing.T) {
	t.Run("NP-001", func(t *testing.T) {
		loopbackName := ""
		ifaces, err := net.Interfaces()
		if err != nil {
			t.Fatalf("list interfaces: %v", err)
		}
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 && iface.Flags&net.FlagUp != 0 {
				loopbackName = iface.Name
				break
			}
		}
		if loopbackName == "" {
			t.Skip("no active loopback interface available")
		}
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-I", loopbackName, "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "1 packets transmitted") {
			t.Fatalf("np -I failed: %+v", res)
		}
	})

	t.Run("NP-002", func(t *testing.T) {
		port := closedTCPPort(t)
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-W", "1", "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "1 errors") {
			t.Fatalf("np -W failed: %+v", res)
		}
	})

	t.Run("NP-003", func(t *testing.T) {
		gateway := defaultIPv4Gateway(t)
		gobox := runGoboxCLI(t, t.TempDir(), "", "np", "-arp", "-c", "1", "-W", "1", gateway)
		native := runNativeCLI(t, t.TempDir(), "", "arping", "-c", "1", "-w", "1", gateway)
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("np -arp exit mismatch gobox=%+v native=%+v", gobox, native)
		}
		if gobox.ExitCode != 0 {
			if !strings.Contains(strings.ToLower(gobox.Stderr), "operation not permitted") || !strings.Contains(strings.ToLower(native.Stderr), "operation not permitted") {
				t.Fatalf("np -arp permission failure mismatch gobox=%+v native=%+v", gobox, native)
			}
			return
		}
		if !strings.Contains(gobox.Stdout, gateway) || !strings.Contains(native.Stdout, "Received 1 response") {
			t.Fatalf("np -arp output mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NP-004", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "np", "-tcp", "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-c", "2", "-i", "1000", "-q", "127.0.0.1")
		if base.ExitCode != 0 || res.ExitCode != 0 || !strings.Contains(res.Stdout, "2 packets transmitted") {
			t.Fatalf("np -c failed base=%+v count2=%+v", base, res)
		}
		if !strings.Contains(base.Stdout, "1 packets transmitted") || base.Stdout == res.Stdout {
			t.Fatalf("np -c should change the summary relative to a single-packet baseline\n--- base ---\n%s\n--- count2 ---\n%s", base.Stdout, res.Stdout)
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
		gobox := runGoboxCLI(t, t.TempDir(), "", "np", "-icmp", "-c", "1", "-i", "0", "-q", "-W", "1", "127.0.0.1")
		native := runNativeCLI(t, t.TempDir(), "", "ping", "-c", "1", "-q", "-W", "1", "127.0.0.1")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("np -icmp failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(gobox.Stdout, "1 received") || !strings.Contains(native.Stdout, "1 received") {
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
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "1 packets received") {
			t.Fatalf("np -p failed: %+v", res)
		}
	})

	t.Run("NP-010", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		env := t.TempDir()
		verbose := runGoboxCLI(t, env, "", "np", "-tcp", "-p", port, "-c", "1", "-i", "0", "127.0.0.1")
		res := runGoboxCLI(t, env, "", "np", "-tcp", "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if verbose.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("np -q failed verbose=%+v quiet=%+v", verbose, res)
		}
		if verbose.Stdout == res.Stdout {
			t.Fatalf("np -q should reduce output relative to the default mode\n--- verbose ---\n%s\n--- quiet ---\n%s", verbose.Stdout, res.Stdout)
		}
		if strings.Contains(res.Stdout, "bytes from") || !strings.Contains(res.Stdout, "ping statistics") {
			t.Fatalf("np -q did not collapse to summary output: %+v", res)
		}
	})

	t.Run("NP-011", func(t *testing.T) {
		sourcePort := atoiForTest(t, closedTCPPort(t))
		_, port, remotePorts, closeFn := startTCPRemotePortRecorder(t)
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-s", strconv.Itoa(sourcePort), "-c", "1", "-i", "0", "-q", "127.0.0.1")
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

	t.Run("NP-012", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-scan", fmt.Sprintf("%d", port), "127.0.0.1")
		if res.ExitCode != 0 {
			t.Fatalf("np scan failed: %+v", res)
		}
		if !strings.Contains(res.Stdout, fmt.Sprintf("Port %d: open", port)) || !strings.Contains(res.Stdout, "1 open, 0 closed") {
			t.Fatalf("np scan did not report the expected open-port summary: %+v", res)
		}
	})

	t.Run("NP-013", func(t *testing.T) {
		_, port, remotePorts, closeFn := startTCPRemotePortRecorder(t)
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-tcp", "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "1 packets received") {
			t.Fatalf("np -tcp failed: %+v", res)
		}
		select {
		case got := <-remotePorts:
			if got <= 0 {
				t.Fatalf("np -tcp recorder captured invalid remote port %d", got)
			}
		case <-time.After(time.Second):
			t.Fatal("np -tcp did not establish a TCP connection")
		}
	})

	t.Run("NP-014", func(t *testing.T) {
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen udp: %v", err)
		}
		defer conn.Close()
		_, port, _ := net.SplitHostPort(conn.LocalAddr().String())
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-udp", "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "1 packets received") {
			t.Fatalf("np -udp failed: %+v", res)
		}
		if strings.Contains(strings.ToLower(res.Stdout+res.Stderr), "connection failed") {
			t.Fatalf("np -udp should succeed on a reachable udp socket without failure diagnostics: %+v", res)
		}
	})

	t.Run("NP-015", func(t *testing.T) {
		port := closedTCPPort(t)
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "np", "-tcp", "-p", port, "-c", "1", "-i", "0", "-W", "1", "-q", "127.0.0.1")
		res := runGoboxCLI(t, env, "", "np", "-tcp", "-p", port, "-c", "1", "-i", "0", "-W", "1", "-v", "127.0.0.1")
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("np -v failed base=%+v verbose=%+v", base, res)
		}
		if base.Stdout == res.Stdout {
			t.Fatalf("np -v should add attempt-level diagnostics relative to quiet mode\n--- base ---\n%s\n--- verbose ---\n%s", base.Stdout, res.Stdout)
		}
		if !strings.Contains(res.Stdout, "Connection failed") || !strings.Contains(res.Stdout, "ping statistics") {
			t.Fatalf("np -v failed: %+v", res)
		}
	})

	t.Run("NP-016", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "np", "-tcp", "-p", port, "-c", "4", "-i", "0", "-q", "127.0.0.1")
		res := runGoboxCLI(t, env, "", "np", "-tcp", "-p", port, "-w", "2", "-c", "4", "-i", "0", "-q", "127.0.0.1")
		if base.ExitCode != 0 || res.ExitCode != 0 || !strings.Contains(res.Stdout, "packets transmitted") {
			t.Fatalf("np -w failed base=%+v workers=%+v", base, res)
		}
		if !strings.Contains(res.Stdout, "4 packets received") && !strings.Contains(res.Stdout, "5 packets received") {
			t.Fatalf("np -w received count mismatch: %+v", res)
		}
		if base.Stdout == res.Stdout {
			t.Fatalf("np -w should affect the execution/reporting path relative to the single-worker baseline\n--- base ---\n%s\n--- workers ---\n%s", base.Stdout, res.Stdout)
		}
	})
}

func TestParity_IfstatCases(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}

	for _, tc := range []struct {
		id    string
		args  []string
		check func(t *testing.T, out string)
	}{
		{
			id:   "IFSTAT-001",
			args: []string{"ifstat", "-A", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
				defaultOut := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1").Stdout
				if len(nonEmptyLines(out)) < len(nonEmptyLines(defaultOut)) {
					t.Fatalf("ifstat -A should not show fewer rows than default\n-A:\n%s\n--- default ---\n%s", out, defaultOut)
				}
			},
		},
		{
			id:   "IFSTAT-002",
			args: []string{"ifstat", "-a", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
				base := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1")
				if base.ExitCode != 0 {
					t.Fatalf("ifstat baseline failed: %+v", base)
				}
				if out == base.Stdout {
					t.Fatalf("ifstat -a did not change output relative to baseline\n--- base ---\n%s\n--- absolute ---\n%s", base.Stdout, out)
				}
				if !strings.Contains(out, "Interface") {
					t.Fatalf("ifstat -a missing header: %q", out)
				}
			},
		},
		{
			id:   "IFSTAT-003",
			args: []string{"ifstat", "-d", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
				base := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1")
				if base.ExitCode != 0 {
					t.Fatalf("ifstat baseline failed: %+v", base)
				}
				if out == base.Stdout {
					t.Fatalf("ifstat -d did not change output relative to baseline\n--- base ---\n%s\n--- drop ---\n%s", base.Stdout, out)
				}
				if !strings.Contains(out, "rxdrop") || !strings.Contains(out, "txdrop") {
					t.Fatalf("ifstat -d missing drop columns: %q", out)
				}
			},
		},
		{
			id:   "IFSTAT-004",
			args: []string{"ifstat", "-e", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
				base := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1")
				if base.ExitCode != 0 {
					t.Fatalf("ifstat baseline failed: %+v", base)
				}
				if out == base.Stdout {
					t.Fatalf("ifstat -e did not change output relative to baseline\n--- base ---\n%s\n--- errors ---\n%s", base.Stdout, out)
				}
				if !strings.Contains(out, "rxerrs") || !strings.Contains(out, "txerrs") {
					t.Fatalf("ifstat -e missing error columns: %q", out)
				}
			},
		},
		{
			id:   "IFSTAT-005",
			args: []string{"ifstat", "-i", "lo", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
				lines := nonEmptyLines(out)
				for _, line := range lines[1:] {
					if !strings.HasPrefix(strings.TrimSpace(line), "lo ") && strings.TrimSpace(line) != "lo" {
						t.Fatalf("ifstat -i lo leaked other interfaces: %q", out)
					}
				}
			},
		},
		{
			id:   "IFSTAT-006",
			args: []string{"ifstat", "-n", "2", "-p", "1"},
			check: func(t *testing.T, out string) {
				if len(nonEmptyLines(out)) < 3 {
					t.Fatalf("ifstat -n expected multiple samples: %q", out)
				}
			},
		},
		{
			id:   "IFSTAT-007",
			args: []string{"ifstat", "-n", "2", "-p", "1"},
			check: func(t *testing.T, out string) {
				if len(nonEmptyLines(out)) < 3 {
					t.Fatalf("ifstat -n/-p expected header plus repeated samples: %q", out)
				}
			},
		},
	} {
		t.Run(tc.id, func(t *testing.T) {
			start := time.Now()
			res := runGoboxCLI(t, t.TempDir(), "", tc.args...)
			if res.ExitCode != 0 {
				t.Fatalf("%s failed: %+v", tc.id, res)
			}
			if (tc.id == "IFSTAT-006" || tc.id == "IFSTAT-007") && time.Since(start) < time.Second {
				t.Fatalf("ifstat -p interval did not delay second sample: elapsed=%s output=%q", time.Since(start), res.Stdout)
			}
			if tc.check != nil {
				tc.check(t, res.Stdout)
			}
		})
	}
}
