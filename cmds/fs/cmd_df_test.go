package fs

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func setupDfFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{{Source: "dev-test", Target: dir, FSType: "tmpfs"}}, nil
	}
	statDfPath = os.Stat
	statfsDfPath = func(_ string, st *syscall.Statfs_t) error {
		st.Bsize = 1024
		st.Blocks = 20
		st.Bfree = 5
		st.Bavail = 5
		st.Files = 10
		st.Ffree = 6
		return nil
	}
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	return dir
}

func TestDfPath(t *testing.T) {
	dir := setupDfFixture(t)
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{dir}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Filesystem", "dev-test", dir} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in df output %q", want, out)
		}
	}
}

func TestDfCmdHelpUsesGroupedSections(t *testing.T) {
	setupDfFixture(t)
	_, out, err := captureFsCmdFull(t, func() error { return DfCmd([]string{"--help"}) })
	if err != nil {
		t.Fatalf("df --help failed: %v", err)
	}
	for _, want := range []string{"Usage: gobox df [OPTION]... [PATH...]", "Display:", "Filters:", "--total", "-t TYPE"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
}

func TestDfHuman(t *testing.T) {
	dir := setupDfFixture(t)
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-h", dir}) })
	if err != nil {
		t.Fatal(err)
	}
	// GNU df -h uses adaptive precision: a rounded magnitude of 10 or more
	// drops the decimal place, so 15360 bytes (15 KiB exactly) renders as
	// "15K", not "15.0K".
	for _, want := range []string{"Filesystem", "Use%", "15K"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in df output %q", want, out)
		}
	}
}

func TestDfSIHuman(t *testing.T) {
	dir := setupDfFixture(t)
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-H", dir}) })
	if err != nil {
		t.Fatal(err)
	}
	// GNU df -H (SI) uses a lowercase "k" specifically for the kilo prefix
	// (mega/giga/etc. stay uppercase), and the same adaptive precision as
	// -h: 15360 bytes / 1000 = 15.36 rounds to 15.4, which is >= 10 so it
	// renders without a decimal: "15k".
	if !strings.Contains(out, "15k") {
		t.Fatalf("expected SI formatted used size in df output %q", out)
	}
}

func TestDfType(t *testing.T) {
	dir := setupDfFixture(t)
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-T", dir}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Type", "tmpfs", "Mounted on"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in df output %q", want, out)
		}
	}
}

func TestDfLongFilesystemAndTypeStayAligned(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{{
			Source: "/dev/mapper/very-long-container-volume-name",
			Target: "/mnt/data",
			FSType: "verylongfilesystemtype",
		}}, nil
	}
	statDfPath = os.Stat
	statfsDfPath = func(_ string, st *syscall.Statfs_t) error {
		st.Bsize = 1024
		st.Blocks = 20
		st.Bfree = 5
		st.Bavail = 5
		return nil
	}
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})

	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-T"}) })
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got %q", out)
	}
	headerType := strings.Index(lines[0], "Type")
	rowType := strings.Index(lines[1], "verylongfilesystemtype")
	if headerType == -1 || rowType != headerType {
		t.Fatalf("expected type column to stay aligned, got header=%q row=%q", lines[0], lines[1])
	}
}

func TestDfInodes(t *testing.T) {
	dir := setupDfFixture(t)
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-i", dir}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Inodes", "IUse%"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in df output %q", want, out)
		}
	}
}

func TestDfHumanWithType(t *testing.T) {
	dir := setupDfFixture(t)
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-h", "-T", dir}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Type", "Use%"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in df output %q", want, out)
		}
	}
}

func TestDfDefaultAllMounts(t *testing.T) {
	_ = setupDfFixture(t)
	out, err := captureFsCmd(t, func() error { return DfCmd(nil) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Filesystem", "Mounted on"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in df output %q", want, out)
		}
	}
}

func TestDfExplicitMissingPathError(t *testing.T) {
	dir := setupDfFixture(t)
	if _, err := captureFsCmd(t, func() error {
		return DfCmd([]string{filepath.Join(dir, "missing")})
	}); err == nil {
		t.Fatal("expected explicit missing path error")
	}
}

func TestDfRelativePathOperand(t *testing.T) {
	dir := setupDfFixture(t)
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Mkdir("sub", 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"sub"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Filesystem") {
		t.Fatalf("unexpected relative df output %q", out)
	}
}

func TestDfUnsupportedOSReturnsError(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "plan9"
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	if _, err := captureFsCmd(t, func() error { return DfCmd(nil) }); err == nil {
		t.Fatal("expected unsupported OS error")
	}
}

func TestDfMountinfoReadError(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) { return nil, os.ErrPermission }
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	if _, err := captureFsCmd(t, func() error { return DfCmd(nil) }); err == nil {
		t.Fatal("expected mountinfo error")
	}
}

func TestDfDefaultDeduplicatesMountTargets(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{
			{Source: "dev-a", Target: "/mnt/a", FSType: "tmpfs"},
			{Source: "dev-b", Target: "/mnt/a", FSType: "tmpfs"},
		}, nil
	}
	statfsDfPath = func(_ string, st *syscall.Statfs_t) error {
		st.Bsize = 1024
		st.Blocks = 10
		st.Bfree = 5
		st.Bavail = 5
		st.Files = 10
		st.Ffree = 8
		return nil
	}
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	out, err := captureFsCmd(t, func() error { return DfCmd(nil) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "/mnt/a") != 1 {
		t.Fatalf("expected duplicate mount target once, got %q", out)
	}
}

func TestDfAllIncludesDuplicateMountTargets(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{
			{Source: "dev-a", Target: "/mnt/a", FSType: "tmpfs"},
			{Source: "dev-b", Target: "/mnt/a", FSType: "tmpfs"},
		}, nil
	}
	statfsDfPath = func(_ string, st *syscall.Statfs_t) error {
		st.Bsize = 1024
		st.Blocks = 10
		st.Bfree = 5
		st.Bavail = 5
		return nil
	}
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-a"}) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "/mnt/a") != 2 {
		t.Fatalf("expected duplicate mount target twice with -a, got %q", out)
	}
}

func TestDfTypeFiltersAndLocal(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{
			{Source: "local-dev", Target: "/local", FSType: "ext4"},
			{Source: "server:/share", Target: "/remote", FSType: "nfs"},
			{Source: "tmp", Target: "/tmpfs", FSType: "tmpfs"},
		}, nil
	}
	statfsDfPath = func(_ string, st *syscall.Statfs_t) error {
		st.Bsize = 1024
		st.Blocks = 10
		st.Bfree = 5
		st.Bavail = 5
		return nil
	}
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-l", "-x", "tmpfs"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "/local") || strings.Contains(out, "/remote") || strings.Contains(out, "/tmpfs") {
		t.Fatalf("unexpected local/exclude output %q", out)
	}

	out, err = captureFsCmd(t, func() error { return DfCmd([]string{"-t", "tmpfs"}) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "/local") || !strings.Contains(out, "/tmpfs") {
		t.Fatalf("unexpected include type output %q", out)
	}
}

func TestDfTotalAndPosix(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{
			{Source: "dev-a", Target: "/a", FSType: "ext4"},
			{Source: "dev-b", Target: "/b", FSType: "ext4"},
		}, nil
	}
	statfsDfPath = func(_ string, st *syscall.Statfs_t) error {
		st.Bsize = 1024
		st.Blocks = 10
		st.Bfree = 4
		st.Bavail = 4
		return nil
	}
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-P", "--total"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1024-blocks", "total"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in df output %q", want, out)
		}
	}
}

func TestDfZeroTotalsRenderDashPercent(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{{Source: "zero", Target: "/zero", FSType: "tmpfs"}}, nil
	}
	statfsDfPath = func(_ string, st *syscall.Statfs_t) error {
		// Nonzero Blocks keeps the row from being hidden by the
		// zero-block visibility filter (matching native df, which hides
		// truly-empty pseudo-filesystems unless -a is given); Files stays
		// zero to exercise the zero-total dash-percent rendering.
		st.Bsize = 1024
		st.Blocks = 10
		return nil
	}
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-i"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, " - ") {
		t.Fatalf("expected dash percent for zero total, got %q", out)
	}
}

func TestDfStatfsErrorReturnsError(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{{Source: "bad", Target: "/bad", FSType: "tmpfs"}}, nil
	}
	statfsDfPath = func(_ string, _ *syscall.Statfs_t) error { return os.ErrPermission }
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	if _, err := captureFsCmd(t, func() error { return DfCmd(nil) }); err == nil {
		t.Fatal("expected statfs error")
	}
}

func TestDfExplicitPathStatErrorReturnsError(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{{Source: "root", Target: "/", FSType: "tmpfs"}}, nil
	}
	statDfPath = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	if _, err := captureFsCmd(t, func() error { return DfCmd([]string{"/missing"}) }); err == nil {
		t.Fatal("expected explicit path stat error")
	}
}

// TestDfHumanAdaptivePrecision is a regression test for gobox df -h always
// showing exactly one decimal place. Native df uses adaptive precision: a
// rounded magnitude >= 10 drops the decimal, < 10 keeps one decimal.
func TestDfHumanAdaptivePrecision(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{{Source: "dev-small", Target: "/small", FSType: "tmpfs"}}, nil
	}
	// Total of exactly 20 MiB (>=10, no decimal) and a free amount giving
	// exactly 8 MiB used (<10, decimal) exercises both branches of the
	// adaptive-precision rule in one row.
	statfsDfPath = func(_ string, st *syscall.Statfs_t) error {
		st.Bsize = 1024
		st.Blocks = 20 * 1024 // total: 20 MiB
		st.Bfree = 12 * 1024  // free: 12 MiB -> used: 8 MiB
		st.Bavail = 12 * 1024
		return nil
	}
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-h"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "20M") {
		t.Fatalf("expected total 20M (no decimal, >=10) in %q", out)
	}
	if !strings.Contains(out, "8.0M") {
		t.Fatalf("expected used 8.0M (decimal, <10) in %q", out)
	}
	if !strings.Contains(out, "12M") {
		t.Fatalf("expected avail 12M (no decimal, >=10) in %q", out)
	}
}

// TestDfSILowercaseKilo is a regression test for the SI (-H) kilo prefix:
// GNU df uses a lowercase "k" specifically at the kilo level, unlike -h's
// uppercase "K" and unlike SI's own mega/giga letters which stay uppercase.
func TestDfSILowercaseKilo(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{{Source: "dev-k", Target: "/k", FSType: "tmpfs"}}, nil
	}
	statfsDfPath = func(_ string, st *syscall.Statfs_t) error {
		st.Bsize = 1024
		st.Blocks = 5 // 5120 bytes -> 5.12k in SI, rendered "5.1k"
		st.Bfree = 0
		st.Bavail = 0
		return nil
	}
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-H"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "5.1k") {
		t.Fatalf("expected lowercase SI kilo suffix \"5.1k\" in %q", out)
	}
	if strings.Contains(out, "5.1K") {
		t.Fatalf("did not expect uppercase K for SI kilo in %q", out)
	}
}

// TestDfHumanAvailHeaderAbbreviated is a regression test: native df
// abbreviates the "Available" header to "Avail" once the size column
// itself switches to human/SI units.
func TestDfHumanAvailHeaderAbbreviated(t *testing.T) {
	dir := setupDfFixture(t)
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-h", dir}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Avail ") {
		t.Fatalf("expected abbreviated \"Avail\" header in -h output %q", out)
	}
	if strings.Contains(out, "Available") {
		t.Fatalf("did not expect full \"Available\" header in -h output %q", out)
	}
}

// TestDfPosixCapacityHeader is a regression test: strict POSIX mode (-P
// without -h/-H) labels the percentage column "Capacity", not "Use%".
func TestDfPosixCapacityHeader(t *testing.T) {
	dir := setupDfFixture(t)
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-P", dir}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Capacity") {
		t.Fatalf("expected \"Capacity\" header in -P output %q", out)
	}
	if strings.Contains(out, "Use%") {
		t.Fatalf("did not expect \"Use%%\" header in -P output %q", out)
	}
}

// TestDfInodesHidesZeroBlockFilesystemsByDefault is a regression test for a
// bug where -i bypassed the same zero-block visibility filter that default
// df and -T apply, leaking dozens of zero-inode pseudo-filesystems
// (bpf/cgroup2/tracefs/...) into -i output unless -a was given.
func TestDfInodesHidesZeroBlockFilesystemsByDefault(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{
			{Source: "real", Target: "/real", FSType: "tmpfs"},
			{Source: "pseudo", Target: "/sys/fs/pseudo", FSType: "pseudofs"},
		}, nil
	}
	statfsDfPath = func(target string, st *syscall.Statfs_t) error {
		st.Bsize = 1024
		if target == "/real" {
			st.Blocks = 10
			st.Bfree = 5
			st.Bavail = 5
			st.Files = 10
			st.Ffree = 5
		}
		// pseudo mount: everything stays zero (Blocks == 0).
		return nil
	}
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})

	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-i"}) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "pseudo") {
		t.Fatalf("expected zero-block pseudo filesystem hidden from -i without -a, got %q", out)
	}
	if !strings.Contains(out, "real") {
		t.Fatalf("expected real filesystem present in -i output %q", out)
	}

	allOut, err := captureFsCmd(t, func() error { return DfCmd([]string{"-i", "-a"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(allOut, "pseudo") {
		t.Fatalf("expected zero-block pseudo filesystem present in -i -a output %q", allOut)
	}
}

// TestDfInodesNegativeUsedDoesNotUnderflow is a regression test for a bug
// where Ffree > Files (observed on some pseudo-filesystems, e.g. vboxsf)
// caused an unsigned-subtraction underflow, printing a huge nonsensical
// "used" inode count instead of a small negative number, and a bogus
// percentage instead of "-".
func TestDfInodesNegativeUsedDoesNotUnderflow(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{{Source: "weird", Target: "/weird", FSType: "vboxsf"}}, nil
	}
	statfsDfPath = func(_ string, st *syscall.Statfs_t) error {
		st.Bsize = 1024
		st.Blocks = 10
		st.Bfree = 5
		st.Bavail = 5
		st.Files = 1000
		st.Ffree = 2000000 // Ffree > Files: structurally inconsistent
		return nil
	}
	t.Cleanup(func() {
		dfGOOS, readMounts, statDfPath, statfsDfPath = oldGOOS, oldReadMounts, oldStatPath, oldStatfs
	})
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-i"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "-1999000") {
		t.Fatalf("expected signed negative used inode count -1999000, got %q", out)
	}
	if strings.Contains(out, "18446744") {
		t.Fatalf("expected no unsigned-underflow wraparound value, got %q", out)
	}
}
