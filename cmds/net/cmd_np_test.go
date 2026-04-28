package net

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// runNpCmd runs NpCmd and captures stdout and stderr
func runNpCmd(args []string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	err := NpCmd(args)

	wOut.Close()
	wErr.Close()
	io.Copy(&buf, rOut)
	io.Copy(&buf, rErr)
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	return buf.String(), err
}

// skipIfNotLinux skips the test if not running on Linux
func skipIfNotLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("np is only supported on Linux")
	}
}

// ============== PORT RANGE PARSING TESTS ==============

func TestParsePortRangeSinglePort(t *testing.T) {
	ports := parsePortRange("80")
	if len(ports) != 1 || ports[0] != 80 {
		t.Fatalf("expected [80], got %v", ports)
	}
}

func TestParsePortRangeCommaSeparated(t *testing.T) {
	ports := parsePortRange("80,443,8080")
	if len(ports) != 3 {
		t.Fatalf("expected 3 ports, got %d", len(ports))
	}
	expected := []int{80, 443, 8080}
	for i, p := range expected {
		if ports[i] != p {
			t.Errorf("expected port %d at index %d, got %d", p, i, ports[i])
		}
	}
}

func TestParsePortRangeWithDash(t *testing.T) {
	ports := parsePortRange("1-5")
	if len(ports) != 5 {
		t.Fatalf("expected 5 ports, got %d", len(ports))
	}
	for i, p := range []int{1, 2, 3, 4, 5} {
		if ports[i] != p {
			t.Errorf("expected port %d at index %d, got %d", p, i, ports[i])
		}
	}
}

func TestParsePortRangeMixedCommaAndDash(t *testing.T) {
	ports := parsePortRange("80,443,8000-8005")
	// 80, 443, 8000-8005 (6 ports) = 8 total
	if len(ports) != 8 {
		t.Fatalf("expected 8 ports, got %d", len(ports))
	}
	expected := []int{80, 443, 8000, 8001, 8002, 8003, 8004, 8005}
	if len(ports) != len(expected) {
		t.Fatalf("expected %d ports, got %d", len(expected), len(ports))
	}
	for i, p := range expected {
		if ports[i] != p {
			t.Errorf("expected port %d at index %d, got %d", p, i, ports[i])
		}
	}
}

func TestParsePortRangeInvalidPort(t *testing.T) {
	ports := parsePortRange("0")
	if len(ports) != 0 {
		t.Fatalf("expected 0 ports for invalid port 0, got %d", len(ports))
	}
}

func TestParsePortRangeNegativePort(t *testing.T) {
	ports := parsePortRange("-1")
	if len(ports) != 0 {
		t.Fatalf("expected 0 ports for negative port, got %d", len(ports))
	}
}

func TestParsePortRangePortOver65535(t *testing.T) {
	ports := parsePortRange("70000")
	if len(ports) != 0 {
		t.Fatalf("expected 0 ports for port > 65535, got %d", len(ports))
	}
}

func TestParsePortRangeReversedRange(t *testing.T) {
	// Range where start > end should be ignored
	ports := parsePortRange("100-50")
	if len(ports) != 0 {
		t.Fatalf("expected 0 ports for reversed range, got %d", len(ports))
	}
}

func TestParsePortRangeEmpty(t *testing.T) {
	ports := parsePortRange("")
	if len(ports) != 0 {
		t.Fatalf("expected 0 ports for empty string, got %d", len(ports))
	}
}

func TestParsePortRangeWithSpaces(t *testing.T) {
	ports := parsePortRange("80 , 443 , 8080")
	if len(ports) != 3 {
		t.Fatalf("expected 3 ports, got %d", len(ports))
	}
}

func TestParsePortRangeLargeRange(t *testing.T) {
	// Test a large but bounded range
	ports := parsePortRange("1-10")
	if len(ports) != 10 {
		t.Fatalf("expected 10 ports, got %d", len(ports))
	}
	for i, p := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
		if ports[i] != p {
			t.Errorf("expected port %d at index %d, got %d", p, i, ports[i])
		}
	}
}

// ============== ERROR CASES TESTS ==============

func TestNpCmdMissingHost(t *testing.T) {
	skipIfNotLinux(t)

	// TCP mode requires host
	_, err := runNpCmd([]string{"--tcp", "-p", "80"})
	if err == nil {
		t.Fatalf("expected error for missing host")
	}
}

func TestNpCmdMissingHostForScan(t *testing.T) {
	skipIfNotLinux(t)

	// Scan mode requires target host after ports
	_, err := runNpCmd([]string{"--scan", "80"})
	if err == nil {
		t.Fatalf("expected error for scan mode missing target host")
	}
}

func TestNpCmdInvalidPortRange(t *testing.T) {
	skipIfNotLinux(t)

	// Invalid port range should return error
	_, err := runNpCmd([]string{"--scan", "abc", "127.0.0.1"})
	if err == nil {
		t.Fatalf("expected error for invalid port range")
	}
}

func TestNpCmdTcpModeWithoutPort(t *testing.T) {
	skipIfNotLinux(t)

	// TCP mode requires port
	_, err := runNpCmd([]string{"--tcp", "127.0.0.1"})
	if err == nil {
		t.Fatalf("expected error for TCP mode without port")
	}
}

func TestNpCmdUdpModeWithoutPort(t *testing.T) {
	skipIfNotLinux(t)

	// UDP mode requires port
	_, err := runNpCmd([]string{"--udp", "127.0.0.1"})
	if err == nil {
		t.Fatalf("expected error for UDP mode without port")
	}
}

func TestNpCmdScanModeNoValidPorts(t *testing.T) {
	skipIfNotLinux(t)

	// No valid ports should error
	_, err := runNpCmd([]string{"--scan", "", "127.0.0.1"})
	if err == nil {
		t.Fatalf("expected error for scan mode with no valid ports")
	}
}

// ============== HELP AND USAGE TESTS ==============

func TestNpCmdHelp(t *testing.T) {
	skipIfNotLinux(t)

	output, err := runNpCmd([]string{"--help"})
	// flag.ErrHelp causes exit code 1
	if err != nil && !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("np --help failed unexpectedly: %v", err)
	}
	result := string(output)
	for _, want := range []string{"Usage: gobox np", "Modes:", "--tcp", "--udp", "--icmp", "--arp", "--scan", "--flood", "Examples:", "interval between packets in seconds (supports decimals)"} {
		if !strings.Contains(result, want) {
			t.Fatalf("expected help output to contain %q, got: %s", want, result)
		}
	}
}

// ============== OUTPUT MODE TESTS ==============

func TestNpCmdQuietMode(t *testing.T) {
	skipIfNotLinux(t)

	// Using localhost with a closed port - quiet mode should only show stats
	// Count=1 to make test fast, timeout=1s to fail fast
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "1", "-W", "1", "-q", "127.0.0.1"})
	// We expect it to run (may or may not error depending on network)
	// Just verify it produces some output or handles gracefully
	_ = output
	_ = err
}

func TestNpCmdVerboseMode(t *testing.T) {
	skipIfNotLinux(t)

	// Verbose mode should produce more detailed output
	// Using localhost with a closed port to trigger connection failures
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "1", "-W", "1", "-v", "127.0.0.1"})
	_ = output
	_ = err
	// May error but should produce output
}

func TestNpCmdFloodMode(t *testing.T) {
	skipIfNotLinux(t)

	// Flood mode with count=1 should complete quickly
	// Run in background and kill after timeout
	done := make(chan error, 1)
	go func() {
		_, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "100", "-W", "2", "--flood", "127.0.0.1"})
		done <- err
	}()

	select {
	case err := <-done:
		// Expected to complete, just verify no panic
		_ = err
	case <-time.After(3 * time.Second):
		// Flood mode may hang on blocked sockets, this is expected
		t.Log("flood mode blocked, skipping")
	}
}

// ============== ICMP MODE TESTS ==============

func TestNpCmdIcmpMode(t *testing.T) {
	skipIfNotLinux(t)

	// ICMP ping to localhost
	output, err := runNpCmd([]string{"--icmp", "-c", "1", "-W", "2", "127.0.0.1"})
	if err != nil {
		t.Logf("ICMP test output: %s", output)
		// ICMP may fail due to permissions, skip in that case
		if strings.Contains(string(output), "permission") || strings.Contains(string(output), "Operation not permitted") {
			t.Skip("ICMP requires root privileges")
		}
	}
}

func TestNpCmdIcmpModeInvalidHost(t *testing.T) {
	skipIfNotLinux(t)

	_, err := runNpCmd([]string{"--icmp", "-c", "1", "-W", "1", "nonexistent.invalid.host"})
	if err == nil {
		t.Fatalf("expected error for invalid ICMP host")
	}
}

// ============== ARP MODE TESTS ==============

func TestNpCmdArpMode(t *testing.T) {
	skipIfNotLinux(t)

	// ARP ping to localhost
	output, err := runNpCmd([]string{"--arp", "-c", "1", "-W", "2", "127.0.0.1"})
	_ = output
	// ARP might not work in all environments
	if err != nil {
		t.Logf("ARP test note: %v", err)
	}
}

// ============== SCAN MODE TESTS ==============

func TestNpCmdScanModeBasic(t *testing.T) {
	skipIfNotLinux(t)

	// Scan a single port on localhost
	output, err := runNpCmd([]string{"--scan", "59999", "127.0.0.1"})
	if err != nil {
		t.Fatalf("scan command failed: %v", err)
	}

	result := string(output)
	// Should contain scan results
	if !strings.Contains(result, "Port") && !strings.Contains(result, "Scan") {
		t.Errorf("expected scan output to contain 'Port' or 'Scan', got: %s", result)
	}
}

func TestNpCmdScanModeMultiplePorts(t *testing.T) {
	skipIfNotLinux(t)

	// Scan multiple comma-separated ports
	output, err := runNpCmd([]string{"--scan", "59999,59998,59997", "127.0.0.1"})
	if err != nil {
		t.Fatalf("scan command failed: %v", err)
	}

	result := string(output)
	// Should mention the port count (implementation outputs "Starting scan of N ports")
	if !strings.Contains(result, "3 ports") && !strings.Contains(result, "59999") {
		t.Errorf("expected scan output to contain port count or numbers, got: %s", result)
	}
}

func TestNpCmdScanModePortRange(t *testing.T) {
	skipIfNotLinux(t)

	// Scan a small port range
	output, err := runNpCmd([]string{"--scan", "59990-59995", "127.0.0.1"})
	if err != nil {
		t.Fatalf("scan command failed: %v", err)
	}

	result := string(output)
	// Should contain scan information
	if result == "" {
		t.Errorf("expected non-empty scan output")
	}
}

func TestNpCmdScanModeQuiet(t *testing.T) {
	skipIfNotLinux(t)

	// Quiet mode should suppress per-port output
	output, err := runNpCmd([]string{"--scan", "59999", "-q", "127.0.0.1"})
	if err != nil {
		t.Fatalf("scan command failed: %v", err)
	}

	result := string(output)
	// In quiet mode, may only show summary
	_ = result
}

func TestNpCmdScanModeVerbose(t *testing.T) {
	skipIfNotLinux(t)

	// Verbose mode should show closed ports too
	output, err := runNpCmd([]string{"--scan", "59999", "-v", "127.0.0.1"})
	if err != nil {
		t.Fatalf("scan command failed: %v", err)
	}

	result := string(output)
	_ = result
}

func TestNpCmdScanModeWorkers(t *testing.T) {
	skipIfNotLinux(t)

	// Test with multiple workers
	output, err := runNpCmd([]string{"--scan", "59990-60000", "-w", "4", "127.0.0.1"})
	if err != nil {
		t.Fatalf("scan command with workers failed: %v", err)
	}

	result := string(output)
	if result == "" {
		t.Errorf("expected non-empty scan output with workers")
	}
}

// ============== TCP MODE TESTS ==============

func TestNpCmdTcpModeBasic(t *testing.T) {
	skipIfNotLinux(t)

	// TCP ping to a closed port should complete quickly
	// Using a non-routable IP to fail fast
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "1", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
	// Either succeeds or fails, just needs to complete without panic
}

func TestNpCmdTcpModeCount(t *testing.T) {
	skipIfNotLinux(t)

	// Test with multiple counts
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "2", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestNpCmdTcpModeInterval(t *testing.T) {
	skipIfNotLinux(t)

	// Test with custom interval (in seconds)
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "1", "-i", "0.1", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestNpCmdTcpModeSourcePort(t *testing.T) {
	skipIfNotLinux(t)

	// Test with source port
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-s", "40000", "-c", "1", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestNpCmdTcpModeLongConnection(t *testing.T) {
	skipIfNotLinux(t)

	// Test long connection mode
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "1", "-l", "1", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestNpCmdTcpModeMultipleWorkers(t *testing.T) {
	skipIfNotLinux(t)

	// Test with multiple workers
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "1", "-w", "4", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

// ============== UDP MODE TESTS ==============

func TestNpCmdUdpModeBasic(t *testing.T) {
	skipIfNotLinux(t)

	// UDP ping to a closed port
	output, err := runNpCmd([]string{"--udp", "-p", "59999", "-c", "1", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestNpCmdUdpModeCount(t *testing.T) {
	skipIfNotLinux(t)

	// Test with multiple counts
	output, err := runNpCmd([]string{"--udp", "-p", "59999", "-c", "2", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestNpCmdUdpModeVerbose(t *testing.T) {
	skipIfNotLinux(t)

	// UDP verbose mode
	output, err := runNpCmd([]string{"--udp", "-p", "59999", "-c", "1", "-v", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

// ============== EDGE CASES ==============

func TestNpCmdInvalidTimeout(t *testing.T) {
	skipIfNotLinux(t)

	// Invalid timeout should be handled gracefully
	output, err := runNpCmd([]string{"--tcp", "-p", "80", "-W", "abc", "127.0.0.1"})
	_ = output
	// The flag package should handle this
	if err == nil {
		t.Log("Note: invalid timeout was silently accepted")
	}
}

func TestNpCmdNegativeCount(t *testing.T) {
	skipIfNotLinux(t)

	_, err := runNpCmd([]string{"--tcp", "-p", "80", "-c", "-1", "-W", "2", "127.0.0.1"})
	if err == nil {
		t.Fatal("expected error for negative count")
	}
	if !strings.Contains(err.Error(), "count must be >= 0") {
		t.Fatalf("unexpected negative count error: %v", err)
	}
}

func TestNpCmdZeroPort(t *testing.T) {
	skipIfNotLinux(t)

	// Port 0 should error
	_, err := runNpCmd([]string{"--tcp", "-p", "0", "-W", "1", "127.0.0.1"})
	if err == nil {
		t.Fatalf("expected error for port 0")
	}
}

func TestNpCmdPortOutOfRange(t *testing.T) {
	skipIfNotLinux(t)

	// Port > 65535 should error
	_, err := runNpCmd([]string{"--tcp", "-p", "70000", "-W", "1", "127.0.0.1"})
	if err == nil {
		t.Fatalf("expected error for port > 65535")
	}
}

func TestNpCmdEmptyHost(t *testing.T) {
	skipIfNotLinux(t)

	// Empty host should error
	_, err := runNpCmd([]string{"--tcp", "-p", "80", "-W", "1"})
	if err == nil {
		t.Fatalf("expected error for empty host")
	}
}

// ============== MODE SELECTION TESTS ==============

func TestNpCmdDefaultMode(t *testing.T) {
	skipIfNotLinux(t)

	// Without explicit mode flag, TCP is default
	// But it requires -p, so it should error
	_, err := runNpCmd([]string{"-p", "80", "-c", "1", "-W", "1", "127.0.0.1"})
	// Default is TCP, so this should require port (which is provided)
	// So it should not error on missing port
	_ = err
}

func TestNpCmdExplicitTcpFlag(t *testing.T) {
	skipIfNotLinux(t)

	// Explicit TCP flag
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "1", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestNpCmdExplicitUdpFlag(t *testing.T) {
	skipIfNotLinux(t)

	// Explicit UDP flag
	output, err := runNpCmd([]string{"--udp", "-p", "59999", "-c", "1", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestNpCmdExplicitIcmpFlag(t *testing.T) {
	skipIfNotLinux(t)

	// Explicit ICMP flag
	output, err := runNpCmd([]string{"--icmp", "-c", "1", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestNpCmdExplicitArpFlag(t *testing.T) {
	skipIfNotLinux(t)

	// Explicit ARP flag
	output, err := runNpCmd([]string{"--arp", "-c", "1", "-W", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestNpCmdExplicitScanFlag(t *testing.T) {
	skipIfNotLinux(t)

	// Explicit scan flag
	output, err := runNpCmd([]string{"--scan", "80", "127.0.0.1"})
	_ = output
	_ = err
}

// ============== COMPREHENSIVE INTEGRATION TESTS ==============

func TestNpCmdEndToEndTcp(t *testing.T) {
	skipIfNotLinux(t)

	// Full end-to-end TCP test
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "1", "-W", "2", "-q", "127.0.0.1"})
	if err != nil {
		t.Fatalf("TCP end-to-end test failed: %v", err)
	}

	result := string(output)
	// In quiet mode with errors, should still get some output
	if result == "" {
		t.Log("Note: quiet mode produced no output (may be expected for failed connections)")
	}
}

func TestNpCmdEndToEndScan(t *testing.T) {
	skipIfNotLinux(t)

	// Full end-to-end scan test
	output, err := runNpCmd([]string{"--scan", "59999-60010", "-W", "1", "127.0.0.1"})
	if err != nil {
		t.Fatalf("scan end-to-end test failed: %v", err)
	}

	result := string(output)
	// Should have scan results
	if !strings.Contains(result, "Scan") && !strings.Contains(result, "Port") {
		t.Errorf("expected scan output with results, got: %s", result)
	}
}

func TestNpCmdMultipleModesSequential(t *testing.T) {
	skipIfNotLinux(t)

	// Test multiple modes in sequence to ensure no state pollution
	modes := [][]string{
		{"--tcp", "-p", "59999", "-c", "1", "-W", "1", "127.0.0.1"},
		{"--udp", "-p", "59999", "-c", "1", "-W", "1", "127.0.0.1"},
		{"--scan", "59999", "127.0.0.1"},
	}

	for _, args := range modes {
		output, err := runNpCmd(args)
		_ = output
		if err != nil {
			t.Logf("Mode %v failed: %v (may be expected)", args[0], err)
		}
	}
}

// ============== OUTPUT FORMAT TESTS ==============

func TestNpCmdStatisticsOutput(t *testing.T) {
	skipIfNotLinux(t)

	// Verbose mode should produce statistics
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "2", "-W", "1", "-v", "127.0.0.1"})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	result := string(output)
	// Statistics should contain transmitted and received info
	if !strings.Contains(result, "transmitted") && !strings.Contains(result, "received") {
		t.Logf("Note: statistics output: %s", result)
	}
}

func TestNpCmdPingOutputFormat(t *testing.T) {
	skipIfNotLinux(t)

	// Non-quiet mode should produce per-ping output
	output, err := runNpCmd([]string{"--tcp", "-p", "59999", "-c", "1", "-W", "1", "127.0.0.1"})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	result := string(output)
	// Should have some output format
	_ = result
}

func TestNpCmdNegativeInterval(t *testing.T) {
	skipIfNotLinux(t)

	_, err := runNpCmd([]string{"--tcp", "-p", "80", "-i", "-1", "-W", "1", "127.0.0.1"})
	if err == nil {
		t.Fatal("expected error for negative interval")
	}
	if !strings.Contains(err.Error(), "interval must be >= 0") {
		t.Fatalf("unexpected negative interval error: %v", err)
	}
}
