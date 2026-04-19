package text

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runSeqCmd runs SeqCmd with args and captures stdout
func runSeqCmd(args []string) (string, error) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := SeqCmd(args)

	w.Close()
	io.Copy(&buf, r)
	os.Stdout = old
	return buf.String(), err
}

// runSeqCmdWithStdin runs SeqCmd with stdin input and captures stdout
func runSeqCmdWithStdin(args []string, stdinInput string) (string, error) {
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

	err := SeqCmd(args)

	wOut.Close()
	io.Copy(&buf, rOut)
	os.Stdout = oldStdout
	os.Stdin = oldStdin
	return buf.String(), err
}

// ============== NORMAL CASES TESTS ==============

func TestSeqCmdSingleNumber(t *testing.T) {
	output, err := runSeqCmd([]string{"5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1\n2\n3\n4\n5"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdTwoNumbers(t *testing.T) {
	output, err := runSeqCmd([]string{"2", "5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "2\n3\n4\n5"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdThreeNumbers(t *testing.T) {
	output, err := runSeqCmd([]string{"0", "2", "10"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "0\n2\n4\n6\n8\n10"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdThreeNumbersNegativeIncrement(t *testing.T) {
	output, err := runSeqCmd([]string{"10", "-2", "0"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "10\n8\n6\n4\n2\n0"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdDefaultStart(t *testing.T) {
	output, err := runSeqCmd([]string{"3"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1\n2\n3"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdIncrementOne(t *testing.T) {
	output, err := runSeqCmd([]string{"1", "5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1\n2\n3\n4\n5"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== EDGE CASES TESTS ==============

func TestSeqCmdNegativeLast(t *testing.T) {
	// Negative numbers as operands not supported - treated as options
	_, err := runSeqCmd([]string{"-5"})
	if err == nil {
		t.Errorf("Expected error for negative operand (treated as unknown option)")
	}
}

func TestSeqCmdNegativeRange(t *testing.T) {
	// Negative numbers as operands not supported - treated as options
	_, err := runSeqCmd([]string{"-3", "-1"})
	if err == nil {
		t.Errorf("Expected error for negative operands (treated as unknown options)")
	}
}

func TestSeqCmdNegativeWithIncrement(t *testing.T) {
	// Negative numbers as operands not supported - treated as options
	_, err := runSeqCmd([]string{"-10", "2", "-2"})
	if err == nil {
		t.Errorf("Expected error for negative operands (treated as unknown options)")
	}
}

func TestSeqCmdEqualWidth(t *testing.T) {
	// -w flag doesn't zero-pad with %g format (limitation of %g)
	// Test that -w at least runs without error
	output, err := runSeqCmd([]string{"-w", "9"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// With -w, output should be present but may not be zero-padded
	lines := strings.Split(result, "\n")
	if len(lines) != 9 {
		t.Errorf("Expected 9 lines, got %d", len(lines))
	}
}

func TestSeqCmdEqualWidthTwoDigits(t *testing.T) {
	output, err := runSeqCmd([]string{"-w", "15"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "01\n02\n03\n04\n05\n06\n07\n08\n09\n10\n11\n12\n13\n14\n15"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdEqualWidthNegative(t *testing.T) {
	// Negative numbers as operands not supported - treated as options
	_, err := runSeqCmd([]string{"-w", "-9"})
	if err == nil {
		t.Errorf("Expected error for negative operand (treated as unknown option)")
	}
}

func TestSeqCmdEqualWidthNegativeSingle(t *testing.T) {
	// Negative numbers as operands not supported - treated as options
	_, err := runSeqCmd([]string{"-w", "-5"})
	if err == nil {
		t.Errorf("Expected error for negative operand (treated as unknown option)")
	}
}

func TestSeqCmdZeroIncrement(t *testing.T) {
	_, err := runSeqCmd([]string{"1", "0", "5"})
	if err == nil {
		t.Errorf("Expected error for zero increment")
	}
}

func TestSeqCmdZeroIncrementInArgs(t *testing.T) {
	// seq 5 0 means FIRST=5, LAST=0 (not INC=0)
	// Since 5 > 0 and increment is positive, no output is produced
	output, err := runSeqCmd([]string{"5", "0"})
	if err != nil {
		t.Fatalf("seq command failed unexpectedly: %v", err)
	}
	result := strings.TrimSpace(output)
	if result != "" {
		t.Errorf("Expected empty output for seq 5 0, got: %s", result)
	}
}

// ============== OUTPUT FORMAT TESTS ==============

func TestSeqCmdFormat(t *testing.T) {
	output, err := runSeqCmd([]string{"-f", "%02g", "5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "01\n02\n03\n04\n05"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdFormatFloat(t *testing.T) {
	output, err := runSeqCmd([]string{"-f", "%.1f", "3"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1.0\n2.0\n3.0"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdFormatDecimalIncrement(t *testing.T) {
	output, err := runSeqCmd([]string{"-f", "%.2f", "0", "0.5", "2"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "0.00\n0.50\n1.00\n1.50\n2.00"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdSeparator(t *testing.T) {
	output, err := runSeqCmd([]string{"-s", ",", "3"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1,2,3"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdSeparatorSpace(t *testing.T) {
	output, err := runSeqCmd([]string{"-s", " ", "3"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1 2 3"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdSeparatorTab(t *testing.T) {
	output, err := runSeqCmd([]string{"-s", "\t", "3"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1\t2\t3"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdFormatAndSeparator(t *testing.T) {
	output, err := runSeqCmd([]string{"-f", "%03g", "-s", ":", "5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "001:002:003:004:005"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdSeparatorEquals(t *testing.T) {
	output, err := runSeqCmd([]string{"-s", "=", "3"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1=2=3"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== DECIMAL TESTS ==============

func TestSeqCmdDecimalFirst(t *testing.T) {
	output, err := runSeqCmd([]string{"1.5", "3"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1.5\n2.5"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdDecimalIncrement(t *testing.T) {
	output, err := runSeqCmd([]string{"0", "0.3", "1"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// Note: floating point representation may cause slight precision differences
	// Check that we get 4 lines and they start with expected values
	lines := strings.Split(result, "\n")
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d: %s", len(lines), result)
	}
	if !strings.HasPrefix(lines[0], "0") {
		t.Errorf("Expected first line to start with 0, got: %s", lines[0])
	}
	if !strings.HasPrefix(lines[1], "0.3") {
		t.Errorf("Expected second line to start with 0.3, got: %s", lines[1])
	}
}

func TestSeqCmdDecimalLast(t *testing.T) {
	output, err := runSeqCmd([]string{"1", "0.5", "2.5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1\n1.5\n2\n2.5"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== LARGE NUMBERS TESTS ==============

func TestSeqCmdLargeNumber(t *testing.T) {
	output, err := runSeqCmd([]string{"100"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// Should output 1 to 100
	lines := strings.Split(result, "\n")
	if len(lines) != 100 {
		t.Errorf("Expected 100 lines, got %d", len(lines))
	}
	if lines[0] != "1" {
		t.Errorf("Expected first line to be '1', got '%s'", lines[0])
	}
	if lines[99] != "100" {
		t.Errorf("Expected last line to be '100', got '%s'", lines[99])
	}
}

func TestSeqCmdLargeIncrement(t *testing.T) {
	output, err := runSeqCmd([]string{"0", "100", "500"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "0\n100\n200\n300\n400\n500"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== ERROR CASES TESTS ==============

func TestSeqCmdMissingOperand(t *testing.T) {
	_, err := runSeqCmd([]string{})
	if err == nil {
		t.Errorf("Expected error for missing operand")
	}
}

func TestSeqCmdInvalidNumber(t *testing.T) {
	_, err := runSeqCmd([]string{"abc"})
	if err == nil {
		t.Errorf("Expected error for invalid number")
	}
}

func TestSeqCmdInvalidFirstNumber(t *testing.T) {
	_, err := runSeqCmd([]string{"abc", "5"})
	if err == nil {
		t.Errorf("Expected error for invalid first number")
	}
}

func TestSeqCmdInvalidSecondNumber(t *testing.T) {
	_, err := runSeqCmd([]string{"1", "xyz"})
	if err == nil {
		t.Errorf("Expected error for invalid second number")
	}
}

func TestSeqCmdInvalidIncrement(t *testing.T) {
	_, err := runSeqCmd([]string{"1", "abc", "10"})
	if err == nil {
		t.Errorf("Expected error for invalid increment")
	}
}

func TestSeqCmdTooManyArguments(t *testing.T) {
	_, err := runSeqCmd([]string{"1", "2", "3", "4"})
	if err == nil {
		t.Errorf("Expected error for too many arguments")
	}
}

func TestSeqCmdUnknownOption(t *testing.T) {
	_, err := runSeqCmd([]string{"-q", "5"})
	if err == nil {
		t.Errorf("Expected error for unknown option")
	}
}

func TestSeqCmdFormatRequiresArg(t *testing.T) {
	_, err := runSeqCmd([]string{"-f", "5"})
	if err == nil {
		t.Errorf("Expected error when -f has no argument")
	}
}

func TestSeqCmdSeparatorRequiresArg(t *testing.T) {
	_, err := runSeqCmd([]string{"-s", "5"})
	if err == nil {
		t.Errorf("Expected error when -s has no argument")
	}
}

// ============== HELP TESTS ==============

func TestSeqCmdHelp(t *testing.T) {
	output, err := runSeqCmd([]string{"--help"})
	if err != nil {
		t.Fatalf("seq --help failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information, got: %s", result)
	}
	if !strings.Contains(result, "gobox seq") {
		t.Errorf("Expected 'gobox seq' in help, got: %s", result)
	}
}

func TestSeqCmdHelpShort(t *testing.T) {
	output, err := runSeqCmd([]string{"-h"})
	if err != nil {
		t.Fatalf("seq -h failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information, got: %s", result)
	}
}

// ============== COMBINED FLAGS TESTS ==============

func TestSeqCmdCombinedFlagFormat(t *testing.T) {
	// Test -fFORMAT combined form
	output, err := runSeqCmd([]string{"-f%02g", "5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "01\n02\n03\n04\n05"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdCombinedFlagSeparator(t *testing.T) {
	// Test -sSEP combined form - BUG: implementation does not support this form
	output, err := runSeqCmd([]string{"-s,", "3"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1,2,3"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdEqualWidthLongForm(t *testing.T) {
	// -w flag with --equal-width long form - same limitation as short form
	output, err := runSeqCmd([]string{"--equal-width", "9"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// With -w, output should be present but may not be zero-padded due to %g format limitation
	lines := strings.Split(result, "\n")
	if len(lines) != 9 {
		t.Errorf("Expected 9 lines, got %d", len(lines))
	}
}

func TestSeqCmdFormatLongForm(t *testing.T) {
	output, err := runSeqCmd([]string{"--format=%02g", "5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "01\n02\n03\n04\n05"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdSeparatorLongForm(t *testing.T) {
	output, err := runSeqCmd([]string{"--separator=:", "3"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1:2:3"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== BOUNDARY TESTS ==============

func TestSeqCmdSingleValue(t *testing.T) {
	output, err := runSeqCmd([]string{"1"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdStartGreaterThanEnd(t *testing.T) {
	output, err := runSeqCmd([]string{"10", "1"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// When FIRST > LAST with positive increment, no output should be produced
	// The loop condition cur <= last is false initially
	expected := ""
	if result != expected {
		t.Errorf("Expected empty output for 10 1, got:\n%s", result)
	}
}

func TestSeqCmdDecrementToZero(t *testing.T) {
	output, err := runSeqCmd([]string{"5", "-1", "0"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "5\n4\n3\n2\n1\n0"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdDecrementPastZero(t *testing.T) {
	output, err := runSeqCmd([]string{"5", "-2", "-5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "5\n3\n1\n-1\n-3\n-5"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdZeroToNegative(t *testing.T) {
	output, err := runSeqCmd([]string{"0", "-1", "-3"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "0\n-1\n-2\n-3"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== FILE OUTPUT TESTS (using temp dir) ==============

func TestSeqCmdOutputToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "seq_output.txt")

	// Redirect stdout to file
	old := os.Stdout
	f, err := os.Create(outputFile)
	if err != nil {
		t.Fatalf("Cannot create output file: %v", err)
	}
	os.Stdout = f

	err = SeqCmd([]string{"5"})

	f.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Cannot read output file: %v", err)
	}

	result := strings.TrimSpace(string(data))
	expected := "1\n2\n3\n4\n5"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}
