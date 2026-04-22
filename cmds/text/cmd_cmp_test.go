package text

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmpDifferentFilesExitCode(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureTextCmd(t, "", func() error {
		return CmpCmd([]string{"-s", a, b})
	})
	if exitErr, ok := err.(cmpExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected cmp exit 1, got %T %v", err, err)
	}
	if out != "" {
		t.Fatalf("expected silent cmp stdout empty, got %q", out)
	}
}

func TestCmpCmdOptionsEqualFiles(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("abcXef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return CmpCmd([]string{a, a})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected no output for equal files, got %q", out)
	}

}

func TestCmpCmdOptionsListDifferences(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("abcXef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	multiA := filepath.Join(dir, "multi-a")
	multiB := filepath.Join(dir, "multi-b")
	if err := os.WriteFile(multiA, []byte("abcXefY"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(multiB, []byte("abcdefZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureTextCmd(t, "", func() error {
		return CmpCmd([]string{"-l", multiA, multiB})
	})
	if exitErr, ok := err.(cmpExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected cmp exit 1, got %T %v", err, err)
	}
	if !strings.Contains(out, "4 130 144") || !strings.Contains(out, "7 131 132") {
		t.Fatalf("unexpected cmp -l output %q", out)
	}

}

func TestCmpCmdOptionsLimitHidesLaterDifference(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("abcXef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return CmpCmd([]string{"-n", "3", a, b})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected no output before limit, got %q", out)
	}

}

func TestCmpCmdOptionsZeroLimitComparesNothing(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("abcXef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return CmpCmd([]string{"-n", "0", a, b})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected no output with zero limit, got %q", out)
	}

}

func TestCmpCmdOptionsInvalidLimitFlag(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("abcXef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return CmpCmd([]string{"-n", "bad", a, b})
	}); err == nil {
		t.Fatal("expected invalid limit flag error")
	}

}

func TestCmpCmdOptionsStdinSide(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("abcXef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "abcXef", func() error {
		return CmpCmd([]string{a, "-"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected stdin comparison to match, got %q", out)
	}

}

func TestCmpCmdOptionsBothStdinRejected(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("abcXef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "abc", func() error {
		return CmpCmd([]string{"-", "-"})
	}); err == nil {
		t.Fatal("expected both-stdin error")
	}

}

func TestCmpCmdOptionsDefaultDifferenceOutput(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("abcXef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return CmpCmd([]string{a, b})
	})
	if exitErr, ok := err.(cmpExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected cmp exit 1, got %T %v", err, err)
	}
	if !strings.Contains(out, "differ: byte 4") {
		t.Fatalf("unexpected cmp difference output %q", out)
	}

}

func TestCmpCmdOptionsMissingOperand(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("abcXef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return CmpCmd([]string{a})
	}); err == nil {
		t.Fatal("expected missing operand error")
	}

}

func TestCmpCmdOptionsMissingFile(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("abcXef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return CmpCmd([]string{filepath.Join(dir, "missing"), b})
	}); err == nil {
		t.Fatal("expected missing file error")
	}

}

func TestCmpCmdOptionsEofDifference(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("abcXef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	short := filepath.Join(dir, "short")
	if err := os.WriteFile(short, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureTextCmd(t, "", func() error {
		return CmpCmd([]string{short, b})
	})
	if exitErr, ok := err.(cmpExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected cmp exit 1, got %T %v", err, err)
	}
	if !strings.Contains(out, "differ") {
		t.Fatalf("unexpected EOF difference output %q", out)
	}

}
