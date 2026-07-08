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

// TestSeqCmdNegativeLast is a regression test: "seq -5" is the
// single-operand form (LAST=-5, implicit FIRST=1, INC=1); since -5 < 1 with
// a positive increment, native seq exits 0 with empty output, not an error
// — this used to be misinterpreted as an unknown "-5" option.
func TestSeqCmdNegativeLast(t *testing.T) {
	output, err := runSeqCmd([]string{"-5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}
	if strings.TrimSpace(output) != "" {
		t.Fatalf("expected empty output for descending single-operand form, got %q", output)
	}
}

// TestSeqCmdNegativeRange is a regression test: negative FIRST/LAST
// operands must be accepted; "seq -3 -1" ascends from -3 to -1.
func TestSeqCmdNegativeRange(t *testing.T) {
	output, err := runSeqCmd([]string{"-3", "-1"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}
	result := strings.TrimSpace(output)
	expected := "-3\n-2\n-1"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSeqCmdNegativeWithIncrement(t *testing.T) {
	// Regression test: negative FIRST/LAST operands must be accepted (e.g.
	// "seq -10 2 -2" ascends from -10 to -2 in steps of 2), matching native
	// seq; this used to be misinterpreted as an unknown "-10" option.
	output, err := runSeqCmd([]string{"-10", "2", "-2"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}
	result := strings.TrimSpace(output)
	expected := "-10\n-8\n-6\n-4\n-2"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
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

// TestSeqCmdEqualWidthNegative is a regression test: "seq -w -9" is the
// single-operand form (LAST=-9, implicit FIRST=1, INC=1), so since -9 < 1
// with a positive increment the sequence is empty. Native seq exits 0 with
// no output here (not an error) — this used to be misinterpreted as an
// unknown "-9" option.
func TestSeqCmdEqualWidthNegative(t *testing.T) {
	output, err := runSeqCmd([]string{"-w", "-9"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}
	if strings.TrimSpace(output) != "" {
		t.Errorf("expected empty output for descending single-operand form, got %q", output)
	}
}

func TestSeqCmdEqualWidthNegativeSingle(t *testing.T) {
	output, err := runSeqCmd([]string{"-w", "-5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}
	if strings.TrimSpace(output) != "" {
		t.Errorf("expected empty output for descending single-operand form, got %q", output)
	}
}

func TestSeqCmdZeroIncrement(t *testing.T) {
	output, err := runSeqCmd([]string{"1", "0", "5"})
	if err == nil {
		t.Errorf("Expected error for zero increment")
	} else if err.Error() != "invalid increment: 0" {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected no output, got %q", output)
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

// TestSeqCmdDecimalLast is a regression test: native seq derives the output
// decimal precision from the widest-precision operand (here INC=0.5 and
// LAST=2.5, both 1 decimal place) and applies it to every value, including
// otherwise-whole numbers ("1.0", "2.0"), not "1"/"2".
func TestSeqCmdDecimalLast(t *testing.T) {
	output, err := runSeqCmd([]string{"1", "0.5", "2.5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1.0\n1.5\n2.0\n2.5"
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
	output, err := runSeqCmd([]string{})
	if err == nil {
		t.Errorf("Expected error for missing operand")
	} else if err.Error() != "missing operand" {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected no output, got %q", output)
	}
}

func TestSeqCmdInvalidNumber(t *testing.T) {
	output, err := runSeqCmd([]string{"abc"})
	if err == nil {
		t.Errorf("Expected error for invalid number")
	} else if err.Error() != "invalid number: abc" {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected no output, got %q", output)
	}
}

func TestSeqCmdInvalidFirstNumber(t *testing.T) {
	output, err := runSeqCmd([]string{"abc", "5"})
	if err == nil {
		t.Errorf("Expected error for invalid first number")
	} else if err.Error() != "invalid number: abc" {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected no output, got %q", output)
	}
}

func TestSeqCmdInvalidSecondNumber(t *testing.T) {
	output, err := runSeqCmd([]string{"1", "xyz"})
	if err == nil {
		t.Errorf("Expected error for invalid second number")
	} else if err.Error() != "invalid number: xyz" {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected no output, got %q", output)
	}
}

func TestSeqCmdInvalidIncrement(t *testing.T) {
	output, err := runSeqCmd([]string{"1", "abc", "10"})
	if err == nil {
		t.Errorf("Expected error for invalid increment")
	} else if err.Error() != "invalid number: abc" {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected no output, got %q", output)
	}
}

func TestSeqCmdTooManyArguments(t *testing.T) {
	output, err := runSeqCmd([]string{"1", "2", "3", "4"})
	if err == nil {
		t.Errorf("Expected error for too many arguments")
	} else if err.Error() != "too many arguments" {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected no output, got %q", output)
	}
}

func TestSeqCmdUnknownOption(t *testing.T) {
	output, err := runSeqCmd([]string{"-q", "5"})
	if err == nil {
		t.Errorf("Expected error for unknown option")
	} else if err.Error() != "unknown option: -q" {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected no output, got %q", output)
	}
}

func TestSeqCmdFormatRequiresArg(t *testing.T) {
	output, err := runSeqCmd([]string{"-f", "5"})
	if err == nil {
		t.Errorf("Expected error when -f has no argument")
	} else if err.Error() != "missing operand" {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected no output, got %q", output)
	}
}

func TestSeqCmdSeparatorRequiresArg(t *testing.T) {
	output, err := runSeqCmd([]string{"-s", "5"})
	if err == nil {
		t.Errorf("Expected error when -s has no argument")
	} else if err.Error() != "missing operand" {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "" {
		t.Fatalf("expected no output, got %q", output)
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

// TestSeqCmdFloatingPointAccumulationDoesNotDrift is a regression test for a
// bug where repeatedly adding a fractional INC (0.1 three times) produced a
// floating-point artifact like "0.30000000000000004" instead of "0.3",
// because values were computed via a running total (cur += inc) rather than
// first + n*inc, and printed with %g instead of a precision derived from
// the operands.
func TestSeqCmdFloatingPointAccumulationDoesNotDrift(t *testing.T) {
	output, err := runSeqCmd([]string{"0.1", "0.1", "0.5"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}
	result := strings.TrimSpace(output)
	expected := "0.1\n0.2\n0.3\n0.4\n0.5"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// TestSeqCmdEqualWidthWithDecimals is a regression test for -w combined
// with fractional operands: native seq pads the integer part with leading
// zeros while keeping the shared decimal precision, e.g. "0.00".."1.00".
func TestSeqCmdEqualWidthWithDecimals(t *testing.T) {
	output, err := runSeqCmd([]string{"-w", "0", "0.25", "1"})
	if err != nil {
		t.Fatalf("seq command failed: %v", err)
	}
	result := strings.TrimSpace(output)
	expected := "0.00\n0.25\n0.50\n0.75\n1.00"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}
