package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gobox/cmds/utils"
)

func TestDiskUsageAndHumanSize(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	rows, total, err := collectDiskUsage(file, duOptions{apparentSize: true})
	if err != nil {
		t.Fatalf("diskUsage: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total size 3, got %d", total)
	}
	if len(rows) != 1 || rows[0].path != file {
		t.Fatalf("expected root file row, got %#v", rows)
	}
	if got := utils.HumanSize(999); got != "999B" {
		t.Fatalf("unexpected HumanSize: %s", got)
	}
	if got := utils.HumanSize(1024); got != "1.0KB" {
		t.Fatalf("unexpected HumanSize for 1KB: %s", got)
	}
}

func TestDuCmdHelpUsesGroupedSections(t *testing.T) {
	_, out, err := captureFsCmdFull(t, func() error { return DuCmd([]string{"--help"}) })
	if err != nil {
		t.Fatalf("du --help failed: %v", err)
	}
	for _, want := range []string{"Usage: gobox du [OPTION]... [PATH...]", "Options:", "-d, --max-depth N", "--apparent-size", "Examples:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
}

func TestDuApparentAllSummaryTotalAndExclude(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("write keep: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.tmp"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("write skip: %v", err)
	}
	out, err := captureFsCmd(t, func() error {
		return DuCmd([]string{"--apparent-size", "-a", "--exclude", "*.tmp", "-c", dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"\t" + filepath.Join(dir, "keep.txt"), "\t" + dir, "\ttotal"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in du output %q", want, out)
		}
	}
	if strings.Contains(out, "skip.tmp") {
		t.Fatalf("excluded file appeared in output %q", out)
	}
}

func TestDuMaxDepthZeroPrintsOnlyRoot(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureFsCmd(t, func() error { return DuCmd([]string{"--apparent-size", "-d", "0", dir}) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "\n") != 1 || !strings.Contains(out, "\t"+dir+"\n") {
		t.Fatalf("expected only root output for -d 0, got %q", out)
	}
}

func TestDuMaxDepthLongOption(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(sub, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := captureFsCmd(t, func() error { return DuCmd([]string{"--apparent-size", "--max-depth", "1", dir}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\t"+sub+"\n") || strings.Contains(out, "\t"+nested+"\n") {
		t.Fatalf("unexpected --max-depth output %q", out)
	}
}
