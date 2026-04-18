package text

import (
	"os"
	"strings"
	"testing"
)

// ============== SUBSTITUTION TESTS ==============

func TestSedBasicSubstitute(t *testing.T) {
	content := "hello world\nfoo bar\nhello again\n"
	writeTestFile(t, "test_sed_basic.txt", content)
	defer os.Remove("test_sed_basic.txt")

	output, err := runSedCmd([]string{"s/hello/hi/", "test_sed_basic.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "hi world") {
		t.Errorf("Expected 'hi world' in output, got: %s", result)
	}
	if !strings.Contains(result, "hi again") {
		t.Errorf("Expected 'hi again' in output, got: %s", result)
	}
}

func TestSedGlobalReplace(t *testing.T) {
	content := "foo foo foo\nbar baz\nfoo\n"
	writeTestFile(t, "test_sed_global.txt", content)
	defer os.Remove("test_sed_global.txt")

	output, err := runSedCmd([]string{"s/foo/X/g", "test_sed_global.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "X X X") {
		t.Errorf("Expected 'X X X' in output, got: %s", result)
	}
	if strings.Contains(result, "foo") {
		t.Errorf("Unexpected 'foo' in output: %s", result)
	}
}

func TestSedIgnoreCase(t *testing.T) {
	content := "HELLO world\nHello Again\nhello\n"
	writeTestFile(t, "test_sed_case.txt", content)
	defer os.Remove("test_sed_case.txt")

	output, err := runSedCmd([]string{"s/hello/hi/i", "test_sed_case.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "hi world") {
		t.Errorf("Expected 'hi world' in output, got: %s", result)
	}
	if !strings.Contains(result, "hi Again") {
		t.Errorf("Expected 'hi Again' in output, got: %s", result)
	}
}

func TestSedQuietMode(t *testing.T) {
	content := "hello world\nfoo bar\nhello again\n"
	writeTestFile(t, "test_sed_quiet.txt", content)
	defer os.Remove("test_sed_quiet.txt")

	output, err := runSedCmd([]string{"-n", "s/hello/hi/p", "test_sed_quiet.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %s", len(lines), result)
	}
	if !strings.Contains(result, "hi world") {
		t.Errorf("Expected 'hi world' in output, got: %s", result)
	}
}

func TestSedNthReplacement(t *testing.T) {
	content := "foo foo foo\nbar foo baz\n"
	writeTestFile(t, "test_sed_nth.txt", content)
	defer os.Remove("test_sed_nth.txt")

	output, err := runSedCmd([]string{"s/foo/X/2", "test_sed_nth.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}
	// First line: foo X foo (2nd occurrence replaced)
	if !strings.Contains(lines[0], "foo X foo") {
		t.Errorf("Expected 'foo X foo', got: %s", lines[0])
	}
	// Second line: bar foo baz (only one foo, so 2nd doesn't exist)
	if !strings.Contains(lines[1], "bar foo baz") {
		t.Errorf("Expected 'bar foo baz', got: %s", lines[1])
	}
}

func TestSedBackreference(t *testing.T) {
	content := "John Doe\nJane Smith\n"
	writeTestFile(t, "test_sed_backref.txt", content)
	defer os.Remove("test_sed_backref.txt")

	output, err := runSedCmd([]string{"s/([A-Za-z]+) ([A-Za-z]+)/${2}, ${1}/", "test_sed_backref.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Doe, John") {
		t.Errorf("Expected 'Doe, John' in output, got: %s", result)
	}
	if !strings.Contains(result, "Smith, Jane") {
		t.Errorf("Expected 'Smith, Jane' in output, got: %s", result)
	}
}

func TestSedBackreferenceBackslash(t *testing.T) {
	content := "John Doe\nJane Smith\n"
	writeTestFile(t, "test_sed_backref2.txt", content)
	defer os.Remove("test_sed_backref2.txt")

	// Test \1 \2 syntax (converted to ${1} ${2} internally)
	output, err := runSedCmd([]string{`s/\([A-Za-z]+\) \([A-Za-z]+\)/\2, \1/`, "test_sed_backref2.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "Doe, John") {
		t.Errorf("Expected 'Doe, John' in output, got: %s", result)
	}
}

func TestSedMultipleSubstitutions(t *testing.T) {
	content := "foo bar baz\n"
	writeTestFile(t, "test_sed_multisub.txt", content)
	defer os.Remove("test_sed_multisub.txt")

	output, err := runSedCmd([]string{"-e", "s/foo/FOO/", "-e", "s/bar/BAR/", "-e", "s/baz/BAZ/", "test_sed_multisub.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "FOO BAR BAZ") {
		t.Errorf("Expected 'FOO BAR BAZ' in output, got: %s", result)
	}
}

func TestSedRegexPatterns(t *testing.T) {
	content := "test123\nabc456\ntest789\n"
	writeTestFile(t, "test_sed_regex.txt", content)
	defer os.Remove("test_sed_regex.txt")

	// Replace digits with X
	output, err := runSedCmd([]string{"s/[0-9]/X/g", "test_sed_regex.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "testXXX") {
		t.Errorf("Expected 'testXXX' in output, got: %s", result)
	}
	if !strings.Contains(result, "abcXXX") {
		t.Errorf("Expected 'abcXXX' in output, got: %s", result)
	}
}

func TestSedAnchorPatterns(t *testing.T) {
	content := "hello world\nworld hello\nhello\n"
	writeTestFile(t, "test_sed_anchor.txt", content)
	defer os.Remove("test_sed_anchor.txt")

	// Replace hello only at start of line
	output, err := runSedCmd([]string{"s/^hello/HELLO/", "test_sed_anchor.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "HELLO world") {
		t.Errorf("Expected 'HELLO world' in output, got: %s", result)
	}
	if !strings.Contains(result, "HELLO\n") {
		t.Errorf("Expected 'HELLO' (standalone) in output, got: %s", result)
	}
	// "world hello" should not be changed
	if !strings.Contains(result, "world hello") {
		t.Errorf("Expected 'world hello' unchanged, got: %s", result)
	}
}

// ============== DELETE TESTS ==============

func TestSedDelete(t *testing.T) {
	content := "line1\nDELETE_ME\nline3\nDELETE_ME\nline5\n"
	writeTestFile(t, "test_sed_delete.txt", content)
	defer os.Remove("test_sed_delete.txt")

	output, err := runSedCmd([]string{"/DELETE_ME/d", "test_sed_delete.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if strings.Contains(result, "DELETE_ME") {
		t.Errorf("Unexpected 'DELETE_ME' in output: %s", result)
	}
}

func TestSedDeleteFirstLine(t *testing.T) {
	content := "first\nsecond\nthird\n"
	writeTestFile(t, "test_sed_del_first.txt", content)
	defer os.Remove("test_sed_del_first.txt")

	output, err := runSedCmd([]string{"1d", "test_sed_del_first.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if strings.Contains(result, "first") {
		t.Errorf("First line should be deleted, got: %s", result)
	}
	if !strings.Contains(result, "second") {
		t.Errorf("Expected 'second' in output, got: %s", result)
	}
}

func TestSedDeleteLastLine(t *testing.T) {
	content := "first\nsecond\nlast\n"
	writeTestFile(t, "test_sed_del_last.txt", content)
	defer os.Remove("test_sed_del_last.txt")

	output, err := runSedCmd([]string{"$d", "test_sed_del_last.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if strings.Contains(result, "last") {
		t.Errorf("Last line should be deleted, got: %s", result)
	}
}

func TestSedDeleteRange(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\n"
	writeTestFile(t, "test_sed_del_range.txt", content)
	defer os.Remove("test_sed_del_range.txt")

	output, err := runSedCmd([]string{"2,4d", "test_sed_del_range.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if strings.Contains(result, "line2") || strings.Contains(result, "line3") || strings.Contains(result, "line4") {
		t.Errorf("Lines 2-4 should be deleted, got: %s", result)
	}
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line5") {
		t.Errorf("Expected line1 and line5, got: %s", result)
	}
}

// ============== PRINT TESTS ==============

func TestSedPrint(t *testing.T) {
	content := "line1\nMATCH\nline3\nMATCH\nline5\n"
	writeTestFile(t, "test_sed_print.txt", content)
	defer os.Remove("test_sed_print.txt")

	output, err := runSedCmd([]string{"-n", "/MATCH/p", "test_sed_print.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "MATCH") {
		t.Errorf("Expected 'MATCH' in output, got: %s", result)
	}
	if strings.Contains(result, "line1") {
		t.Errorf("Unexpected 'line1' in output: %s", result)
	}
}

func TestSedPrintAll(t *testing.T) {
	content := "line1\nline2\nline3\n"
	writeTestFile(t, "test_sed_print_all.txt", content)
	defer os.Remove("test_sed_print_all.txt")

	output, err := runSedCmd([]string{"-n", "p", "test_sed_print_all.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line2") || !strings.Contains(result, "line3") {
		t.Errorf("Expected all lines, got: %s", result)
	}
}

// ============== LINE NUMBER TESTS ==============

func TestSedPrintLineNumber(t *testing.T) {
	content := "line1\nline2\nline3\n"
	writeTestFile(t, "test_sed_linenum.txt", content)
	defer os.Remove("test_sed_linenum.txt")

	output, err := runSedCmd([]string{"=", "test_sed_linenum.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "1") {
		t.Errorf("Expected line number 1 in output, got: %s", result)
	}
	if !strings.Contains(result, "2") {
		t.Errorf("Expected line number 2 in output, got: %s", result)
	}
	if !strings.Contains(result, "3") {
		t.Errorf("Expected line number 3 in output, got: %s", result)
	}
}

func TestSedLineNumberWithPattern(t *testing.T) {
	content := "foo\nbar\nbaz\n"
	writeTestFile(t, "test_sed_linenum_pat.txt", content)
	defer os.Remove("test_sed_linenum_pat.txt")

	// Print line number for matching lines
	output, err := runSedCmd([]string{"-n", "/bar/=", "test_sed_linenum_pat.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "2" {
		t.Errorf("Expected line number 2, got: %s", result)
	}
}

// ============== INSERT TESTS ==============

func TestSedInsert(t *testing.T) {
	content := "line1\nline2\nline3\n"
	writeTestFile(t, "test_sed_insert.txt", content)
	defer os.Remove("test_sed_insert.txt")

	output, err := runSedCmd([]string{"/line2/i\\INSERTED", "test_sed_insert.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "INSERTED") {
		t.Errorf("Expected 'INSERTED' in output, got: %s", result)
	}
	// INSERTED should come before line2
	idxInserted := strings.Index(result, "INSERTED")
	idxLine2 := strings.Index(result, "line2")
	if idxInserted >= idxLine2 {
		t.Errorf("INSERTED should come before line2, got: %s", result)
	}
}

func TestSedInsertNumeric(t *testing.T) {
	content := "line1\nline2\nline3\n"
	writeTestFile(t, "test_sed_insert_num.txt", content)
	defer os.Remove("test_sed_insert_num.txt")

	output, err := runSedCmd([]string{"2i\\BEFORE_LINE_2", "test_sed_insert_num.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "BEFORE_LINE_2") {
		t.Errorf("Expected 'BEFORE_LINE_2' in output, got: %s", result)
	}
}

func TestSedInsertFirstLine(t *testing.T) {
	content := "line1\nline2\n"
	writeTestFile(t, "test_sed_insert_first.txt", content)
	defer os.Remove("test_sed_insert_first.txt")

	output, err := runSedCmd([]string{"1i\\FIRST", "test_sed_insert_first.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if lines[0] != "FIRST" {
		t.Errorf("Expected FIRST as first line, got: %s", lines[0])
	}
}

// ============== APPEND TESTS ==============

func TestSedAppend(t *testing.T) {
	content := "line1\nline2\nline3\n"
	writeTestFile(t, "test_sed_append.txt", content)
	defer os.Remove("test_sed_append.txt")

	output, err := runSedCmd([]string{"/line2/a\\APPENDED", "test_sed_append.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "APPENDED") {
		t.Errorf("Expected 'APPENDED' in output, got: %s", result)
	}
	// APPENDED should come after line2
	idxLine2 := strings.Index(result, "line2")
	idxAppended := strings.Index(result, "APPENDED")
	if idxAppended <= idxLine2 {
		t.Errorf("APPENDED should come after line2, got: %s", result)
	}
}

func TestSedAppendNumeric(t *testing.T) {
	content := "line1\nline2\nline3\n"
	writeTestFile(t, "test_sed_append_num.txt", content)
	defer os.Remove("test_sed_append_num.txt")

	output, err := runSedCmd([]string{"2a\\AFTER_LINE_2", "test_sed_append_num.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "AFTER_LINE_2") {
		t.Errorf("Expected 'AFTER_LINE_2' in output, got: %s", result)
	}
}

func TestSedAppendLastLine(t *testing.T) {
	content := "line1\nline2\n"
	writeTestFile(t, "test_sed_append_last.txt", content)
	defer os.Remove("test_sed_append_last.txt")

	output, err := runSedCmd([]string{"2a\\LAST", "test_sed_append_last.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if lines[len(lines)-1] != "LAST" {
		t.Errorf("Expected LAST as last line, got: %s", lines[len(lines)-1])
	}
}

// ============== CHANGE TESTS ==============

func TestSedChange(t *testing.T) {
	content := "line1\nline2\nline3\n"
	writeTestFile(t, "test_sed_change.txt", content)
	defer os.Remove("test_sed_change.txt")

	output, err := runSedCmd([]string{"/line2/c\\CHANGED", "test_sed_change.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "CHANGED") {
		t.Errorf("Expected 'CHANGED' in output, got: %s", result)
	}
	if strings.Contains(result, "line2") {
		t.Errorf("line2 should be replaced, got: %s", result)
	}
}

func TestSedChangeNumeric(t *testing.T) {
	content := "line1\nline2\nline3\n"
	writeTestFile(t, "test_sed_change_num.txt", content)
	defer os.Remove("test_sed_change_num.txt")

	output, err := runSedCmd([]string{"2c\\REPLACED_LINE_2", "test_sed_change_num.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "REPLACED_LINE_2") {
		t.Errorf("Expected 'REPLACED_LINE_2' in output, got: %s", result)
	}
	if strings.Contains(result, "line2") {
		t.Errorf("line2 should be replaced, got: %s", result)
	}
}

func TestSedChangeFirstLine(t *testing.T) {
	content := "ORIGINAL\nline2\n"
	writeTestFile(t, "test_sed_change_first.txt", content)
	defer os.Remove("test_sed_change_first.txt")

	output, err := runSedCmd([]string{"1c\\NEW_FIRST", "test_sed_change_first.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if lines[0] != "NEW_FIRST" {
		t.Errorf("Expected NEW_FIRST as first line, got: %s", lines[0])
	}
}

// ============== IN-PLACE EDITING TESTS ==============

func TestSedInPlace(t *testing.T) {
	content := "old value\nkeep this\nold again\n"
	filename := "test_sed_inplace.txt"
	writeTestFile(t, filename, content)
	defer os.Remove(filename)
	defer os.Remove(filename + ".bak")

	err := SedCmd([]string{"-i.bak", "s/old/new/", filename})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	// Read modified file
	modified, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("cannot read modified file: %v", err)
	}

	result := string(modified)
	if !strings.Contains(result, "new value") {
		t.Errorf("Expected 'new value' in modified file, got: %s", result)
	}

	// Check backup exists
	backup, err := os.ReadFile(filename + ".bak")
	if err != nil {
		t.Fatalf("backup file not created: %v", err)
	}

	if !strings.Contains(string(backup), "old value") {
		t.Errorf("Backup should contain original content")
	}
}

func TestSedInPlaceNoBackup(t *testing.T) {
	content := "old value\nkeep this\n"
	filename := "test_sed_inplace_nobak.txt"
	writeTestFile(t, filename, content)
	defer os.Remove(filename)

	err := SedCmd([]string{"-i", "s/old/new/", filename})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	modified, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("cannot read modified file: %v", err)
	}

	if !strings.Contains(string(modified), "new value") {
		t.Errorf("Expected 'new value' in modified file")
	}

	// Check no backup was created
	if _, err := os.Stat(filename + ".bak"); err == nil {
		t.Errorf("Backup file should not exist without suffix")
	}
}

// ============== STDIN TESTS ==============

func TestSedStdin(t *testing.T) {
	output, err := runSedCmdWithStdin([]string{"s/foo/bar/"}, "hello foo world\nfoo again\n")
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "hello bar world") {
		t.Errorf("Expected 'hello bar world' in output, got: %s", result)
	}
}

func TestSedStdinMultiple(t *testing.T) {
	output, err := runSedCmdWithStdin([]string{"-e", "s/foo/FOO/", "-e", "s/bar/BAR/"}, "foo bar baz\n")
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := output
	if !strings.Contains(result, "FOO BAR baz") {
		t.Errorf("Expected 'FOO BAR baz' in output, got: %s", result)
	}
}

// ============== SCRIPT FILE TESTS ==============

func TestSedScriptFile(t *testing.T) {
	content := "foo bar\nbaz qux\n"
	writeTestFile(t, "test_sed_script_input.txt", content)
	defer os.Remove("test_sed_script_input.txt")

	scriptContent := "s/foo/FOO/\ns/bar/BAR/"
	writeTestFile(t, "test_sed_script.sed", scriptContent)
	defer os.Remove("test_sed_script.sed")

	output, err := runSedCmd([]string{"-f", "test_sed_script.sed", "test_sed_script_input.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "FOO BAR") {
		t.Errorf("Expected 'FOO BAR' in output, got: %s", result)
	}
}

// ============== COMBINED OPERATIONS TESTS ==============

func TestSedInsertAndAppend(t *testing.T) {
	content := "line1\nline2\nline3\n"
	writeTestFile(t, "test_sed_ins_app.txt", content)
	defer os.Remove("test_sed_ins_app.txt")

	output, err := runSedCmd([]string{"-e", "/line2/i\\BEFORE", "-e", "/line2/a\\AFTER", "test_sed_ins_app.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "BEFORE") {
		t.Errorf("Expected 'BEFORE' in output")
	}
	if !strings.Contains(result, "AFTER") {
		t.Errorf("Expected 'AFTER' in output")
	}
}

func TestSedDeleteAndSubstitute(t *testing.T) {
	content := "DELETE\nkeep this\nDELETE\nmodify me\n"
	writeTestFile(t, "test_sed_del_sub.txt", content)
	defer os.Remove("test_sed_del_sub.txt")

	output, err := runSedCmd([]string{"-e", "/DELETE/d", "-e", "s/modify/CHANGED/", "test_sed_del_sub.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if strings.Contains(result, "DELETE") {
		t.Errorf("DELETE lines should be removed")
	}
	if !strings.Contains(result, "CHANGED me") {
		t.Errorf("Expected 'CHANGED me' in output, got: %s", result)
	}
}

// ============== EDGE CASES ==============

func TestSedEmptyPattern(t *testing.T) {
	content := "foo\nbar\nfoo\n"
	writeTestFile(t, "test_sed_empty.txt", content)
	defer os.Remove("test_sed_empty.txt")

	// Empty pattern should match every line
	_, err := runSedCmd([]string{"s//X/", "test_sed_empty.txt"})
	// This should error or handle gracefully
	if err != nil {
		// Expected - empty pattern not supported
		t.Logf("Empty pattern correctly rejected: %v", err)
	}
}

func TestSedNoMatch(t *testing.T) {
	content := "foo bar\nbaz qux\n"
	writeTestFile(t, "test_sed_nomatch.txt", content)
	defer os.Remove("test_sed_nomatch.txt")

	output, err := runSedCmd([]string{"s/xyz/ABC/", "test_sed_nomatch.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	// Should output original content unchanged
	if !strings.Contains(result, "foo bar") {
		t.Errorf("Expected original content, got: %s", result)
	}
}

func TestSedSpecialChars(t *testing.T) {
	content := "price: 100 USD\ndiscount: 50%\n"
	writeTestFile(t, "test_sed_special.txt", content)
	defer os.Remove("test_sed_special.txt")

	// Test with special chars in pattern
	output, err := runSedCmd([]string{"s/50%/75%/", "test_sed_special.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "75%") {
		t.Errorf("Expected 75 percent in output, got: %s", result)
	}
}

func TestSedDotPattern(t *testing.T) {
	content := "test.txt\ntestXtxt\n"
	writeTestFile(t, "test_sed_dot.txt", content)
	defer os.Remove("test_sed_dot.txt")

	// . matches any char in regex
	output, err := runSedCmd([]string{"s/test\\.txt/match/", "test_sed_dot.txt"})
	if err != nil {
		t.Fatalf("sed command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "match") {
		t.Errorf("Expected 'match' in output, got: %s", result)
	}
	if !strings.Contains(result, "testXtxt") {
		t.Errorf("Expected 'testXtxt' unchanged, got: %s", result)
	}
}
