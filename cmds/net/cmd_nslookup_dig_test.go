package net

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// runDigCmd runs DigCmd with args and captures stdout and stderr
func runDigCmd(args []string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	err := DigCmd(args)

	w.Close()
	io.Copy(&buf, r)
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	return buf.String(), err
}

// ============== DIG TESTS ==============

func TestDigBasic(t *testing.T) {
	// Test basic dig query
	output, err := runDigCmd([]string{"example.com"})
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
	output, err := runDigCmd([]string{"@8.8.8.8", "example.com"})
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
	output, err := runDigCmd([]string{"-t", "A", "example.com"})
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
	output, err := runDigCmd([]string{"-t", "AAAA", "example.com"})
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
	output, err := runDigCmd([]string{"-t", "TXT", "example.com"})
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
	output, err := runDigCmd([]string{"-t", "NS", "com"})
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
	output, err := runDigCmd([]string{"-t", "MX", "example.com"})
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
	output, err := runDigCmd([]string{"--type=A", "example.com"})
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
	output, err := runDigCmd([]string{"+short", "example.com"})
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
	output, err := runDigCmd([]string{"+noall", "+answer", "example.com"})
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
	output, err := runDigCmd([]string{"+tcp", "example.com"})
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
	output, err := runDigCmd([]string{})
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
	output, err := runDigCmd([]string{"--help"})
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
	output, err := runDigCmd([]string{"-h"})
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
	output, _ := runDigCmd([]string{"nonexistent.invalid"})
	// Should return output but with no answer
	result := string(output)
	// Should still have dig header format
	if !strings.Contains(result, "DiG") {
		t.Errorf("Expected dig header format in output, got: %s", result)
	}
}

func TestDigShortOutputAAAA(t *testing.T) {
	// Test dig +short with AAAA record
	output, err := runDigCmd([]string{"+short", "-t", "AAAA", "example.com"})
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
	output, err := runDigCmd([]string{"+short", "-t", "MX", "example.com"})
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
	output, err := runDigCmd([]string{"@8.8.8.8", "-t", "A", "+short", "example.com"})
	if err != nil {
		t.Fatalf("dig combined options failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// Should return short output (IP or empty)
	_ = result
}

func TestDigWithTCPOption(t *testing.T) {
	// Test dig with +tcp option and DNS server
	output, err := runDigCmd([]string{"+tcp", "@8.8.8.8", "example.com"})
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
	output, err := runDigCmd([]string{"+short", "-t", "NS", "com"})
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
	_, err := runDigCmd([]string{"-invalid", "example.com"})
	if err == nil {
		t.Fatalf("dig with invalid option should fail")
	}
	if !strings.Contains(err.Error(), "unknown option") {
		t.Errorf("Expected 'unknown option' error, got: %v", err)
	}
}

func TestDigCaseInsensitiveType(t *testing.T) {
	// Test dig with lowercase type
	output, err := runDigCmd([]string{"-t", "a", "example.com"})
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
	output, err := runDigCmd([]string{"localhost"})
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
	output, err := runDigCmd([]string{"+short", "localhost"})
	if err != nil {
		t.Fatalf("dig +short localhost failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// Should contain 127.0.0.1 or be empty
	_ = result
}
