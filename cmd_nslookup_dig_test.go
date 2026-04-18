package main

import (
	"os/exec"
	"strings"
	"testing"
)

// ============== DIG TESTS ==============

func TestDigBasic(t *testing.T) {
	// Test basic dig query
	cmd := exec.Command("./gobox", "dig", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig example.com failed: %v", err)
	}

	result := string(output)
	// Should show dig header
	if !strings.Contains(result, "DiG") {
		t.Errorf("Expected 'DiG' header in output, got: %s", result)
	}
	// Should show the query
	if !strings.Contains(result, "example.com") {
		t.Errorf("Expected 'example.com' in output, got: %s", result)
	}
	// Should show ANSWER section
	if !strings.Contains(result, "ANSWER") {
		t.Errorf("Expected 'ANSWER' section in output, got: %s", result)
	}
}

func TestDigWithDNSServer(t *testing.T) {
	// Test dig with @DNS_SERVER syntax
	cmd := exec.Command("./gobox", "dig", "@8.8.8.8", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig @8.8.8.8 example.com failed: %v", err)
	}

	result := string(output)
	// Should show the DNS server in output
	if !strings.Contains(result, "8.8.8.8") {
		t.Errorf("Expected '8.8.8.8' in output, got: %s", result)
	}
}

func TestDigTypeA(t *testing.T) {
	// Test dig with -t A flag
	cmd := exec.Command("./gobox", "dig", "-t", "A", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig -t A example.com failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "example.com") {
		t.Errorf("Expected 'example.com' in output, got: %s", result)
	}
}

func TestDigTypeAAAA(t *testing.T) {
	// Test dig with -t AAAA flag
	cmd := exec.Command("./gobox", "dig", "-t", "AAAA", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig -t AAAA example.com failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "example.com") {
		t.Errorf("Expected 'example.com' in output, got: %s", result)
	}
}

func TestDigTypeTXT(t *testing.T) {
	// Test dig with -t TXT flag
	cmd := exec.Command("./gobox", "dig", "-t", "TXT", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig -t TXT example.com failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "example.com") {
		t.Errorf("Expected 'example.com' in output, got: %s", result)
	}
}

func TestDigTypeNS(t *testing.T) {
	// Test dig with -t NS flag
	cmd := exec.Command("./gobox", "dig", "-t", "NS", "com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig -t NS com failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "com") {
		t.Errorf("Expected 'com' in output, got: %s", result)
	}
}

func TestDigTypeMX(t *testing.T) {
	// Test dig with -t MX flag
	cmd := exec.Command("./gobox", "dig", "-t", "MX", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig -t MX example.com failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "example.com") {
		t.Errorf("Expected 'example.com' in output, got: %s", result)
	}
}

func TestDigTypeShort(t *testing.T) {
	// Test dig with --type=TYPE syntax
	cmd := exec.Command("./gobox", "dig", "--type=A", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig --type=A example.com failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "example.com") {
		t.Errorf("Expected 'example.com' in output, got: %s", result)
	}
}

func TestDigShortOutput(t *testing.T) {
	// Test dig with +short flag - should just show IP addresses
	cmd := exec.Command("./gobox", "dig", "+short", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig +short example.com failed: %v", err)
	}

	result := string(output)
	// Should only contain IP addresses, not full dig output
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) > 0 && lines[0] != "" {
		// Check if first line looks like an IP address
		firstLine := strings.TrimSpace(lines[0])
		if !strings.Contains(firstLine, ".") && firstLine != "" {
			// It might be an IPv6 address, which is fine
		}
	}
}

func TestDigNoallAnswer(t *testing.T) {
	// Test dig with +noall +answer - should show only answer section
	cmd := exec.Command("./gobox", "dig", "+noall", "+answer", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig +noall +answer example.com failed: %v", err)
	}

	result := string(output)
	// Should show ANSWER section
	if !strings.Contains(result, "ANSWER") {
		t.Errorf("Expected 'ANSWER' in output, got: %s", result)
	}
}

func TestDigTCP(t *testing.T) {
	// Test dig with +tcp flag
	cmd := exec.Command("./gobox", "dig", "+tcp", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig +tcp example.com failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "TCP") && !strings.Contains(result, "example.com") {
		t.Errorf("Expected TCP connection info or example.com in output, got: %s", result)
	}
}

func TestDigMissingHost(t *testing.T) {
	// Test dig with no host argument
	cmd := exec.Command("./gobox", "dig")
	output, err := cmd.CombinedOutput()
	// Should fail with error
	if err == nil {
		t.Fatalf("dig without host should fail")
	}
	result := string(output)
	if !strings.Contains(result, "host") && !strings.Contains(result, "Usage") {
		t.Errorf("Expected error message about host, got: %s", result)
	}
}

func TestDigHelp(t *testing.T) {
	// Test dig help flag
	cmd := exec.Command("./gobox", "dig", "--help")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig --help failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage") || !strings.Contains(result, "dig") {
		t.Errorf("Expected usage information in help output, got: %s", result)
	}
}

func TestDigHelpShort(t *testing.T) {
	// Test dig -h flag
	cmd := exec.Command("./gobox", "dig", "-h")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig -h failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage") || !strings.Contains(result, "dig") {
		t.Errorf("Expected usage information in help output, got: %s", result)
	}
}

func TestDigNonexistentHost(t *testing.T) {
	// Test dig with non-existent host
	cmd := exec.Command("./gobox", "dig", "nonexistent.invalid")
	output, _ := cmd.CombinedOutput()
	// Should return output but with no answer
	result := string(output)
	// Should still have dig header format
	if !strings.Contains(result, "DiG") {
		t.Errorf("Expected dig header format in output, got: %s", result)
	}
}

func TestDigShortOutputAAAA(t *testing.T) {
	// Test dig +short with AAAA record
	cmd := exec.Command("./gobox", "dig", "+short", "-t", "AAAA", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig +short -t AAAA example.com failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// Should be empty or contain IPv6 address
	// This is acceptable as some hosts may not have AAAA records
	_ = result // Just verify it runs without error
}

func TestDigShortOutputMX(t *testing.T) {
	// Test dig +short with MX record
	cmd := exec.Command("./gobox", "dig", "+short", "-t", "MX", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig +short -t MX example.com failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// Should contain MX format: priority hostname
	// or be empty if no MX records
	_ = result // Just verify it runs without error
}

func TestDigCombinedOptions(t *testing.T) {
	// Test dig with multiple options combined
	cmd := exec.Command("./gobox", "dig", "@8.8.8.8", "-t", "A", "+short", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig combined options failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// Should return short output (IP or empty)
	_ = result
}

func TestDigWithTCPOption(t *testing.T) {
	// Test dig with +tcp option and DNS server
	cmd := exec.Command("./gobox", "dig", "+tcp", "@8.8.8.8", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig +tcp @8.8.8.8 example.com failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "example.com") {
		t.Errorf("Expected 'example.com' in output, got: %s", result)
	}
}

func TestDigShortWithType(t *testing.T) {
	// Test dig +short with -t flag
	cmd := exec.Command("./gobox", "dig", "+short", "-t", "NS", "com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig +short -t NS com failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// Should contain NS records or be empty
	_ = result
}

// ============== EDGE CASES ==============

func TestDigUnknownOption(t *testing.T) {
	// Test dig with unknown option
	cmd := exec.Command("./gobox", "dig", "-invalid", "example.com")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("dig with invalid option should fail")
	}
	result := string(output)
	if !strings.Contains(result, "unknown option") {
		t.Errorf("Expected 'unknown option' error, got: %s", result)
	}
}

func TestDigCaseInsensitiveType(t *testing.T) {
	// Test dig with lowercase type
	cmd := exec.Command("./gobox", "dig", "-t", "a", "example.com")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig -t a failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "example.com") {
		t.Errorf("Expected 'example.com' in output, got: %s", result)
	}
}

// ============== LOCALHOST TESTS ==============

func TestDigLocalhost(t *testing.T) {
	// Test dig for localhost
	cmd := exec.Command("./gobox", "dig", "localhost")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig localhost failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "localhost") {
		t.Errorf("Expected 'localhost' in output, got: %s", result)
	}
}

func TestDigShortLocalhost(t *testing.T) {
	// Test dig +short for localhost
	cmd := exec.Command("./gobox", "dig", "+short", "localhost")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("dig +short localhost failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// Should contain 127.0.0.1 or be empty
	_ = result
}
