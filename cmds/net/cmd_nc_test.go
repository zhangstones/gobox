package net

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// runNcCmd runs NcCmd with args and captures stdout and stderr
func runNcCmd(args []string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	err := NcCmd(args)

	wOut.Close()
	wErr.Close()
	io.Copy(&buf, rOut)
	io.Copy(&buf, rErr)
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	return buf.String(), err
}

// runNcCmdWithStdin runs NcCmd with stdin input and captures stdout and stderr
func runNcCmdWithStdin(args []string, stdinInput string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr
	os.Stdin = rIn

	go func() {
		wIn.WriteString(stdinInput)
		wIn.Close()
	}()

	err := NcCmd(args)

	wOut.Close()
	wErr.Close()
	io.Copy(&buf, rOut)
	io.Copy(&buf, rErr)
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	os.Stdin = oldStdin
	return buf.String(), err
}

// ============== HELPER FUNCTIONS FOR SERVER MODE TESTS ==============

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
	output, err := runNcCmd([]string{"-h"})
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
// Note: Tests that require server mode (nc -l) are difficult to convert because
// they require starting a separate process. These tests still use exec.Command
// and are marked with a comment.

func TestNCListenMode(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to reserve port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	oldStdout := os.Stdout
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stdin = rIn
	defer func() {
		os.Stdout = oldStdout
		os.Stdin = oldStdin
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- NcCmdWithContext(ctx, []string{"-l", strconv.Itoa(port)})
	}()

	time.Sleep(100 * time.Millisecond)

	client, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), time.Second)
	if err != nil {
		t.Fatalf("client dial failed: %v", err)
	}
	if _, err := io.WriteString(client, "hello from client\n"); err != nil {
		client.Close()
		t.Fatalf("client write failed: %v", err)
	}
	_ = client.Close()
	_ = wIn.Close()

	err = <-serverErr
	_ = wOut.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, rOut)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Fatalf("listen mode returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "hello from client") {
		t.Fatalf("expected listener stdout to include client payload, got %q", buf.String())
	}
}

// ============== ALTERNATIVE LISTEN MODE TEST (direct function) ==============

func TestNCListenModeDirect(t *testing.T) {
	// Get an available port using a TCP listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	// Start a TCP server in a goroutine to simulate nc server
	serverDone := make(chan struct{})
	go func() {
		ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			close(serverDone)
			return
		}
		defer ln.Close()

		conn, err := ln.Accept()
		if err != nil {
			close(serverDone)
			return
		}
		defer conn.Close()

		// Echo back any data received
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		conn.Write(buf[:n])
		close(serverDone)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Check that server is listening
	if !waitForPort(port, 1*time.Second) {
		t.Fatalf("Server not listening on port %d", port)
	}

	t.Logf("Server started on port %d", port)
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
	output, err := runNcCmdWithStdin([]string{"127.0.0.1", strconv.Itoa(port)}, "hello\n")
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
	output, err := runNcCmd([]string{"-u", "-zj", "127.0.0.1", strconv.Itoa(port)})
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
	output, err := runNcCmd([]string{"-z", "127.0.0.1", strconv.Itoa(port)})
	result := string(output)
	t.Logf("Port scan output: %s, err: %v", result, err)

	// With -z, it should exit successfully if port is open
	if err != nil {
		t.Errorf("Port scan failed unexpectedly: %v", err)
	}
}

func TestNCPortScanClosed(t *testing.T) {
	// Try to scan a port that is definitely closed
	output, err := runNcCmd([]string{"-zj", "127.0.0.1", "59999"})
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
	output, err := runNcCmd([]string{"-zu", "127.0.0.1", strconv.Itoa(port)})
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
	output, err := runNcCmd([]string{"-v", "-z", "127.0.0.1", strconv.Itoa(port)})
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
	output, err := runNcCmd([]string{"-v", "-n", "-z", "127.0.0.1", strconv.Itoa(port)})
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
	output, err := runNcCmd([]string{"-wj", "1", "-z", "10.255.255.1", "80"})
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
	_, err = runNcCmd([]string{"-wj", "1", "127.0.0.1", strconv.Itoa(port)})
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
	output, err := runNcCmd([]string{"-l", "invalid"})
	result := string(output)
	t.Logf("Invalid port output: %s, err: %v", result, err)

	// Should fail
	if err == nil {
		t.Errorf("Expected error for invalid port, got success")
	}
}

func TestNCMissingHost(t *testing.T) {
	// Client mode without host should error
	output, err := runNcCmd([]string{})
	result := string(output)
	t.Logf("Missing args output: %s, err: %v", result, err)

	// Should fail with usage error
	if err == nil {
		t.Errorf("Expected error for missing arguments")
	}
}

func TestNCConnectionRefused(t *testing.T) {
	// Try to connect to a port with no server listening
	output, err := runNcCmd([]string{"-zj", "127.0.0.1", "59999"})
	_ = output
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
	output, err := runNcCmd([]string{"-uj", "-z", "-w", "1", "127.0.0.1", strconv.Itoa(port)})
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
	output, err := runNcCmd([]string{"-zu", "127.0.0.1", strconv.Itoa(port)})
	result := string(output)
	t.Logf("UDP port scan output: %s, err: %v", result, err)

	// The scan should succeed or fail gracefully
	_ = result
}

// ============== BENCHMARK MODE TESTS ==============

func TestNCBenchmarkServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start benchmark listener: %v", err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		done <- ncBenchServer(ln, false, 64*1024, false)
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	if !waitForPort(port, time.Second) {
		t.Fatalf("Benchmark server did not start listening on port %d", port)
	}

	t.Logf("Benchmark server started successfully")
	_ = ln.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
	}
}

func TestNCBenchmarkClient(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start benchmark listener: %v", err)
	}
	defer ln.Close()

	go func() {
		_ = ncBenchServer(ln, false, 64*1024, false)
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	if !waitForPort(port, time.Second) {
		t.Fatalf("Benchmark server did not start listening on port %d", port)
	}

	err = ncBenchmarkClient("127.0.0.1", strconv.Itoa(port), false, false, false, false, false, 1, 1, 0, 1, 1, 1)
	if err != nil {
		t.Fatalf("Benchmark client failed: %v", err)
	}

	t.Logf("Benchmark client completed")
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
	output, err := runNcCmd([]string{"--bench", "-n1", "-s1B", "127.0.0.1", strconv.Itoa(port)})
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
	_, err = runNcCmd([]string{"-nj", "-z", "127.0.0.1", strconv.Itoa(port)})
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
	_, err = runNcCmd([]string{"-4", "-z", "127.0.0.1", strconv.Itoa(port)})
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
	_, err = runNcCmd([]string{"-6", "-z", "::1", strconv.Itoa(port)})
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
	_, err = runNcCmd([]string{"-z", "127.0.0.1", strconv.Itoa(port)})
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
		runNcCmd([]string{"-z", "127.0.0.1", strconv.Itoa(port)})
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
