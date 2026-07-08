package fs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestStatCmdOptionsFormatTokens(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-c", "%n:%s:%F:%a", file})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, file+":5:regular file:") {
		t.Fatalf("unexpected format output %q", out)
	}

}

// TestStatCmdOptionsExpandedFormatDirectives is a regression test for the
// previously-missing GNU stat -c directives: %f (raw hex mode), %u/%g
// (numeric uid/gid), %U/%G (user/group name), %A (rwx permission string),
// %i (inode), %h (link count), %d/%D (device decimal/hex), %o (block size),
// %b (blocks), %X/%Y/%Z (atime/mtime/ctime epoch), %x/%z (atime/ctime
// human-readable).
func TestStatCmdOptionsExpandedFormatDirectives(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(file)
	if err != nil {
		t.Fatal(err)
	}
	st := info.Sys().(*syscall.Stat_t)

	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-c", "%u|%g|%U|%G|%A|%i|%h|%o|%b|%Y", file})
	})
	if err != nil {
		t.Fatal(err)
	}
	want := strconv.FormatUint(uint64(st.Uid), 10) + "|" +
		strconv.FormatUint(uint64(st.Gid), 10) + "|" +
		lookupUserName(st.Uid) + "|" +
		lookupGroupName(st.Gid) + "|" +
		"-rw-r--r--|" +
		strconv.FormatUint(st.Ino, 10) + "|" +
		strconv.FormatUint(st.Nlink, 10) + "|" +
		strconv.FormatInt(st.Blksize, 10) + "|" +
		strconv.FormatInt(st.Blocks, 10) + "|" +
		strconv.FormatInt(st.Mtim.Sec, 10)
	if !strings.Contains(out, want) {
		t.Fatalf("expected expanded format directives %q in output %q", want, out)
	}

	rawOut, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-c", "%f", file})
	})
	if err != nil {
		t.Fatal(err)
	}
	wantRaw := strconv.FormatUint(uint64(st.Mode), 16)
	if !strings.Contains(rawOut, wantRaw) {
		t.Fatalf("expected raw hex mode %q in output %q", wantRaw, rawOut)
	}
}

// TestStatCmdOptionsDefaultOutputIncludesDeviceAndOwner is a regression test
// for the default (non -c, non -t) output previously only showing
// File/Size/Modify; it must now also show Device/Inode/Links and
// Access/Uid/Gid, matching GNU stat's default layout.
func TestStatCmdOptionsDefaultOutputIncludesDeviceAndOwner(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{file})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Device:", "Inode:", "Links:", "Uid:", "Gid:", "Access:", "Modify:", "Change:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in default stat output %q", want, out)
		}
	}
}

func TestStatCmdHelpUsesMergedLongFlags(t *testing.T) {
	_, out, err := captureFsCmdFull(t, func() error {
		return StatCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("stat --help failed: %v", err)
	}
	for _, want := range []string{"Usage: gobox stat [OPTION]... FILE...", "-L, --dereference", "-f, --file-system", "-c, --format FORMAT"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
}

func TestStatCmdOptionsEmptyFileSizeAndTimestampToken(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-c", "%s %Y", empty})
	})
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(out)
	if len(fields) != 2 || fields[0] != "0" {
		t.Fatalf("unexpected empty file stat output %q", out)
	}
	if _, err := strconv.ParseInt(fields[1], 10, 64); err != nil {
		t.Fatalf("expected numeric timestamp, got %q", fields[1])
	}

}

func TestStatCmdOptionsDereferenceSymlink(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-L", "-c", "%F %s", link})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "regular file 5" {
		t.Fatalf("unexpected dereference output %q", out)
	}

}

func TestStatCmdOptionsDereferenceSymlinkToDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	dirLink := filepath.Join(dir, "dir-link")
	if err := os.Symlink(".", dirLink); err != nil {
		t.Fatal(err)
	}
	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-L", "-c", "%F", dirLink})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "directory" {
		t.Fatalf("unexpected dereference directory output %q", out)
	}

}

func TestStatCmdOptionsTerse(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-t", file})
	})
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(out)
	if len(fields) < 4 || fields[0] != file || fields[1] != "5" {
		t.Fatalf("unexpected terse output %q", out)
	}

}

func TestStatCmdOptionsTerseFilenameWithSpaces(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	spaced := filepath.Join(dir, "file with spaces")
	if err := os.WriteFile(spaced, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-t", spaced})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, spaced) || !strings.Contains(out, " 1 ") {
		t.Fatalf("unexpected terse spaced filename output %q", out)
	}

}

func TestStatCmdOptionsFilesystemFormat(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-f", "-c", "%n %s %b %f", dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(out)
	if len(fields) != 4 || fields[0] != dir {
		t.Fatalf("unexpected statfs output %q", out)
	}
	for _, field := range fields[1:] {
		if _, err := strconv.ParseUint(field, 10, 64); err != nil {
			t.Fatalf("expected numeric statfs field %q: %v", field, err)
		}
	}

}

// TestFormatFsid is a regression test for a bug where `stat -f`'s default
// output printed the raw Go struct for the filesystem ID (e.g. "{[64768 0]}")
// instead of a human-readable hex value like GNU coreutils' stat does.
func TestFormatFsid(t *testing.T) {
	fsid := syscall.Fsid{X__val: [2]int32{64768, 0}}
	got := formatFsid(fsid)
	want := "fd0000000000"
	if got != want {
		t.Fatalf("formatFsid(%v) = %q, want %q", fsid, got, want)
	}
	if strings.ContainsAny(got, "{[]}") {
		t.Fatalf("formatFsid should not leak raw struct formatting, got %q", got)
	}
}

// TestStatCmdOptionsFilesystemDefaultOutput ensures the default (non -c,
// non -t) `stat -f` output renders the ID field as a hex string rather than
// Go's default struct representation of syscall.Fsid.
func TestStatCmdOptionsFilesystemDefaultOutput(t *testing.T) {
	dir := t.TempDir()

	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-f", dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ID: ") {
		t.Fatalf("expected ID field in output, got %q", out)
	}
	if strings.ContainsAny(out, "{[]}") {
		t.Fatalf("expected human-readable ID, got raw struct formatting in %q", out)
	}
	idLine := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "ID: ") {
			idLine = line
			break
		}
	}
	fields := strings.Fields(idLine)
	// fields: "ID:" "<hex>" "Namelen:" "N" "Type:" "<name>"
	if len(fields) < 2 {
		t.Fatalf("unexpected ID line %q", idLine)
	}
	if _, err := strconv.ParseUint(fields[1], 16, 64); err != nil {
		t.Fatalf("expected hex ID value, got %q: %v", fields[1], err)
	}
}

// TestStatCmdOptionsFilesystemInodesLine is a regression test for the
// default (non -c, non -t) `stat -f` output missing the "Inodes: Total/Free"
// line and "Fundamental block size" that GNU coreutils' stat -f prints.
func TestStatCmdOptionsFilesystemInodesLine(t *testing.T) {
	dir := t.TempDir()

	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-f", dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Fundamental block size:") {
		t.Fatalf("expected Fundamental block size field in output, got %q", out)
	}
	var inodesLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Inodes:") {
			inodesLine = line
			break
		}
	}
	if inodesLine == "" {
		t.Fatalf("expected an Inodes: line in output, got %q", out)
	}
	fields := strings.Fields(inodesLine)
	// fields: "Inodes:" "Total:" "<n>" "Free:" "<n>"
	if len(fields) != 5 || fields[0] != "Inodes:" || fields[1] != "Total:" || fields[3] != "Free:" {
		t.Fatalf("unexpected Inodes line %q", inodesLine)
	}
	if _, err := strconv.ParseUint(fields[2], 10, 64); err != nil {
		t.Fatalf("expected numeric Inodes total, got %q: %v", fields[2], err)
	}
	if _, err := strconv.ParseUint(fields[4], 10, 64); err != nil {
		t.Fatalf("expected numeric Inodes free, got %q: %v", fields[4], err)
	}
}

func TestStatFSTypeName(t *testing.T) {
	if got := statFSTypeName(0x58465342); got != "xfs" {
		t.Fatalf("expected xfs magic to map to xfs, got %q", got)
	}
	if got := statFSTypeName(0x1234); got != "1234" {
		t.Fatalf("expected unknown fs type to fall back to hex, got %q", got)
	}
}

func TestStatCmdOptionsFilesystemMissingPathErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	if _, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-f", filepath.Join(dir, "missing")})
	}); err == nil {
		t.Fatal("expected missing filesystem path error")
	}

}

func TestStatCmdOptionsDefaultOutput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{file})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"File:", "Size:", "Modify:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in default stat output %q", want, out)
		}
	}

}

func TestStatCmdOptionsSymlinkWithoutDereference(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-c", "%F", link})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "symbolic link" {
		t.Fatalf("unexpected symlink type output %q", out)
	}

}

func TestStatCmdOptionsMissingOperand(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	if _, err := captureFsCmd(t, func() error {
		return StatCmd(nil)
	}); err == nil {
		t.Fatal("expected missing operand error")
	}

}

func TestStatCmdOptionsNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	if _, err := captureFsCmd(t, func() error {
		return StatCmd([]string{filepath.Join(dir, "missing")})
	}); err == nil {
		t.Fatal("expected nonexistent file error")
	}

}

func TestStatCmdOptionsDanglingSymlinkDereferenceErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("file", link); err != nil {
		t.Fatal(err)
	}

	dangling := filepath.Join(dir, "dangling")
	if err := os.Symlink("missing-target", dangling); err != nil {
		t.Fatal(err)
	}
	if _, err := captureFsCmd(t, func() error {
		return StatCmd([]string{"-L", dangling})
	}); err == nil {
		t.Fatal("expected dangling symlink dereference error")
	}

}
