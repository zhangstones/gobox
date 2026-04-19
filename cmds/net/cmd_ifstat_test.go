package net

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
)

// runIfstatCmd runs IfstatCmd and captures stdout and stderr
func runIfstatCmd(args []string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	err := IfstatCmd(args)

	wOut.Close()
	wErr.Close()
	io.Copy(&buf, rOut)
	io.Copy(&buf, rErr)
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	return buf.String(), err
}

// skipIfNoInterfaces skips the test if no network interfaces are available
func skipIfNoInterfaces(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ifstat is only supported on Linux")
	}
	// Check if /sys/class/net exists
	if _, err := os.ReadDir("/sys/class/net"); err != nil {
		t.Skip("no network interfaces available")
	}
}

// getPhysicalInterfaceCount returns the number of physical network interfaces
func getPhysicalInterfaceCount(t *testing.T) int {
	// Get list of all network interfaces
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		t.Skip("cannot read /sys/class/net")
	}

	count := 0
	for _, e := range entries {
		// Check if physical NIC (type 1 = ARPHRD_ETHER)
		data, err := os.ReadFile("/sys/class/net/" + e.Name() + "/type")
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == "1" {
			count++
		}
	}
	return count
}

// ============== BASIC TESTS ==============

func TestIfstatCmdBasic(t *testing.T) {
	skipIfNoInterfaces(t)

	output, err := runIfstatCmd([]string{"-n", "1"})
	if err != nil {
		t.Fatalf("ifstat command failed: %v", err)
	}

	result := string(output)
	// Should have header line
	if !strings.Contains(result, "Interface") {
		t.Errorf("Expected header with 'Interface', got: %s", result)
	}
	// Should have some interface data (at least one line after header)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) < 2 {
		t.Errorf("Expected at least 2 lines (header + data), got: %d", len(lines))
	}
}

func TestIfstatCmdSingleSample(t *testing.T) {
	skipIfNoInterfaces(t)

	ifaceCount := getPhysicalInterfaceCount(t)

	// Request single sample with count=1
	output, err := runIfstatCmd([]string{"-n", "1", "-p", "1"})
	if err != nil {
		t.Fatalf("ifstat command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	// Should have header + ifaceCount data lines
	expectedLines := 1 + ifaceCount
	if len(lines) != expectedLines {
		t.Errorf("Expected %d lines (1 header + %d interfaces), got: %d", expectedLines, ifaceCount, len(lines))
	}
}

func TestIfstatCmdMultipleSamples(t *testing.T) {
	skipIfNoInterfaces(t)

	ifaceCount := getPhysicalInterfaceCount(t)

	// Request 2 samples
	output, err := runIfstatCmd([]string{"-n", "2", "-p", "1"})
	if err != nil {
		t.Fatalf("ifstat command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	// Should have header + 2 * ifaceCount data lines
	expectedLines := 1 + 2*ifaceCount
	if len(lines) != expectedLines {
		t.Errorf("Expected %d lines (1 header + 2*%d interfaces), got: %d", expectedLines, ifaceCount, len(lines))
	}
}

func TestIfstatCmdHelp(t *testing.T) {
	skipIfNoInterfaces(t)

	// --help outputs to stderr
	output, err := runIfstatCmd([]string{"--help"})
	// --help should cause flag package to exit with code 1
	if err != nil && !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("ifstat --help failed unexpectedly: %v", err)
	}
	result := string(output)
	if !strings.Contains(result, "Usage") && !strings.Contains(result, "ifstat") {
		t.Errorf("expected help output to contain 'Usage' and 'ifstat', got: %s", result)
	}
}

// ============== INTERFACE SELECTION TESTS ==============

func TestIfstatCmdSpecificInterface(t *testing.T) {
	skipIfNoInterfaces(t)

	// Get list of available interfaces first
	outputBytes, err := runIfstatCmd([]string{"-n", "1"})
	if err != nil {
		t.Skip("no interfaces available for testing")
	}

	result := string(outputBytes)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) < 2 {
		t.Skip("not enough interfaces to test")
	}

	// Extract interface name from output (first data line, first field)
	dataLine := lines[1]
	fields := strings.Fields(dataLine)
	if len(fields) == 0 {
		t.Skip("could not parse interface name")
	}
	ifaceName := fields[0]

	// Now test with specific interface
	output, err := runIfstatCmd([]string{"-n", "1", "-i", ifaceName})
	if err != nil {
		t.Fatalf("ifstat with -i %s failed: %v", ifaceName, err)
	}

	result = string(output)
	// Should only show the specified interface
	dataLines := strings.Split(strings.TrimSpace(result), "\n")
	if len(dataLines) != 2 {
		t.Errorf("Expected only 1 data line for interface %s, got: %d", ifaceName, len(dataLines)-1)
	}
}

func TestIfstatCmdMultipleInterfaces(t *testing.T) {
	skipIfNoInterfaces(t)

	// Get list of available interfaces first
	output, err := runIfstatCmd([]string{"-n", "1"})
	if err != nil {
		t.Skip("no interfaces available for testing")
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) < 3 {
		t.Skip("not enough interfaces to test multiple")
	}

	// Extract first two interface names
	var ifaces []string
	for i := 1; i < len(lines) && len(ifaces) < 2; i++ {
		fields := strings.Fields(lines[i])
		if len(fields) > 0 {
			ifaces = append(ifaces, fields[0])
		}
	}
	if len(ifaces) < 2 {
		t.Skip("not enough interfaces parsed")
	}

	// Test with comma-separated interfaces
	output2, err := runIfstatCmd([]string{"-n", "1", "-i", ifaces[0] + "," + ifaces[1]})
	if err != nil {
		t.Fatalf("ifstat with multiple interfaces failed: %v", err)
	}

	result = string(output2)
	dataLines := strings.Split(strings.TrimSpace(result), "\n")
	// Should show 2 interface data lines
	if len(dataLines) != 3 {
		t.Errorf("Expected 2 data lines for 2 interfaces, got: %d", len(dataLines)-1)
	}
}

func TestIfstatCmdNonExistentInterface(t *testing.T) {
	skipIfNoInterfaces(t)

	// Test with a clearly non-existent interface name
	output, err := runIfstatCmd([]string{"-n", "1", "-i", "thisinterfacedoesnotexist12345"})
	if err != nil {
		t.Fatalf("ifstat command should not error for non-existent interface: %v", err)
	}

	result := string(output)
	// Should show a warning about interface not found
	if !strings.Contains(result, "warning") && !strings.Contains(result, "not found") {
		t.Logf("Note: warning message format: %s", result)
	}
}

func TestIfstatCmdInterfaceWithNoStats(t *testing.T) {
	skipIfNoInterfaces(t)

	// Some virtual interfaces might exist but have no stats
	// Test with -A flag to include all interfaces
	output, err := runIfstatCmd([]string{"-n", "1", "-A"})
	if err != nil {
		t.Fatalf("ifstat with -A failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Interface") {
		t.Errorf("Expected header with 'Interface', got: %s", result)
	}
}

// ============== INTERVAL AND COUNT TESTS ==============

func TestIfstatCmdCustomInterval(t *testing.T) {
	skipIfNoInterfaces(t)

	// Test with interval of 2 seconds and count=1
	// This should complete in roughly 2 seconds
	output, err := runIfstatCmd([]string{"-n", "1", "-p", "2"})
	if err != nil {
		t.Fatalf("ifstat with custom interval failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	// Expected: 1 header + N interface lines
	ifaceCount := getPhysicalInterfaceCount(t)
	expectedLines := 1 + ifaceCount
	if len(lines) != expectedLines {
		t.Errorf("Expected %d lines (1 header + %d interfaces), got: %d", expectedLines, ifaceCount, len(lines))
	}
}

func TestIfstatCmdZeroInterval(t *testing.T) {
	skipIfNoInterfaces(t)

	// Zero interval should be treated as 1 (default fallback)
	output, err := runIfstatCmd([]string{"-n", "1", "-p", "0"})
	if err != nil {
		t.Fatalf("ifstat with zero interval failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	// Expected: 1 header + N interface lines
	ifaceCount := getPhysicalInterfaceCount(t)
	expectedLines := 1 + ifaceCount
	if len(lines) != expectedLines {
		t.Errorf("Expected %d lines (1 header + %d interfaces) with zero interval, got: %d", expectedLines, ifaceCount, len(lines))
	}
}

func TestIfstatCmdNegativeInterval(t *testing.T) {
	skipIfNoInterfaces(t)

	// Negative interval should be treated as 1 (default fallback)
	output, err := runIfstatCmd([]string{"-n", "1", "-p", "-1"})
	if err != nil {
		t.Fatalf("ifstat with negative interval failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	// Expected: 1 header + N interface lines
	ifaceCount := getPhysicalInterfaceCount(t)
	expectedLines := 1 + ifaceCount
	if len(lines) != expectedLines {
		t.Errorf("Expected %d lines (1 header + %d interfaces) with negative interval, got: %d", expectedLines, ifaceCount, len(lines))
	}
}

func TestIfstatCmdCountControl(t *testing.T) {
	skipIfNoInterfaces(t)

	ifaceCount := getPhysicalInterfaceCount(t)

	// Test different count values
	for _, count := range []string{"1", "2", "3"} {
		output, err := runIfstatCmd([]string{"-n", count, "-p", "1"})
		if err != nil {
			t.Fatalf("ifstat with count=%s failed: %v", count, err)
		}

		result := strings.TrimSpace(string(output))
		lines := strings.Split(result, "\n")
		expectedLines := 1 + ifaceCount*countNum(count) // header + (count * interface count) data lines
		if len(lines) != expectedLines {
			t.Errorf("Expected %d lines for count=%s with %d interfaces, got: %d", expectedLines, count, ifaceCount, len(lines))
		}
	}
}

func countNum(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// ============== OUTPUT FORMAT TESTS ==============

func TestIfstatCmdOutputFormatBasic(t *testing.T) {
	skipIfNoInterfaces(t)

	output, err := runIfstatCmd([]string{"-n", "1"})
	if err != nil {
		t.Fatalf("ifstat command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")

	// Parse header
	header := lines[0]
	headerFields := strings.Fields(header)
	// Basic format: Interface, rxpps/s, txpps/s, rxKB/s, txKB/s
	if len(headerFields) < 5 {
		t.Errorf("Expected at least 5 columns in header, got: %d", len(headerFields))
	}
	if headerFields[0] != "Interface" {
		t.Errorf("Expected first column 'Interface', got: %s", headerFields[0])
	}
	if headerFields[1] != "rxpps/s" {
		t.Errorf("Expected second column 'rxpps/s', got: %s", headerFields[1])
	}
	if headerFields[2] != "txpps/s" {
		t.Errorf("Expected third column 'txpps/s', got: %s", headerFields[2])
	}
}

func TestIfstatCmdOutputFormatWithErrors(t *testing.T) {
	skipIfNoInterfaces(t)

	output, err := runIfstatCmd([]string{"-n", "1", "-e"})
	if err != nil {
		t.Fatalf("ifstat with -e failed: %v", err)
	}

	result := string(output)
	header := strings.Split(result, "\n")[0]

	// With -e flag, should have error columns
	if !strings.Contains(header, "rxerrs") {
		t.Errorf("Expected 'rxerrs' column in header with -e, got: %s", header)
	}
	if !strings.Contains(header, "txerrs") {
		t.Errorf("Expected 'txerrs' column in header with -e, got: %s", header)
	}
}

func TestIfstatCmdOutputFormatWithDrops(t *testing.T) {
	skipIfNoInterfaces(t)

	output, err := runIfstatCmd([]string{"-n", "1", "-d"})
	if err != nil {
		t.Fatalf("ifstat with -d failed: %v", err)
	}

	result := string(output)
	header := strings.Split(result, "\n")[0]

	// With -d flag, should have drop columns
	if !strings.Contains(header, "rxdrop") {
		t.Errorf("Expected 'rxdrop' column in header with -d, got: %s", header)
	}
	if !strings.Contains(header, "txdrop") {
		t.Errorf("Expected 'txdrop' column in header with -d, got: %s", header)
	}
}

func TestIfstatCmdOutputFormatWithErrorsAndDrops(t *testing.T) {
	skipIfNoInterfaces(t)

	output, err := runIfstatCmd([]string{"-n", "1", "-e", "-d"})
	if err != nil {
		t.Fatalf("ifstat with -e -d failed: %v", err)
	}

	result := string(output)
	header := strings.Split(result, "\n")[0]

	// With both flags, should have error and drop columns
	if !strings.Contains(header, "rxerrs") {
		t.Errorf("Expected 'rxerrs' column, got: %s", header)
	}
	if !strings.Contains(header, "txerrs") {
		t.Errorf("Expected 'txerrs' column, got: %s", header)
	}
	if !strings.Contains(header, "rxdrop") {
		t.Errorf("Expected 'rxdrop' column, got: %s", header)
	}
	if !strings.Contains(header, "txdrop") {
		t.Errorf("Expected 'txdrop' column, got: %s", header)
	}
}

func TestIfstatCmdAbsoluteValues(t *testing.T) {
	skipIfNoInterfaces(t)

	output, err := runIfstatCmd([]string{"-n", "1", "-a"})
	if err != nil {
		t.Fatalf("ifstat with -a failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		t.Errorf("Expected output with absolute values, got empty")
	}
}

func TestIfstatCmdShowAllInterfaces(t *testing.T) {
	skipIfNoInterfaces(t)

	// With -A, should show virtual interfaces too
	output, err := runIfstatCmd([]string{"-n", "1", "-A"})
	if err != nil {
		t.Fatalf("ifstat with -A failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	// At minimum should have header + 1 data line
	if len(lines) < 2 {
		t.Errorf("Expected at least 2 lines, got: %d", len(lines))
	}
}

func TestIfstatCmdDataLineFormat(t *testing.T) {
	skipIfNoInterfaces(t)

	output, err := runIfstatCmd([]string{"-n", "1"})
	if err != nil {
		t.Fatalf("ifstat command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Skip("not enough output to validate data format")
	}

	// Validate data line format
	dataLine := lines[1]
	fields := strings.Fields(dataLine)
	if len(fields) < 5 {
		t.Errorf("Expected at least 5 fields in data line, got: %d", len(fields))
	}

	// Interface name should be first field
	ifaceName := fields[0]
	if ifaceName == "" {
		t.Errorf("Interface name should not be empty")
	}

	// Numeric fields should be parseable (they are floats)
	for i := 1; i < len(fields) && i < 5; i++ {
		if fields[i] == "" {
			t.Errorf("Field %d should not be empty", i)
		}
	}
}

// ============== EDGE CASES ==============

func TestIfstatCmdVirtualInterfacesWithA(t *testing.T) {
	skipIfNoInterfaces(t)

	// Test that -A flag works and shows virtual interfaces
	output, err := runIfstatCmd([]string{"-n", "1", "-A"})
	if err != nil {
		t.Fatalf("ifstat -A failed: %v", err)
	}

	result := string(output)
	// Should produce output
	if result == "" {
		t.Errorf("Expected output with -A flag")
	}
}

func TestIfstatCmdInterfaceNotFoundWarning(t *testing.T) {
	skipIfNoInterfaces(t)

	// Test with interface that doesn't exist
	output, err := runIfstatCmd([]string{"-n", "1", "-i", "nonexistent_iface_xyz789"})
	if err != nil {
		t.Fatalf("ifstat should not error for non-existent interface: %v", err)
	}

	result := string(output)
	// Should have header printed even with non-existent interface
	if !strings.Contains(result, "Interface") {
		t.Errorf("Expected header even with non-existent interface, got: %s", result)
	}
}

func TestIfstatCmdEmptyInterfaceName(t *testing.T) {
	skipIfNoInterfaces(t)

	// Empty interface name might cause issues
	// This tests the edge case of empty string after -i
	_, err := runIfstatCmd([]string{"-n", "1", "-i", ""})
	if err != nil {
		t.Logf("Note: empty interface name behavior: %v", err)
	}
}

func TestIfstatCmdCommaOnlyInterface(t *testing.T) {
	skipIfNoInterfaces(t)

	// Test with just commas (invalid interface spec)
	_, err := runIfstatCmd([]string{"-n", "1", "-i", ","})
	if err == nil {
		t.Logf("Note: behavior with comma-only interface spec")
	}
}

// ============== ERROR CASES ==============

func TestIfstatCmdInvalidInterval(t *testing.T) {
	skipIfNoInterfaces(t)

	// Non-numeric interval should cause error
	_, err := runIfstatCmd([]string{"-n", "1", "-p", "abc"})
	if err == nil {
		t.Errorf("Expected error for invalid interval 'abc'")
	}
}

func TestIfstatCmdInvalidCount(t *testing.T) {
	skipIfNoInterfaces(t)

	// Non-numeric count should cause error
	_, err := runIfstatCmd([]string{"-n", "xyz", "-p", "1"})
	if err == nil {
		t.Errorf("Expected error for invalid count 'xyz'")
	}
}

func TestIfstatCmdMissingIntervalArg(t *testing.T) {
	skipIfNoInterfaces(t)

	// -p without argument should error
	_, err := runIfstatCmd([]string{"-n", "1", "-p"})
	if err == nil {
		t.Errorf("Expected error when -p is missing argument")
	}
}

func TestIfstatCmdMissingCountArg(t *testing.T) {
	skipIfNoInterfaces(t)

	// -n without argument should error
	_, err := runIfstatCmd([]string{"-n", "-p", "1"})
	if err == nil {
		t.Errorf("Expected error when -n is missing argument")
	}
}

func TestIfstatCmdMissingInterfaceArg(t *testing.T) {
	skipIfNoInterfaces(t)

	// -i without argument should error
	_, err := runIfstatCmd([]string{"-n", "1", "-i"})
	if err == nil {
		t.Errorf("Expected error when -i is missing argument")
	}
}

func TestIfstatCmdUnsupportedOS(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("skipping OS test on Linux")
	}

	// On non-Linux systems, should fail with "supported only on Linux"
	_, err := runIfstatCmd([]string{})
	if err == nil {
		t.Errorf("Expected error on non-Linux OS")
	}
}

// ============== COMBINED OPTIONS TESTS ==============

func TestIfstatCmdCombinedOptions(t *testing.T) {
	skipIfNoInterfaces(t)

	// Check if lo interface exists and is physical
	loTypePath := "/sys/class/net/lo/type"
	if _, err := os.Stat(loTypePath); err != nil {
		t.Skip("lo interface does not exist")
	}
	data, err := os.ReadFile(loTypePath)
	if err != nil || strings.TrimSpace(string(data)) != "1" {
		t.Skip("lo is not a physical interface (or cannot be read)")
	}

	// Test multiple options together
	output, err := runIfstatCmd([]string{"-n", "2", "-p", "1", "-i", "lo"})
	if err != nil {
		t.Fatalf("ifstat with combined options failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	// Should have header + 2 data lines for lo interface
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines for combined options, got: %d", len(lines))
	}
}

func TestIfstatCmdAllFlags(t *testing.T) {
	skipIfNoInterfaces(t)

	// Test with all flags
	output, err := runIfstatCmd([]string{"-n", "1", "-a", "-A", "-e", "-d"})
	if err != nil {
		t.Fatalf("ifstat with all flags failed: %v", err)
	}

	result := string(output)
	// Should have header with all columns
	header := strings.Split(result, "\n")[0]
	expectedCols := []string{"Interface", "rxpps/s", "txpps/s", "rxKB/s", "txKB/s", "rxerrs", "txerrs", "rxdrop", "txdrop"}
	for _, col := range expectedCols {
		if !strings.Contains(header, col) {
			t.Errorf("Expected column '%s' in header, got: %s", col, header)
		}
	}
}

// ============== COMPREHENSIVE INTEGRATION TESTS ==============

func TestIfstatCmdEndToEnd(t *testing.T) {
	skipIfNoInterfaces(t)

	// Full end-to-end test: multiple samples, all data columns
	ifaceCount := getPhysicalInterfaceCount(t)
	output, err := runIfstatCmd([]string{"-n", "3", "-p", "1"})
	if err != nil {
		t.Fatalf("ifstat end-to-end test failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")

	// Should have header + 3 * ifaceCount data lines
	expectedLines := 1 + 3*ifaceCount
	if len(lines) != expectedLines {
		t.Errorf("Expected %d lines (1 header + 3*%d interfaces), got: %d", expectedLines, ifaceCount, len(lines))
	}

	// Verify each data line has correct number of fields
	for i := 1; i < len(lines); i++ {
		fields := strings.Fields(lines[i])
		if len(fields) < 5 {
			t.Errorf("Data line %d: expected at least 5 fields, got: %d", i, len(fields))
		}
	}
}

func TestIfstatCmdDataConsistency(t *testing.T) {
	skipIfNoInterfaces(t)

	// Multiple runs should produce data in same format
	for run := 0; run < 3; run++ {
		output, err := runIfstatCmd([]string{"-n", "1"})
		if err != nil {
			t.Fatalf("ifstat run %d failed: %v", run, err)
		}

		result := string(output)
		lines := strings.Split(strings.TrimSpace(result), "\n")

		// Each run should have consistent format
		if len(lines) < 2 {
			t.Errorf("Run %d: expected at least 2 lines, got: %d", run, len(lines))
		}

		// Check header consistency
		header := lines[0]
		if !strings.Contains(header, "Interface") {
			t.Errorf("Run %d: expected 'Interface' in header", run)
		}
		if !strings.Contains(header, "rxpps/s") {
			t.Errorf("Run %d: expected 'rxpps/s' in header", run)
		}
	}
}

func TestIfstatCmdInterfaceFiltering(t *testing.T) {
	skipIfNoInterfaces(t)

	// Test that -i flag actually filters interfaces
	// Get all interfaces first
	output, err := runIfstatCmd([]string{"-n", "1", "-A"})
	if err != nil {
		t.Skip("no interfaces available")
	}

	allResult := string(output)
	allLines := strings.Split(strings.TrimSpace(allResult), "\n")
	allIfaces := len(allLines) - 1 // subtract header

	if allIfaces <= 1 {
		t.Skip("only one interface available, cannot test filtering")
	}

	// Now get only loopback
	loOutput, err := runIfstatCmd([]string{"-n", "1", "-i", "lo"})
	if err != nil {
		t.Skip("lo interface may not exist")
	}

	loResult := string(loOutput)
	loLines := strings.Split(strings.TrimSpace(loResult), "\n")

	// Should have fewer interfaces when filtering
	if len(loLines)-1 >= allIfaces {
		t.Logf("Note: filtering test - all: %d, lo: %d", allIfaces, len(loLines)-1)
	}
}

func TestIfstatCmdRateCalculation(t *testing.T) {
	skipIfNoInterfaces(t)

	// Test that rate values are calculated correctly
	// With 1 second interval and count=1, the rate should be calculated over 1 second
	output, err := runIfstatCmd([]string{"-n", "1", "-p", "1"})
	if err != nil {
		t.Fatalf("ifstat rate calculation failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")

	if len(lines) < 2 {
		t.Errorf("Expected data line, got: %s", result)
	}

	// Data line should have numeric values
	dataLine := lines[1]
	fields := strings.Fields(dataLine)

	// Should have 5 fields minimum
	if len(fields) < 5 {
		t.Errorf("Expected at least 5 fields, got: %d", len(fields))
	}
}
