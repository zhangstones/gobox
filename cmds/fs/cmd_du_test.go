package fs

import (
	"os"
	"path/filepath"
	"testing"

	"gobox/cmds/utils"
)

func TestDiskUsageAndHumanSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("defg"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	total, err := diskUsage(dir)
	if err != nil {
		t.Fatalf("diskUsage: %v", err)
	}
	if total != 7 {
		t.Fatalf("expected total size 7, got %d", total)
	}
	if got := utils.HumanSize(999); got != "999B" {
		t.Fatalf("unexpected HumanSize: %s", got)
	}
	if got := utils.HumanSize(1024); got != "1.0KB" {
		t.Fatalf("unexpected HumanSize for 1KB: %s", got)
	}
}
