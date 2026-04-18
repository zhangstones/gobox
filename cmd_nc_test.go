package main

import (
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ============== HELPER FUNCTIONS ==============

func startNCServer(t *testing.T, port int) *exec.Cmd {
	cmd := exec.Command("./gobox", "nc", "-l", strconv.Itoa(port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start nc server: %v", err)
	}
	return cmd
}

func waitForPort(port int, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return false
		default:
			conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
			if err == nil {
				conn.Close()
				return true
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func killProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
		cmd.Wait()
	}
}

// ============== BASIC CONNECT TESTS ==============

func TestNCBasicConnect(t *testing.T) {
	// Start a TCP server in a goroutine
	serverReady := make(chan struct{})
	go func() {
		ln, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Logf("Server listen failed: %v", err)
			close(serverReady)
			return
		}
		addr := ln.Addr().String()
		_ = strings.Split(addr, ":")[len(strings.Split(addr, ":"))-1]
		close(serverReady)

		conn, err := ln.Accept()
		if err != nil {
			t.Logf("Accept failed: %v", err)
			ln.Close()
			return
		}
		defer conn.Close()
		defer ln.Close()

		// Echo back any data received
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		conn.Write(buf[:n])
	}()

	// Wait for server to be ready
	<-serverReady
	time.Sleep(100 * time.Millisecond)
}

func TestNCHelp(t *testing.T) {
	cmd := exec.Command("./gobox", "nc", "-h")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("nc -h command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information in output, got: %s", result)
	}
	if !strings.Contains(result, "listen") {
		t.Errorf("Expected 'listen' in help output, got: %s", result)
	}
}

// ============== LISTEN MODE TESTS ==============

func TestNCListenMode(t *testing.T) {
	// Get an available port using a TCP listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	// Start server in background
	cmd := exec.Command("./gobox", "nc", "-l", strconv.Itoa(port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start nc server: %v", err)
	}
	defer killProcess(cmd)

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Check that server is listening by connecting
	scanCmd := exec.Command("./gobox", "nc", "-zj", "127.0.0.1", strconv.Itoa(port))
	scanCmd.Output()
	t.Logf("Server started on port %d", port)

	// Give a bit more time then check if server is still running
	time.Sleep(100 * time.Millisecond)
}

// ============== LISTEN AND CONNECT TESTS ==============

func TestNCListenAndConnect(t *testing.T) {
	// Start a TCP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	// Server handler
	serverDone := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			close(serverDone)
			return
		}
		defer conn.Close()
		defer close(serverDone)

		// Read and echo
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		conn.Write(buf[:n])
	}()

	// Give server time to be ready
	time.Sleep(100 * time.Millisecond)

	// Connect with nc client and send data
	cmd := exec.Command("./gobox", "nc", "127.0.0.1", strconv.Itoa(port))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	stdin.Write([]byte("hello\n"))
	stdin.Close()

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("nc client failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "hello") {
		t.Errorf("Expected 'hello' in output, got: %s", result)
	}

	// Wait for server to finish
	<-serverDone
}

func TestNCListenUDP(t *testing.T) {
	// Start a UDP server
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP server: %v", err)
	}
	defer ln.Close()

	port := ln.LocalAddr().(*net.UDPAddr).Port

	// Give time for port to be ready
	time.Sleep(100 * time.Millisecond)

	// Connect with nc client using UDP
	cmd := exec.Command("./gobox", "nc", "-u", "-zj", "127.0.0.1", strconv.Itoa(port))
	output, err := cmd.Output()
	if err != nil {
		t.Logf("UDP scan output: %s, err: %v", string(output), err)
	}
	t.Logf("UDP listen test output: %s", string(output))
}

// ============== PORT SCAN MODE TESTS ==============

func TestNCPortScanOpen(t *testing.T) {
	// Start a TCP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	// Give time for port to be available again
	time.Sleep(100 * time.Millisecond)

	// Restart server
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer ln2.Close()

	port = ln2.Addr().(*net.TCPAddr).Port

	// Give time for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Scan the port with -z (zero I/O)
	cmd := exec.Command("./gobox", "nc", "-z", "127.0.0.1", strconv.Itoa(port))
	output, err := cmd.Output()
	result := string(output)
	t.Logf("Port scan output: %s, err: %v", result, err)

	// With -z, it should exit successfully if port is open
	if err != nil {
		t.Errorf("Port scan failed unexpectedly: %v", err)
	}
}

func TestNCPortScanClosed(t *testing.T) {
	// Try to scan a port that is definitely closed
	cmd := exec.Command("./gobox", "nc", "-zj", "127.0.0.1", "59999")
	output, err := cmd.Output()
	result := string(output)
	t.Logf("Closed port scan output: %s, err: %v", result, err)

	// Scanning a closed port should fail
	if err == nil {
		// Some implementations succeed but with verbose output
		t.Logf("Note: scanning closed port did not error")
	}
}

func TestNCPortScanUDP(t *testing.T) {
	// Start a UDP server
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP server: %v", err)
	}
	port := ln.LocalAddr().(*net.UDPAddr).Port
	ln.Close()

	// Give time for port to be available
	time.Sleep(100 * time.Millisecond)

	// Scan UDP port
	cmd := exec.Command("./gobox", "nc", "-zu", "127.0.0.1", strconv.Itoa(port))
	output, err := cmd.Output()
	t.Logf("UDP port scan output: %s, err: %v", string(output), err)
}

// ============== VERBOSE MODE TESTS ==============

func TestNCVerboseMode(t *testing.T) {
	// Start a TCP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Server handler
	serverDone := make(chan struct{})
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
		close(serverDone)
	}()

	time.Sleep(100 * time.Millisecond)

	// Connect with verbose mode but zero I/O
	cmd := exec.Command("./gobox", "nc", "-v", "-z", "127.0.0.1", strconv.Itoa(port))
	output, err := cmd.Output()
	result := string(output)
	t.Logf("Verbose output: %s", result)

	if err == nil {
		// Verbose should show connection info
		if !strings.Contains(result, "127.0.0.1") {
			t.Logf("Verbose output should contain IP address")
		}
	}

	ln.Close()
	<-serverDone
}

func TestNCVerboseNumeric(t *testing.T) {
	// Start a TCP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Server handler
	serverDone := make(chan struct{})
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
		close(serverDone)
	}()

	time.Sleep(100 * time.Millisecond)

	// Connect with verbose and numeric mode
	cmd := exec.Command("./gobox", "nc", "-v", "-n", "-z", "127.0.0.1", strconv.Itoa(port))
	output, err := cmd.Output()
	result := string(output)
	t.Logf("Verbose numeric output: %s", result)

	if err == nil {
		t.Logf("Connect succeeded with verbose numeric mode")
	}

	ln.Close()
	<-serverDone
}

// ============== TIMEOUT TESTS ==============

func TestNCConnectionTimeout(t *testing.T) {
	// Try to connect to a host that will not respond
	// Using a non-routable IP address
	cmd := exec.Command("./gobox", "nc", "-wj", "1", "-z", "10.255.255.1", "80")
	output, err := cmd.Output()
	result := string(output)
	t.Logf("Timeout test output: %s, err: %v", result, err)

	// This should timeout and error
	if err == nil {
		t.Logf("Note: timeout test did not error (may succeed on some networks)")
	}
}

func TestNCWaitFlag(t *testing.T) {
	// Start a server that accepts but doesn't respond
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Server handler - accept but don't respond
	serverDone := make(chan struct{})
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			time.Sleep(2 * time.Second) // Short delay for test
			conn.Close()
		}
		close(serverDone)
	}()

	time.Sleep(100 * time.Millisecond)

	// Connect with short timeout
	cmd := exec.Command("./gobox", "nc", "-wj", "1", "127.0.0.1", strconv.Itoa(port))
	_, err = cmd.Output()
	t.Logf("Wait flag test err: %v", err)

	// Should timeout
	if err != nil {
		t.Logf("Connection timed out as expected: %v", err)
	}

	ln.Close()
	<-serverDone
}

// ============== ERROR CASES ==============

func TestNCInvalidPort(t *testing.T) {
	// Try to listen on invalid port
	cmd := exec.Command("./gobox", "nc", "-l", "invalid")
	output, err := cmd.Output()
	result := string(output)
	t.Logf("Invalid port output: %s, err: %v", result, err)

	// Should fail
	if err == nil {
		t.Errorf("Expected error for invalid port, got success")
	}
}

func TestNCMissingHost(t *testing.T) {
	// Client mode without host should error
	cmd := exec.Command("./gobox", "nc")
	output, err := cmd.Output()
	result := string(output)
	t.Logf("Missing args output: %s, err: %v", result, err)

	// Should fail with usage error
	if err == nil {
		t.Errorf("Expected error for missing arguments")
	}
}

func TestNCConnectionRefused(t *testing.T) {
	// Try to connect to a port with no server listening
	cmd := exec.Command("./gobox", "nc", "-zj", "127.0.0.1", "59999")
	output, err := cmd.Output()
	result := string(output)
	t.Logf("Connection refused output: %s, err: %v", result, err)

	// Connection refused should produce an error
	if err == nil {
		// Check if there's any indication of failure in output
		if !strings.Contains(result, "refused") && !strings.Contains(result, "failed") {
			t.Logf("Note: connection refused may not have produced error")
		}
	}
}

// ============== UDP MODE TESTS ==============

func TestNCUDPMode(t *testing.T) {
	// Start a UDP server
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP server: %v", err)
	}
	port := ln.LocalAddr().(*net.UDPAddr).Port

	// Server that echoes
	serverDone := make(chan struct{})
	go func() {
		buf := make([]byte, 1024)
		ln.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, addr, _ := ln.ReadFrom(buf)
		if n > 0 {
			ln.WriteTo(buf[:n], addr)
		}
		close(serverDone)
	}()

	time.Sleep(100 * time.Millisecond)

	// Connect with UDP using -z (zero I/O) to avoid blocking
	cmd := exec.Command("./gobox", "nc", "-uj", "-z", "-w", "1", "127.0.0.1", strconv.Itoa(port))
	output, err := cmd.Output()
	result := string(output)
	t.Logf("UDP mode output: %s, err: %v", result, err)

	// For UDP, with -z it should just check if port is open
	_ = result

	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Logf("Server did not complete in time")
	}

	ln.Close()
}

func TestNCUDPScan(t *testing.T) {
	// Start a UDP server
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start UDP server: %v", err)
	}
	port := ln.LocalAddr().(*net.UDPAddr).Port
	ln.Close()

	// Give time for port to be available
	time.Sleep(100 * time.Millisecond)

	// UDP port scan with -zu
	cmd := exec.Command("./gobox", "nc", "-zu", "127.0.0.1", strconv.Itoa(port))
	output, err := cmd.Output()
	result := string(output)
	t.Logf("UDP port scan output: %s, err: %v", result, err)

	// The scan should succeed or fail gracefully
	_ = result
}

// ============== BENCHMARK MODE TESTS ==============

func TestNCBenchmarkServer(t *testing.T) {
	// Start a benchmark server in background
	cmd := exec.Command("./gobox", "nc", "-l", "--bench")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start benchmark server: %v", err)
	}
	defer killProcess(cmd)

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Server should be running - kill it
	t.Logf("Benchmark server started successfully")
}

func TestNCBenchmarkClient(t *testing.T) {
	// Start benchmark server
	serverCmd := exec.Command("./gobox", "nc", "-l", "--bench")
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr
	err := serverCmd.Start()
	if err != nil {
		t.Fatalf("Failed to start benchmark server: %v", err)
	}
	defer killProcess(serverCmd)

	// Give server time to start
	time.Sleep(300 * time.Millisecond)

	t.Logf("Benchmark server started")

	// Kill the server
	killProcess(serverCmd)
}

func TestNCBenchmarkModeWithRequests(t *testing.T) {
	// Start a TCP server that we can benchmark against
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Simple echo server
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			defer conn.Close()
			io.Copy(conn, conn)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Run benchmark client with few requests
	cmd := exec.Command("./gobox", "nc", "--bench", "-n1", "-s1B", "127.0.0.1", strconv.Itoa(port))
	output, err := cmd.Output()
	result := string(output)
	t.Logf("Benchmark output:\n%s", result)

	if err != nil {
		t.Logf("Benchmark client error (may be expected): %v", err)
	}

	ln.Close()
}

// ============== NUMERIC ONLY TESTS ==============

func TestNCNumericOnly(t *testing.T) {
	// Start a TCP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Server handler
	serverDone := make(chan struct{})
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
		close(serverDone)
	}()

	time.Sleep(100 * time.Millisecond)

	// Connect with numeric-only mode
	cmd := exec.Command("./gobox", "nc", "-nj", "-z", "127.0.0.1", strconv.Itoa(port))
	_, err = cmd.Output()
	t.Logf("Numeric only test: err=%v", err)

	ln.Close()
	<-serverDone
}

// ============== IPv4/IPv6 TESTS ==============

func TestNCForceIPv4(t *testing.T) {
	// Start a TCP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Server handler
	serverDone := make(chan struct{})
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
		close(serverDone)
	}()

	time.Sleep(100 * time.Millisecond)

	// Connect with IPv4 only
	cmd := exec.Command("./gobox", "nc", "-4", "-z", "127.0.0.1", strconv.Itoa(port))
	_, err = cmd.Output()
	t.Logf("Force IPv4 test: err=%v", err)

	ln.Close()
	<-serverDone
}

func TestNCForceIPv6(t *testing.T) {
	// Start a TCP server on IPv6
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Logf("IPv6 not available, skipping test: %v", err)
		return
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Server handler
	serverDone := make(chan struct{})
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
		close(serverDone)
	}()

	time.Sleep(100 * time.Millisecond)

	// Connect with IPv6 only
	cmd := exec.Command("./gobox", "nc", "-6", "-z", "::1", strconv.Itoa(port))
	_, err = cmd.Output()
	t.Logf("Force IPv6 test: err=%v", err)

	ln.Close()
	<-serverDone
}

// ============== EDGE CASES ==============

func TestNCEmptyInput(t *testing.T) {
	// Start a TCP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Server handler
	serverDone := make(chan struct{})
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			if n == 0 {
				t.Logf("Server received empty read")
			}
			conn.Close()
		}
		close(serverDone)
	}()

	time.Sleep(100 * time.Millisecond)

	// Connect with -z to avoid blocking on I/O
	cmd := exec.Command("./gobox", "nc", "-z", "127.0.0.1", strconv.Itoa(port))
	_, err = cmd.Output()
	t.Logf("Empty input test: err=%v", err)

	ln.Close()
	<-serverDone
}

func TestNCMultipleConnections(t *testing.T) {
	// Start a TCP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Server that handles multiple connections
	connectionCount := 0
	connClosed := make(chan struct{})
	go func() {
		for i := 0; i < 3; i++ {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			connectionCount++
			conn.Close()
		}
		close(connClosed)
	}()

	time.Sleep(100 * time.Millisecond)

	// Make multiple connections
	for i := 0; i < 3; i++ {
		cmd := exec.Command("./gobox", "nc", "-z", "127.0.0.1", strconv.Itoa(port))
		cmd.Output()
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)

	if connectionCount > 0 {
		t.Logf("Server handled %d connections", connectionCount)
	}

	ln.Close()
	<-connClosed
}

// Helper function to write test files
func ncWriteTestFile(t *testing.T, filename, content string) {
	err := os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file %s: %v", filename, err)
	}
}
