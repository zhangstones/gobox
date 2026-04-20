package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParity_NetLightweightCases(t *testing.T) {
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

	t.Run("CURL-002", func(t *testing.T) {
		res := runGoboxMainCLI(t, t.TempDir(), "", "curl", "-s", "-S", "://bad-url")
		if res.ExitCode == 0 || !strings.Contains(strings.ToLower(res.Stderr), "curl:") {
			t.Fatalf("curl -s -S failed: %+v", res)
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
		} {
			t.Run(tc.id, func(t *testing.T) {
				res := runGoboxCLI(t, t.TempDir(), "", tc.args...)
				if res.ExitCode != 0 {
					t.Fatalf("%s failed: %+v", tc.id, res)
				}
			})
		}
	}
}

func TestParity_NetStructured(t *testing.T) {
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
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "netstat", "-port", fmt.Sprintf("%d", port))
		if res.ExitCode != 0 {
			t.Fatalf("netstat failed: %+v", res)
		}
		if !strings.Contains(res.Stdout, fmt.Sprintf(":%d", port)) {
			t.Fatalf("netstat -port missing listener: %q", res.Stdout)
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
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "netstat", "-l", "-port", fmt.Sprintf("%d", port))
		if res.ExitCode != 0 {
			t.Fatalf("netstat -l failed: %+v", res)
		}
		if !strings.Contains(res.Stdout, "LISTEN") {
			t.Fatalf("netstat -l missing LISTEN: %q", res.Stdout)
		}
	})

	t.Run("NETSTAT-005", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "netstat", "-n")
		if res.ExitCode != 0 {
			t.Fatalf("netstat -n failed: %+v", res)
		}
	})
}

func TestParity_CurlBehavior(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("native curl not found")
	}
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
				http.Error(w, err.Error(), 400)
				return
			}
			part, err := mr.NextPart()
			if err != nil {
				http.Error(w, err.Error(), 400)
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

	cases := []parityCase{
		{ID: "CURL-001", Name: "curl -s", GoboxArgs: []string{"curl", "-s", server.URL}, NativeCommand: "curl", NativeArgs: []string{"-s", server.URL}},
		{ID: "CURL-005", Name: "curl -L", GoboxArgs: []string{"curl", "-L", server.URL + "/redirect"}, NativeCommand: "curl", NativeArgs: []string{"-L", server.URL + "/redirect"}},
		{ID: "CURL-006", Name: "curl -I", GoboxArgs: []string{"curl", "-I", server.URL}, NativeCommand: "curl", NativeArgs: []string{"-I", server.URL}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -I exit mismatch")
			}
			if !strings.Contains(gobox.Stdout, "HTTP/") {
				t.Fatalf("curl -I missing status line: %q", gobox.Stdout)
			}
		}},
		{ID: "CURL-007", Name: "curl -w", GoboxArgs: []string{"curl", "-w", "%{http_code}", "-o", os.DevNull, server.URL}, NativeCommand: "curl", NativeArgs: []string{"-w", "%{http_code}", "-o", os.DevNull, server.URL}},
		{ID: "CURL-009", Name: "curl -X", GoboxArgs: []string{"curl", "-X", "POST", server.URL + "/echo"}, NativeCommand: "curl", NativeArgs: []string{"-X", "POST", server.URL + "/echo"}},
		{ID: "CURL-010", Name: "curl -H", GoboxArgs: []string{"curl", "-H", "X-Test: 1", server.URL}, NativeCommand: "curl", NativeArgs: []string{"-H", "X-Test: 1", server.URL}},
		{ID: "CURL-011", Name: "curl -d", GoboxArgs: []string{"curl", "-d", "name=test", server.URL + "/echo"}, NativeCommand: "curl", NativeArgs: []string{"-d", "name=test", server.URL + "/echo"}},
		{ID: "CURL-015", Name: "curl -f", GoboxArgs: []string{"curl", "-f", server.URL + "/fail"}, NativeCommand: "curl", NativeArgs: []string{"-f", server.URL + "/fail"}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -f exit mismatch %d != %d", gobox.ExitCode, native.ExitCode)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			env := &parityEnv{Dir: t.TempDir()}
			gobox := runGoboxCLI(t, env.Dir, tc.Stdin, tc.GoboxArgs...)
			native := runNativeCLI(t, env.Dir, tc.Stdin, tc.NativeCommand, tc.NativeArgs...)
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

	t.Run("CURL-017", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		file := filepath.Join(env.Dir, "payload.txt")
		writeFile(t, file, "upload-body")
		gobox := runGoboxCLI(t, env.Dir, "", "curl", "-T", file, server.URL+"/upload")
		native := runNativeCLI(t, env.Dir, "", "curl", "-T", file, server.URL+"/upload")
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -T mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("CURL-018", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		file := filepath.Join(env.Dir, "payload.txt")
		writeFile(t, file, "form-body")
		gobox := runGoboxCLI(t, env.Dir, "", "curl", "-F", "file=@payload.txt", server.URL+"/multipart")
		native := runNativeCLI(t, env.Dir, "", "curl", "-F", "file=@payload.txt", server.URL+"/multipart")
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -F mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
		}
	})
}

func TestParity_NetContracts(t *testing.T) {
	t.Run("TW-004", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "tw", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("tw -h failed: %+v", res)
		}
	})

	t.Run("IFSTAT-006", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1")
		if res.ExitCode != 0 {
			t.Fatalf("ifstat failed: %+v", res)
		}
	})

	t.Run("NP-012", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
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
	})
}

func TestParity_CurlResolveContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "resolved")
	}))
	defer server.Close()
	hostPort := strings.TrimPrefix(server.URL, "http://")
	host, port, _ := strings.Cut(hostPort, ":")
	_ = host
	res := runGoboxCLI(t, t.TempDir(), "", "curl", "--resolve", "example.invalid:"+port+":127.0.0.1", "http://example.invalid:"+port)
	if res.ExitCode != 0 {
		t.Fatalf("curl --resolve failed: %+v", res)
	}
	if !strings.Contains(res.Stdout, "resolved") {
		t.Fatalf("curl --resolve missing response: %q", res.Stdout)
	}
}
