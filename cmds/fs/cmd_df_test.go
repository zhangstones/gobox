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

func TestDfHuman(t *testing.T) {
	dir := setupDfFixture(t)
	out, err := captureFsCmd(t, func() error { return DfCmd([]string{"-h", dir}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Filesystem", "Use%", "15.0K"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in df output %q", want, out)
		}
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

func TestDfZeroTotalsRenderDashPercent(t *testing.T) {
	oldGOOS, oldReadMounts, oldStatPath, oldStatfs := dfGOOS, readMounts, statDfPath, statfsDfPath
	dfGOOS = "linux"
	readMounts = func() ([]mountInfo, error) {
		return []mountInfo{{Source: "zero", Target: "/zero", FSType: "tmpfs"}}, nil
	}
	statfsDfPath = func(_ string, st *syscall.Statfs_t) error {
		st.Bsize = 1024
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
