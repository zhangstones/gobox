package text

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepBasicMatch(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar\nhello again\n"
	filename := filepath.Join(tmpDir, "test_basic.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"hello", filename})
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
	tmpDir := t.TempDir()
	content := "HELLO world\nfoo BAR\nHello Again\n"
	filename := filepath.Join(tmpDir, "test_case.txt")
	os.WriteFile(filename, []byte(content), 0644)

	output, err := runGrepCmd([]string{"-i", "hello", filename})
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
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar\nhello again\nbaz qux\n"
	filename := filepath.Join(tmpDir, "test_invert.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"-v", "hello", filename})
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
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar\nhello again\nhello third\n"
	filename := filepath.Join(tmpDir, "test_count.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"-c", "hello", filename})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if !strings.Contains(result, "3") {
		t.Errorf("Expected count 3 in output, got: %s", result)
	}
}

func TestGrepLineNumber(t *testing.T) {
	tmpDir := t.TempDir()
	content := "first line\nsecond line with hello\nthird line\n"
	filename := filepath.Join(tmpDir, "test_linenum.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"-n", "hello", filename})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "2:second line with hello") {
		t.Errorf("Expected line number 2 in output, got: %s", result)
	}
}

func TestGrepFixedString(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello.world\nfoo bar\nhelloXworld\n"
	filename := filepath.Join(tmpDir, "test_fixed.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"-F", "hello.world", filename})
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
	tmpDir := t.TempDir()
	content := "test123\nfoo456\ntest789\nbar\n"
	filename := filepath.Join(tmpDir, "test_regex.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"test[0-9]+", filename})
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
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar\n"
	filename := filepath.Join(tmpDir, "test_nomatch.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"notfound", filename})
	_ = output
	_ = err

	if len(output) != 0 {
		t.Errorf("Expected empty output for no matches, got: %s", string(output))
	}
}

func TestGrepRecursive(t *testing.T) {
	tmpDir := t.TempDir()
	testdir := filepath.Join(tmpDir, "testdir")
	os.MkdirAll(filepath.Join(testdir, "subdir"), 0755)
	defer os.RemoveAll(testdir)

	file1 := filepath.Join(testdir, "file1.txt")
	os.WriteFile(file1, []byte("hello world\n"), 0644)
	defer os.Remove(file1)

	file2 := filepath.Join(testdir, "subdir", "file2.txt")
	os.WriteFile(file2, []byte("hello again\n"), 0644)
	defer os.Remove(file2)

	file3 := filepath.Join(testdir, "subdir", "file3.txt")
	os.WriteFile(file3, []byte("goodbye\n"), 0644)
	defer os.Remove(file3)

	output, err := runGrepCmd([]string{"-r", "hello", testdir})
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
	tmpDir := t.TempDir()
	content := "test123\nfoo456bar\ntest789test\n"
	filename := filepath.Join(tmpDir, "test_only.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"-o", "test", filename})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
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
	tmpDir := t.TempDir()
	content := "test123\nfoo456bar\ntest789test\n"
	filename := filepath.Join(tmpDir, "test_only_regex.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"-o", "[0-9]+", filename})
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
	tmpDir := t.TempDir()
	content := "hello123world456\nfoo123bar\n"
	filename := filepath.Join(tmpDir, "test_fixed_only.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"-F", "-o", "123", filename})
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
	tmpDir := t.TempDir()
	content := "TEST123test\nfoo\n"
	filename := filepath.Join(tmpDir, "test_fixed_only_i.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"-F", "-o", "-i", "test", filename})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 matches, got %d: %s", len(lines), result)
	}
	if !strings.Contains(result, "TEST") && !strings.Contains(result, "test") {
		t.Errorf("Expected TEST and test in output, got: %s", result)
	}
}

func TestGrepQuiet(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar\nhello again\n"
	filename := filepath.Join(tmpDir, "test_quiet.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"-q", "hello", filename})
	if err != nil {
		t.Fatalf("grep -q with match should succeed, got: %v", err)
	}
	if len(output) != 0 {
		t.Errorf("Expected no output with -q, got: %s", string(output))
	}

	_, err = runGrepCmd([]string{"-q", "notfound", filename})
	if err == nil {
		t.Fatal("Expected error for quiet mode with no match")
	}
	if exitErr, ok := err.(exitCodeError); !ok || int(exitErr) != 1 {
		t.Fatalf("Expected exitCodeError(1), got: %v", err)
	}
}

func TestGrepExtendedRegex(t *testing.T) {
	tmpDir := t.TempDir()
	content := "test123\nfoo456\ntest789\nbar\n"
	filename := filepath.Join(tmpDir, "test_extended.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"-E", "test[0-9]+", filename})
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
	tmpDir := t.TempDir()
	content := "line1\nline2 with hello\nline3\n"
	filename := filepath.Join(tmpDir, "test_buffered.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runGrepCmd([]string{"--line-buffered", "hello", filename})
	if err != nil {
		t.Fatalf("grep command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "line2 with hello") {
		t.Errorf("Expected 'line2 with hello' in output, got: %s", result)
	}
}

// writeTestFile helper kept for compatibility with other test files in this package
func writeTestFile(t *testing.T, filename, content string) {
	err := os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file %s: %v", filename, err)
	}
}
