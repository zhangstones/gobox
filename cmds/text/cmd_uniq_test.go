package text

import (
	"os"
	"strings"
	"testing"
)

// ============== NORMAL CASES ==============

func TestUniqBasic(t *testing.T) {
	// Basic uniq: removes adjacent duplicates
	content := "apple\napple\nbanana\norange\norange\norange\n"
	uniqWriteTestFile(t, "test_uniq_basic.txt", content)
	defer os.Remove("test_uniq_basic.txt")

	output, err := runUniqCmd([]string{"test_uniq_basic.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
	if lines[0] != "apple" {
		t.Errorf("Expected first line 'apple', got: %s", lines[0])
	}
	if lines[1] != "banana" {
		t.Errorf("Expected second line 'banana', got: %s", lines[1])
	}
	if lines[2] != "orange" {
		t.Errorf("Expected third line 'orange', got: %s", lines[2])
	}
}

func TestUniqCount(t *testing.T) {
	// -c flag: prefix lines by number of occurrences
	content := "foo\nfoo\nfoo\nbar\nbar\nbaz\n"
	uniqWriteTestFile(t, "test_uniq_count.txt", content)
	defer os.Remove("test_uniq_count.txt")

	output, err := runUniqCmd([]string{"-c", "test_uniq_count.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "3 foo") {
		t.Errorf("Expected '3 foo' in output, got: %s", result)
	}
	if !strings.Contains(result, "2 bar") {
		t.Errorf("Expected '2 bar' in output, got: %s", result)
	}
	if !strings.Contains(result, "1 baz") {
		t.Errorf("Expected '1 baz' in output, got: %s", result)
	}
}

func TestUniqRepeated(t *testing.T) {
	// -d flag: only print duplicate lines
	content := "apple\napple\nbanana\ncherry\ncherry\ncherry\ndate\n"
	uniqWriteTestFile(t, "test_uniq_dup.txt", content)
	defer os.Remove("test_uniq_dup.txt")

	output, err := runUniqCmd([]string{"-d", "test_uniq_dup.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), result)
	}
	if lines[0] != "apple" {
		t.Errorf("Expected 'apple', got: %s", lines[0])
	}
	if lines[1] != "cherry" {
		t.Errorf("Expected 'cherry', got: %s", lines[1])
	}
	// 'date' appears once, should not be in output
	if strings.Contains(result, "date") {
		t.Errorf("'date' should not appear (only appears once), got: %s", result)
	}
}

func TestUniqUnique(t *testing.T) {
	// -u flag: only print unique lines
	content := "apple\napple\nbanana\ncherry\ncherry\ncherry\ndate\negg\n"
	uniqWriteTestFile(t, "test_uniq_uniq.txt", content)
	defer os.Remove("test_uniq_uniq.txt")

	output, err := runUniqCmd([]string{"-u", "test_uniq_uniq.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// Only 'banana' and 'date' and 'egg' appear once
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
	if !strings.Contains(result, "banana") {
		t.Errorf("Expected 'banana' in output, got: %s", result)
	}
	if !strings.Contains(result, "date") {
		t.Errorf("Expected 'date' in output, got: %s", result)
	}
	if !strings.Contains(result, "egg") {
		t.Errorf("Expected 'egg' in output, got: %s", result)
	}
}

// ============== EDGE CASES ==============

func TestUniqEmptyFile(t *testing.T) {
	content := ""
	uniqWriteTestFile(t, "test_uniq_empty.txt", content)
	defer os.Remove("test_uniq_empty.txt")

	output, err := runUniqCmd([]string{"test_uniq_empty.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	if result != "" {
		t.Errorf("Expected empty output for empty file, got: %s", result)
	}
}

func TestUniqSingleLine(t *testing.T) {
	content := "onlyone\n"
	uniqWriteTestFile(t, "test_uniq_single.txt", content)
	defer os.Remove("test_uniq_single.txt")

	output, err := runUniqCmd([]string{"test_uniq_single.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	if result != "onlyone" {
		t.Errorf("Expected 'onlyone', got: %s", result)
	}
}

func TestUniqAllIdentical(t *testing.T) {
	content := "same\nsame\nsame\nsame\nsame\n"
	uniqWriteTestFile(t, "test_uniq_all_same.txt", content)
	defer os.Remove("test_uniq_all_same.txt")

	output, err := runUniqCmd([]string{"test_uniq_all_same.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	if result != "same" {
		t.Errorf("Expected 'same', got: %s", result)
	}
}

func TestUniqAllUnique(t *testing.T) {
	content := "one\ntwo\nthree\nfour\nfive\n"
	uniqWriteTestFile(t, "test_uniq_all_unique.txt", content)
	defer os.Remove("test_uniq_all_unique.txt")

	output, err := runUniqCmd([]string{"test_uniq_all_unique.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// All lines are unique, so all should appear
	lines := strings.Split(result, "\n")
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines, got %d: %s", len(lines), result)
	}
}

func TestUniqAlternating(t *testing.T) {
	content := "a\na\nb\nb\nc\nc\n"
	uniqWriteTestFile(t, "test_uniq_alternating.txt", content)
	defer os.Remove("test_uniq_alternating.txt")

	output, err := runUniqCmd([]string{"test_uniq_alternating.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
}

func TestUniqCountWithSingleLines(t *testing.T) {
	// All lines are unique, each should show count of 1
	content := "a\nb\nc\nd\n"
	uniqWriteTestFile(t, "test_uniq_count_single.txt", content)
	defer os.Remove("test_uniq_count_single.txt")

	output, err := runUniqCmd([]string{"-c", "test_uniq_count_single.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	lines := strings.Split(strings.TrimSpace(result), "\n")
	// Format is right-justified: "      1 a", "      1 b", etc.
	for i, line := range lines {
		if !strings.Contains(line, " 1 ") && !strings.HasSuffix(line, " a") && !strings.HasSuffix(line, " b") {
			t.Errorf("Expected count 1 for line %d, got: %s", i+1, line)
		}
	}
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d: %s", len(lines), result)
	}
}

func TestUniqCountWithAllIdentical(t *testing.T) {
	content := "x\nx\nx\nx\n"
	uniqWriteTestFile(t, "test_uniq_count_all_same.txt", content)
	defer os.Remove("test_uniq_count_all_same.txt")

	output, err := runUniqCmd([]string{"-c", "test_uniq_count_all_same.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	if result != "4 x" {
		t.Errorf("Expected '4 x', got: %s", result)
	}
}

// ============== IGNORE CASE TESTS ==============

func TestUniqIgnoreCase(t *testing.T) {
	content := "Apple\nAPPLE\napple\nBanana\nBANANA\nbanana\n"
	uniqWriteTestFile(t, "test_uniq_ignore_case.txt", content)
	defer os.Remove("test_uniq_ignore_case.txt")

	output, err := runUniqCmd([]string{"-i", "test_uniq_ignore_case.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines (Apple and Banana groups), got %d: %s", len(lines), result)
	}
}

func TestUniqIgnoreCaseCount(t *testing.T) {
	content := "Foo\nfoo\nFoo\nBar\nbar\n"
	uniqWriteTestFile(t, "test_uniq_ignore_case_count.txt", content)
	defer os.Remove("test_uniq_ignore_case_count.txt")

	output, err := runUniqCmd([]string{"-c", "-i", "test_uniq_ignore_case_count.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "3 Foo") {
		t.Errorf("Expected '3 Foo' in output, got: %s", result)
	}
	if !strings.Contains(result, "2 Bar") {
		t.Errorf("Expected '2 Bar' in output, got: %s", result)
	}
}

func TestUniqIgnoreCaseRepeated(t *testing.T) {
	content := "AAA\naaa\nBBB\nbbb\nCCC\n"
	uniqWriteTestFile(t, "test_uniq_ignore_case_dup.txt", content)
	defer os.Remove("test_uniq_ignore_case_dup.txt")

	output, err := runUniqCmd([]string{"-d", "-i", "test_uniq_ignore_case_dup.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), result)
	}
}

func TestUniqIgnoreCaseUnique(t *testing.T) {
	content := "AAA\naaa\nBBB\nbbb\nCCC\n"
	uniqWriteTestFile(t, "test_uniq_ignore_case_uniq.txt", content)
	defer os.Remove("test_uniq_ignore_case_uniq.txt")

	output, err := runUniqCmd([]string{"-u", "-i", "test_uniq_ignore_case_uniq.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// CCC appears only once (case-sensitive), so it should appear even with -i
	// AAA/aaa is 3 when ignoring case, BBB/bbb is 2 when ignoring case
	if result != "CCC" {
		t.Errorf("Expected 'CCC' (only line that appears once even with case ignored), got: %s", result)
	}
}

// ============== CHECK CHARS TESTS ==============

func TestUniqCheckChars(t *testing.T) {
	// -w 4 truncates comparison to first 4 characters
	// "abcdef" -> "abcd", "ghijkl" -> "ghij", "abcxyz" -> "abcx", "ghmnop" -> "ghmn", "abc123" -> "abc1"
	// None of these are equal to their predecessor, so all 5 appear
	content := "abcdef\nghijkl\nabcxyz\nghmnop\nabc123\n"
	uniqWriteTestFile(t, "test_uniq_check_chars.txt", content)
	defer os.Remove("test_uniq_check_chars.txt")

	output, err := runUniqCmd([]string{"-w", "4", "test_uniq_check_chars.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// All 5 lines have different first-4-char prefixes
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines (all different after truncation), got %d: %s", len(lines), result)
	}
}

func TestUniqCheckCharsCount(t *testing.T) {
	content := "hello world\nhello there\nhello again\n"
	uniqWriteTestFile(t, "test_uniq_check_chars_count.txt", content)
	defer os.Remove("test_uniq_check_chars_count.txt")

	output, err := runUniqCmd([]string{"-c", "-w", "5", "test_uniq_check_chars_count.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "3 hello world") {
		t.Errorf("Expected '3 hello world' in output, got: %s", result)
	}
}

func TestUniqCheckCharsRepeated(t *testing.T) {
	// -w 3 means only first 3 chars are compared
	content := "foobar1\nfoobar2\nbazbar1\nbazbar2\n"
	uniqWriteTestFile(t, "test_uniq_check_chars_dup.txt", content)
	defer os.Remove("test_uniq_check_chars_dup.txt")

	output, err := runUniqCmd([]string{"-d", "-w", "3", "test_uniq_check_chars_dup.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// "foobar1" -> "foo", "foobar2" -> "foo", "bazbar1" -> "baz", "bazbar2" -> "baz"
	// foo group has 2 entries, baz group has 2 entries
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines (foo and baz both repeated), got %d: %s", len(lines), result)
	}
}

func TestUniqCheckCharsZero(t *testing.T) {
	// -w 0 means no truncation, full line comparison
	content := "a\na\nb\nb\n"
	uniqWriteTestFile(t, "test_uniq_check_chars_zero.txt", content)
	defer os.Remove("test_uniq_check_chars_zero.txt")

	output, err := runUniqCmd([]string{"-w", "0", "test_uniq_check_chars_zero.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// a and b are each duplicated, so they get merged
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines (a and b merged), got %d: %s", len(lines), result)
	}
}

func TestUniqCheckCharsExact(t *testing.T) {
	// Compare exactly 3 characters: "abc" and "abd" are different
	content := "abcdef\nabdxyz\nabc123\n"
	uniqWriteTestFile(t, "test_uniq_check_chars_exact.txt", content)
	defer os.Remove("test_uniq_check_chars_exact.txt")

	output, err := runUniqCmd([]string{"-w", "3", "test_uniq_check_chars_exact.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines (abc vs abd differ at char 3), got %d: %s", len(lines), result)
	}
}

// ============== SKIP FIELDS TESTS ==============

func TestUniqSkipFields(t *testing.T) {
	// Skip first field (whitespace separated)
	// After skipping 1 field: "word1", "word2", "hello", "world" - all different
	content := "field1 word1\nfield1 word2\nfield2 hello\nfield2 world\n"
	uniqWriteTestFile(t, "test_uniq_skip_fields.txt", content)
	defer os.Remove("test_uniq_skip_fields.txt")

	output, err := runUniqCmd([]string{"-f", "1", "test_uniq_skip_fields.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// All 4 lines have different second fields
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines (all different after skipping 1 field), got %d: %s", len(lines), result)
	}
}

func TestUniqSkipFieldsCount(t *testing.T) {
	// After skipping first field: "X", "Y", "Z", "P", "Q" - all different
	content := "a X\na Y\na Z\nb P\nb Q\n"
	uniqWriteTestFile(t, "test_uniq_skip_fields_count.txt", content)
	defer os.Remove("test_uniq_skip_fields_count.txt")

	output, err := runUniqCmd([]string{"-c", "-f", "1", "test_uniq_skip_fields_count.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	lines := strings.Split(strings.TrimSpace(result), "\n")
	// All 5 lines have different second fields, each count = 1
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines (all different after skipping 1 field), got %d: %s", len(lines), result)
	}
}

func TestUniqSkipFieldsRepeated(t *testing.T) {
	// After skipping 1 field: data1, data2, data3, data4, data5 - all different
	content := "ID1 data1\nID1 data2\nID2 data3\nID2 data4\nID3 data5\n"
	uniqWriteTestFile(t, "test_uniq_skip_fields_dup.txt", content)
	defer os.Remove("test_uniq_skip_fields_dup.txt")

	output, err := runUniqCmd([]string{"-d", "-f", "1", "test_uniq_skip_fields_dup.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// All lines have different data values after skipping first field, so nothing printed with -d
	if result != "" {
		t.Errorf("Expected empty output (all data values are different), got: %s", result)
	}
}

func TestUniqSkipFieldsMultiple(t *testing.T) {
	// Skip first 2 fields
	// After skip 2: "value1", "value2", "value3", "value4" - all different
	content := "f1 f2 value1\nf1 f2 value2\nf1 f3 value3\nf2 f2 value4\n"
	uniqWriteTestFile(t, "test_uniq_skip_fields_multi.txt", content)
	defer os.Remove("test_uniq_skip_fields_multi.txt")

	output, err := runUniqCmd([]string{"-f", "2", "test_uniq_skip_fields_multi.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// All values after skip 2 fields are different, so all 4 lines appear
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines (all different after skipping 2 fields), got %d: %s", len(lines), result)
	}
}

func TestUniqSkipFieldsZero(t *testing.T) {
	// -f 0 means no fields skipped, just regular uniq
	// Input "a\na\nb\nb\n" should output "a", "b"
	content := "a\na\nb\nb\n"
	uniqWriteTestFile(t, "test_uniq_skip_fields_zero.txt", content)
	defer os.Remove("test_uniq_skip_fields_zero.txt")

	output, err := runUniqCmd([]string{"-f", "0", "test_uniq_skip_fields_zero.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// With -f 0, no fields skipped. Input has a,a,b,b -> output a,b (2 lines)
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), result)
	}
}

func TestUniqSkipFieldsMoreThanLineHas(t *testing.T) {
	// Skip more fields than line has - line becomes empty
	content := "singlefield\nanother\n"
	uniqWriteTestFile(t, "test_uniq_skip_fields_over.txt", content)
	defer os.Remove("test_uniq_skip_fields_over.txt")

	output, err := runUniqCmd([]string{"-f", "5", "test_uniq_skip_fields_over.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// Both lines become empty after skipping 5 fields, so only one output
	if len(lines) != 1 {
		t.Errorf("Expected 1 line (both lines become empty), got %d: %s", len(lines), result)
	}
}

// ============== COMBINED FLAGS TESTS ==============

func TestUniqIgnoreCaseAndCheckChars(t *testing.T) {
	// -f 0, -i, -w 3: first 3 chars compared, ignoring case
	// "ABCdef" -> "abc", "abcDEF" -> "abc", "abcxyz" -> "abc", "ABCGHI" -> "abc"
	// All the same after truncation and case fold
	content := "ABCdef\nabcDEF\nabcxyz\nABCGHI\n"
	uniqWriteTestFile(t, "test_uniq_combo1.txt", content)
	defer os.Remove("test_uniq_combo1.txt")

	output, err := runUniqCmd([]string{"-i", "-w", "3", "test_uniq_combo1.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// All 4 lines become "abc" after truncation to 3 chars and case fold
	if len(lines) != 1 {
		t.Errorf("Expected 1 line (all same after truncation and case fold), got %d: %s", len(lines), result)
	}
}

func TestUniqSkipFieldsAndIgnoreCase(t *testing.T) {
	// -f 1, -i, -u: skip 1 field, ignore case, unique only
	// "ID Hello" -> "hello", "ID hello" -> "hello", "ID World" -> "world", "id hello" -> "hello"
	// hello appears 2 times then once separated, world appears once
	// With -u: only lines with count=1 are printed: "ID World" and "id hello"
	content := "ID Hello\nID hello\nID World\nid hello\n"
	uniqWriteTestFile(t, "test_uniq_combo2.txt", content)
	defer os.Remove("test_uniq_combo2.txt")

	output, err := runUniqCmd([]string{"-u", "-f", "1", "-i", "test_uniq_combo2.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// Only unique lines (count=1): "ID World" and "id hello"
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), result)
	}
	if lines[0] != "ID World" {
		t.Errorf("Expected 'ID World' as first line, got: %s", lines[0])
	}
	if lines[1] != "id hello" {
		t.Errorf("Expected 'id hello' as second line, got: %s", lines[1])
	}
}

func TestUniqSkipFieldsAndCheckChars(t *testing.T) {
	content := "f1 abcdef\nf1 abcxyz\nf2 abc123\nf2 abc789\n"
	uniqWriteTestFile(t, "test_uniq_combo3.txt", content)
	defer os.Remove("test_uniq_combo3.txt")

	output, err := runUniqCmd([]string{"-f", "1", "-w", "3", "test_uniq_combo3.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// Skip first field: "abcdef", "abcxyz", "abc123", "abc789"
	// Compare first 3 chars: "abc", "abc", "abc", "abc" - all same group
	if len(lines) != 1 {
		t.Errorf("Expected 1 line, got %d: %s", len(lines), result)
	}
}

func TestUniqCountWithRepeatedAndUnique(t *testing.T) {
	// -c and -d together: shows count for repeated lines only
	content := "a\na\na\nb\nc\nc\n"
	uniqWriteTestFile(t, "test_uniq_combo4.txt", content)
	defer os.Remove("test_uniq_combo4.txt")

	output, err := runUniqCmd([]string{"-c", "-d", "test_uniq_combo4.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	// Only 'a' (count 3) and 'c' (count 2) are duplicated
	if !strings.Contains(result, "3 a") {
		t.Errorf("Expected '3 a' in output, got: %s", result)
	}
	if !strings.Contains(result, "2 c") {
		t.Errorf("Expected '2 c' in output, got: %s", result)
	}
	if strings.Contains(result, "1 b") {
		t.Errorf("'b' should not appear (only once), got: %s", result)
	}
}

func TestUniqAllFlagsCombined(t *testing.T) {
	// -f 1, -i, -w 3: skip 1 field, ignore case, compare first 3 chars
	// Tracing:
	// "f1 AAaaa" -> skip1: "AAaaa", w3: "AAA", -i: "aaa"
	// "aa f2 AAAbb" -> skip1: "f2 AAAAbb", w3: "f2 ", -i: "f2 "
	// "f1 aaaaA" -> skip1: "aaaaA", w3: "aaa", -i: "aaa"
	// "f2 AAa" -> skip1: "AAa", w3 (no trunc, len=3): "AAa", -i: "aaa"
	// Groups: "aaa"(f1 AAaaa), "f2 "(aa f2 AAAbb), "aaa"(f1 aaaaA), "aaa"(f2 AAa)
	// Output: 3 lines: f1 AAaaa, aa f2 AAAbb, f1 aaaaA
	content := "f1 AAaaa\naa f2 AAAbb\nf1 aaaaA\nf2 AAa\n"
	uniqWriteTestFile(t, "test_uniq_combo_all.txt", content)
	defer os.Remove("test_uniq_combo_all.txt")

	output, err := runUniqCmd([]string{"-f", "1", "-i", "-w", "3", "test_uniq_combo_all.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
}

// ============== STDIN TESTS ==============

func TestUniqStdin(t *testing.T) {
	output, err := runUniqCmdWithStdin([]string{}, "a\na\nb\nb\nc\n")
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
}

func TestUniqStdinCount(t *testing.T) {
	output, err := runUniqCmdWithStdin([]string{"-c"}, "x\nx\ny\nz\nz\n")
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "2 x") {
		t.Errorf("Expected '2 x' in output, got: %s", result)
	}
	if !strings.Contains(result, "1 y") {
		t.Errorf("Expected '1 y' in output, got: %s", result)
	}
	if !strings.Contains(result, "2 z") {
		t.Errorf("Expected '2 z' in output, got: %s", result)
	}
}

func TestUniqStdinRepeated(t *testing.T) {
	output, err := runUniqCmdWithStdin([]string{"-d"}, "a\na\nb\nc\nc\n")
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), result)
	}
	if lines[0] != "a" || lines[1] != "c" {
		t.Errorf("Expected 'a' and 'c', got: %s", result)
	}
}

func TestUniqStdinUnique(t *testing.T) {
	output, err := runUniqCmdWithStdin([]string{"-u"}, "a\na\nb\nc\nc\nd\n")
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// Only 'b' and 'd' appear once
	if !strings.Contains(result, "b") {
		t.Errorf("Expected 'b' in output, got: %s", result)
	}
	if !strings.Contains(result, "d") {
		t.Errorf("Expected 'd' in output, got: %s", result)
	}
	if strings.Contains(result, "a") || strings.Contains(result, "c") {
		t.Errorf("'a' and 'c' should not appear (duplicates), got: %s", result)
	}
}

func TestUniqStdinIgnoreCase(t *testing.T) {
	output, err := runUniqCmdWithStdin([]string{"-i"}, "Foo\nfoo\nBAR\nbar\nBaz\n")
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// Foo/foo -> one group, BAR/bar -> one group, Baz -> one group
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
}

func TestUniqStdinCheckChars(t *testing.T) {
	// -w 4: truncate comparison to first 4 chars
	// "abcdef" -> "abcd", "ghijkl" -> "ghij", "abcxyz" -> "abcx"
	// All different, so all 3 appear
	output, err := runUniqCmdWithStdin([]string{"-w", "4"}, "abcdef\nghijkl\nabcxyz\n")
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// All 3 lines have different first-4-char prefixes
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines (all different after truncation), got %d: %s", len(lines), result)
	}
}

func TestUniqStdinSkipFields(t *testing.T) {
	output, err := runUniqCmdWithStdin([]string{"-f", "1"}, "id1 value1\nid1 value2\nid2 value3\n")
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// Skip first field: "value1", "value2", "value3"
	// value1 != value2, value2 != value3 -> all 3
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
}

// ============== ERROR CASES ==============

func TestUniqNonexistentFile(t *testing.T) {
	_, err := runUniqCmd([]string{"nonexistent_file_12345.txt"})
	if err == nil {
		t.Fatalf("Expected error for non-existent file, got none")
	}
}

func TestUniqInvalidCheckChars(t *testing.T) {
	content := "a\nb\n"
	uniqWriteTestFile(t, "test_uniq_invalid_chars.txt", content)
	defer os.Remove("test_uniq_invalid_chars.txt")

	_, err := runUniqCmd([]string{"-w", "abc", "test_uniq_invalid_chars.txt"})
	if err == nil {
		t.Fatalf("Expected error for invalid -w argument, got none")
	}
}

func TestUniqInvalidSkipFields(t *testing.T) {
	content := "a\nb\n"
	uniqWriteTestFile(t, "test_uniq_invalid_fields.txt", content)
	defer os.Remove("test_uniq_invalid_fields.txt")

	_, err := runUniqCmd([]string{"-f", "xyz", "test_uniq_invalid_fields.txt"})
	if err == nil {
		t.Fatalf("Expected error for invalid -f argument, got none")
	}
}

func TestUniqNegativeCheckChars(t *testing.T) {
	content := "a\nb\n"
	uniqWriteTestFile(t, "test_uniq_neg_chars.txt", content)
	defer os.Remove("test_uniq_neg_chars.txt")

	_, err := runUniqCmd([]string{"-w", "-1", "test_uniq_neg_chars.txt"})
	if err == nil {
		t.Fatalf("Expected error for negative -w argument, got none")
	}
}

func TestUniqNegativeSkipFields(t *testing.T) {
	content := "a\nb\n"
	uniqWriteTestFile(t, "test_uniq_neg_fields.txt", content)
	defer os.Remove("test_uniq_neg_fields.txt")

	_, err := runUniqCmd([]string{"-f", "-5", "test_uniq_neg_fields.txt"})
	if err == nil {
		t.Fatalf("Expected error for negative -f argument, got none")
	}
}

func TestUniqUnknownOption(t *testing.T) {
	content := "a\nb\n"
	uniqWriteTestFile(t, "test_uniq_unknown.txt", content)
	defer os.Remove("test_uniq_unknown.txt")

	_, err := runUniqCmd([]string{"-z", "test_uniq_unknown.txt"})
	if err == nil {
		t.Fatalf("Expected error for unknown option, got none")
	}
}

func TestUniqMissingCheckCharsArg(t *testing.T) {
	content := "a\nb\n"
	uniqWriteTestFile(t, "test_uniq_missing_chars.txt", content)
	defer os.Remove("test_uniq_missing_chars.txt")

	_, err := runUniqCmd([]string{"-w", "test_uniq_missing_chars.txt"})
	if err == nil {
		t.Fatalf("Expected error for missing -w argument, got none")
	}
}

func TestUniqMissingSkipFieldsArg(t *testing.T) {
	content := "a\nb\n"
	uniqWriteTestFile(t, "test_uniq_missing_fields.txt", content)
	defer os.Remove("test_uniq_missing_fields.txt")

	_, err := runUniqCmd([]string{"-f", "test_uniq_missing_fields.txt"})
	if err == nil {
		t.Fatalf("Expected error for missing -f argument, got none")
	}
}

// ============== MULTIPLE FILES TESTS ==============

func TestUniqMultipleFiles(t *testing.T) {
	content1 := "a\na\nb\n"
	content2 := "b\nc\nc\n"
	uniqWriteTestFile(t, "test_uniq_multi1.txt", content1)
	uniqWriteTestFile(t, "test_uniq_multi2.txt", content2)
	defer os.Remove("test_uniq_multi1.txt")
	defer os.Remove("test_uniq_multi2.txt")

	output, err := runUniqCmd([]string{"test_uniq_multi1.txt", "test_uniq_multi2.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	// File 1: a, b (a appears twice -> one a, b once)
	// File 2: b, c (b appears twice -> one b, c twice -> one c)
	// Combined: a, b, b, c
	if !strings.Contains(result, "a") {
		t.Errorf("Expected 'a' in output, got: %s", result)
	}
	if !strings.Contains(result, "b") {
		t.Errorf("Expected 'b' in output, got: %s", result)
	}
	if !strings.Contains(result, "c") {
		t.Errorf("Expected 'c' in output, got: %s", result)
	}
}

func TestUniqMultipleFilesCount(t *testing.T) {
	content1 := "x\nx\n"
	content2 := "y\ny\ny\n"
	uniqWriteTestFile(t, "test_uniq_multi_count1.txt", content1)
	uniqWriteTestFile(t, "test_uniq_multi_count2.txt", content2)
	defer os.Remove("test_uniq_multi_count1.txt")
	defer os.Remove("test_uniq_multi_count2.txt")

	output, err := runUniqCmd([]string{"-c", "test_uniq_multi_count1.txt", "test_uniq_multi_count2.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	// Note: counts are per file, not global
	if !strings.Contains(result, "2 x") {
		t.Errorf("Expected '2 x' in output, got: %s", result)
	}
	if !strings.Contains(result, "3 y") {
		t.Errorf("Expected '3 y' in output, got: %s", result)
	}
}

func TestUniqMultipleFilesOneMissing(t *testing.T) {
	content := "a\nb\n"
	uniqWriteTestFile(t, "test_uniq_multi_exists.txt", content)
	defer os.Remove("test_uniq_multi_exists.txt")

	_, err := runUniqCmd([]string{"test_uniq_multi_exists.txt", "nonexistent_file.txt"})
	if err == nil {
		t.Fatalf("Expected error when one file is missing, got none")
	}
}

// ============== HELP FLAG TESTS ==============

func TestUniqHelp(t *testing.T) {
	output, err := runUniqCmd([]string{"-h"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information, got: %s", result)
	}
	if !strings.Contains(result, "-c") || !strings.Contains(result, "--count") {
		t.Errorf("Expected -c/--count in help, got: %s", result)
	}
}

func TestUniqHelpLong(t *testing.T) {
	output, err := runUniqCmd([]string{"--help"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information, got: %s", result)
	}
}

// ============== LONG FLAG TESTS ==============

func TestUniqLongFlags(t *testing.T) {
	content := "a\na\nb\nb\nc\n"
	uniqWriteTestFile(t, "test_uniq_long1.txt", content)
	defer os.Remove("test_uniq_long1.txt")

	output, err := runUniqCmd([]string{"--count", "test_uniq_long1.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "2 a") || !strings.Contains(result, "2 b") || !strings.Contains(result, "1 c") {
		t.Errorf("Expected count output, got: %s", result)
	}
}

func TestUniqLongFlagRepeated(t *testing.T) {
	content := "x\nx\ny\nz\nz\n"
	uniqWriteTestFile(t, "test_uniq_long2.txt", content)
	defer os.Remove("test_uniq_long2.txt")

	output, err := runUniqCmd([]string{"--repeated", "test_uniq_long2.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), result)
	}
}

func TestUniqLongFlagUnique(t *testing.T) {
	content := "x\nx\ny\nz\nz\n"
	uniqWriteTestFile(t, "test_uniq_long3.txt", content)
	defer os.Remove("test_uniq_long3.txt")

	output, err := runUniqCmd([]string{"--unique", "test_uniq_long3.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	if result != "y" {
		t.Errorf("Expected 'y', got: %s", result)
	}
}

func TestUniqLongFlagIgnoreCase(t *testing.T) {
	content := "AAA\naaa\nBBB\n"
	uniqWriteTestFile(t, "test_uniq_long4.txt", content)
	defer os.Remove("test_uniq_long4.txt")

	output, err := runUniqCmd([]string{"--ignore-case", "test_uniq_long4.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines (AAA/aaa merged, BBB), got %d: %s", len(lines), result)
	}
}

func TestUniqLongFlagCheckChars(t *testing.T) {
	content := "abcdef\nghijkl\n"
	uniqWriteTestFile(t, "test_uniq_long5.txt", content)
	defer os.Remove("test_uniq_long5.txt")

	output, err := runUniqCmd([]string{"--check-chars=3", "test_uniq_long5.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), result)
	}
}

func TestUniqLongFlagSkipFields(t *testing.T) {
	content := "id1 val1\nid1 val2\n"
	uniqWriteTestFile(t, "test_uniq_long6.txt", content)
	defer os.Remove("test_uniq_long6.txt")

	output, err := runUniqCmd([]string{"--skip-fields=1", "test_uniq_long6.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// val1 != val2, so both appear
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines (val1 != val2), got %d: %s", len(lines), result)
	}
}

// ============== SPECIAL CHARACTER TESTS ==============

func TestUniqSpecialChars(t *testing.T) {
	content := "hello world\nhello world\nprice: $100\nprice: $100\ntest\ttab\ntest\ttab\n"
	uniqWriteTestFile(t, "test_uniq_special.txt", content)
	defer os.Remove("test_uniq_special.txt")

	output, err := runUniqCmd([]string{"test_uniq_special.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 unique lines, got %d: %s", len(lines), result)
	}
}

func TestUniqWhitespace(t *testing.T) {
	content := "  leading\n  leading\ntrailing  \ntrailing  \n"
	uniqWriteTestFile(t, "test_uniq_whitespace.txt", content)
	defer os.Remove("test_uniq_whitespace.txt")

	output, err := runUniqCmd([]string{"test_uniq_whitespace.txt"})
	if err != nil {
		t.Fatalf("uniq command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// "  leading" and "trailing  " are different (leading vs trailing whitespace preserved)
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), result)
	}
}

// Helper function to write test files
func uniqWriteTestFile(t *testing.T, filename, content string) {
	err := os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file %s: %v", filename, err)
	}
}
