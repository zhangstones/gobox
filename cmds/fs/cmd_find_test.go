package fs

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runFindCmd(t *testing.T, args []string) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	defer rOut.Close()

	os.Stdout = wOut
	runErr := FindCmd(args)
	_ = wOut.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, rOut)
	return buf.String(), runErr
}

func TestFindCmdHelpUsesGroupedSections(t *testing.T) {
	_, out, err := captureFsCmdFull(t, func() error {
		return FindCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("find --help failed: %v", err)
	}
	for _, want := range []string{"Usage: gobox find [OPTION]... [PATH...]", "Filters:", "Traversal:", "-name PATTERN", "-maxdepth N"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
}

func TestPathDepth(t *testing.T) {
	cases := map[string]int{
		"":                         0,
		".":                        0,
		string(filepath.Separator): 0,
		"a":                        1,
		"a/b":                      2,
		"a/b/":                     2,
		"a/b/c":                    3,
	}
	for input, want := range cases {
		if got := pathDepth(input); got != want {
			t.Fatalf("pathDepth(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParseSize(t *testing.T) {
	if _, _, err := parseSize(""); err == nil {
		t.Fatalf("expected error for empty size spec")
	}
	size, op, err := parseSize("+10K")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 10*1024 || op != 1 {
		t.Fatalf("unexpected parseSize result: size=%d op=%d", size, op)
	}
}

func TestMatchSize(t *testing.T) {
	if matchSize(5, "bogus") {
		t.Fatalf("expected bogus size spec to return false")
	}
	if !matchSize(12, "+10") {
		t.Fatalf("expected size to match +10")
	}
	if matchSize(9, "+10") {
		t.Fatalf("expected size 9 to not match +10")
	}
	if !matchSize(9, "-10") {
		t.Fatalf("expected size 9 to match -10")
	}
	if !matchSize(10, "10") {
		t.Fatalf("expected size 10 to match 10")
	}
}

func TestParseTime(t *testing.T) {
	if _, _, err := parseTime(""); err == nil {
		t.Fatalf("expected error for empty time spec")
	}
	dur, op, err := parseTime("-2h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op != -1 || dur != 2*time.Hour {
		t.Fatalf("unexpected parseTime result: op=%d dur=%v", op, dur)
	}
}

func TestMatchTimeMTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if matchTime(info, "1d", "mtime") {
		t.Fatalf("expected fresh mtime to not match exactly 1 day old")
	}
	if matchTime(info, "bogus", "mtime") {
		t.Fatalf("expected bogus time spec to return false")
	}
	if matchTime(info, "1d", "unsupported") {
		t.Fatalf("expected unsupported time type to return false")
	}
}

func TestMatchTimeATimeOlder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if !matchTime(info, "+1h", "atime") {
		t.Fatalf("expected atime older than 1h to match")
	}
}

func TestFindPathFilter(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "pods", "app.log")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(nested, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	output, err := runFindCmd(t, []string{"-path", "*/pods/*", dir})
	if err != nil {
		t.Fatalf("FindCmd failed: %v", err)
	}
	if !strings.Contains(output, nested) {
		t.Fatalf("expected path-filtered output to contain %s, got %q", nested, output)
	}
}

func TestFindNotPathFilter(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.log")
	skip := filepath.Join(dir, "skip", "skip.log")
	if err := os.MkdirAll(filepath.Dir(skip), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write keep: %v", err)
	}
	if err := os.WriteFile(skip, []byte("skip"), 0o644); err != nil {
		t.Fatalf("write skip: %v", err)
	}

	output, err := runFindCmd(t, []string{"-not", "-path", "*/skip/*", dir})
	if err != nil {
		t.Fatalf("FindCmd failed: %v", err)
	}
	if strings.Contains(output, skip) {
		t.Fatalf("expected negated path filter to exclude %s, got %q", skip, output)
	}
	if !strings.Contains(output, keep) {
		t.Fatalf("expected negated path filter to keep %s, got %q", keep, output)
	}
}

func TestFindBangAliasForNot(t *testing.T) {
	dir := t.TempDir()
	match := filepath.Join(dir, "visible.txt")
	hidden := filepath.Join(dir, "hidden", "secret.txt")
	if err := os.MkdirAll(filepath.Dir(hidden), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_ = os.WriteFile(match, []byte("x"), 0o644)
	_ = os.WriteFile(hidden, []byte("x"), 0o644)

	output, err := runFindCmd(t, []string{"!", "-path", "*/hidden/*", dir})
	if err != nil {
		t.Fatalf("FindCmd failed: %v", err)
	}
	if strings.Contains(output, hidden) {
		t.Fatalf("expected ! alias to exclude hidden path, got %q", output)
	}
	if !strings.Contains(output, match) {
		t.Fatalf("expected ! alias to keep visible path, got %q", output)
	}
}

// TestFindPathBeforeFlags is a regression test for a bug where find's own
// help text documents `gobox find . -type f -name '*.log'` (path before
// flags) but the parser rejected any flag-shaped argument once a bare path
// had been seen, failing with "unexpected option found after flags".
func TestFindPathBeforeFlags(t *testing.T) {
	dir := t.TempDir()
	match := filepath.Join(dir, "app.log")
	other := filepath.Join(dir, "app.txt")
	if err := os.WriteFile(match, []byte("data"), 0o644); err != nil {
		t.Fatalf("write match: %v", err)
	}
	if err := os.WriteFile(other, []byte("data"), 0o644); err != nil {
		t.Fatalf("write other: %v", err)
	}

	output, err := runFindCmd(t, []string{dir, "-type", "f", "-name", "*.log"})
	if err != nil {
		t.Fatalf("FindCmd failed with path before flags: %v", err)
	}
	if !strings.Contains(output, match) {
		t.Fatalf("expected output to contain %s, got %q", match, output)
	}
	if strings.Contains(output, other) {
		t.Fatalf("expected output to exclude %s, got %q", other, output)
	}
}

// TestFindPathInterleavedWithFlags exercises paths given between flags to
// confirm order-independence beyond the simple "path first" case.
func TestFindPathInterleavedWithFlags(t *testing.T) {
	dir := t.TempDir()
	match := filepath.Join(dir, "app.log")
	if err := os.WriteFile(match, []byte("data"), 0o644); err != nil {
		t.Fatalf("write match: %v", err)
	}

	output, err := runFindCmd(t, []string{"-type", "f", dir, "-name", "*.log"})
	if err != nil {
		t.Fatalf("FindCmd failed with interleaved path: %v", err)
	}
	if !strings.Contains(output, match) {
		t.Fatalf("expected output to contain %s, got %q", match, output)
	}
}

// The following tests exercise -maxdepth/-mindepth/-empty/-size/-atime/-mtime
// end-to-end through FindCmd. Previously only their underlying helpers
// (pathDepth/matchSize/matchTime) were unit-tested directly, so a broken
// wiring between the flag and the walk closure in FindCmd would have gone
// undetected.

func TestFindMaxDepthEndToEnd(t *testing.T) {
	dir := t.TempDir()
	level1 := filepath.Join(dir, "level1")
	level2 := filepath.Join(level1, "level2")
	deepFile := filepath.Join(level2, "deep.txt")
	if err := os.MkdirAll(level2, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(deepFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	output, err := runFindCmd(t, []string{"-maxdepth", "1", dir})
	if err != nil {
		t.Fatalf("FindCmd failed: %v", err)
	}
	if !strings.Contains(output, level1) {
		t.Fatalf("expected -maxdepth 1 to include depth-1 entry %s, got %q", level1, output)
	}
	if strings.Contains(output, level2) || strings.Contains(output, deepFile) {
		t.Fatalf("expected -maxdepth 1 to exclude entries deeper than 1, got %q", output)
	}
}

func TestFindMinDepthEndToEnd(t *testing.T) {
	dir := t.TempDir()
	level1 := filepath.Join(dir, "level1")
	level2 := filepath.Join(level1, "level2")
	deepFile := filepath.Join(level2, "deep.txt")
	if err := os.MkdirAll(level2, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(deepFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	output, err := runFindCmd(t, []string{"-mindepth", "2", dir})
	if err != nil {
		t.Fatalf("FindCmd failed: %v", err)
	}
	if strings.Contains(output, dir+"\n") || strings.Contains(output, level1+"\n") {
		t.Fatalf("expected -mindepth 2 to exclude the root and depth-1 entries, got %q", output)
	}
	if !strings.Contains(output, level2) || !strings.Contains(output, deepFile) {
		t.Fatalf("expected -mindepth 2 to include depth-2 and deeper entries, got %q", output)
	}
}

func TestFindEmptyEndToEnd(t *testing.T) {
	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "empty.txt")
	nonEmptyFile := filepath.Join(dir, "nonempty.txt")
	emptyDir := filepath.Join(dir, "emptydir")
	nonEmptyDir := filepath.Join(dir, "nonemptydir")
	nonEmptyDirChild := filepath.Join(nonEmptyDir, "child.txt")
	if err := os.WriteFile(emptyFile, nil, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	if err := os.WriteFile(nonEmptyFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write nonempty file: %v", err)
	}
	if err := os.Mkdir(emptyDir, 0o755); err != nil {
		t.Fatalf("mkdir empty: %v", err)
	}
	if err := os.MkdirAll(nonEmptyDir, 0o755); err != nil {
		t.Fatalf("mkdir nonempty: %v", err)
	}
	if err := os.WriteFile(nonEmptyDirChild, []byte("data"), 0o644); err != nil {
		t.Fatalf("write nonempty dir child: %v", err)
	}

	output, err := runFindCmd(t, []string{"-empty", dir})
	if err != nil {
		t.Fatalf("FindCmd failed: %v", err)
	}
	if !strings.Contains(output, emptyFile) || !strings.Contains(output, emptyDir) {
		t.Fatalf("expected -empty to include the empty file and empty dir, got %q", output)
	}
	if strings.Contains(output, nonEmptyFile) || strings.Contains(output, nonEmptyDirChild) {
		t.Fatalf("expected -empty to exclude non-empty entries, got %q", output)
	}
	if strings.Contains(output, nonEmptyDir+"\n") {
		t.Fatalf("expected -empty to exclude the non-empty directory itself, got %q", output)
	}
}

func TestFindSizeEndToEnd(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small.txt")
	big := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(small, []byte("x"), 0o644); err != nil {
		t.Fatalf("write small: %v", err)
	}
	if err := os.WriteFile(big, make([]byte, 20*1024), 0o644); err != nil {
		t.Fatalf("write big: %v", err)
	}

	output, err := runFindCmd(t, []string{"-size", "+10k", dir})
	if err != nil {
		t.Fatalf("FindCmd failed: %v", err)
	}
	if !strings.Contains(output, big) {
		t.Fatalf("expected -size +10k to include the 20KB file, got %q", output)
	}
	if strings.Contains(output, small) {
		t.Fatalf("expected -size +10k to exclude the 1-byte file, got %q", output)
	}
}

func TestFindMtimeEndToEnd(t *testing.T) {
	dir := t.TempDir()
	oldFile := filepath.Join(dir, "old.txt")
	freshFile := filepath.Join(dir, "fresh.txt")
	if err := os.WriteFile(oldFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write old: %v", err)
	}
	if err := os.WriteFile(freshFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write fresh: %v", err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldFile, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	output, err := runFindCmd(t, []string{"-mtime", "+1d", dir})
	if err != nil {
		t.Fatalf("FindCmd failed: %v", err)
	}
	if !strings.Contains(output, oldFile) {
		t.Fatalf("expected -mtime +1d to include the 48h-old file, got %q", output)
	}
	if strings.Contains(output, freshFile) {
		t.Fatalf("expected -mtime +1d to exclude the fresh file, got %q", output)
	}
}

func TestFindAtimeEndToEnd(t *testing.T) {
	dir := t.TempDir()
	oldFile := filepath.Join(dir, "old.txt")
	freshFile := filepath.Join(dir, "fresh.txt")
	if err := os.WriteFile(oldFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write old: %v", err)
	}
	if err := os.WriteFile(freshFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write fresh: %v", err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldFile, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	output, err := runFindCmd(t, []string{"-atime", "+1d", dir})
	if err != nil {
		t.Fatalf("FindCmd failed: %v", err)
	}
	if !strings.Contains(output, oldFile) {
		t.Fatalf("expected -atime +1d to include the 48h-old file, got %q", output)
	}
	if strings.Contains(output, freshFile) {
		t.Fatalf("expected -atime +1d to exclude the fresh file, got %q", output)
	}
}
