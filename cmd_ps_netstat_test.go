package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTruncateString(t *testing.T) {
	if got := truncateString("hello", 0); got != "hello" {
		t.Fatalf("expected no truncation, got %q", got)
	}
	if got := truncateString("hello", 3); got != "hel" {
		t.Fatalf("expected hard truncation to 3, got %q", got)
	}
	if got := truncateString("hello", 4); got != "h..." {
		t.Fatalf("expected ellipsis truncation, got %q", got)
	}
	if got := truncateString("hi", 5); got != "hi" {
		t.Fatalf("expected no truncation, got %q", got)
	}
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
