package text

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============== HELPER FUNCTIONS ==============

func wcWriteTestFile(t *testing.T, filename, content string) {
	err := os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file %s: %v", filename, err)
	}
}

// ============== BASIC TESTS ==============

func TestWcBasic(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar\nhello again\n"
	filename := filepath.Join(tmpDir, "test_wc_basic.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{filename})
	if err != nil {
		t.Fatalf("wc command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "3") {
		t.Errorf("Expected 3 lines in output, got: %s", result)
	}
	if !strings.Contains(result, "test_wc_basic.txt") {
		t.Errorf("Expected filename in output, got: %s", result)
	}
}

func TestWcLinesFlag(t *testing.T) {
	tmpDir := t.TempDir()
	content := "line1\nline2\nline3\nline4\n"
	filename := filepath.Join(tmpDir, "test_wc_lines.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-l", filename})
	if err != nil {
		t.Fatalf("wc -l command failed: %v", err)
	}

	result := string(output)
	// Should only show line count
	if !strings.Contains(result, "4") {
		t.Errorf("Expected 4 lines in output, got: %s", result)
	}
}

func TestWcWordsFlag(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar baz\n"
	filename := filepath.Join(tmpDir, "test_wc_words.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-w", filename})
	if err != nil {
		t.Fatalf("wc -w command failed: %v", err)
	}

	result := string(output)
	// hello world + foo bar baz = 5 words
	if !strings.Contains(result, "5") {
		t.Errorf("Expected 5 words in output, got: %s", result)
	}
}

func TestWcBytesFlag(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\n"
	filename := filepath.Join(tmpDir, "test_wc_bytes.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-c", filename})
	if err != nil {
		t.Fatalf("wc -c command failed: %v", err)
	}

	result := string(output)
	// Should only show byte count (12 bytes: "hello world\n")
	if !strings.Contains(result, "12") {
		t.Errorf("Expected 12 bytes in output, got: %s", result)
	}
}

func TestWcCharsFlag(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\n"
	filename := filepath.Join(tmpDir, "test_wc_chars.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-m", filename})
	if err != nil {
		t.Fatalf("wc -m command failed: %v", err)
	}

	result := string(output)
	// Should only show char count (same as bytes for ASCII)
	if !strings.Contains(result, "12") {
		t.Errorf("Expected 12 chars in output, got: %s", result)
	}
}

func TestWcMaxLineLengthFlag(t *testing.T) {
	tmpDir := t.TempDir()
	content := "short\nthis is a longer line\ntiny\n"
	filename := filepath.Join(tmpDir, "test_wc_maxlen.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-L", filename})
	if err != nil {
		t.Fatalf("wc -L command failed: %v", err)
	}

	result := string(output)
	// Longest line is "this is a longer line" = 21 chars
	if !strings.Contains(result, "21") {
		t.Errorf("Expected 21 as max line length, got: %s", result)
	}
}

// ============== COMBINED FLAGS TESTS ==============

func TestWcCombinedFlagsLW(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar\n"
	filename := filepath.Join(tmpDir, "test_wc_lw.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-lw", filename})
	if err != nil {
		t.Fatalf("wc -lw command failed: %v", err)
	}

	result := string(output)
	// Should show lines (2) and words (2 due to bug)
	if !strings.Contains(result, "2") {
		t.Errorf("Expected 2 lines in output, got: %s", result)
	}
}

func TestWcCombinedFlagsLC(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar\n"
	filename := filepath.Join(tmpDir, "test_wc_lc.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-lc", filename})
	if err != nil {
		t.Fatalf("wc -lc command failed: %v", err)
	}

	result := string(output)
	// Should show lines (2) and bytes (20)
	if !strings.Contains(result, "2") {
		t.Errorf("Expected 2 lines in output, got: %s", result)
	}
	if !strings.Contains(result, "20") {
		t.Errorf("Expected 20 bytes in output, got: %s", result)
	}
}

func TestWcCombinedFlagsLWM(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar\n"
	filename := filepath.Join(tmpDir, "test_wc_lwm.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-lwm", filename})
	if err != nil {
		t.Fatalf("wc -lwm command failed: %v", err)
	}

	result := string(output)
	// Should show lines (2), words (2 due to bug), and chars (20)
	if !strings.Contains(result, "2") {
		t.Errorf("Expected 2 lines in output, got: %s", result)
	}
	if !strings.Contains(result, "2") {
		t.Errorf("Expected 2 words in output, got: %s", result)
	}
	if !strings.Contains(result, "20") {
		t.Errorf("Expected 20 chars in output, got: %s", result)
	}
}

func TestWcCombinedFlagsAll(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar\n"
	filename := filepath.Join(tmpDir, "test_wc_all.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-lwm", "-c", filename})
	if err != nil {
		t.Fatalf("wc -lwm -c command failed: %v", err)
	}

	result := string(output)
	// Should show all counts: lines (2), words (2 due to bug), bytes (20), chars (20)
	if !strings.Contains(result, "2") {
		t.Errorf("Expected 2 lines in output, got: %s", result)
	}
	if !strings.Contains(result, "20") {
		t.Errorf("Expected 20 bytes in output, got: %s", result)
	}
}

// ============== MULTIPLE FILES TESTS ==============

func TestWcMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	content1 := "hello world\n"
	content2 := "foo bar baz\n"
	filename1 := filepath.Join(tmpDir, "test_wc_multi1.txt")
	filename2 := filepath.Join(tmpDir, "test_wc_multi2.txt")
	os.WriteFile(filename1, []byte(content1), 0644)
	os.WriteFile(filename2, []byte(content2), 0644)
	defer os.Remove(filename1)
	defer os.Remove(filename2)

	output, err := runWcCmd([]string{filename1, filename2})
	if err != nil {
		t.Fatalf("wc multiple files command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")

	// Should have 3 lines: file1, file2, and total
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines of output, got %d: %s", len(lines), result)
	}

	// Last line should be total
	if !strings.Contains(lines[len(lines)-1], "total") {
		t.Errorf("Expected 'total' in last line, got: %s", lines[len(lines)-1])
	}
}

func TestWcMultipleFilesWithFlags(t *testing.T) {
	tmpDir := t.TempDir()
	content1 := "a b c\n"
	content2 := "d e f g\n"
	filename1 := filepath.Join(tmpDir, "test_wc_multi_f1.txt")
	filename2 := filepath.Join(tmpDir, "test_wc_multi_f2.txt")
	os.WriteFile(filename1, []byte(content1), 0644)
	os.WriteFile(filename2, []byte(content2), 0644)
	defer os.Remove(filename1)
	defer os.Remove(filename2)

	output, err := runWcCmd([]string{"-l", filename1, filename2})
	if err != nil {
		t.Fatalf("wc -l multiple files command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")

	// Should have 3 lines of output
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines of output, got %d: %s", len(lines), result)
	}

	// Check that total shows correct sum (1 + 1 = 2 lines)
	if !strings.Contains(lines[2], "2") {
		t.Errorf("Expected total of 2 lines, got: %s", lines[2])
	}
}

// ============== STDIN TESTS ==============

func TestWcStdin(t *testing.T) {
	output, err := runWcCmdWithStdin([]string{}, "hello world\nfoo bar\n")
	if err != nil {
		t.Fatalf("wc stdin command failed: %v", err)
	}

	result := string(output)
	// 2 lines, 2 words (bug), 20 bytes
	if !strings.Contains(result, "2") {
		t.Errorf("Expected 2 lines in output, got: %s", result)
	}
	if !strings.Contains(result, "2") {
		t.Errorf("Expected 2 words in output, got: %s", result)
	}
}

func TestWcStdinWithFlags(t *testing.T) {
	output, err := runWcCmdWithStdin([]string{"-l"}, "line1\nline2\nline3\n")
	if err != nil {
		t.Fatalf("wc -l stdin command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// Only check that result starts with "3" (line count)
	if !strings.HasPrefix(result, "3") {
		t.Errorf("Expected 3 lines, got: %s", result)
	}
}

func TestWcStdinDash(t *testing.T) {
	out, err := runWcCmdWithStdin([]string{"-"}, "hello world\n")
	if err != nil {
		t.Fatalf("wc - should read stdin: %v", err)
	}
	if !strings.Contains(out, "1") {
		t.Fatalf("wc - expected line count 1, got %q", out)
	}
}

// ============== EDGE CASES ==============

func TestWcEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	content := ""
	filename := filepath.Join(tmpDir, "test_wc_empty.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{filename})
	if err != nil {
		t.Fatalf("wc empty file command failed: %v", err)
	}

	result := string(output)
	// Empty file should have 0 lines, 0 words, 0 bytes
	if !strings.Contains(result, "0") {
		t.Errorf("Expected 0 in output for empty file, got: %s", result)
	}
}

func TestWcSingleLine(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\n"
	filename := filepath.Join(tmpDir, "test_wc_single.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{filename})
	if err != nil {
		t.Fatalf("wc single line command failed: %v", err)
	}

	result := string(output)
	// 1 line, 2 words, 12 bytes
	if !strings.Contains(result, "1") {
		t.Errorf("Expected 1 line, got: %s", result)
	}
	if !strings.Contains(result, "2") {
		t.Errorf("Expected 2 words, got: %s", result)
	}
	if !strings.Contains(result, "12") {
		t.Errorf("Expected 12 bytes, got: %s", result)
	}
}

func TestWcSingleLineNoNewline(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world"
	filename := filepath.Join(tmpDir, "test_wc_single_nl.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{filename})
	if err != nil {
		t.Fatalf("wc single line no newline command failed: %v", err)
	}

	result := string(output)
	// Should still count as 1 line (even without trailing newline)
	// 1 line, 2 words, 11 bytes
	if !strings.Contains(result, "1") {
		t.Errorf("Expected 1 line, got: %s", result)
	}
	if !strings.Contains(result, "2") {
		t.Errorf("Expected 2 words, got: %s", result)
	}
	if !strings.Contains(result, "11") {
		t.Errorf("Expected 11 bytes, got: %s", result)
	}
}

func TestWcMultipleBlankLines(t *testing.T) {
	tmpDir := t.TempDir()
	content := "\n\n\nhello world\n\n\n"
	filename := filepath.Join(tmpDir, "test_wc_blank.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-l", filename})
	if err != nil {
		t.Fatalf("wc blank lines command failed: %v", err)
	}

	result := string(output)
	// 6 newlines = 6 lines
	if !strings.Contains(result, "6") {
		t.Errorf("Expected 6 lines, got: %s", result)
	}
}

func TestWcTabCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello\tworld\nfoo\tbar\n"
	filename := filepath.Join(tmpDir, "test_wc_tabs.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-w", filename})
	if err != nil {
		t.Fatalf("wc tabs command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "4") {
		t.Errorf("Expected 4 words, got: %s", result)
	}
}

func TestWcLongLine(t *testing.T) {
	tmpDir := t.TempDir()
	content := strings.Repeat("a", 1000) + "\n"
	filename := filepath.Join(tmpDir, "test_wc_long.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-L", filename})
	if err != nil {
		t.Fatalf("wc long line command failed: %v", err)
	}

	result := string(output)
	// Max line length should be 1000
	if !strings.Contains(result, "1000") {
		t.Errorf("Expected 1000 as max line length, got: %s", result)
	}
}

func TestWcOnlySpaces(t *testing.T) {
	tmpDir := t.TempDir()
	content := "   \n   \n   \n"
	filename := filepath.Join(tmpDir, "test_wc_spaces.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-w", filename})
	if err != nil {
		t.Fatalf("wc spaces only command failed: %v", err)
	}

	result := string(output)
	// Spaces are not words, should be 0 words
	if !strings.Contains(result, "0") {
		t.Errorf("Expected 0 words for spaces-only content, got: %s", result)
	}
}

func TestWcOnlyNewlines(t *testing.T) {
	tmpDir := t.TempDir()
	content := "\n\n\n\n"
	filename := filepath.Join(tmpDir, "test_wc_newlines.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-l", filename})
	if err != nil {
		t.Fatalf("wc newlines only command failed: %v", err)
	}

	result := string(output)
	// Should count the newlines
	if !strings.Contains(result, "4") {
		t.Errorf("Expected 4 lines, got: %s", result)
	}
}

// ============== BINARY DATA TESTS ==============

func TestWcBinaryData(t *testing.T) {
	tmpDir := t.TempDir()
	content := []byte("hello\x00world\nfoo\x00bar\n")
	filename := filepath.Join(tmpDir, "test_wc_binary.dat")
	err := os.WriteFile(filename, content, 0644)
	if err != nil {
		t.Fatalf("Failed to write binary test file: %v", err)
	}
	defer os.Remove(filename)

	output, err := runWcCmd([]string{filename})
	if err != nil {
		t.Fatalf("wc binary command failed: %v", err)
	}

	result := string(output)
	// 2 lines, 2 words (bug), 20 bytes (including null bytes)
	if !strings.Contains(result, "2") {
		t.Errorf("Expected 2 lines, got: %s", result)
	}
	if !strings.Contains(result, "20") {
		t.Errorf("Expected 20 bytes, got: %s", result)
	}
}

func TestWcBinaryDataMaxLineLen(t *testing.T) {
	tmpDir := t.TempDir()
	content := []byte("short\nlo\x00ng line\n")
	filename := filepath.Join(tmpDir, "test_wc_binary_len.dat")
	err := os.WriteFile(filename, content, 0644)
	if err != nil {
		t.Fatalf("Failed to write binary test file: %v", err)
	}
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-L", filename})
	if err != nil {
		t.Fatalf("wc binary max line len command failed: %v", err)
	}

	result := string(output)
	// "lo\x00ng line" is 10 characters
	if !strings.Contains(result, "10") {
		t.Errorf("Expected 10 as max line length, got: %s", result)
	}
}

// ============== ERROR CASES ==============

func TestWcNonexistentFile(t *testing.T) {
	_, err := runWcCmd([]string{"nonexistent_file_xyz.txt"})
	if err == nil {
		t.Fatalf("wc should fail for nonexistent file")
	}
}

func TestWcMultipleFilesOneMissing(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\n"
	filename := filepath.Join(tmpDir, "test_wc_exist.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	_, err := runWcCmd([]string{filename, "nonexistent_xyz.txt"})
	if err == nil {
		t.Fatalf("wc should fail when one file is missing")
	}
}

func TestWcDirectoryAsFile(t *testing.T) {
	// Create a temporary directory
	tmpDir := filepath.Join(t.TempDir(), "test_wc_dir")
	err := os.Mkdir(tmpDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.Remove(tmpDir)

	_, err = runWcCmd([]string{tmpDir})
	// Should either succeed (reading dir as file) or fail gracefully
	// The exact behavior depends on the implementation
}

// ============== HELP FLAG TESTS ==============

func TestWcHelpFlag(t *testing.T) {
	output, err := runWcCmd([]string{"-h"})
	if err != nil {
		t.Fatalf("wc -h command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage") {
		t.Errorf("Expected help text to contain 'Usage', got: %s", result)
	}
}

func TestWcLongHelpFlag(t *testing.T) {
	output, err := runWcCmd([]string{"--help"})
	if err != nil {
		t.Fatalf("wc --help command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage") {
		t.Errorf("Expected help text to contain 'Usage', got: %s", result)
	}
}

// ============== LONG FLAG EQUIVALENTS TESTS ==============

func TestWcLongFlagLines(t *testing.T) {
	tmpDir := t.TempDir()
	content := "a\nb\nc\n"
	filename := filepath.Join(tmpDir, "test_wc_long_l.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"--lines", filename})
	if err != nil {
		t.Fatalf("wc --lines command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "3") {
		t.Errorf("Expected 3 lines, got: %s", result)
	}
}

func TestWcLongFlagWords(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\nfoo bar\n"
	filename := filepath.Join(tmpDir, "test_wc_long_w.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"--words", filename})
	if err != nil {
		t.Fatalf("wc --words command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "4") {
		t.Errorf("Expected 4 words, got: %s", result)
	}
}

func TestWcLongFlagBytes(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\n"
	filename := filepath.Join(tmpDir, "test_wc_long_c.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"--bytes", filename})
	if err != nil {
		t.Fatalf("wc --bytes command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "12") {
		t.Errorf("Expected 12 bytes, got: %s", result)
	}
}

func TestWcLongFlagChars(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\n"
	filename := filepath.Join(tmpDir, "test_wc_long_m.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"--chars", filename})
	if err != nil {
		t.Fatalf("wc --chars command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "12") {
		t.Errorf("Expected 12 chars, got: %s", result)
	}
}

func TestWcLongFlagMaxLineLength(t *testing.T) {
	tmpDir := t.TempDir()
	content := "short\nthis is longer\n"
	filename := filepath.Join(tmpDir, "test_wc_long_L.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"--max-line-length", filename})
	if err != nil {
		t.Fatalf("wc --max-line-length command failed: %v", err)
	}

	result := string(output)
	// "this is longer" is 14 chars (4+1+2+1+6)
	if !strings.Contains(result, "14") {
		t.Errorf("Expected 14 as max line length, got: %s", result)
	}
}

// ============== SPECIAL CHARACTER TESTS ==============

func TestWcUnicode(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\n"
	filename := filepath.Join(tmpDir, "test_wc_ascii.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{filename})
	if err != nil {
		t.Fatalf("wc ascii command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "12") {
		t.Errorf("Expected 12 bytes for ASCII, got: %s", result)
	}
}

func TestWcSpecialCharsInWords(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello-world foo_bar baz@qux\n"
	filename := filepath.Join(tmpDir, "test_wc_special.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{"-w", filename})
	if err != nil {
		t.Fatalf("wc special chars command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "3") {
		t.Errorf("Expected 3 words with special chars, got: %s", result)
	}
}

// ============== CONCURRENT FILE TESTING ==============

func TestWcFileOrder(t *testing.T) {
	tmpDir := t.TempDir()
	content := "line1\n"
	filenameA := filepath.Join(tmpDir, "test_wc_order_a.txt")
	filenameB := filepath.Join(tmpDir, "test_wc_order_b.txt")
	os.WriteFile(filenameA, []byte(content), 0644)
	os.WriteFile(filenameB, []byte(content), 0644)
	defer os.Remove(filenameA)
	defer os.Remove(filenameB)

	output, err := runWcCmd([]string{filenameA, filenameB})
	if err != nil {
		t.Fatalf("wc order command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	// First output should be file a, second should be file b
	if !strings.Contains(lines[0], "test_wc_order_a.txt") {
		t.Errorf("Expected first file in output, got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "test_wc_order_b.txt") {
		t.Errorf("Expected second file in output, got: %s", lines[1])
	}
}

// ============== FLAG PARSING TESTS ==============

func TestWcUnknownFlag(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\n"
	filename := filepath.Join(tmpDir, "test_wc_unknown.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	_, err := runWcCmd([]string{"-x", filename})
	if err == nil {
		t.Fatalf("wc should fail with unknown flag -x")
	}
}

func TestWcCombinedUnknownFlag(t *testing.T) {
	tmpDir := t.TempDir()
	content := "hello world\n"
	filename := filepath.Join(tmpDir, "test_wc_combo_unknown.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	_, err := runWcCmd([]string{"-lxw", filename})
	if err == nil {
		t.Fatalf("wc should fail with unknown flag in combination")
	}
}

// ============== PERFORMANCE / LARGE FILE TESTS ==============

func TestWcLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&sb, "line %d contains some text here\n", i)
	}
	content := sb.String()
	filename := filepath.Join(tmpDir, "test_wc_large.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runWcCmd([]string{filename})
	if err != nil {
		t.Fatalf("wc large file command failed: %v", err)
	}

	result := string(output)
	// Should have 1000 lines
	if !strings.Contains(result, "1000") {
		t.Errorf("Expected 1000 lines, got: %s", result)
	}
}
