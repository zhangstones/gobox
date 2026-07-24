package disk

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureSha256Cmd(t *testing.T, stdin string, fn func() error) (string, error) {
	t.Helper()
	out, _, err := captureSha256CmdFull(t, stdin, fn)
	return out, err
}

func captureSha256CmdFull(t *testing.T, stdin string, fn func() error) (string, string, error) {
	t.Helper()
	oldOut := os.Stdout
	oldErr := os.Stderr
	oldIn := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	var rIn *os.File
	if stdin != "" {
		var wIn *os.File
		rIn, wIn, _ = os.Pipe()
		go func() {
			_, _ = io.WriteString(wIn, stdin)
			_ = wIn.Close()
		}()
		os.Stdin = rIn
	}
	os.Stdout = wOut
	os.Stderr = wErr
	err := fn()
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr
	os.Stdin = oldIn
	if rIn != nil {
		_ = rIn.Close()
	}
	var buf bytes.Buffer
	var errBuf bytes.Buffer
	_, _ = io.Copy(&buf, rOut)
	_, _ = io.Copy(&errBuf, rErr)
	return buf.String(), errBuf.String(), err
}

func TestSha256sumCmdHelpUsesMergedLongFlags(t *testing.T) {
	stdout, stderr, err := captureSha256CmdFull(t, "", func() error {
		return Sha256sumCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("sha256sum --help failed: %v", err)
	}
	out := stdout + stderr
	for _, want := range []string{"Usage: gobox sha256sum [OPTION]... [FILE]...", "Modes:", "Output:", "-c, --check", "-q, --quiet"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
}

func TestSha256sumDefaultAndCheck(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{file})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2cf24dba5fb0a30e") {
		t.Fatalf("unexpected sha256 output %q", out)
	}
	check := filepath.Join(dir, "check")
	if err := os.WriteFile(check, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	checkOut, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"-c", check})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(checkOut, "OK") {
		t.Fatalf("unexpected check output %q", checkOut)
	}
}

func TestSha256sumCmdOptionsStdin(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	out, err := captureSha256Cmd(t, "hello", func() error {
		return Sha256sumCmd(nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, helloSHA+"  -") {
		t.Fatalf("unexpected stdin output %q", out)
	}

}

func TestSha256sumCmdOptionsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := os.WriteFile("empty", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"empty"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "e3b0c44298fc1c149afbf4c8996fb924") || !strings.Contains(out, "  empty") {
		t.Fatalf("unexpected empty hash output %q", out)
	}

}

func TestSha256sumCmdOptionsTag(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	out, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"--tag", "file"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "SHA256 (file) = "+helloSHA {
		t.Fatalf("unexpected tag output %q", out)
	}

}

func TestSha256sumCmdOptionsTagMissingFileExitsOne(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	stdout, stderr, err := captureSha256CmdFull(t, "", func() error {
		return Sha256sumCmd([]string{"--tag", "missing-tag"})
	})
	if stdout != "" || !strings.Contains(stderr, "missing-tag") {
		t.Fatalf("unexpected tag missing output stdout=%q stderr=%q", stdout, stderr)
	}
	if exitErr, ok := err.(sha256sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected tag missing exit 1, got %T %v", err, err)
	}

}

func TestSha256sumCmdOptionsStdinTag(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	out, err := captureSha256Cmd(t, "hello", func() error {
		return Sha256sumCmd([]string{"--tag"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "SHA256 (stdin) = "+helloSHA {
		t.Fatalf("unexpected stdin tag output %q", out)
	}

}

func TestSha256sumCmdOptionsQuiet(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	out, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"-q", "file"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != helloSHA {
		t.Fatalf("unexpected quiet output %q", out)
	}

}

func TestSha256sumCmdOptionsQuietMissingFileExitsOneAndSuppressesStderr(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	stdout, stderr, err := captureSha256CmdFull(t, "", func() error {
		return Sha256sumCmd([]string{"-q", "missing-quiet"})
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("expected quiet missing silence, stdout=%q stderr=%q", stdout, stderr)
	}
	if exitErr, ok := err.(sha256sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected quiet missing exit 1, got %T %v", err, err)
	}

}

func TestSha256sumCmdOptionsStatusFailure(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := os.WriteFile("bad.check", []byte(strings.Repeat("0", 64)+"  file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "-s", "bad.check"})
	})
	if out != "" {
		t.Fatalf("expected status mode silence, got %q", out)
	}
	if exitErr, ok := err.(sha256sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected status failure exit 1, got %T %v", err, err)
	}

}

func TestSha256sumCmdOptionsStatusSuccessIsSilent(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := os.WriteFile("status-ok.check", []byte(helloSHA+"  file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := captureSha256CmdFull(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "-s", "status-ok.check"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected status success silence, stdout=%q stderr=%q", stdout, stderr)
	}

}

func TestSha256sumCmdOptionsCheckPartialFailureAggregatesExit(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := os.WriteFile("partial.check", []byte(helloSHA+"  file\n"+strings.Repeat("0", 64)+"  file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "partial.check"})
	})
	if !strings.Contains(out, "file: OK") || !strings.Contains(out, "file: FAILED") {
		t.Fatalf("unexpected partial check output %q", out)
	}
	if exitErr, ok := err.(sha256sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected partial failure exit 1, got %T %v", err, err)
	}

}

// TestSha256sumCmdOptionsCheckMismatchReportsFailedNotExitCode is a regression
// test for a bug where a mismatched checksum caused sha256sum -c to return an
// uninformative "sha256sum: exit code 1" style error instead of the same kind
// of "<file>: FAILED" summary error that md5sum -c produces on mismatch.
func TestSha256sumCmdOptionsCheckMismatchReportsFailedNotExitCode(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Mismatched checksum (all zeros) for "file".
	if err := os.WriteFile("mismatch.check", []byte(strings.Repeat("0", 64)+"  file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "mismatch.check"})
	})
	if !strings.Contains(out, "file: FAILED") {
		t.Fatalf("expected %q to contain \"file: FAILED\", got %q", out, out)
	}
	exitErr, ok := err.(sha256sumExitError)
	if !ok {
		t.Fatalf("expected sha256sumExitError, got %T %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.ExitCode())
	}
	if err.Error() == "exit code 1" {
		t.Fatalf("expected an informative mismatch message like md5sum's, got uninformative %q", err.Error())
	}
}

func TestSha256sumCmdOptionsWarnMalformed(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := os.WriteFile("malformed.check", []byte("not-a-check-line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := captureSha256CmdFull(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "-w", "malformed.check"})
	})
	if !strings.Contains(stderr, "improperly formatted") {
		t.Fatalf("expected malformed warning, got %q", stderr)
	}
	if exitErr, ok := err.(sha256sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected malformed check exit 1, got %T %v", err, err)
	}

}

func TestSha256sumCmdOptionsStatusMalformedIsSilent(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := os.WriteFile("status-malformed.check", []byte("not-a-check-line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := captureSha256CmdFull(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "-s", "status-malformed.check"})
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("expected status mode silence, stdout=%q stderr=%q", stdout, stderr)
	}
	if exitErr, ok := err.(sha256sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected malformed status exit 1, got %T %v", err, err)
	}

}

func TestSha256sumCmdOptionsMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := os.WriteFile("file2", []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"file", "file2"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "  file\n") || !strings.Contains(out, "  file2\n") {
		t.Fatalf("unexpected multi-file output %q", out)
	}

}

func TestSha256sumCmdOptionsCheckBsdTagFormat(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := os.WriteFile("tag.check", []byte("SHA256 (file) = "+helloSHA+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "tag.check"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "file: OK") {
		t.Fatalf("unexpected BSD check output %q", out)
	}

}

func TestSha256sumCmdOptionsQuietCheckSuppressesOk(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := os.WriteFile("ok.check", []byte(helloSHA+"  file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "-q", "ok.check"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected quiet check output to be empty, got %q", out)
	}

}

// TestSha256sumCmdOptionsQuietCheckStillReportsFailed is a regression test:
// GNU coreutils' --quiet only suppresses "OK" lines for successfully
// verified files; a FAILED checksum must still be printed even under
// --quiet, or a real verification failure would be silently swallowed.
func TestSha256sumCmdOptionsQuietCheckStillReportsFailed(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("mismatch-quiet.check", []byte(strings.Repeat("0", 64)+"  file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "-q", "mismatch-quiet.check"})
	})
	if !strings.Contains(out, "file: FAILED") {
		t.Fatalf("expected quiet check failure to still print FAILED, got %q", out)
	}
	exitErr, ok := err.(sha256sumExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected quiet check failure exit 1, got %T %v", err, err)
	}
}

func TestSha256sumCmdOptionsWarnIgnoresEmptyLines(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := os.WriteFile("empty-lines.check", []byte("\n"+helloSHA+"  file\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := captureSha256CmdFull(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "-w", "empty-lines.check"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" || !strings.Contains(stdout, "file: OK") {
		t.Fatalf("unexpected empty-line check stdout=%q stderr=%q", stdout, stderr)
	}

}

func TestSha256sumCmdOptionsInvalidChecksumHexExitsOne(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("invalid-hex.check", []byte(strings.Repeat("z", 64)+"  file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "invalid-hex.check"})
	})
	if exitErr, ok := err.(sha256sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected invalid hex exit 1, got %T %v", err, err)
	}

}

func TestSha256sumCmdOptionsMissingFileExitsOne(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = captureSha256Cmd(t, "", func() error {
		return Sha256sumCmd([]string{"missing"})
	})
	if exitErr, ok := err.(sha256sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected missing file exit 1, got %T %v", err, err)
	}

}

func TestSha256sumCmdOptionsCheckMissingChecksumFile(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := Sha256sumCmd([]string{"-c"}); err == nil {
		t.Fatal("expected check mode without file error")
	}

}

func TestSha256sumCmdOptionsCheckNonexistentChecksumFileExitsOne(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	stdout, stderr, err := captureSha256CmdFull(t, "", func() error {
		return Sha256sumCmd([]string{"-c", "missing.check"})
	})
	if stdout != "" || !strings.Contains(stderr, "missing.check") {
		t.Fatalf("unexpected missing checksum output stdout=%q stderr=%q", stdout, stderr)
	}
	if exitErr, ok := err.(sha256sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected missing checksum exit 1, got %T %v", err, err)
	}

}

func TestSha256sumCmdOptionsParseMalformedLines(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("file", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if _, _, ok := parseSHA256CheckLine("bad line"); ok {
		t.Fatal("expected malformed line to be rejected")
	}
	if sum, name, ok := parseSHA256CheckLine(helloSHA + " *file"); !ok || sum != helloSHA || name != "file" {
		t.Fatalf("unexpected parsed binary line sum=%q name=%q ok=%v", sum, name, ok)
	}

}
