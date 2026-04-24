package text

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffEqualFiles(t *testing.T) {
	dir := t.TempDir()
	a := writeDiffTestFile(t, dir, "a", "same\n")
	b := writeDiffTestFile(t, dir, "b", "same\n")

	out, err := captureTextCmd(t, "", func() error {
		return DiffCmd([]string{a, b})
	})
	if err != nil {
		t.Fatalf("expected equal files to succeed: %v", err)
	}
	if out != "" {
		t.Fatalf("expected no output for equal files, got %q", out)
	}
}

func TestDiffNormalChangedAddedDeletedRanges(t *testing.T) {
	dir := t.TempDir()
	a := writeDiffTestFile(t, dir, "a", "one\ntwo\nthree\n")
	b := writeDiffTestFile(t, dir, "b", "one\nTWO\nthree\nfour\n")

	out, err := captureTextCmd(t, "", func() error {
		return DiffCmd([]string{a, b})
	})
	assertDiffExit(t, err)
	for _, want := range []string{"2c2\n< two\n---\n> TWO\n", "3a4\n> four\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected normal diff to contain %q, got %q", want, out)
		}
	}

	c := writeDiffTestFile(t, dir, "c", "one\nthree\n")
	out, err = captureTextCmd(t, "", func() error {
		return DiffCmd([]string{a, c})
	})
	assertDiffExit(t, err)
	if !strings.Contains(out, "2d1\n< two\n") {
		t.Fatalf("expected delete range, got %q", out)
	}
}

func TestDiffUnifiedOutput(t *testing.T) {
	dir := t.TempDir()
	a := writeDiffTestFile(t, dir, "a", "one\ntwo\n")
	b := writeDiffTestFile(t, dir, "b", "one\nTWO\n")

	out, err := captureTextCmd(t, "", func() error {
		return DiffCmd([]string{"-u", a, b})
	})
	assertDiffExit(t, err)
	for _, want := range []string{"--- " + a + "\n", "+++ " + b + "\n", "@@ -1,2 +1,2 @@\n", "-two\n", "+TWO\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected unified diff to contain %q, got %q", want, out)
		}
	}
}

func TestDiffBriefOutput(t *testing.T) {
	dir := t.TempDir()
	a := writeDiffTestFile(t, dir, "a", "a\n")
	b := writeDiffTestFile(t, dir, "b", "b\n")

	out, err := captureTextCmd(t, "", func() error {
		return DiffCmd([]string{"--brief", a, b})
	})
	assertDiffExit(t, err)
	want := "Files " + a + " and " + b + " differ\n"
	if out != want {
		t.Fatalf("expected brief output %q, got %q", want, out)
	}
}

func TestDiffRecursiveSortedTraversal(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left")
	right := filepath.Join(dir, "right")
	writeDiffTestFile(t, left, "z.txt", "same\n")
	writeDiffTestFile(t, left, "sub/a.txt", "old\n")
	writeDiffTestFile(t, right, "z.txt", "same\n")
	writeDiffTestFile(t, right, "sub/a.txt", "new\n")
	writeDiffTestFile(t, right, "sub/b.txt", "extra\n")

	out, err := captureTextCmd(t, "", func() error {
		return DiffCmd([]string{"-r", left, right})
	})
	assertDiffExit(t, err)
	diffAt := strings.Index(out, "diff -r "+filepath.Join(left, "sub/a.txt"))
	onlyAt := strings.Index(out, "Only in")
	if diffAt < 0 || onlyAt < 0 || diffAt > onlyAt {
		t.Fatalf("expected sorted recursive comparison before missing entry, got %q", out)
	}
	if !strings.Contains(out, "1c1\n< old\n---\n> new\n") {
		t.Fatalf("expected nested file diff, got %q", out)
	}
	if !strings.Contains(out, "Only in "+filepath.Join(right, "sub")+": b.txt\n") {
		t.Fatalf("expected missing file report, got %q", out)
	}
}

func TestDiffNewFileTreatsMissingAsEmpty(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	b := writeDiffTestFile(t, dir, "b", "created\n")

	out, err := captureTextCmd(t, "", func() error {
		return DiffCmd([]string{"-N", missing, b})
	})
	assertDiffExit(t, err)
	if !strings.Contains(out, "0a1\n> created\n") {
		t.Fatalf("expected missing file to compare as empty, got %q", out)
	}
}

func TestDiffRecursiveNewFileSkipsMissingDirectoryEntries(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left")
	right := filepath.Join(dir, "right")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDiffTestFile(t, right, "sub/file.txt", "created\n")

	out, err := captureTextCmd(t, "", func() error {
		return DiffCmd([]string{"-r", "-N", left, right})
	})
	assertDiffExit(t, err)
	if !strings.Contains(out, "diff -r "+filepath.Join(left, "sub/file.txt")+" "+filepath.Join(right, "sub/file.txt")+"\n") {
		t.Fatalf("expected recursive -N file diff header, got %q", out)
	}
	if !strings.Contains(out, "0a1\n> created\n") {
		t.Fatalf("expected missing nested file to compare as empty, got %q", out)
	}
}

func TestDiffStripTrailingCR(t *testing.T) {
	dir := t.TempDir()
	a := writeDiffTestFile(t, dir, "a", "one\r\ntwo\r\n")
	b := writeDiffTestFile(t, dir, "b", "one\ntwo\n")

	out, err := captureTextCmd(t, "", func() error {
		return DiffCmd([]string{"--strip-trailing-cr", a, b})
	})
	if err != nil {
		t.Fatalf("expected CR-stripped files to match: %v", err)
	}
	if out != "" {
		t.Fatalf("expected no output for CR-stripped match, got %q", out)
	}
}

func TestDiffStdinOperand(t *testing.T) {
	dir := t.TempDir()
	b := writeDiffTestFile(t, dir, "b", "stdin\n")

	out, err := captureTextCmd(t, "stdin\n", func() error {
		return DiffCmd([]string{"-", b})
	})
	if err != nil {
		t.Fatalf("expected stdin comparison to match: %v", err)
	}
	if out != "" {
		t.Fatalf("expected no output for stdin match, got %q", out)
	}
}

func TestDiffBinaryDifferenceReportsWithoutDumping(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte{0, 1, 2}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte{0, 1, 3}, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return DiffCmd([]string{a, b})
	})
	assertDiffExit(t, err)
	want := "Binary files " + a + " and " + b + " differ\n"
	if out != want {
		t.Fatalf("expected binary report %q, got %q", want, out)
	}
}

func writeDiffTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertDiffExit(t *testing.T, err error) {
	t.Helper()
	exitErr, ok := err.(diffExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected diff exit 1, got %T %v", err, err)
	}
}
