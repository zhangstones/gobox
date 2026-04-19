package net

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func captureNetOutput(t *testing.T, fn func() error) (string, error) {
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

func TestParsePortFromAddr(t *testing.T) {
	if got := parsePortFromAddr("0100007F:0035"); got != 53 {
		t.Fatalf("expected port 53, got %d", got)
	}
	if got := parsePortFromAddr("bad"); got != 0 {
		t.Fatalf("expected port 0 for bad input, got %d", got)
	}
}

func TestParseIPFromAddr(t *testing.T) {
	if got := parseIPFromAddr("0100007F:0035"); got != "127.0.0.1" {
		t.Fatalf("expected IPv4 127.0.0.1, got %q", got)
	}
	if got := parseIPFromAddr("00000000000000000000000000000001:0035"); got != "::1" {
		t.Fatalf("expected IPv6 ::1, got %q", got)
	}
	if got := parseIPFromAddr("bad"); got != "" {
		t.Fatalf("expected empty IP for bad input, got %q", got)
	}
}

func TestTCPStateName(t *testing.T) {
	if got := tcpStateName("01"); got != "ESTABLISHED" {
		t.Fatalf("expected ESTABLISHED, got %q", got)
	}
	if got := tcpStateName("0A"); got != "LISTEN" {
		t.Fatalf("expected LISTEN, got %q", got)
	}
	if got := tcpStateName("ZZ"); got != "ZZ" {
		t.Fatalf("expected fallback to input, got %q", got)
	}
}

func TestParseProcNetTCP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tcp")
	content := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:0035 00000000:0000 0A 00000000:00000000 00:00000000 00000000   100        0 12345 1 0000000000000000 100 0 0 10 0\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	conns, err := parseProcNetTCP(path)
	if err != nil {
		t.Fatalf("parseProcNetTCP: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 conn, got %d", len(conns))
	}
	c := conns[0]
	if c.LocalPort != 53 || c.LocalIP != "127.0.0.1" || c.State != "LISTEN" {
		t.Fatalf("unexpected conn: %+v", c)
	}
}

func TestParseProcNetUDP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "udp")
	content := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:1F90 00000000:0000 07 00000000:00000000 00:00000000 00000000   100        0 54321 1 0000000000000000 100 0 0 10 0\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	conns, err := parseProcNetUDP(path, "UDP")
	if err != nil {
		t.Fatalf("parseProcNetUDP: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 conn, got %d", len(conns))
	}
	if conns[0].Proto != "UDP" {
		t.Fatalf("expected proto UDP, got %q", conns[0].Proto)
	}
}

func TestNetstatCmdPortFilterMatchesExactPort(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	output, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-port", strconv.Itoa(port)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd failed: %v", err)
	}

	if !strings.Contains(output, ":"+strconv.Itoa(port)) {
		t.Fatalf("expected output to contain exact port %d, got %q", port, output)
	}
}

func TestNetstatCmdStateFilterSupportsStateList(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	output, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-state", "LISTEN,ESTABLISHED", "-port", strconv.Itoa(port)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd failed: %v", err)
	}

	if !strings.Contains(output, "LISTEN") {
		t.Fatalf("expected LISTEN entry for port %d, got %q", port, output)
	}
	if strings.Contains(output, "TIME_WAIT") {
		t.Fatalf("expected state filter to exclude unrelated states, got %q", output)
	}
}

func TestNetstatCmdSortByPID(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	output, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-sort", "pid"})
	})
	if err != nil {
		t.Fatalf("NetstatCmd failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and at least one row, got %q", output)
	}

	prev := -1
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		pidProg := fields[len(fields)-1]
		pidStr, _, _ := strings.Cut(pidProg, "/")
		if pidStr == "-" {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		if prev != -1 && pid < prev {
			t.Fatalf("expected pid-sorted output, saw %d before %d in %q", prev, pid, output)
		}
		prev = pid
	}
}

func TestNetstatCmdPortFilterDoesNotDoRangeMatch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	missPort := port + 1
	time.Sleep(20 * time.Millisecond)

	output, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-port", strconv.Itoa(missPort)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd failed: %v", err)
	}

	if strings.Contains(output, ":"+strconv.Itoa(port)) {
		t.Fatalf("expected exact port filtering, but matched listener on %d for filter %d: %q", port, missPort, output)
	}
}
