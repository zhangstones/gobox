package fs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
