package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTruncateCmdHelpUsesStructuredUsage(t *testing.T) {
	_, out, err := captureFsCmdFull(t, func() error {
		return TruncateCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("truncate --help failed: %v", err)
	}
	for _, want := range []string{"Usage: gobox truncate -s SIZE FILE... | gobox truncate -r RFILE FILE...", "Options:", "-c, --no-create", "Examples:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
}

func TestTruncateCmdOptionsNoCreateSkipsMissing(t *testing.T) {
	dir := t.TempDir()

	missing := filepath.Join(dir, "missing")
	if err := TruncateCmd([]string{"-c", "-s", "4", missing}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("expected missing file to remain absent, err=%v", err)
	}

}

func TestTruncateCmdOptionsNoCreateChangesExistingFile(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "existing-no-create")
	if err := os.WriteFile(file, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := TruncateCmd([]string{"-c", "-s", "2", file}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(file); info.Size() != 2 {
		t.Fatalf("expected existing -c file size 2, got %d", info.Size())
	}

}

func TestTruncateCmdOptionsReferenceFileSize(t *testing.T) {
	dir := t.TempDir()

	ref := filepath.Join(dir, "ref")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(ref, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := TruncateCmd([]string{"-r", ref, dst}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(dst); info.Size() != 6 {
		t.Fatalf("expected dst size 6, got %d", info.Size())
	}

}

func TestTruncateCmdOptionsSuffixAndRelativeSize(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "size")
	if err := TruncateCmd([]string{"-s", "1K", file}); err != nil {
		t.Fatal(err)
	}
	if err := TruncateCmd([]string{"-s", "-24", file}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(file); info.Size() != 1000 {
		t.Fatalf("expected 1000 after relative shrink, got %d", info.Size())
	}

}

func TestTruncateCmdOptionsZeroSize(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "zero")
	if err := os.WriteFile(file, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := TruncateCmd([]string{"-s", "0", file}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(file); info.Size() != 0 {
		t.Fatalf("expected size 0, got %d", info.Size())
	}

}

func TestTruncateCmdOptionsLowercaseSuffix(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "lower")
	if err := TruncateCmd([]string{"-s", "1k", file}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(file); info.Size() != 1024 {
		t.Fatalf("expected lowercase suffix size 1024, got %d", info.Size())
	}

}

func TestTruncateCmdOptionsMegabyteAndGigabyteSuffixParse(t *testing.T) {
	if size, rel, err := parseTruncateSize("1M"); err != nil || rel || size != 1024*1024 {
		t.Fatalf("unexpected 1M parse size=%d rel=%v err=%v", size, rel, err)
	}
	if size, rel, err := parseTruncateSize("1G"); err != nil || rel || size != 1024*1024*1024 {
		t.Fatalf("unexpected 1G parse size=%d rel=%v err=%v", size, rel, err)
	}

}

func TestTruncateCmdOptionsInvalidSizeErrors(t *testing.T) {
	dir := t.TempDir()

	badPath := filepath.Join(dir, "bad")
	if err := TruncateCmd([]string{"-s", "bad", badPath}); err == nil {
		t.Fatal("expected invalid size error")
	}
	if _, err := os.Stat(badPath); !os.IsNotExist(err) {
		t.Fatalf("invalid size should not create target, stat err=%v", err)
	}

}

func TestTruncateCmdOptionsRelativeGrow(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "grow")
	if err := os.WriteFile(file, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := TruncateCmd([]string{"-s", "+6", file}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(file); info.Size() != 10 {
		t.Fatalf("expected size 10, got %d", info.Size())
	}

}

func TestTruncateCmdOptionsRelativeShrinkClampsToZero(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "clamp")
	if err := os.WriteFile(file, []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := TruncateCmd([]string{"-s", "-99", file}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(file); info.Size() != 0 {
		t.Fatalf("expected clamped size 0, got %d", info.Size())
	}

}

func TestTruncateCmdOptionsReferenceSizeZero(t *testing.T) {
	dir := t.TempDir()

	ref := filepath.Join(dir, "empty-ref")
	dst := filepath.Join(dir, "ref-dst")
	if err := os.WriteFile(ref, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("123"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := TruncateCmd([]string{"-r", ref, dst}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(dst); info.Size() != 0 {
		t.Fatalf("expected reference zero size, got %d", info.Size())
	}

}

func TestTruncateCmdOptionsReferenceMissingErrors(t *testing.T) {
	dir := t.TempDir()

	dst := filepath.Join(dir, "missing-ref-dst")
	if err := TruncateCmd([]string{"-r", filepath.Join(dir, "missing-ref"), dst}); err == nil {
		t.Fatal("expected missing reference error")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("missing reference should not create target, stat err=%v", err)
	}

}

func TestTruncateCmdOptionsDirectoryTargetErrors(t *testing.T) {
	dir := t.TempDir()

	if err := TruncateCmd([]string{"-s", "1", dir}); err == nil {
		t.Fatal("expected directory truncate error")
	}

}

func TestTruncateCmdOptionsMissingSizeAndReferenceErrors(t *testing.T) {
	dir := t.TempDir()

	if err := TruncateCmd([]string{filepath.Join(dir, "missing-size")}); err == nil {
		t.Fatal("expected missing size error")
	}

}

func TestTruncateCmdOptionsMissingFileOperandErrors(t *testing.T) {
	if err := TruncateCmd([]string{"-s", "1"}); err == nil {
		t.Fatal("expected missing file operand error")
	}

}
