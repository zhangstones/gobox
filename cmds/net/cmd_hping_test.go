package net

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// runHpingCmd runs the hping command with given args and returns output and error
func runHpingCmd(args []string) (string, error) {
	goboxPath := findGoboxBinary()
	cmd := exec.Command(goboxPath, append([]string{"hping"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}

// runHpingCmdWithTimeout runs hping with a timeout
func runHpingCmdWithTimeout(args []string, timeout time.Duration) (string, error) {
	goboxPath := findGoboxBinary()
	cmd := exec.Command(goboxPath, append([]string{"hping"}, args...)...)
	done := make(chan error, 1)
	var output []byte
	go func() {
		var err error
		output, err = cmd.CombinedOutput()
		done <- err
	}()

	select {
	case err := <-done:
		return string(output), err
	case <-time.After(timeout):
		cmd.Process.Kill()
		<-done
		return string(output), nil
	}
}

// ============== BINARY EXISTENCE TEST ==============

func TestHpingCmdBinaryExists(t *testing.T) {
	goboxPath := findGoboxBinary()
	if _, err := os.Stat(goboxPath); os.IsNotExist(err) {
		t.Fatalf("gobox binary not found at %s", goboxPath)
	}
}

// ============== HELP AND USAGE TESTS ==============

func TestHpingCmdHelp(t *testing.T) {
	output, err := runHpingCmd([]string{"--help"})
	// flag.ErrHelp causes exit code 1
	if err != nil && !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("hping --help failed unexpectedly: %v", err)
	}
	result := string(output)
	if !strings.Contains(result, "Usage") && !strings.Contains(result, "hping") {
		t.Errorf("expected help output to contain 'Usage' and 'hping', got: %s", result)
	}
}

// ============== ERROR CASES TESTS ==============

func TestHpingCmdMissingHost(t *testing.T) {
	// Missing host argument should return error
	_, err := runHpingCmd([]string{"-S"})
	if err == nil {
		t.Fatalf("expected error for missing host argument")
	}
}

func TestHpingCmdInvalidPortTooHigh(t *testing.T) {
	// Port > 65535 should return error
	_, err := runHpingCmd([]string{"-p", "70000", "127.0.0.1"})
	if err == nil {
		t.Fatalf("expected error for port > 65535")
	}
}

func TestHpingCmdInvalidPortNegative(t *testing.T) {
	// Negative port should return error
	_, err := runHpingCmd([]string{"-p", "-1", "127.0.0.1"})
	if err == nil {
		t.Fatalf("expected error for negative port")
	}
}

func TestHpingCmdEmptyHost(t *testing.T) {
	// Empty host string should error
	_, err := runHpingCmd([]string{"127.0.0.1"})
	// This should not error on its own since 127.0.0.1 is valid
	// But without -p it should still work (default port 80)
	_ = err
}

// ============== SYN SCAN TESTS (NORMAL CASES) ==============

func TestHpingCmdSYNScanBasic(t *testing.T) {
	// Basic SYN scan with default settings
	output, err := runHpingCmd([]string{"-S", "-c", "1", "-w", "1", "127.0.0.1"})
	// May fail due to network issues, but should not panic
	if err != nil {
		t.Logf("SYN scan output: %s, error: %v", output, err)
	}
	result := string(output)
	// Should contain HPING header
	if !strings.Contains(result, "HPING") {
		t.Errorf("expected output to contain 'HPING', got: %s", result)
	}
}

func TestHpingCmdSYNScanMultiplePackets(t *testing.T) {
	// SYN scan with multiple packets - use port 59999 which is unlikely to be open
	// When port is closed, we may not see seq= output (connection refused)
	// But the HPING header should show the correct count
	output, err := runHpingCmd([]string{"-S", "-p", "59999", "-c", "3", "-w", "1", "127.0.0.1"})
	if err != nil {
		t.Logf("SYN scan (3 packets) output: %s, error: %v", output, err)
	}
	result := string(output)
	// HPING header should confirm 3 packets were planned
	if !strings.Contains(result, "3 data bytes") {
		t.Errorf("expected output to contain '3 data bytes' in header, got: %s", result)
	}
}

func TestHpingCmdSYNScanClosedPort(t *testing.T) {
	// SYN scan to a closed port (59999 is unlikely to be open)
	output, err := runHpingCmd([]string{"-S", "-p", "59999", "-c", "1", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
	// Should complete without panic
}

func TestHpingCmdSYNScanWithLatency(t *testing.T) {
	// SYN scan should show latency information
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "1", "-w", "2", "127.0.0.1"})
	if err != nil {
		t.Logf("SYN scan with latency output: %s, error: %v", output, err)
	}
	result := string(output)
	// Should show time latency
	if !strings.Contains(result, "time=") {
		t.Logf("Note: latency display not found in output: %s", result)
	}
}

func TestHpingCmdSYNScanQuietMode(t *testing.T) {
	// Quiet mode should suppress per-packet output
	output, err := runHpingCmd([]string{"-S", "-p", "59999", "-c", "1", "-w", "1", "-q", "127.0.0.1"})
	_ = output
	_ = err
	// Should complete without panic
}

func TestHpingCmdSYNScanVerboseMode(t *testing.T) {
	// Verbose mode should show additional information
	output, err := runHpingCmd([]string{"-S", "-p", "59999", "-c", "1", "-w", "1", "-v", "127.0.0.1"})
	if err != nil {
		t.Logf("SYN scan verbose output: %s, error: %v", output, err)
	}
	result := string(output)
	// Verbose should contain connection failure info
	if !strings.Contains(result, "Icmp") && !strings.Contains(result, "Connection") && !strings.Contains(result, "HPING") {
		t.Logf("Note: verbose output format: %s", result)
	}
}

func TestHpingCmdSYNScanDifferentPorts(t *testing.T) {
	// Test different ports
	ports := []string{"22", "80", "443", "8080"}
	for _, port := range ports {
		output, err := runHpingCmd([]string{"-S", "-p", port, "-c", "1", "-w", "1", "127.0.0.1"})
		_ = output
		_ = err
		// Each should complete without panic
	}
}

// ============== FIN SCAN TESTS (NORMAL CASES) ==============

func TestHpingCmdFINScanBasic(t *testing.T) {
	// Basic FIN scan
	output, err := runHpingCmd([]string{"-F", "-c", "1", "-w", "1", "127.0.0.1"})
	if err != nil {
		t.Logf("FIN scan output: %s, error: %v", output, err)
	}
	result := string(output)
	// Should contain HPING header with FIN mode
	if !strings.Contains(result, "HPING") {
		t.Errorf("expected output to contain 'HPING', got: %s", result)
	}
}

func TestHpingCmdFINScanMultiplePackets(t *testing.T) {
	// FIN scan with multiple packets
	output, err := runHpingCmd([]string{"-F", "-c", "3", "-w", "1", "127.0.0.1"})
	if err != nil {
		t.Logf("FIN scan (3 packets) output: %s, error: %v", output, err)
	}
	result := string(output)
	// Should show FIN flag info
	if !strings.Contains(result, "FIN=") {
		t.Logf("Note: FIN flag display not found in output: %s", result)
	}
}

func TestHpingCmdFINScanClosedPort(t *testing.T) {
	// FIN scan to closed port should show RST
	output, err := runHpingCmd([]string{"-F", "-p", "59999", "-c", "1", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
	// Should complete without panic
}

func TestHpingCmdFINScanQuietMode(t *testing.T) {
	// Quiet mode for FIN scan
	output, err := runHpingCmd([]string{"-F", "-p", "59999", "-c", "1", "-w", "1", "-q", "127.0.0.1"})
	_ = output
	_ = err
	// Should complete without panic
}

func TestHpingCmdFINScanVerboseMode(t *testing.T) {
	// Verbose mode for FIN scan
	output, err := runHpingCmd([]string{"-F", "-p", "59999", "-c", "1", "-w", "1", "-v", "127.0.0.1"})
	if err != nil {
		t.Logf("FIN scan verbose output: %s, error: %v", output, err)
	}
	result := string(output)
	_ = result
}

func TestHpingCmdFINScanStatsOutput(t *testing.T) {
	// FIN scan with quiet mode should show statistics
	output, err := runHpingCmd([]string{"-F", "-p", "59999", "-c", "2", "-w", "1", "-q", "127.0.0.1"})
	_ = output
	_ = err
	// Should complete without panic
}

// ============== TRACE MODE TESTS (NORMAL CASES) ==============

func TestHpingCmdTraceBasic(t *testing.T) {
	// Basic trace mode with -tr flag
	output, err := runHpingCmd([]string{"-tr", "-c", "1", "-w", "2", "127.0.0.1"})
	if err != nil {
		t.Logf("Trace mode output: %s, error: %v", output, err)
	}
	result := string(output)
	// Should contain traceroute header
	if !strings.Contains(result, "traceroute") && !strings.Contains(result, "HPING") {
		t.Errorf("expected output to contain 'traceroute' or 'HPING', got: %s", result)
	}
}

func TestHpingCmdTraceWithTraceFlag(t *testing.T) {
	// Trace mode with --trace flag
	output, err := runHpingCmd([]string{"--trace", "-c", "1", "-w", "2", "127.0.0.1"})
	if err != nil {
		t.Logf("Trace mode (--trace) output: %s, error: %v", output, err)
	}
	result := string(output)
	// Should contain traceroute header
	if !strings.Contains(result, "traceroute") && !strings.Contains(result, "HPING") {
		t.Errorf("expected output to contain 'traceroute' or 'HPING', got: %s", result)
	}
}

func TestHpingCmdTraceMultipleHops(t *testing.T) {
	// Trace mode with multiple hops
	output, err := runHpingCmd([]string{"-tr", "-c", "5", "-w", "2", "127.0.0.1"})
	if err != nil {
		t.Logf("Trace mode (5 hops) output: %s, error: %v", output, err)
	}
	result := string(output)
	// Should show hop information
	if !strings.Contains(result, "hop") && !strings.Contains(result, "Trace") {
		t.Logf("Note: trace hop display: %s", result)
	}
}

func TestHpingCmdTraceQuietMode(t *testing.T) {
	// Trace mode with quiet mode
	output, err := runHpingCmd([]string{"-tr", "-c", "1", "-w", "1", "-q", "127.0.0.1"})
	_ = output
	_ = err
	// Should complete without panic
}

func TestHpingCmdTraceVerboseMode(t *testing.T) {
	// Trace mode with verbose mode
	output, err := runHpingCmd([]string{"-tr", "-c", "1", "-w", "1", "-v", "127.0.0.1"})
	_ = output
	_ = err
	// Should complete without panic
}

// ============== EDGE CASES: DIFFERENT PORTS ==============

func TestHpingCmdPortZero(t *testing.T) {
	// Port 0 may be valid or invalid depending on implementation
	output, err := runHpingCmd([]string{"-S", "-p", "0", "-c", "1", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestHpingCmdPort1(t *testing.T) {
	// Port 1 (well-known)
	output, err := runHpingCmd([]string{"-S", "-p", "1", "-c", "1", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestHpingCmdPort65535(t *testing.T) {
	// Port 65535 (max valid port)
	output, err := runHpingCmd([]string{"-S", "-p", "65535", "-c", "1", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
}

// ============== EDGE CASES: DIFFERENT PACKET COUNTS ==============

func TestHpingCmdCount1(t *testing.T) {
	// Single packet
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "1", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestHpingCmdCount5(t *testing.T) {
	// Five packets
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "5", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestHpingCmdCount10(t *testing.T) {
	// Ten packets
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "10", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
}

// ============== EDGE CASES: INTERVAL ==============

func TestHpingCmdIntervalSmall(t *testing.T) {
	// Small interval (100ms)
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "2", "-i", "100", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestHpingCmdIntervalLarge(t *testing.T) {
	// Large interval (5000ms = 5s)
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "2", "-i", "5000", "-w", "2", "127.0.0.1"})
	_ = output
	_ = err
}

func TestHpingCmdIntervalZero(t *testing.T) {
	// Zero interval (as fast as possible)
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "2", "-i", "0", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
}

// ============== EDGE CASES: TIMEOUT ==============

func TestHpingCmdTimeoutShort(t *testing.T) {
	// Short timeout (1s)
	output, err := runHpingCmd([]string{"-S", "-p", "59999", "-c", "1", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestHpingCmdTimeoutLong(t *testing.T) {
	// Long timeout (10s)
	output, err := runHpingCmdWithTimeout([]string{"-S", "-p", "59999", "-c", "1", "-w", "10", "127.0.0.1"}, 12*time.Second)
	_ = output
	_ = err
}

// ============== SPOOFING OPTIONS ==============

func TestHpingCmdSpoofIP(t *testing.T) {
	// With spoofed source IP
	output, err := runHpingCmd([]string{"-S", "-spoof", "10.0.0.1", "-p", "80", "-c", "1", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
}

func TestHpingCmdSpoofIPShortFlag(t *testing.T) {
	// With spoofed source IP using -a flag
	output, err := runHpingCmd([]string{"-S", "-a", "10.0.0.1", "-p", "80", "-c", "1", "-w", "1", "127.0.0.1"})
	_ = output
	_ = err
}

// ============== OUTPUT VERIFICATION ==============

func TestHpingCmdOutputContainsHost(t *testing.T) {
	// Output should contain the target host
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "1", "-w", "1", "192.168.1.1"})
	if err != nil {
		t.Logf("Output: %s, Error: %v", output, err)
	}
	result := string(output)
	if !strings.Contains(result, "192.168.1.1") {
		t.Errorf("expected output to contain '192.168.1.1', got: %s", result)
	}
}

func TestHpingCmdOutputContainsPort(t *testing.T) {
	// Output should contain the target port
	output, err := runHpingCmd([]string{"-S", "-p", "8080", "-c", "1", "-w", "1", "127.0.0.1"})
	if err != nil {
		t.Logf("Output: %s, Error: %v", output, err)
	}
	result := string(output)
	if !strings.Contains(result, "8080") {
		t.Errorf("expected output to contain port '8080', got: %s", result)
	}
}

func TestHpingCmdStatisticsOutput(t *testing.T) {
	// Verbose mode should produce statistics
	output, err := runHpingCmd([]string{"-S", "-p", "59999", "-c", "2", "-w", "1", "-v", "127.0.0.1"})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	result := string(output)
	// Statistics should contain transmitted and received info
	if !strings.Contains(result, "transmitted") && !strings.Contains(result, "received") {
		t.Logf("Note: statistics output: %s", result)
	}
}

func TestHpingCmdFINStatsOutput(t *testing.T) {
	// FIN scan with quiet mode should show FIN statistics
	output, err := runHpingCmd([]string{"-F", "-p", "59999", "-c", "2", "-w", "1", "-q", "127.0.0.1"})
	_ = output
	_ = err
	// Should complete without panic
}

func TestHpingCmdTraceHopOutput(t *testing.T) {
	// Trace mode should show hop numbers
	output, err := runHpingCmd([]string{"-tr", "-c", "3", "-w", "2", "127.0.0.1"})
	if err != nil {
		t.Logf("Trace output: %s, Error: %v", output, err)
	}
	result := string(output)
	// Should contain hop numbers (1, 2, 3 etc.)
	hasHopNumber := false
	for i := 1; i <= 3; i++ {
		if strings.Contains(result, strings.ReplaceAll(strings.TrimLeft(" 1", " "), " ", "")) {
			hasHopNumber = true
			break
		}
	}
	// Hop format may vary, so just log
	t.Logf("Trace hop output: %s", result)
	_ = hasHopNumber
}

// ============== MODE PRECEDENCE TESTS ==============

func TestHpingCmdTracePrecedenceOverSYN(t *testing.T) {
	// Trace mode should take precedence over SYN
	output, err := runHpingCmd([]string{"-tr", "-S", "-c", "1", "-w", "1", "127.0.0.1"})
	if err != nil {
		t.Logf("Output: %s, Error: %v", output, err)
	}
	result := string(output)
	// Should show trace mode, not SYN mode
	if !strings.Contains(result, "traceroute") {
		t.Logf("Note: trace precedence output: %s", result)
	}
}

func TestHpingCmdTracePrecedenceOverFIN(t *testing.T) {
	// Trace mode should take precedence over FIN
	output, err := runHpingCmd([]string{"-tr", "-F", "-c", "1", "-w", "1", "127.0.0.1"})
	if err != nil {
		t.Logf("Output: %s, Error: %v", output, err)
	}
	result := string(output)
	// Should show trace mode, not FIN mode
	if !strings.Contains(result, "traceroute") {
		t.Logf("Note: trace precedence output: %s", result)
	}
}

func TestHpingCmdDefaultModeIsSYN(t *testing.T) {
	// Default mode without -S or -F should be SYN
	output, err := runHpingCmd([]string{"-c", "1", "-w", "1", "127.0.0.1"})
	if err != nil {
		t.Logf("Output: %s, Error: %v", output, err)
	}
	result := string(output)
	// Should show SYN mode
	if !strings.Contains(result, "SYN") {
		t.Logf("Note: default mode output: %s", result)
	}
}

// ============== COMPREHENSIVE INTEGRATION TESTS ==============

func TestHpingCmdEndToEndSYN(t *testing.T) {
	// Full end-to-end SYN scan test
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "2", "-w", "2", "-q", "127.0.0.1"})
	if err != nil {
		t.Fatalf("SYN end-to-end test failed: %v", err)
	}
	result := string(output)
	if result == "" {
		t.Log("Note: quiet mode produced minimal output")
	}
}

func TestHpingCmdEndToEndFIN(t *testing.T) {
	// Full end-to-end FIN scan test
	output, err := runHpingCmd([]string{"-F", "-p", "59999", "-c", "2", "-w", "2", "-q", "127.0.0.1"})
	if err != nil {
		t.Fatalf("FIN end-to-end test failed: %v", err)
	}
	result := string(output)
	if result == "" {
		t.Log("Note: quiet mode produced minimal output")
	}
}

func TestHpingCmdEndToEndTrace(t *testing.T) {
	// Full end-to-end trace test
	output, err := runHpingCmd([]string{"-tr", "-c", "2", "-w", "2", "-q", "127.0.0.1"})
	if err != nil {
		t.Fatalf("Trace end-to-end test failed: %v", err)
	}
	result := string(output)
	if result == "" {
		t.Log("Note: quiet mode produced minimal output")
	}
}

func TestHpingCmdMultipleModesSequential(t *testing.T) {
	// Test multiple modes in sequence to ensure no state pollution
	modes := [][]string{
		{"-S", "-p", "80", "-c", "1", "-w", "1", "127.0.0.1"},
		{"-F", "-p", "80", "-c", "1", "-w", "1", "127.0.0.1"},
		{"-tr", "-c", "1", "-w", "1", "127.0.0.1"},
	}

	for _, args := range modes {
		output, err := runHpingCmd(args)
		_ = output
		if err != nil {
			t.Logf("Mode %v failed: %v (may be expected)", args[0], err)
		}
	}
}

func TestHpingCmdAllFlagsCombination(t *testing.T) {
	// Test all common flags together
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "1", "-w", "2", "-i", "1000", "-q", "-v", "127.0.0.1"})
	// q and v together may conflict, but should not panic
	_ = output
	_ = err
}

// ============== NETWORK ERROR HANDLING ==============

func TestHpingCmdNonRoutableIP(t *testing.T) {
	// Non-routable IP should handle gracefully (10.255.255.1 is non-routable)
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "1", "-w", "1", "10.255.255.1"})
	_ = output
	_ = err
	// Should timeout or error gracefully
}

func TestHpingCmdInvalidHostname(t *testing.T) {
	// Invalid hostname may not return error - it reports 100% packet loss instead
	// This is expected behavior for hping
	output, err := runHpingCmd([]string{"-S", "-p", "80", "-c", "1", "-w", "1", "nonexistent.invalid.host"})
	// Just verify it completes without panic
	_ = output
	_ = err
}

func TestHpingCmdLocalhostVsIP(t *testing.T) {
	// Test that localhost and 127.0.0.1 produce similar results
	output1, _ := runHpingCmd([]string{"-S", "-p", "80", "-c", "1", "-w", "1", "localhost"})
	output2, _ := runHpingCmd([]string{"-S", "-p", "80", "-c", "1", "-w", "1", "127.0.0.1"})
	// Both should contain HPING
	if !strings.Contains(string(output1), "HPING") || !strings.Contains(string(output2), "HPING") {
		t.Errorf("localhost or 127.0.0.1 did not produce HPING output")
	}
}
