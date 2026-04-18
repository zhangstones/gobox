package text

import (
	"os"
	"strings"
	"testing"
)

// ============== DEFAULT BEHAVIOR TESTS ==============

func TestHeadDefault(t *testing.T) {
	// Default should show 10 lines
	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12\n"
	writeTestFile(t, "test_head_default.txt", content)
	defer os.Remove("test_head_default.txt")

	output, err := runHeadCmd([]string{"test_head_default.txt"})
	if err != nil {
		t.Fatalf("head command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Errorf("Expected 10 lines by default, got %d: %s", len(lines), result)
	}
	if !strings.Contains(result, "line1") {
		t.Errorf("Expected 'line1' in output, got: %s", result)
	}
	if !strings.Contains(result, "line10") {
		t.Errorf("Expected 'line10' in output, got: %s", result)
	}
	if strings.Contains(result, "line11") {
		t.Errorf("Should not contain 'line11', got: %s", result)
	}
}

func TestHeadNoNewlineAtEnd(t *testing.T) {
	// File without trailing newline - scanner reads until EOF
	content := "line1\nline2\nline3\nno newline at end"
	writeTestFile(t, "test_head_nonl.txt", content)
	defer os.Remove("test_head_nonl.txt")

	output, err := runHeadCmd([]string{"test_head_nonl.txt"})
	if err != nil {
		t.Fatalf("head command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	// Without trailing newline, the last "line" is still read by scanner
	// So we get 4 lines (but last one without newline)
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines (scanner reads to EOF), got %d: %s", len(lines), result)
	}
}

// ============== -n FLAG TESTS ==============

func TestHeadNLinesFlag(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\n"
	writeTestFile(t, "test_head_n.txt", content)
	defer os.Remove("test_head_n.txt")

	output, err := runHeadCmd([]string{"-n", "3", "test_head_n.txt"})
	if err != nil {
		t.Fatalf("head -n command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line3") {
		t.Errorf("Expected lines 1-3, got: %s", result)
	}
}

func TestHeadNLinesEqualsFlag(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\n"
	writeTestFile(t, "test_head_n_equals.txt", content)
	defer os.Remove("test_head_n_equals.txt")

	output, err := runHeadCmd([]string{"-n=3", "test_head_n_equals.txt"})
	if err != nil {
		t.Fatalf("head -n= command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
}

func TestHeadNLinesZero(t *testing.T) {
	content := "line1\nline2\nline3\n"
	writeTestFile(t, "test_head_n_zero.txt", content)
	defer os.Remove("test_head_n_zero.txt")

	output, err := runHeadCmd([]string{"-n", "0", "test_head_n_zero.txt"})
	if err != nil {
		t.Fatalf("head -n 0 command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "" {
		t.Errorf("Expected empty output, got: %s", result)
	}
}

func TestHeadNLinesMoreThanFile(t *testing.T) {
	// Request more lines than file has
	content := "line1\nline2\n"
	writeTestFile(t, "test_head_n_more.txt", content)
	defer os.Remove("test_head_n_more.txt")

	output, err := runHeadCmd([]string{"-n", "100", "test_head_n_more.txt"})
	if err != nil {
		t.Fatalf("head -n 100 command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines (file has only 2), got %d: %s", len(lines), result)
	}
}

// ============== -c FLAG TESTS ==============

func TestHeadCBytesFlag(t *testing.T) {
	content := "hello world"
	writeTestFile(t, "test_head_c.txt", content)
	defer os.Remove("test_head_c.txt")

	output, err := runHeadCmd([]string{"-c", "5", "test_head_c.txt"})
	if err != nil {
		t.Fatalf("head -c command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "hello" {
		t.Errorf("Expected 'hello', got: %s", result)
	}
}

func TestHeadCBytesEqualsFlag(t *testing.T) {
	content := "hello world"
	writeTestFile(t, "test_head_c_equals.txt", content)
	defer os.Remove("test_head_c_equals.txt")

	output, err := runHeadCmd([]string{"-c=5", "test_head_c_equals.txt"})
	if err != nil {
		t.Fatalf("head -c= command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "hello" {
		t.Errorf("Expected 'hello', got: %s", result)
	}
}

func TestHeadCBytesMoreThanFile(t *testing.T) {
	content := "hi"
	writeTestFile(t, "test_head_c_more.txt", content)
	defer os.Remove("test_head_c_more.txt")

	output, err := runHeadCmd([]string{"-c", "100", "test_head_c_more.txt"})
	if err != nil {
		t.Fatalf("head -c 100 command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "hi" {
		t.Errorf("Expected 'hi', got: %s", result)
	}
}

func TestHeadCBytesZero(t *testing.T) {
	content := "hello world"
	writeTestFile(t, "test_head_c_zero.txt", content)
	defer os.Remove("test_head_c_zero.txt")

	output, err := runHeadCmd([]string{"-c", "0", "test_head_c_zero.txt"})
	if err != nil {
		t.Fatalf("head -c 0 command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "" {
		t.Errorf("Expected empty output, got: %s", result)
	}
}

// ============== -q FLAG TESTS (QUIET MODE) ==============

func TestHeadQuietMode(t *testing.T) {
	// With multiple files, -q should suppress headers
	content := "line1\nline2\n"
	writeTestFile(t, "test_head_q1.txt", content)
	defer os.Remove("test_head_q1.txt")
	writeTestFile(t, "test_head_q2.txt", content)
	defer os.Remove("test_head_q2.txt")

	output, err := runHeadCmd([]string{"-q", "test_head_q1.txt", "test_head_q2.txt"})
	if err != nil {
		t.Fatalf("head -q command failed: %v", err)
	}

	result := string(output)
	if strings.Contains(result, "==>") {
		t.Errorf("Should not contain file headers in quiet mode, got: %s", result)
	}
	// Should contain content from both files
	count := strings.Count(result, "line1")
	if count != 2 {
		t.Errorf("Expected line1 twice, got %d: %s", count, result)
	}
}

func TestHeadQuietModeSingleFile(t *testing.T) {
	content := "line1\nline2\n"
	writeTestFile(t, "test_head_q_single.txt", content)
	defer os.Remove("test_head_q_single.txt")

	output, err := runHeadCmd([]string{"-q", "test_head_q_single.txt"})
	if err != nil {
		t.Fatalf("head -q command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if !strings.Contains(result, "line1") {
		t.Errorf("Expected content, got: %s", result)
	}
}

func TestHeadSilentFlag(t *testing.T) {
	content := "line1\nline2\n"
	writeTestFile(t, "test_head_silent.txt", content)
	defer os.Remove("test_head_silent.txt")

	output, err := runHeadCmd([]string{"--silent", "test_head_silent.txt"})
	if err != nil {
		t.Fatalf("head --silent command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if !strings.Contains(result, "line1") {
		t.Errorf("Expected content, got: %s", result)
	}
}

// ============== MULTIPLE FILES TESTS ==============

func TestHeadMultipleFiles(t *testing.T) {
	content1 := "file1_line1\nfile1_line2\n"
	content2 := "file2_line1\nfile2_line2\nfile2_line3\n"
	writeTestFile(t, "test_head_multi1.txt", content1)
	defer os.Remove("test_head_multi1.txt")
	writeTestFile(t, "test_head_multi2.txt", content2)
	defer os.Remove("test_head_multi2.txt")

	output, err := runHeadCmd([]string{"test_head_multi1.txt", "test_head_multi2.txt"})
	if err != nil {
		t.Fatalf("head multiple files command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "==> test_head_multi1.txt <==") {
		t.Errorf("Missing header for first file, got: %s", result)
	}
	if !strings.Contains(result, "==> test_head_multi2.txt <==") {
		t.Errorf("Missing header for second file, got: %s", result)
	}
	if !strings.Contains(result, "file1_line1") {
		t.Errorf("Missing content from file1, got: %s", result)
	}
	if !strings.Contains(result, "file2_line1") {
		t.Errorf("Missing content from file2, got: %s", result)
	}
}

// ============== STDIN TESTS ==============

func TestHeadStdin(t *testing.T) {
	output, err := runHeadCmdWithStdin([]string{"-n", "3"}, "line1\nline2\nline3\nline4\nline5\n")
	if err != nil {
		t.Fatalf("head stdin command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
}

func TestHeadStdinDefault(t *testing.T) {
	// Default 10 lines from stdin
	output, err := runHeadCmdWithStdin([]string{}, "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12\n")
	if err != nil {
		t.Fatalf("head stdin command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Errorf("Expected 10 lines, got %d: %s", len(lines), result)
	}
}

func TestHeadStdinBytes(t *testing.T) {
	output, err := runHeadCmdWithStdin([]string{"-c", "5"}, "hello world")
	if err != nil {
		t.Fatalf("head -c stdin command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "hello" {
		t.Errorf("Expected 'hello', got: %s", result)
	}
}

// ============== EDGE CASES ==============

func TestHeadEmptyFile(t *testing.T) {
	content := ""
	writeTestFile(t, "test_head_empty.txt", content)
	defer os.Remove("test_head_empty.txt")

	output, err := runHeadCmd([]string{"test_head_empty.txt"})
	if err != nil {
		t.Fatalf("head empty file command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "" {
		t.Errorf("Expected empty output, got: %s", result)
	}
}

func TestHeadSingleLine(t *testing.T) {
	content := "single line\n"
	writeTestFile(t, "test_head_single.txt", content)
	defer os.Remove("test_head_single.txt")

	output, err := runHeadCmd([]string{"test_head_single.txt"})
	if err != nil {
		t.Fatalf("head single line command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "single line" {
		t.Errorf("Expected 'single line', got: %s", result)
	}
}

func TestHeadVeryLongLine(t *testing.T) {
	// Long line (8KB to avoid bufio scanner 64KB limit issues)
	longContent := strings.Repeat("x", 8192)
	content := "short line\n" + longContent + "\nanother short\n"
	writeTestFile(t, "test_head_long.txt", content)
	defer os.Remove("test_head_long.txt")

	output, err := runHeadCmd([]string{"-n", "2", "test_head_long.txt"})
	if err != nil {
		t.Fatalf("head long line command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), result)
	}
	if !strings.HasPrefix(lines[0], "short line") {
		t.Errorf("Expected first line to be 'short line', got: %s", lines[0])
	}
	if !strings.HasPrefix(lines[1], "x") {
		t.Errorf("Expected second line to start with x's, got: %s", lines[1])
	}
}

func TestHeadSpecialCharacters(t *testing.T) {
	content := "hello\tworld\nspecial: !@#$%^&*()\nunicode: \u00E9\u00E8\u00EA\n"
	writeTestFile(t, "test_head_special.txt", content)
	defer os.Remove("test_head_special.txt")

	output, err := runHeadCmd([]string{"-n", "3", "test_head_special.txt"})
	if err != nil {
		t.Fatalf("head special chars command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
	if !strings.Contains(result, "hello\tworld") {
		t.Errorf("Expected tab character preserved, got: %s", result)
	}
}

func TestHeadNewlinesOnly(t *testing.T) {
	// Multiple newlines - bufio.Scanner splits on newlines
	// content has 4 lines: line1, empty, empty, line5
	// With head -n 3: tokens are "line1", "", "", and we stop at 3
	// Output is "line1\n\n\n" which TrimSpace reduces to "line1"
	content := "line1\n\n\nline5\n"
	writeTestFile(t, "test_head_newlines.txt", content)
	defer os.Remove("test_head_newlines.txt")

	output, err := runHeadCmd([]string{"-n", "3", "test_head_newlines.txt"})
	if err != nil {
		t.Fatalf("head newlines command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// TrimSpace removes trailing newlines, so we get "line1" with no newlines
	// strings.Split by \n gives ["line1"] with length 1
	lines := strings.Split(result, "\n")
	if len(lines) != 1 {
		t.Errorf("Expected 1 line (TrimSpace removes trailing newlines), got %d: %s", len(lines), result)
	}
	if result != "line1" {
		t.Errorf("Expected 'line1', got: %s", result)
	}
}

// ============== ERROR CASES ==============

func TestHeadNonExistentFile(t *testing.T) {
	_, err := runHeadCmd([]string{"nonexistent_file.txt"})
	if err == nil {
		t.Fatalf("head should fail on non-existent file")
	}
}

func TestHeadInvalidNLines(t *testing.T) {
	content := "line1\nline2\n"
	writeTestFile(t, "test_head_invalid_n.txt", content)
	defer os.Remove("test_head_invalid_n.txt")

	_, err := runHeadCmd([]string{"-n", "abc", "test_head_invalid_n.txt"})
	if err == nil {
		t.Fatalf("head -n with invalid argument should fail")
	}
}

func TestHeadNegativeNLines(t *testing.T) {
	content := "line1\nline2\n"
	writeTestFile(t, "test_head_neg_n.txt", content)
	defer os.Remove("test_head_neg_n.txt")

	_, err := runHeadCmd([]string{"-n", "-5", "test_head_neg_n.txt"})
	if err == nil {
		t.Fatalf("head -n with negative should fail")
	}
}

func TestHeadInvalidBytes(t *testing.T) {
	content := "line1\nline2\n"
	writeTestFile(t, "test_head_invalid_c.txt", content)
	defer os.Remove("test_head_invalid_c.txt")

	_, err := runHeadCmd([]string{"-c", "xyz", "test_head_invalid_c.txt"})
	if err == nil {
		t.Fatalf("head -c with invalid argument should fail")
	}
}

func TestHeadUnknownOption(t *testing.T) {
	content := "line1\nline2\n"
	writeTestFile(t, "test_head_unknown.txt", content)
	defer os.Remove("test_head_unknown.txt")

	_, err := runHeadCmd([]string{"-z", "test_head_unknown.txt"})
	if err == nil {
		t.Fatalf("head with unknown option should fail")
	}
}

func TestHeadNNRequiresArgument(t *testing.T) {
	content := "line1\nline2\n"
	writeTestFile(t, "test_head_n_arg.txt", content)
	defer os.Remove("test_head_n_arg.txt")

	_, err := runHeadCmd([]string{"-n", "test_head_n_arg.txt"})
	if err == nil {
		t.Fatalf("head -n without argument should fail")
	}
}

// ============== HELP FLAG TESTS ==============

func TestHeadHelpFlag(t *testing.T) {
	output, err := runHeadCmd([]string{"--help"})
	if err != nil {
		t.Fatalf("head --help command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information, got: %s", result)
	}
	if !strings.Contains(result, "-n") {
		t.Errorf("Expected -n option in help, got: %s", result)
	}
}

func TestHeadHFlag(t *testing.T) {
	output, err := runHeadCmd([]string{"-h"})
	if err != nil {
		t.Fatalf("head -h command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information, got: %s", result)
	}
}

// ============== PIPELINE TESTS ==============

func TestHeadPipeline(t *testing.T) {
	// Simulate pipeline by providing stdin
	output, err := runHeadCmdWithStdin([]string{"-n", "2"}, "line1\nline2\nline3\nline4\n")
	if err != nil {
		t.Fatalf("head pipeline command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), result)
	}
}

// ============== COMBINED OPTIONS TESTS ==============

func TestHeadQuietWithBytes(t *testing.T) {
	content := "hello world"
	writeTestFile(t, "test_head_q_c.txt", content)
	defer os.Remove("test_head_q_c.txt")

	// -q and -c together should work (quiet mode with byte limit)
	output, err := runHeadCmd([]string{"-q", "-c", "5", "test_head_q_c.txt"})
	if err != nil {
		t.Fatalf("head -q -c command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "hello" {
		t.Errorf("Expected 'hello', got: %s", result)
	}
}

func TestHeadMultipleFlagsOrder(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\n"
	writeTestFile(t, "test_head_order.txt", content)
	defer os.Remove("test_head_order.txt")

	// Test that flag order doesn't matter
	output1, err1 := runHeadCmd([]string{"-n", "2", "-q", "test_head_order.txt"})
	output2, err2 := runHeadCmd([]string{"-q", "-n", "2", "test_head_order.txt"})

	if err1 != nil {
		t.Fatalf("first command failed: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("second command failed: %v", err2)
	}

	if string(output1) != string(output2) {
		t.Errorf("Different flag order should produce same result: %s vs %s", output1, output2)
	}
}
