package text

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runRandCmd runs RandCmd with args and captures stdout
func runRandCmd(args []string) (string, error) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RandCmd(args)

	w.Close()
	io.Copy(&buf, r)
	os.Stdout = old
	return buf.String(), err
}

// runRandCmdWithStdin runs RandCmd with stdin input and captures stdout
func runRandCmdWithStdin(args []string, stdinInput string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stdin = rIn

	go func() {
		wIn.WriteString(stdinInput)
		wIn.Close()
	}()

	err := RandCmd(args)

	wOut.Close()
	io.Copy(&buf, rOut)
	os.Stdout = oldStdout
	os.Stdin = oldStdin
	return buf.String(), err
}

// ============== NORMAL CASES TESTS ==============

func TestRandCmdDefault(t *testing.T) {
	output, err := runRandCmd([]string{})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	// Default is 32 bytes, hex output (64 hex chars + newline)
	result := strings.TrimSpace(output)
	if len(result) != 64 {
		t.Errorf("Expected 64 hex characters for 32 bytes, got %d: %s", len(result), result)
	}

	// Verify it's valid hex
	_, err = hex.DecodeString(result)
	if err != nil {
		t.Errorf("Output is not valid hex: %s", result)
	}
}

func TestRandCmdHexExplicit(t *testing.T) {
	output, err := runRandCmd([]string{"-hex"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	if len(result) != 64 {
		t.Errorf("Expected 64 hex characters for 32 bytes, got %d", len(result))
	}

	// Verify it's valid hex
	_, err = hex.DecodeString(result)
	if err != nil {
		t.Errorf("Output is not valid hex: %s", result)
	}
}

func TestRandCmdBase64(t *testing.T) {
	output, err := runRandCmd([]string{"-base64"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 32 bytes in base64 is approximately 44 chars (4 chars per 3 bytes, with padding)
	if len(result) < 40 || len(result) > 50 {
		t.Errorf("Expected approximately 44 base64 characters for 32 bytes, got %d: %s", len(result), result)
	}

	// Verify it's valid base64
	_, err = base64.StdEncoding.DecodeString(result)
	if err != nil {
		t.Errorf("Output is not valid base64: %s", result)
	}
}

func TestRandCmdBytes16(t *testing.T) {
	output, err := runRandCmd([]string{"-n", "16"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 16 bytes = 32 hex chars
	if len(result) != 32 {
		t.Errorf("Expected 32 hex characters for 16 bytes, got %d: %s", len(result), result)
	}

	// Verify it's valid hex
	_, err = hex.DecodeString(result)
	if err != nil {
		t.Errorf("Output is not valid hex: %s", result)
	}
}

func TestRandCmdBytes8(t *testing.T) {
	output, err := runRandCmd([]string{"-n", "8"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 8 bytes = 16 hex chars
	if len(result) != 16 {
		t.Errorf("Expected 16 hex characters for 8 bytes, got %d: %s", len(result), result)
	}

	// Verify it's valid hex
	_, err = hex.DecodeString(result)
	if err != nil {
		t.Errorf("Output is not valid hex: %s", result)
	}
}

func TestRandCmdPositionalArg(t *testing.T) {
	output, err := runRandCmd([]string{"16"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 16 bytes = 32 hex chars
	if len(result) != 32 {
		t.Errorf("Expected 32 hex characters for 16 bytes, got %d: %s", len(result), result)
	}
}

func TestRandCmdCombinedFlags(t *testing.T) {
	// Test -n16 (combined flag)
	output, err := runRandCmd([]string{"-n16"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	if len(result) != 32 {
		t.Errorf("Expected 32 hex characters for 16 bytes, got %d", len(result))
	}
}

func TestRandCmdShortFlagN(t *testing.T) {
	// Test -n with separate argument
	output, err := runRandCmd([]string{"-n", "10"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 10 bytes = 20 hex chars
	if len(result) != 20 {
		t.Errorf("Expected 20 hex characters for 10 bytes, got %d", len(result))
	}
}

func TestRandCmdBase64Explicit(t *testing.T) {
	output, err := runRandCmd([]string{"-base64"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 32 bytes = approximately 44 base64 chars
	if len(result) < 40 || len(result) > 50 {
		t.Errorf("Expected approximately 44 base64 characters for 32 bytes, got %d", len(result))
	}
}

func TestRandCmdBase64WithBytes(t *testing.T) {
	output, err := runRandCmd([]string{"-n", "24", "-base64"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 24 bytes in base64 = 32 chars exactly (24 * 8 / 6 = 32)
	if len(result) != 32 {
		t.Errorf("Expected 32 base64 characters for 24 bytes, got %d: %s", len(result), result)
	}

	// Verify it's valid base64
	_, err = base64.StdEncoding.DecodeString(result)
	if err != nil {
		t.Errorf("Output is not valid base64: %s", result)
	}
}

// ============== EDGE CASES TESTS ==============

func TestRandCmdZeroBytes(t *testing.T) {
	output, err := runRandCmd([]string{"-n", "0"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 0 bytes = empty string
	if result != "" {
		t.Errorf("Expected empty output for 0 bytes, got: %s", result)
	}
}

func TestRandCmdZeroBytesHex(t *testing.T) {
	output, err := runRandCmd([]string{"-n", "0", "-hex"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	if result != "" {
		t.Errorf("Expected empty output for 0 bytes with -hex, got: %s", result)
	}
}

func TestRandCmdZeroBytesBase64(t *testing.T) {
	output, err := runRandCmd([]string{"-n", "0", "-base64"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	if result != "" {
		t.Errorf("Expected empty output for 0 bytes with -base64, got: %s", result)
	}
}

func TestRandCmdLargeBytes(t *testing.T) {
	output, err := runRandCmd([]string{"-n", "1024"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 1024 bytes = 2048 hex chars
	if len(result) != 2048 {
		t.Errorf("Expected 2048 hex characters for 1024 bytes, got %d", len(result))
	}

	// Verify it's valid hex
	_, err = hex.DecodeString(result)
	if err != nil {
		t.Errorf("Output is not valid hex: %s", result)
	}
}

func TestRandCmdVeryLargeBytes(t *testing.T) {
	output, err := runRandCmd([]string{"-n", "4096"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 4096 bytes = 8192 hex chars
	if len(result) != 8192 {
		t.Errorf("Expected 8192 hex characters for 4096 bytes, got %d", len(result))
	}

	// Verify it's valid hex
	_, err = hex.DecodeString(result)
	if err != nil {
		t.Errorf("Output is not valid hex: %s", err)
	}
}

func TestRandCmdLargeBytesBase64(t *testing.T) {
	output, err := runRandCmd([]string{"-n", "1024", "-base64"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 1024 bytes in base64 = 1372 chars (1024 * 8 / 6 = 1365.33, rounded up to 1368 with padding)
	// Actually base64 encoding: 1024 bytes -> ceil(1024/3) * 4 = 341 * 4 = 1364 chars
	if len(result) < 1300 || len(result) > 1400 {
		t.Errorf("Expected approximately 1364 base64 characters for 1024 bytes, got %d", len(result))
	}

	// Verify it's valid base64
	_, err = base64.StdEncoding.DecodeString(result)
	if err != nil {
		t.Errorf("Output is not valid base64: %s", result)
	}
}

// ============== OUTPUT VERIFICATION TESTS ==============

func TestRandCmdHexOutputFormat(t *testing.T) {
	output, err := runRandCmd([]string{"-n", "32"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)

	// Must be exactly 64 hex characters
	if len(result) != 64 {
		t.Errorf("Hex output length incorrect: got %d, want 64", len(result))
	}

	// Verify all characters are valid hex
	for _, c := range result {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Invalid hex character: %c", c)
		}
	}
}

func TestRandCmdBase64OutputFormat(t *testing.T) {
	output, err := runRandCmd([]string{"-n", "32", "-base64"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)

	// Must be valid base64 characters
	for _, c := range result {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=') {
			t.Errorf("Invalid base64 character: %c", c)
		}
	}

	// Verify it decodes to 32 bytes
	decoded, err := base64.StdEncoding.DecodeString(result)
	if err != nil {
		t.Errorf("Failed to decode base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("Decoded length incorrect: got %d, want 32", len(decoded))
	}
}

func TestRandCmdDifferentOutputs(t *testing.T) {
	// Run multiple times and verify outputs are different (random)
	output1, err := runRandCmd([]string{"-n", "32"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	output2, err := runRandCmd([]string{"-n", "32"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	if output1 == output2 {
		t.Errorf("Two rand outputs should be different, got same: %s", strings.TrimSpace(output1))
	}
}

func TestRandCmdHexVsBase64Different(t *testing.T) {
	// Same seed/number should produce same random bytes, but different encoding
	// Note: Since we can't control the seed, we just verify both produce valid output
	hexOutput, err := runRandCmd([]string{"-n", "32", "-hex"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	base64Output, err := runRandCmd([]string{"-n", "32", "-base64"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	hexResult := strings.TrimSpace(hexOutput)
	base64Result := strings.TrimSpace(base64Output)

	// Both should be valid and non-empty
	if hexResult == "" || base64Result == "" {
		t.Error("Both outputs should be non-empty")
	}

	// They should be different strings
	if hexResult == base64Result {
		t.Error("Hex and base64 outputs should be different")
	}
}

// ============== FILE OUTPUT TESTS ==============

func TestRandCmdOutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "rand_output.txt")

	_, err := runRandCmd([]string{"-n", "32", "-out", outputFile})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Cannot read output file: %v", err)
	}

	result := strings.TrimSpace(string(data))
	if len(result) != 64 {
		t.Errorf("Expected 64 hex characters in file for 32 bytes, got %d", len(result))
	}

	// Verify it's valid hex
	_, err = hex.DecodeString(result)
	if err != nil {
		t.Errorf("File content is not valid hex: %s", result)
	}
}

func TestRandCmdOutputFileBase64(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "rand_output.txt")

	_, err := runRandCmd([]string{"-n", "24", "-base64", "-out", outputFile})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Cannot read output file: %v", err)
	}

	result := strings.TrimSpace(string(data))
	if len(result) != 32 {
		t.Errorf("Expected 32 base64 characters in file for 24 bytes, got %d", len(result))
	}

	// Verify it's valid base64
	_, err = base64.StdEncoding.DecodeString(result)
	if err != nil {
		t.Errorf("File content is not valid base64: %s", result)
	}
}

func TestRandCmdOutputFileLarge(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "rand_large.txt")

	_, err := runRandCmd([]string{"-n", "1024", "-out", outputFile})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Cannot read output file: %v", err)
	}

	result := strings.TrimSpace(string(data))
	// 1024 bytes = 2048 hex chars
	if len(result) != 2048 {
		t.Errorf("Expected 2048 hex characters in file for 1024 bytes, got %d", len(result))
	}
}

func TestRandCmdOutputFileOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "rand_overwrite.txt")

	// Create initial file
	err := os.WriteFile(outputFile, []byte("initial content"), 0644)
	if err != nil {
		t.Fatalf("Cannot create initial file: %v", err)
	}

	// Run rand to overwrite
	_, err = runRandCmd([]string{"-n", "16", "-out", outputFile})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	// Verify file was overwritten
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Cannot read output file: %v", err)
	}

	result := strings.TrimSpace(string(data))
	if len(result) != 32 {
		t.Errorf("Expected 32 hex characters after overwrite, got %d", len(result))
	}

	// Verify it doesn't contain old content
	if strings.Contains(string(data), "initial") {
		t.Error("File should not contain old 'initial' content")
	}
}

func TestRandCmdOutputFileZeroBytes(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "rand_zero.txt")

	_, err := runRandCmd([]string{"-n", "0", "-out", outputFile})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	// Verify file exists but is empty (or just newline since Fprintln adds \n)
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Cannot read output file: %v", err)
	}

	// 0 bytes generates empty string, but Fprintln adds a newline
	// So file should contain just "\n" (1 byte)
	if len(data) != 1 || data[0] != '\n' {
		t.Errorf("Expected single newline for 0 bytes, got %d bytes: %v", len(data), data)
	}
}

// ============== ERROR CASES TESTS ==============

func TestRandCmdNegativeBytes(t *testing.T) {
	_, err := runRandCmd([]string{"-n", "-5"})
	if err == nil {
		t.Errorf("Expected error for negative bytes")
	}
}

func TestRandCmdInvalidBytes(t *testing.T) {
	_, err := runRandCmd([]string{"-n", "abc"})
	if err == nil {
		t.Errorf("Expected error for invalid bytes argument")
	}
}

func TestRandCmdNegativePositional(t *testing.T) {
	_, err := runRandCmd([]string{"-10"})
	if err == nil {
		t.Errorf("Expected error for negative positional argument")
	}
}

func TestRandCmdInvalidPositional(t *testing.T) {
	_, err := runRandCmd([]string{"xyz"})
	if err == nil {
		t.Errorf("Expected error for invalid positional argument")
	}
}

func TestRandCmdOutRequiresArg(t *testing.T) {
	_, err := runRandCmd([]string{"-out"})
	if err == nil {
		t.Errorf("Expected error when -out has no argument")
	}
}

func TestRandCmdUnknownOption(t *testing.T) {
	_, err := runRandCmd([]string{"-q"})
	if err == nil {
		t.Errorf("Expected error for unknown option")
	}
}

func TestRandCmdHelp(t *testing.T) {
	output, err := runRandCmd([]string{"--help"})
	if err != nil {
		t.Fatalf("rand --help failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information, got: %s", result)
	}
	if !strings.Contains(result, "gobox rand") {
		t.Errorf("Expected 'gobox rand' in help, got: %s", result)
	}
}

func TestRandCmdHelpShort(t *testing.T) {
	output, err := runRandCmd([]string{"-h"})
	if err != nil {
		t.Fatalf("rand -h failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information, got: %s", result)
	}
}

// ============== COMBINED FLAGS TESTS ==============

func TestRandCmdCombinedShortFlags(t *testing.T) {
	// Test -n24 -b (24 bytes with base64)
	output, err := runRandCmd([]string{"-n24", "-b"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 24 bytes in base64 = 32 chars
	if len(result) != 32 {
		t.Errorf("Expected 32 base64 chars for 24 bytes, got %d: %s", len(result), result)
	}
}

func TestRandCmdCombinedNHex(t *testing.T) {
	// Test -n16 -hex (16 bytes with hex explicit)
	output, err := runRandCmd([]string{"-n16", "-hex"})
	if err != nil {
		t.Fatalf("rand command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// 16 bytes = 32 hex chars
	if len(result) != 32 {
		t.Errorf("Expected 32 hex chars for 16 bytes, got %d", len(result))
	}
}
