package fs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

// TestDuCmdBundledShortFlags is a regression test for a bug where du's own
// help text documents `gobox du -sh .` (bundled short flags) but the flag
// parser rejected it with "flag provided but not defined: -sh" since Go's
// flag package does not support bundling by default.
func TestDuCmdBundledShortFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("abcdef"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	out, err := captureFsCmd(t, func() error {
		return DuCmd([]string{"-sh", dir})
	})
	if err != nil {
		t.Fatalf("du -sh failed: %v", err)
	}
	if !strings.Contains(out, dir) {
		t.Fatalf("expected summarized output to contain %s, got %q", dir, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected -s to summarize into a single line, got %q", out)
	}
	fields := strings.Fields(lines[0])
	if len(fields) != 2 {
		t.Fatalf("expected two fields in output, got %q", lines[0])
	}

	// Without --apparent-size, du reports real disk-block usage (st.Blocks*512),
	// summed over the directory's own allocation plus its child file's --
	// compute the expected value the same way and compare exactly, rather than
	// just checking that the printed size parses as a number.
	expectedBlockSize := func(path string) int64 {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("lstat %s: %v", path, err)
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatal("expected *syscall.Stat_t")
		}
		return st.Blocks * 512
	}
	wantTotal := expectedBlockSize(dir) + expectedBlockSize(filepath.Join(dir, "a.txt"))
	wantHuman := utils.HumanSize(wantTotal)
	if fields[0] != wantHuman {
		t.Fatalf("expected disk-block size %s, got %q (full line: %q)", wantHuman, fields[0], lines[0])
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
	if strings.Contains(out, "skip.tmp") {
		t.Fatalf("excluded file appeared in output %q", out)
	}

	// Parse each "BLOCKS\tPATH" row and verify the totals are real sums, not
	// just present-somewhere-in-the-output. With --apparent-size, size is
	// exact byte count (info.Size()), so we can compute the expected 1K-block
	// count for each row the same way printDuRow does and compare exactly.
	blocksFor := func(t *testing.T, path string) int64 {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("lstat %s: %v", path, err)
		}
		return (info.Size() + 1023) / 1024
	}
	rowBlocks := map[string]int64{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("unexpected du row %q", line)
		}
		n, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			t.Fatalf("expected numeric block count in row %q: %v", line, err)
		}
		rowBlocks[parts[1]] = n
	}

	keep := filepath.Join(dir, "keep.txt")
	wantKeepBlocks := blocksFor(t, keep)
	if got, ok := rowBlocks[keep]; !ok || got != wantKeepBlocks {
		t.Fatalf("expected %s row = %d blocks, got %v (present=%v)", keep, wantKeepBlocks, got, ok)
	}

	wantDirBytes := int64(0)
	if info, err := os.Lstat(dir); err == nil {
		wantDirBytes = info.Size()
	} else {
		t.Fatalf("lstat %s: %v", dir, err)
	}
	if info, err := os.Lstat(keep); err == nil {
		wantDirBytes += info.Size()
	} else {
		t.Fatalf("lstat %s: %v", keep, err)
	}
	wantDirBlocks := (wantDirBytes + 1023) / 1024
	if got, ok := rowBlocks[dir]; !ok || got != wantDirBlocks {
		t.Fatalf("expected %s row (dir total, excluding skip.tmp) = %d blocks, got %v (present=%v)", dir, wantDirBlocks, got, ok)
	}
	if got, ok := rowBlocks["total"]; !ok || got != wantDirBlocks {
		t.Fatalf("expected grand total row = %d blocks (same as the single root dir), got %v (present=%v)", wantDirBlocks, got, ok)
	}
}

// TestDuExcludeMalformedPatternReturnsError is a regression test: an invalid
// glob pattern must surface as an error (matching GNU du), not be silently
// ignored (which would leave every file un-excluded with no diagnostic).
func TestDuExcludeMalformedPatternReturnsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := DuCmd([]string{"--exclude", "[", dir})
	if err == nil {
		t.Fatal("expected an error for a malformed --exclude pattern")
	}
	if !strings.Contains(err.Error(), "[") {
		t.Fatalf("expected error to mention the offending pattern, got: %v", err)
	}
}

// TestDuExcludeNoSlashPatternMatchesBasenameOnly verifies a pattern without
// "/" excludes any file with that basename regardless of depth, matching
// GNU du/fnmatch semantics for no-slash patterns.
func TestDuExcludeNoSlashPatternMatchesBasenameOnly(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "skip.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "keep.txt"), []byte("c"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureFsCmd(t, func() error {
		return DuCmd([]string{"-a", "--exclude", "skip.txt", dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "skip.txt") {
		t.Fatalf("expected skip.txt excluded at every depth, got %q", out)
	}
	if !strings.Contains(out, "keep.txt") {
		t.Fatalf("expected keep.txt to remain, got %q", out)
	}
}

// TestDuExcludeWithSlashPatternMatchesRelativePath verifies a pattern
// containing "/" matches consistently against the path relative to root,
// regardless of whether root was spelled as an absolute or relative path.
func TestDuExcludeWithSlashPatternMatchesRelativePath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "skip.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "keep.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	absOut, err := captureFsCmd(t, func() error {
		return DuCmd([]string{"-a", "--exclude", "sub/skip.txt", dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(absOut, "skip.txt") || !strings.Contains(absOut, "keep.txt") {
		t.Fatalf("absolute root: expected skip.txt excluded and keep.txt present, got %q", absOut)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)
	if err := os.Chdir(filepath.Dir(dir)); err != nil {
		t.Fatal(err)
	}
	relRoot := filepath.Base(dir)
	relOut, err := captureFsCmd(t, func() error {
		return DuCmd([]string{"-a", "--exclude", "sub/skip.txt", relRoot})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(relOut, "skip.txt") || !strings.Contains(relOut, "keep.txt") {
		t.Fatalf("relative root: expected skip.txt excluded and keep.txt present, got %q", relOut)
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

// TestDuOneFilesystemSkipsChildOnDifferentDevice is a regression test for
// -x/--one-file-system. Constructing a real cross-filesystem fixture isn't
// possible in this sandbox (see tests/parity DU-007), so this drives walkDu
// directly with a rootDev that deliberately doesn't match the real device of
// the tree being walked, exercising the same comparison collectDiskUsage
// performs when a subtree actually lives on a different filesystem.
func TestDuOneFilesystemSkipsChildOnDifferentDevice(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Skip("syscall.Stat_t not available on this platform")
	}
	realRootDev := uint64(st.Dev)
	fakeRootDev := realRootDev + 1

	// Same-device baseline: -x must not exclude anything when rootDev matches
	// reality, so the subtree's contribution is included.
	var sameFsRows []duRow
	sameFsTotal, err := walkDu(dir, info, 0, dir, realRootDev, duOptions{oneFS: true, apparentSize: true}, &sameFsRows)
	if err != nil {
		t.Fatal(err)
	}

	var rows []duRow
	total, err := walkDu(dir, info, 0, dir, fakeRootDev, duOptions{oneFS: true, apparentSize: true}, &rows)
	if err != nil {
		t.Fatal(err)
	}
	rootOnly := duFileSize(info, true)
	if total != rootOnly {
		t.Fatalf("expected sub tree on a different device to contribute nothing (root-only size %d), got %d", rootOnly, total)
	}
	if total >= sameFsTotal {
		t.Fatalf("expected -x total (%d) to be smaller than the same-filesystem total (%d)", total, sameFsTotal)
	}
	for _, row := range rows {
		if row.path == sub || strings.HasPrefix(row.path, sub+string(filepath.Separator)) {
			t.Fatalf("expected -x to skip %s entirely, but it appeared in rows: %#v", sub, rows)
		}
	}
	if len(rows) != 1 || rows[0].path != dir {
		t.Fatalf("expected only the root dir row, got %#v", rows)
	}
}
