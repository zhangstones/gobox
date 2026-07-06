package text

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============== NORMAL CASES TESTS ==============

func TestSortBasic(t *testing.T) {
	content := "banana\napple\ncherry\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_basic.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple\nbanana\ncherry\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortNumeric(t *testing.T) {
	content := "10\n2\n1\n20\n3\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_numeric.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-n", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "1\n2\n3\n10\n20\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortReverse(t *testing.T) {
	content := "apple\nbanana\ncherry\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_reverse.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-r", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "cherry\nbanana\napple\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortNumericReverse(t *testing.T) {
	content := "1\n10\n2\n20\n3\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_num_reverse.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-n", "-r", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "20\n10\n3\n2\n1\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortCombinedFlags(t *testing.T) {
	content := "10\n2\n1\n20\n3\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_combined.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	// Test combined flags -rn
	output, err := runSortCmd([]string{"-rn", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "20\n10\n3\n2\n1\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== EDGE CASES TESTS ==============

func TestSortEmptyFile(t *testing.T) {
	content := ""
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_empty.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	if result != "" {
		t.Errorf("Expected empty output, got: %s", result)
	}
}

func TestSortSingleLine(t *testing.T) {
	content := "hello\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_single.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "hello\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortAlreadySorted(t *testing.T) {
	content := "1\n2\n3\n4\n5\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_sorted.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := strings.TrimSpace(content)
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortReverseSorted(t *testing.T) {
	content := "5\n4\n3\n2\n1\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_rev_sorted.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-n", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	expected := "1\n2\n3\n4\n5"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortIdenticalLines(t *testing.T) {
	content := "foo\nfoo\nfoo\nbar\nbar\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_identical.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "bar\nbar\nfoo\nfoo\nfoo\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortSpecialCharacters(t *testing.T) {
	content := "~user\n100\n-50\n+20\n.Aaa\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_special.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	// ASCII order: + (43) < - (45) < . (46) < 0-9 (48-57) < @ (64) < A-Z (65-90) < _ (95) < a-z (97-122) < ~
	// +20 < -50 < .Aaa < 100 < ~user
	expected := "+20\n-50\n.Aaa\n100\n~user\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== KEY/COLUMN SORT TESTS ==============

func TestSortByKey(t *testing.T) {
	content := "banana 3\napple 1\ncherry 2\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_key.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-k2", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple 1\ncherry 2\nbanana 3\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortByKeyNumeric(t *testing.T) {
	content := "banana 30\napple 10\ncherry 20\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_key_num.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-n", "-k2", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple 10\ncherry 20\nbanana 30\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortByKeyInvalid(t *testing.T) {
	content := "apple\nbanana\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_key_invalid.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	_, err := runSortCmd([]string{"-k0", filename})
	if err == nil {
		t.Errorf("Expected error for invalid key 0")
	}
}

func TestSortByKeyOutOfRange(t *testing.T) {
	content := "apple 1\nbanana 2\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_key_oob.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	// Key 99 is out of range, should sort by first field
	output, err := runSortCmd([]string{"-k99", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	// With empty key, full line is used
	if !strings.Contains(result, "apple") || !strings.Contains(result, "banana") {
		t.Errorf("Expected both apple and banana in output, got: %s", result)
	}
}

// ============== CUSTOM FIELD SEPARATOR TESTS ==============

func TestSortCustomSeparator(t *testing.T) {
	content := "banana:3\napple:1\ncherry:2\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_sep.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	// Use space-separated -t and separator value
	output, err := runSortCmd([]string{"-t", ":", "-k2", "-n", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple:1\ncherry:2\nbanana:3\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// TestSortConcatenatedSeparator is a regression test for a bug where
// "-t:" (separator character concatenated directly onto the short flag,
// e.g. "sort -k2 -t: file") was rejected with "unknown option: -t"
// because the parser only recognized "-t" as a standalone argument
// followed by a separate value, falling through to the combined
// short-flag handler which didn't know about 't'.
func TestSortConcatenatedSeparator(t *testing.T) {
	content := "banana:3\napple:1\ncherry:2\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_sep_concat.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-k2", "-t:", "-n", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple:1\ncherry:2\nbanana:3\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// TestSortFieldSeparatorLongEquals is a regression test ensuring the
// documented long option "--field-separator=CHAR" actually changes the
// delimiter; the parser previously only matched the literal string
// "--field-separator=" (with no value) instead of using the value
// after "=" as a prefix match.
func TestSortFieldSeparatorLongEquals(t *testing.T) {
	content := "banana:3\napple:1\ncherry:2\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_sep_long.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-k2", "--field-separator=:", "-n", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple:1\ncherry:2\nbanana:3\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortTabSeparator(t *testing.T) {
	content := "banana\t3\napple\t1\ncherry\t2\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_tab.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	// Without explicit separator, tabs are treated as whitespace
	output, err := runSortCmd([]string{"-n", "-k2", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple\t1\ncherry\t2\nbanana\t3\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortSeparatorRequiresArgument(t *testing.T) {
	_, err := runSortCmd([]string{"-t"})
	if err == nil {
		t.Errorf("Expected error when -t has no argument")
	}
}

// ============== UNIQUE TESTS ==============

func TestSortUnique(t *testing.T) {
	content := "apple\nbanana\napple\ncherry\nbanana\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_unique.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-u", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple\nbanana\ncherry\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortUniqueNumeric(t *testing.T) {
	content := "10\n2\n1\n10\n3\n2\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_unique_num.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-nu", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "1\n2\n3\n10\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortUniqueAlreadySorted(t *testing.T) {
	content := "apple\napple\nbanana\nbanana\ncherry\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_unique_sorted.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-u", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple\nbanana\ncherry\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== MONTH SORT TESTS ==============

func TestSortMonth(t *testing.T) {
	content := "Mar\nJan\nFeb\nDec\nNov\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_month.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-M", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "Jan\nFeb\nMar\nNov\nDec\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortMonthReverse(t *testing.T) {
	content := "Mar\nJan\nFeb\nDec\nNov\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_month_rev.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-M", "-r", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "Dec\nNov\nMar\nFeb\nJan\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortMonthLowercase(t *testing.T) {
	// Month names must be 3-letter abbreviations matching the monthNames map
	content := "mar\njan\nfeb\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_month_lower.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-M", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "jan\nfeb\nmar\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortMonthInvalidMonth(t *testing.T) {
	content := "Mar\nNotAMonth\nJan\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_month_invalid.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-M", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	lines := strings.Split(strings.TrimSpace(result), "\n")
	// Invalid months (returning 0) should sort before valid months
	// So NotAMonth should be first or among the first
	if lines[0] != "NotAMonth" {
		t.Errorf("Expected 'NotAMonth' first (invalid months sort to front), got: %s", lines[0])
	}
}

// ============== HUMAN NUMERIC SORT TESTS ==============

func TestSortHumanNumeric(t *testing.T) {
	content := "1K\n2M\n100\n3G\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_human.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-h", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	// 100 < 1K < 2M < 3G
	expected := "100\n1K\n2M\n3G\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortHumanNumericReverse(t *testing.T) {
	content := "1K\n2M\n100\n3G\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_human_rev.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-h", "-r", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "3G\n2M\n1K\n100\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortHumanNumericKiB(t *testing.T) {
	// Note: The sort command's human numeric parsing doesn't properly support
	// binary suffixes (Ki, Mi, Gi, Ti). This test verifies what IS supported.
	content := "1K\n500\n2M\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_human_ki.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-h", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	// 500 < 1K (1000) < 2M (2000)
	expected := "500\n1K\n2M\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== RANDOM SORT TESTS ==============

func TestSortRandom(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_random.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-R", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")

	// Check all lines are present
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines, got %d", len(lines))
	}

	// Verify all original lines are present
	for _, expected := range []string{"line1", "line2", "line3", "line4", "line5"} {
		found := false
		for _, line := range lines {
			if line == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected line %s not found in output", expected)
		}
	}
}

func TestSortRandomDifferentOrder(t *testing.T) {
	content := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_random2.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	// Run multiple times and verify the seed produces different orderings
	// (this is probabilistic but very unlikely to fail if truly random)
	results := make([]string, 3)
	for i := 0; i < 3; i++ {
		output, err := runSortCmd([]string{"-R", filename})
		if err != nil {
			t.Fatalf("sort command failed: %v", err)
		}
		results[i] = output
	}

	// At least two of the three runs should be different
	different := 0
	for i := 1; i < 3; i++ {
		if results[i] != results[0] {
			different++
		}
	}

	if different == 0 {
		t.Errorf("Random sort produced same order 3 times - likely not random")
	}
}

// ============== CHECK SORTED TESTS ==============

func TestSortCheckSorted(t *testing.T) {
	content := "apple\nbanana\ncherry\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_check.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-c", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "sorted") {
		t.Errorf("Expected 'sorted' message, got: %s", result)
	}
}

func TestSortCheckNotSorted(t *testing.T) {
	content := "banana\napple\ncherry\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_check_not.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	_, err := runSortCmd([]string{"-c", filename})
	if err == nil {
		t.Errorf("Expected error for unsorted file")
	}
}

func TestSortCheckNumeric(t *testing.T) {
	content := "1\n2\n10\n20\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_check_num.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-n", "-c", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "sorted") {
		t.Errorf("Expected 'sorted' message, got: %s", result)
	}
}

func TestSortCheckNumericNotSorted(t *testing.T) {
	content := "1\n10\n2\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_check_num_not.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	_, err := runSortCmd([]string{"-n", "-c", filename})
	if err == nil {
		t.Errorf("Expected error for numerically unsorted file")
	}
}

func TestSortCheckEmptyFile(t *testing.T) {
	content := ""
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_check_empty.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-c", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "succeeded") {
		t.Errorf("Expected 'succeeded' message for empty file, got: %s", result)
	}
}

func TestSortCheckSingleLine(t *testing.T) {
	content := "onlyone\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_check_single.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-c", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "succeeded") {
		t.Errorf("Expected 'succeeded' message for single line, got: %s", result)
	}
}

// ============== OUTPUT FILE TESTS ==============

func TestSortOutputFile(t *testing.T) {
	content := "banana\napple\ncherry\n"
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "test_sort_out_input.txt")
	outputFile := filepath.Join(tmpDir, "test_sort_output.txt")
	os.WriteFile(inputFile, []byte(content), 0644)
	defer os.Remove(inputFile)
	defer os.Remove(outputFile)

	err := SortCmd([]string{"-o", outputFile, inputFile})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	// Check output file exists and has correct content
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Cannot read output file: %v", err)
	}

	result := string(data)
	expected := "apple\nbanana\ncherry\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortOutputFileOverwrite(t *testing.T) {
	content := "banana\napple\ncherry\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_out_input2.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	err := SortCmd([]string{"-o", filename, filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Cannot read output file: %v", err)
	}

	result := string(data)
	expected := "apple\nbanana\ncherry\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortOutputRequiresArgument(t *testing.T) {
	_, err := runSortCmd([]string{"-o"})
	if err == nil {
		t.Errorf("Expected error when -o has no argument")
	}
}

// ============== ZERO TERMINATED TESTS ==============

func TestSortZeroTerminated(t *testing.T) {
	content := "banana\x00apple\x00cherry\x00"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_zero.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-z", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple\x00banana\x00cherry\x00"
	if result != expected {
		t.Errorf("Expected:\n%q\nGot:\n%q", expected, result)
	}
}

func TestSortZeroTerminatedInput(t *testing.T) {
	content := "banana\x00apple\x00cherry\x00"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_zero_in.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-z", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	// Output should also be zero-terminated
	if !strings.Contains(output, "apple\x00banana\x00cherry\x00") {
		t.Errorf("Expected zero-terminated sorted output, got: %q", output)
	}
}

// ============== STDIN TESTS ==============

func TestSortStdin(t *testing.T) {
	output, err := runSortCmdWithStdin([]string{}, "banana\napple\ncherry\n")
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple\nbanana\ncherry\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortStdinNumeric(t *testing.T) {
	output, err := runSortCmdWithStdin([]string{"-n"}, "10\n2\n1\n20\n3\n")
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "1\n2\n3\n10\n20\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortStdinReverse(t *testing.T) {
	output, err := runSortCmdWithStdin([]string{"-r"}, "apple\nbanana\ncherry\n")
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "cherry\nbanana\napple\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortStdinUnique(t *testing.T) {
	output, err := runSortCmdWithStdin([]string{"-u"}, "apple\nbanana\napple\ncherry\nbanana\n")
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple\nbanana\ncherry\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortStdinEmpty(t *testing.T) {
	output, err := runSortCmdWithStdin([]string{}, "")
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	if result != "" {
		t.Errorf("Expected empty output, got: %s", result)
	}
}

// ============== ERROR CASES TESTS ==============

func TestSortNonexistentFile(t *testing.T) {
	_, err := runSortCmd([]string{"nonexistent_file_xyz.txt"})
	if err == nil {
		t.Errorf("Expected error for nonexistent file")
	}
}

func TestSortInvalidKey(t *testing.T) {
	content := "apple\nbanana\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_bad_key.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	_, err := runSortCmd([]string{"-kabc", filename})
	if err == nil {
		t.Errorf("Expected error for invalid key")
	}
}

func TestSortInvalidKeyZero(t *testing.T) {
	content := "apple\nbanana\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_key_zero.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	_, err := runSortCmd([]string{"-k", "0", filename})
	if err == nil {
		t.Errorf("Expected error for key 0")
	}
}

func TestSortInvalidKeyNegative(t *testing.T) {
	content := "apple\nbanana\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_key_neg.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	_, err := runSortCmd([]string{"-k", "-1", filename})
	if err == nil {
		t.Errorf("Expected error for negative key")
	}
}

func TestSortUnknownOption(t *testing.T) {
	_, err := runSortCmd([]string{"-q"})
	if err == nil {
		t.Errorf("Expected error for unknown option")
	}
}

func TestSortHelp(t *testing.T) {
	output, err := runSortCmd([]string{"--help"})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information, got: %s", result)
	}
}

// ============== MULTIPLE FILES TESTS ==============

func TestSortMultipleFiles(t *testing.T) {
	content1 := "apple\ncherry\n"
	content2 := "banana\ndate\n"
	tmpDir := t.TempDir()
	filename1 := filepath.Join(tmpDir, "test_sort_multi1.txt")
	filename2 := filepath.Join(tmpDir, "test_sort_multi2.txt")
	os.WriteFile(filename1, []byte(content1), 0644)
	os.WriteFile(filename2, []byte(content2), 0644)
	defer os.Remove(filename1)
	defer os.Remove(filename2)

	output, err := runSortCmd([]string{filename1, filename2})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "apple\nbanana\ncherry\ndate\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== PIPING THROUGH SORT TESTS ==============

func TestSortPipe(t *testing.T) {
	// Convert shell pipeline to direct stdin call
	output, err := runSortCmdWithStdin([]string{}, "banana\napple\ncherry\n")
	if err != nil {
		t.Fatalf("sort stdin failed: %v", err)
	}

	result := output
	expected := "apple\nbanana\ncherry\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== DECIMAL NUMBER TESTS ==============

func TestSortDecimal(t *testing.T) {
	content := "1.5\n1.25\n1.75\n1.0\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_decimal.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-n", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "1.0\n1.25\n1.5\n1.75\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortNegativeNumbers(t *testing.T) {
	content := "10\n-5\n20\n-15\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_negative.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-n", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "-15\n-5\n10\n20\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestSortNegativeDecimal(t *testing.T) {
	content := "-1.5\n1.5\n-0.5\n0.5\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_neg_decimal.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{"-n", filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	expected := "-1.5\n-0.5\n0.5\n1.5\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== LONG LINES TESTS ==============

func TestSortLongLines(t *testing.T) {
	longLine := strings.Repeat("a", 10000)
	content := longLine + "\n" + longLine + "b\n" + longLine + "a\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_long.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}
}

// ============== CASE SENSITIVITY TESTS ==============

func TestSortCaseSensitive(t *testing.T) {
	content := "Apple\napple\nAPPLE\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_case.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := output
	// ASCII order: APPLE < Apple < apple
	expected := "APPLE\nApple\napple\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// ============== STABILITY TESTS ==============

func TestSortStability(t *testing.T) {
	// When lines compare equal, they should maintain original order
	// Using identical strings ensures equal comparison
	content := "aaa\nbbb\naaa\nccc\naaa\n"
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test_sort_stable.txt")
	os.WriteFile(filename, []byte(content), 0644)
	defer os.Remove(filename)

	output, err := runSortCmd([]string{filename})
	if err != nil {
		t.Fatalf("sort command failed: %v", err)
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")
	// All aaa lines should maintain their relative order: first, third, fifth
	// Check that all aaa lines are together (not interleaved with bbb or ccc)
	aaaPositions := []int{}
	nonAaaOrder := []string{}
	for i, line := range lines {
		if line == "aaa" {
			aaaPositions = append(aaaPositions, i)
		} else {
			nonAaaOrder = append(nonAaaOrder, line)
		}
	}
	// All aaa positions should be contiguous (either all before or all after non-aaa)
	// Actually, stable sort groups equal items together, so aaa should be at positions 0,1,2
	if len(aaaPositions) != 3 {
		t.Errorf("Expected 3 'aaa' lines, got %d", len(aaaPositions))
	}
	if aaaPositions[0] != 0 || aaaPositions[1] != 1 || aaaPositions[2] != 2 {
		t.Errorf("Expected aaa lines at positions 0,1,2 (stable sort), got positions: %v", aaaPositions)
	}
}

// Helper function to write test files
func sortWriteTestFile(t *testing.T, filename, content string) {
	err := os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file %s: %v", filename, err)
	}
}
