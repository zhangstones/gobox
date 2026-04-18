package text

import (
	"os"
	"strings"
	"testing"
)

func TestGrepBasicMatch(t *testing.T) {
	// Create test file
	content := "hello world\nfoo bar\nhello again\n"
	writeTestFile(t, "test_basic.txt", content)
	defer os.Remove("test_basic.txt")

	// Run grep
	output, err := runGrepCmd([]string{"hello", "test_basic.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "hello world") {
		t.Errorf("Expected 'hello world' in output, got: %s", result)
	}
	if !strings.Contains(result, "hello again") {
		t.Errorf("Expected 'hello again' in output, got: %s", result)
	}
	if strings.Contains(result, "foo bar") {
		t.Errorf("Unexpected 'foo bar' in output: %s", result)
	}
}

func TestGrepIgnoreCase(t *testing.T) {
	content := "HELLO world\nfoo BAR\nHello Again\n"
	writeTestFile(t, "test_case.txt", content)
	defer os.Remove("test_case.txt")

	output, err := runGrepCmd([]string{"-i", "hello", "test_case.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "HELLO world") {
		t.Errorf("Expected 'HELLO world' in output, got: %s", result)
	}
	if !strings.Contains(result, "Hello Again") {
		t.Errorf("Expected 'Hello Again' in output, got: %s", result)
	}
}

func TestGrepInvertMatch(t *testing.T) {
	content := "hello world\nfoo bar\nhello again\nbaz qux\n"
	writeTestFile(t, "test_invert.txt", content)
	defer os.Remove("test_invert.txt")

	output, err := runGrepCmd([]string{"-v", "hello", "test_invert.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if strings.Contains(result, "hello world") {
		t.Errorf("Unexpected 'hello world' in output: %s", result)
	}
	if strings.Contains(result, "hello again") {
		t.Errorf("Unexpected 'hello again' in output: %s", result)
	}
	if !strings.Contains(result, "foo bar") {
		t.Errorf("Expected 'foo bar' in output, got: %s", result)
	}
	if !strings.Contains(result, "baz qux") {
		t.Errorf("Expected 'baz qux' in output, got: %s", result)
	}
}

func TestGrepCount(t *testing.T) {
	content := "hello world\nfoo bar\nhello again\nhello third\n"
	writeTestFile(t, "test_count.txt", content)
	defer os.Remove("test_count.txt")

	output, err := runGrepCmd([]string{"-c", "hello", "test_count.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// Count output includes filename when file is specified
	if result != "test_count.txt:3" && result != "3" {
		t.Errorf("Expected count 3 (with or without filename), got: %s", result)
	}
}

func TestGrepLineNumber(t *testing.T) {
	content := "first line\nsecond line with hello\nthird line\n"
	writeTestFile(t, "test_linenum.txt", content)
	defer os.Remove("test_linenum.txt")

	output, err := runGrepCmd([]string{"-n", "hello", "test_linenum.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "2:second line with hello") {
		t.Errorf("Expected line number 2 in output, got: %s", result)
	}
}

func TestGrepFixedString(t *testing.T) {
	content := "hello.world\nfoo bar\nhelloXworld\n"
	writeTestFile(t, "test_fixed.txt", content)
	defer os.Remove("test_fixed.txt")

	output, err := runGrepCmd([]string{"-F", "hello.world", "test_fixed.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "hello.world") {
		t.Errorf("Expected 'hello.world' in output, got: %s", result)
	}
	if strings.Contains(result, "helloXworld") {
		t.Errorf("Unexpected 'helloXworld' in output: %s", result)
	}
}

func TestGrepRegex(t *testing.T) {
	content := "test123\nfoo456\ntest789\nbar\n"
	writeTestFile(t, "test_regex.txt", content)
	defer os.Remove("test_regex.txt")

	output, err := runGrepCmd([]string{"test[0-9]+", "test_regex.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "test123") {
		t.Errorf("Expected 'test123' in output, got: %s", result)
	}
	if !strings.Contains(result, "test789") {
		t.Errorf("Expected 'test789' in output, got: %s", result)
	}
	if strings.Contains(result, "foo456") {
		t.Errorf("Unexpected 'foo456' in output: %s", result)
	}
}

func TestGrepNoMatch(t *testing.T) {
	content := "hello world\nfoo bar\n"
	writeTestFile(t, "test_nomatch.txt", content)
	defer os.Remove("test_nomatch.txt")

	output, err := runGrepCmd([]string{"notfound", "test_nomatch.txt"})
	// GrepCmd doesn't propagate exit codes - just verify no panic
	_ = output
	_ = err

	if len(output) != 0 {
		t.Errorf("Expected empty output for no matches, got: %s", string(output))
	}
}

func TestGrepRecursive(t *testing.T) {
	// Create test directory structure
	os.MkdirAll("testdir/subdir", 0755)
	defer os.RemoveAll("testdir")

	writeTestFile(t, "testdir/file1.txt", "hello world\n")
	writeTestFile(t, "testdir/subdir/file2.txt", "hello again\n")
	writeTestFile(t, "testdir/subdir/file3.txt", "goodbye\n")

	output, err := runGrepCmd([]string{"-r", "hello", "testdir"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "hello world") {
		t.Errorf("Expected 'hello world' in output, got: %s", result)
	}
	if !strings.Contains(result, "hello again") {
		t.Errorf("Expected 'hello again' in output, got: %s", result)
	}
	if strings.Contains(result, "goodbye") {
		t.Errorf("Unexpected 'goodbye' in output: %s", result)
	}
}

func TestGrepStdin(t *testing.T) {
	output, err := runGrepCmdWithStdin([]string{"test"}, "hello\ntest line\nworld\n")
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "test line") {
		t.Errorf("Expected 'test line' in output, got: %s", result)
	}
}

func TestGrepOnlyMatching(t *testing.T) {
	content := "test123\nfoo456bar\ntest789test\n"
	writeTestFile(t, "test_only.txt", content)
	defer os.Remove("test_only.txt")

	output, err := runGrepCmd([]string{"-o", "test", "test_only.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	// With filename, format is: filename:match
	if len(lines) != 3 {
		t.Errorf("Expected 3 matches, got %d: %s", len(lines), result)
	}
	for _, line := range lines {
		if !strings.HasSuffix(line, ":test") {
			t.Errorf("Expected line ending with ':test', got: %s", line)
		}
	}
}

func TestGrepOnlyMatchingRegex(t *testing.T) {
	content := "test123\nfoo456bar\ntest789test\n"
	writeTestFile(t, "test_only_regex.txt", content)
	defer os.Remove("test_only_regex.txt")

	output, err := runGrepCmd([]string{"-o", "[0-9]+", "test_only_regex.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	expected := []string{"123", "456", "789"}
	for i, exp := range expected {
		if i >= len(lines) {
			t.Errorf("Missing expected match: %s", exp)
			continue
		}
		// Format is filename:match
		if !strings.HasSuffix(lines[i], ":"+exp) {
			t.Errorf("Expected line ending with ':%s', got '%s'", exp, lines[i])
		}
	}
}

func TestGrepOnlyMatchingStdin(t *testing.T) {
	// Test -o without filename (stdin)
	output, err := runGrepCmdWithStdin([]string{"-o", "test"}, "test123\nfoo456bar\ntest789test\n")
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	expected := []string{"test", "test", "test"}
	for i, exp := range expected {
		if i >= len(lines) {
			t.Errorf("Missing expected match: %s", exp)
			continue
		}
		if lines[i] != exp {
			t.Errorf("Expected '%s', got '%s'", exp, lines[i])
		}
	}
}

func TestGrepFixedStringOnlyMatching(t *testing.T) {
	// Test -F -o combination (fixed string with only matching)
	content := "hello123world456\nfoo123bar\n"
	writeTestFile(t, "test_fixed_only.txt", content)
	defer os.Remove("test_fixed_only.txt")

	output, err := runGrepCmd([]string{"-F", "-o", "123", "test_fixed_only.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 matches, got %d: %s", len(lines), result)
	}
	for _, line := range lines {
		if !strings.HasSuffix(line, ":123") {
			t.Errorf("Expected line ending with ':123', got: %s", line)
		}
	}
}

func TestGrepFixedStringOnlyMatchingStdin(t *testing.T) {
	// Test -F -o with stdin
	output, err := runGrepCmdWithStdin([]string{"-F", "-o", "test"}, "test123test\nfoo\nbar test baz\n")
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	expected := []string{"test", "test", "test"}
	for i, exp := range expected {
		if i >= len(lines) {
			t.Errorf("Missing expected match: %s", exp)
			continue
		}
		if lines[i] != exp {
			t.Errorf("Expected '%s', got '%s'", exp, lines[i])
		}
	}
}

func TestGrepFixedStringOnlyMatchingIgnoreCase(t *testing.T) {
	// Test -F -o -i combination
	content := "TEST123test\nfoo\n"
	writeTestFile(t, "test_fixed_only_i.txt", content)
	defer os.Remove("test_fixed_only_i.txt")

	output, err := runGrepCmd([]string{"-F", "-o", "-i", "test", "test_fixed_only_i.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 matches, got %d: %s", len(lines), result)
	}
	// Check that both TEST and test are matched (case preserved in output)
	if !strings.Contains(result, "TEST") && !strings.Contains(result, "test") {
		t.Errorf("Expected TEST and test in output, got: %s", result)
	}
}

func TestGrepQuiet(t *testing.T) {
	content := "hello world\nfoo bar\nhello again\n"
	writeTestFile(t, "test_quiet.txt", content)
	defer os.Remove("test_quiet.txt")

	// Test quiet with match (exit code 0)
	output, err := runGrepCmd([]string{"-q", "hello", "test_quiet.txt"})
	if err != nil {
		t.Fatalf("grep -q with match should succeed, got: %v", err)
	}
	if len(output) != 0 {
		t.Errorf("Expected no output with -q, got: %s", string(output))
	}

	// Test quiet without match returns exitCodeError(1)
	_, err = runGrepCmd([]string{"-q", "notfound", "test_quiet.txt"})
	if err == nil {
		t.Fatal("Expected error for quiet mode with no match")
	}
	if exitErr, ok := err.(exitCodeError); !ok || int(exitErr) != 1 {
		t.Fatalf("Expected exitCodeError(1), got: %v", err)
	}
}

func TestGrepExtendedRegex(t *testing.T) {
	content := "test123\nfoo456\ntest789\nbar\n"
	writeTestFile(t, "test_extended.txt", content)
	defer os.Remove("test_extended.txt")

	// -E flag enables extended regex (same as default in Go, but tests the flag exists)
	output, err := runGrepCmd([]string{"-E", "test[0-9]+", "test_extended.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "test123") {
		t.Errorf("Expected 'test123' in output, got: %s", result)
	}
	if !strings.Contains(result, "test789") {
		t.Errorf("Expected 'test789' in output, got: %s", result)
	}
}

func TestGrepLineBuffered(t *testing.T) {
	content := "line1\nline2 with hello\nline3\n"
	writeTestFile(t, "test_buffered.txt", content)
	defer os.Remove("test_buffered.txt")

	// --line-buffered flag should not cause errors
	output, err := runGrepCmd([]string{"--line-buffered", "hello", "test_buffered.txt"})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "line2 with hello") {
		t.Errorf("Expected 'line2 with hello' in output, got: %s", result)
	}
}

// Helper function to write test files
func writeTestFile(t *testing.T, filename, content string) {
	err := os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file %s: %v", filename, err)
	}
}
