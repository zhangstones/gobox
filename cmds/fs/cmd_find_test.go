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
