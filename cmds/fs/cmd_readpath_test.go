package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadpathCmdOptionsDefaultRealpathSemantics(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{link})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != file {
		t.Fatalf("expected %q, got %q", file, out)
	}

}

func TestReadpathCmdOptionsCanonicalizeMissing(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	missing := filepath.Join(dir, "missing", "..", "new")
	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{"-m", missing})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != filepath.Join(dir, "new") {
		t.Fatalf("unexpected canonicalize-missing output %q", out)
	}

}

func TestReadpathCmdOptionsCanonicalizeExistingSymlink(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{"-f", link})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != file {
		t.Fatalf("unexpected canonicalized output %q", out)
	}

}

func TestReadpathCmdOptionsCanonicalizeMissingFinalComponent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{"-f", filepath.Join(dir, "new-file")})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != filepath.Join(dir, "new-file") {
		t.Fatalf("unexpected -f missing final component output %q", out)
	}

}

func TestReadpathCmdOptionsCanonicalizeMissingParentErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := captureFsCmdFull(t, func() error {
		return ReadpathCmd([]string{"-f", filepath.Join(dir, "missing-parent", "new-file")})
	})
	if err == nil {
		t.Fatal("expected missing parent error")
	}
	if stdout != "" {
		t.Fatalf("expected no stdout on missing parent error, got %q", stdout)
	}
	if !strings.Contains(stderr, "missing-parent") {
		t.Fatalf("expected stderr to mention missing parent, got %q", stderr)
	}

}

func TestReadpathCmdOptionsExistingPathRequired(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := captureFsCmdFull(t, func() error {
		return ReadpathCmd([]string{"-e", filepath.Join(dir, "missing")})
	})
	if err == nil {
		t.Fatal("expected -e missing path error")
	}
	if stdout != "" {
		t.Fatalf("expected no stdout for -e missing path, got %q", stdout)
	}
	if !strings.Contains(stderr, "missing") {
		t.Fatalf("expected stderr to mention missing path, got %q", stderr)
	}

}

func TestReadpathCmdOptionsCanonicalizeExistingSymlinkAndDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{"-e", link, dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Fields(out)
	if len(lines) != 2 || lines[0] != file || lines[1] != dir {
		t.Fatalf("unexpected -e output %q", out)
	}

}

func TestReadpathCmdOptionsCanonicalizeMissingCleansDotComponents(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{"-m", filepath.Join(dir, "a", ".", "b", "..", "c")})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != filepath.Join(dir, "a", "c") {
		t.Fatalf("unexpected cleaned path %q", out)
	}

}

func TestReadpathCmdOptionsReadlinkNonSymlinkErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := captureFsCmdFull(t, func() error {
		return ReadpathCmd([]string{"-l", file})
	})
	if err == nil {
		t.Fatal("expected non-symlink readlink error")
	}
	if stdout != "" {
		t.Fatalf("expected no stdout for non-symlink readlink, got %q", stdout)
	}
	if !strings.Contains(stderr, file) {
		t.Fatalf("expected stderr to mention target file, got %q", stderr)
	}

}

func TestReadpathCmdOptionsReadlinkNoNewline(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{"-l", "-n", link})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "target" {
		t.Fatalf("unexpected readlink -n output %q", out)
	}

}

func TestReadpathCmdOptionsZeroSeparated(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{"-z", file, link})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\x00") || !strings.HasSuffix(out, "\x00") {
		t.Fatalf("expected NUL separated output, got %q", out)
	}

}

func TestReadpathCmdOptionsZeroSeparatedSinglePath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{"-z", file})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != file+"\x00" {
		t.Fatalf("expected single NUL terminator, got %q", out)
	}

}

func TestReadpathCmdOptionsZeroSeparatedFailureDoesNotEmitFakeNul(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{"-z", file, filepath.Join(dir, "missing")})
	})
	if err == nil {
		t.Fatal("expected partial failure")
	}
	if out != file+"\x00" {
		t.Fatalf("expected only successful path NUL output, got %q", out)
	}

}

func TestReadpathCmdOptionsNoNewlineMultiplePathsStillSeparates(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	other := filepath.Join(dir, "other")
	if err := os.WriteFile(other, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{"-n", file, other})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != file+"\n"+other+"\n" {
		t.Fatalf("unexpected multi path -n output %q", out)
	}

}

func TestReadpathCmdOptionsQuietPartialFailurePreservesSuccessfulOutput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	out, err := captureFsCmd(t, func() error {
		return ReadpathCmd([]string{"-q", file, filepath.Join(dir, "missing")})
	})
	if err == nil {
		t.Fatal("expected partial failure error")
	}
	if strings.TrimSpace(out) != file {
		t.Fatalf("expected successful path output, got %q", out)
	}

}

func TestReadpathCmdOptionsQuietMissingSuppressesStderrPath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := captureFsCmdFull(t, func() error {
		return ReadpathCmd([]string{"-q", filepath.Join(dir, "missing")})
	})
	if err == nil {
		t.Fatal("expected missing path error")
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected quiet missing silence, stdout=%q stderr=%q", stdout, stderr)
	}

}

func TestReadpathCmdOptionsMissingOperand(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "target")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := captureFsCmdFull(t, func() error {
		return ReadpathCmd(nil)
	})
	if err == nil {
		t.Fatal("expected missing operand error")
	}
	if stdout != "" {
		t.Fatalf("expected no stdout for missing operand, got %q", stdout)
	}
	if !strings.Contains(err.Error(), "missing operand") && !strings.Contains(stderr, "missing operand") {
		t.Fatalf("expected missing operand diagnostic, stderr=%q err=%v", stderr, err)
	}

}
