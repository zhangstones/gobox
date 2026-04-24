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
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-n")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-n")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "Proto") || !strings.Contains(native.Stdout, "Proto") {
			t.Fatalf("netstat -n mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("NETSTAT-006", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-a")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-a")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "Proto") || !strings.Contains(native.Stdout, "Proto") {
			t.Fatalf("netstat -a mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
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
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-p")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-p")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "PID/Program") || !strings.Contains(native.Stdout, "PID/Program") {
			t.Fatalf("netstat -p mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
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
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-e")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-e")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "User") || !strings.Contains(res.Stdout, "Inode") || !strings.Contains(native.Stdout, "User") || !strings.Contains(native.Stdout, "Inode") {
			t.Fatalf("netstat -e mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("NETSTAT-014", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-o")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-o")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "Timer") || !strings.Contains(native.Stdout, "Timer") {
			t.Fatalf("netstat -o mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("NETSTAT-015", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-W", "-n", "-l")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-W", "-n", "-l")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "Proto") || !strings.Contains(native.Stdout, "Proto") {
			t.Fatalf("netstat -W/-n/-l mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
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
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-tnlp", "-port", strconv.Itoa(port))
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-tnlp")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, "TCP") || !strings.Contains(res.Stdout, "PID/Program") || !strings.Contains(native.Stdout, "tcp") {
			t.Fatalf("netstat combined flags mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
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
	})

	t.Run("NETSTAT-019", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-s")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-s")
		if res.ExitCode != native.ExitCode || !strings.Contains(res.Stdout, ":") || !strings.Contains(native.Stdout, ":") {
			t.Fatalf("netstat -s mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
	})

	t.Run("NETSTAT-020", func(t *testing.T) {
		t.Skip("netstat -c is intentionally continuous; signal-loop behavior is covered by command-level tests")
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
	})

	t.Run("IP-002", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "ip", "-o", "addr")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "-o", "addr")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip -o addr exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if !strings.Contains(gobox.Stdout, " lo ") || !strings.Contains(native.Stdout, " lo ") {
			t.Fatalf("ip -o addr missing loopback line\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
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
	})

	t.Run("IP-004", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "ip", "-s", "link")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "-s", "link")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip -s link exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		for _, want := range []string{"RX", "TX"} {
			if !strings.Contains(strings.ToUpper(gobox.Stdout), want) || !strings.Contains(strings.ToUpper(native.Stdout), want) {
				t.Fatalf("ip -s link missing %q\ngobox=%s\nnative=%s", want, gobox.Stdout, native.Stdout)
			}
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

	for _, tc := range []parityCase{
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
		gobox := runGoboxCLI(t, t.TempDir(), "", "curl", "-i", headerServer.URL)
		native := runNativeCLI(t, t.TempDir(), "", "curl", "-i", headerServer.URL)
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("curl -i exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if !strings.Contains(gobox.Stdout, "X-Test: 1") || !strings.Contains(gobox.Stdout, "body") {
			t.Fatalf("curl -i gobox output incomplete: %+v", gobox)
		}
		if !strings.Contains(native.Stdout, "X-Test: 1") || !strings.Contains(native.Stdout, "body") {
			t.Fatalf("curl -i native output incomplete: %+v", native)
		}
	})

	t.Run("CURL-017", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "payload.txt")
		writeFile(t, file, "upload-body")
		gobox := runGoboxCLI(t, env, "", "curl", "-T", file, server.URL+"/upload")
		native := runNativeCLI(t, env, "", "curl", "-T", file, server.URL+"/upload")
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -T mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("CURL-018", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "payload.txt")
		writeFile(t, file, "form-body")
		gobox := runGoboxCLI(t, env, "", "curl", "-F", "file=@payload.txt", server.URL+"/multipart")
		native := runNativeCLI(t, env, "", "curl", "-F", "file=@payload.txt", server.URL+"/multipart")
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -F mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("CURL-019", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "--bench", "-c", "2", "-n", "4", server.URL)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Concurrency: 2") {
			t.Fatalf("curl bench concurrent failed: %+v", res)
		}
	})

	t.Run("CURL-020", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "--bench", "-n", "3", server.URL)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Requests: 3") {
			t.Fatalf("curl bench requests failed: %+v", res)
		}
	})

	t.Run("CURL-021", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "--bench", "--warmup", "2", "-n", "2", server.URL)
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Requests: 2") {
			t.Fatalf("curl bench warmup failed: %+v", res)
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
				if !strings.Contains(out, "Interface") {
					t.Fatalf("ifstat -a missing header: %q", out)
				}
			},
		},
		{
			id:   "IFSTAT-003",
			args: []string{"ifstat", "-d", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
				if !strings.Contains(out, "rxdrop") || !strings.Contains(out, "txdrop") {
					t.Fatalf("ifstat -d missing drop columns: %q", out)
				}
			},
		},
		{
			id:   "IFSTAT-004",
			args: []string{"ifstat", "-e", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
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
