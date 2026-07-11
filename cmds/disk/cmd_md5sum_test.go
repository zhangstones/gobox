package disk

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runMd5sumCmdFull runs Md5sumCmd and captures stdout and stderr separately.
// If dir is provided, the command will be executed in that directory.
func runMd5sumCmdFull(args []string, dir string) (string, string, error) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	var oldDir string
	var err error
	if dir != "" {
		oldDir, err = os.Getwd()
		if err != nil {
			return "", "", err
		}
		os.Chdir(dir)
	}

	err = Md5sumCmd(args)

	if dir != "" {
		os.Chdir(oldDir)
	}

	wOut.Close()
	wErr.Close()
	io.Copy(&outBuf, rOut)
	io.Copy(&errBuf, rErr)
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	return outBuf.String(), errBuf.String(), err
}

func runMd5sumCmd(args []string, dir string) (string, error) {
	stdout, stderr, err := runMd5sumCmdFull(args, dir)
	return stdout + stderr, err
}

// runMd5sumCmdWithStdinFull runs Md5sumCmd with stdin input and captures stdout/stderr separately.
// If dir is provided, the command will be executed in that directory.
func runMd5sumCmdWithStdinFull(args []string, stdinInput string, dir string) (string, string, error) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr
	os.Stdin = rIn

	var oldDir string
	if dir != "" {
		oldDir, _ = os.Getwd()
		os.Chdir(dir)
	}

	go func() {
		wIn.WriteString(stdinInput)
		wIn.Close()
	}()

	err := Md5sumCmd(args)

	if dir != "" {
		os.Chdir(oldDir)
	}

	wOut.Close()
	wErr.Close()
	io.Copy(&outBuf, rOut)
	io.Copy(&errBuf, rErr)
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	os.Stdin = oldStdin
	return outBuf.String(), errBuf.String(), err
}

// runMd5sumCmdWithStdin runs Md5sumCmd with stdin input and captures combined output.
func runMd5sumCmdWithStdin(args []string, stdinInput string, dir string) (string, error) {
	stdout, stderr, err := runMd5sumCmdWithStdinFull(args, stdinInput, dir)
	return stdout + stderr, err
}

// ============== NORMAL CASES TESTS ==============

func TestMd5sumCmdSingleFile(t *testing.T) {
	// Create a temp file with known content
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	content := "hello world"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	output, err := runMd5sumCmd([]string{testFile}, "")
	if err != nil {
		t.Fatalf("md5sum command failed: %v, output: %s", err, output)
	}

	result := strings.TrimSpace(string(output))
	// POSIX format: <hash> <filename>
	parts := strings.Fields(result)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d: %s", len(parts), result)
	}
	hash := parts[0]
	filename := parts[1]

	// Verify hash is 32 hex characters (MD5)
	if len(hash) != 32 {
		t.Errorf("expected 32-char MD5 hash, got %d chars: %s", len(hash), hash)
	}
	if !strings.HasSuffix(filename, "test.txt") {
		t.Errorf("expected filename to end with test.txt, got: %s", filename)
	}

	// Verify the hash is correct for "hello world"
	// MD5 of "hello world" is 5eb63bbbe01eeed093cb22bb8f5acdc3
	if hash != "5eb63bbbe01eeed093cb22bb8f5acdc3" {
		t.Errorf("unexpected hash: %s (expected 5eb63bbbe01eeed093cb22bb8f5acdc3)", hash)
	}
}

func TestMd5sumCmdMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("write file2: %v", err)
	}

	output, err := runMd5sumCmd([]string{file1, file2}, "")
	if err != nil {
		t.Fatalf("md5sum multiple files failed: %v, output: %s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d: %s", len(lines), output)
	}

	// Both lines should have hash and filename
	for i, line := range lines {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			t.Errorf("line %d: expected 2 parts, got %d: %s", i, len(parts), line)
		}
	}
}

func TestMd5sumCmdStdin(t *testing.T) {
	output, err := runMd5sumCmdWithStdin([]string{}, "hello world", "")
	if err != nil {
		t.Fatalf("md5sum stdin failed: %v, output: %s", err, output)
	}

	result := strings.TrimSpace(string(output))
	parts := strings.Fields(result)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d: %s", len(parts), result)
	}

	hash := parts[0]
	// For stdin, filename is "-"
	if parts[1] != "-" {
		t.Errorf("expected stdin filename '-', got: %s", parts[1])
	}

	// Verify the hash
	if hash != "5eb63bbbe01eeed093cb22bb8f5acdc3" {
		t.Errorf("unexpected hash: %s (expected 5eb63bbbe01eeed093cb22bb8f5acdc3)", hash)
	}
}

func TestMd5sumCmdBSDTagStyle(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	content := "hello world"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	output, err := runMd5sumCmd([]string{"--tag", testFile}, "")
	if err != nil {
		t.Fatalf("md5sum --tag failed: %v, output: %s", err, output)
	}

	result := strings.TrimSpace(string(output))
	// BSD style: MD5 (file) = hash
	if !strings.HasPrefix(result, "MD5 (") {
		t.Errorf("expected BSD style output to start with 'MD5 (', got: %s", result)
	}
	if !strings.Contains(result, ") = ") {
		t.Errorf("expected BSD style output to contain ') = ', got: %s", result)
	}
	if !strings.HasSuffix(result, "5eb63bbbe01eeed093cb22bb8f5acdc3") {
		t.Errorf("expected BSD style output to end with hash, got: %s", result)
	}
}

func TestMd5sumCmdStdinBSDTag(t *testing.T) {
	output, err := runMd5sumCmdWithStdin([]string{"--tag"}, "hello world", "")
	if err != nil {
		t.Fatalf("md5sum --tag stdin failed: %v, output: %s", err, output)
	}

	result := strings.TrimSpace(string(output))
	// BSD style for stdin: MD5 (stdin) = hash
	if !strings.Contains(result, "MD5 (stdin)") {
		t.Errorf("expected BSD style stdin output to contain 'MD5 (stdin)', got: %s", result)
	}
	if !strings.Contains(result, "= 5eb63bbbe01eeed093cb22bb8f5acdc3") {
		t.Errorf("expected BSD style stdin output to end with hash, got: %s", result)
	}
}

// ============== EDGE CASES TESTS ==============

func TestMd5sumCmdEmptyFile(t *testing.T) {
	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	output, err := runMd5sumCmd([]string{emptyFile}, "")
	if err != nil {
		t.Fatalf("md5sum empty file failed: %v, output: %s", err, output)
	}

	result := strings.TrimSpace(string(output))
	parts := strings.Fields(result)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d: %s", len(parts), result)
	}

	// MD5 of empty string is d41d8cd98f00b204e9800998ecf8427e
	if parts[0] != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Errorf("unexpected hash for empty file: %s (expected d41d8cd98f00b204e9800998ecf8427e)", parts[0])
	}
}

func TestMd5sumCmdLargeFile(t *testing.T) {
	dir := t.TempDir()
	largeFile := filepath.Join(dir, "large.txt")
	// Create a 1MB file with repeated content
	largeContent := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 40000) // ~1MB
	if err := os.WriteFile(largeFile, []byte(largeContent), 0644); err != nil {
		t.Fatalf("write large file: %v", err)
	}

	output, err := runMd5sumCmd([]string{largeFile}, "")
	if err != nil {
		t.Fatalf("md5sum large file failed: %v, output: %s", err, output)
	}

	result := strings.TrimSpace(string(output))
	parts := strings.Fields(result)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d: %s", len(parts), result)
	}

	// Hash should be 32 hex characters
	if len(parts[0]) != 32 {
		t.Errorf("expected 32-char MD5 hash, got %d chars: %s", len(parts[0]), parts[0])
	}
}

func TestMd5sumCmdBinaryContent(t *testing.T) {
	dir := t.TempDir()
	binaryFile := filepath.Join(dir, "binary.dat")
	// Create binary content with null bytes and high bytes
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD, 0x00, 0x80, 0x90}
	if err := os.WriteFile(binaryFile, binaryContent, 0644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}

	output, err := runMd5sumCmd([]string{binaryFile}, "")
	if err != nil {
		t.Fatalf("md5sum binary file failed: %v, output: %s", err, output)
	}

	result := strings.TrimSpace(string(output))
	parts := strings.Fields(result)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d: %s", len(parts), result)
	}

	// Verify hash is valid 32-char MD5
	hash := parts[0]
	if len(hash) != 32 {
		t.Errorf("expected 32-char MD5 hash, got %d chars: %s", len(hash), hash)
	}
	// Verify it's all hex characters
	for _, c := range hash {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("hash contains non-hex char: %c", c)
		}
	}

	// Also verify by computing expected hash
	expectedHash := fmt.Sprintf("%x", md5.Sum(binaryContent))
	if hash != expectedHash {
		t.Errorf("hash mismatch: got %s, expected %s", hash, expectedHash)
	}
}

func TestMd5sumCmdSpecialCharacters(t *testing.T) {
	dir := t.TempDir()
	specialFile := filepath.Join(dir, "special.txt")
	// Content with special chars
	specialContent := "hello\tworld\nspecial: !@#$%^&*()\n"
	if err := os.WriteFile(specialFile, []byte(specialContent), 0644); err != nil {
		t.Fatalf("write special file: %v", err)
	}

	output, err := runMd5sumCmd([]string{specialFile}, "")
	if err != nil {
		t.Fatalf("md5sum special chars file failed: %v, output: %s", err, output)
	}

	result := strings.TrimSpace(string(output))
	parts := strings.Fields(result)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d: %s", len(parts), result)
	}

	// Hash should be valid hex
	hash := parts[0]
	for _, c := range hash {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			t.Errorf("hash contains non-hex char: %c", c)
		}
	}
}

// ============== ERROR CASES TESTS ==============

func TestMd5sumCmdNonExistentFile(t *testing.T) {
	output, _ := runMd5sumCmd([]string{"/nonexistent/file/path.txt"}, "")
	// Command doesn't error out, it writes to stderr and continues
	// So we check stderr for the error message
	if !strings.Contains(output, "no such file or directory") && !strings.Contains(output, " nonexistent") {
		t.Errorf("expected 'no such file or directory' in stderr for non-existent file, got: %s", output)
	}
}

func TestMd5sumCmdPermissionDenied(t *testing.T) {
	dir := t.TempDir()
	restrictedFile := filepath.Join(dir, "restricted.txt")
	if err := os.WriteFile(restrictedFile, []byte("secret"), 0600); err != nil {
		t.Fatalf("write restricted file: %v", err)
	}

	// Remove read permission
	if err := os.Chmod(restrictedFile, 0000); err != nil {
		t.Fatalf("chmod file: %v", err)
	}
	// Ensure we restore permissions for cleanup
	defer os.Chmod(restrictedFile, 0644)

	output, _ := runMd5sumCmd([]string{restrictedFile}, "")
	// Command doesn't error out, it writes to stderr and continues
	if !strings.Contains(output, "permission denied") && !strings.Contains(output, restrictedFile) {
		t.Errorf("expected 'permission denied' in stderr, got: %s", output)
	}
}

func TestMd5sumCmdNoFiles(t *testing.T) {
	// When no files and no stdin, should show usage and error
	output, _ := runMd5sumCmd([]string{}, "")
	// Should error (no files specified) - but still produces output
	if !strings.Contains(output, "Usage:") {
		t.Errorf("expected usage information, got: %s", output)
	}
}

// ============== VERIFICATION MODE TESTS ==============

func TestMd5sumCmdCheckValid(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	checkFile := filepath.Join(dir, "test.txt.md5")

	// Create test file
	content := "hello world"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Create valid checksum file (POSIX format) with relative path
	// MD5 of "hello world" is 5eb63bbbe01eeed093cb22bb8f5acdc3
	checksumContent := "5eb63bbbe01eeed093cb22bb8f5acdc3  test.txt\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	// Use cmd.Dir to run command in the directory so relative paths work
	output, err := runMd5sumCmd([]string{"-c", "test.txt.md5"}, dir)
	if err != nil {
		t.Fatalf("md5sum -c failed: %v, output: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "test.txt: OK") {
		t.Errorf("expected 'test.txt: OK' in output, got: %s", result)
	}
}

func TestMd5sumCmdCheckInvalid(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	checkFile := filepath.Join(dir, "test.txt.md5")

	// Create test file
	content := "hello world"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Create checksum file with WRONG hash
	checksumContent := "00000000000000000000000000000000  test.txt\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	// Use cmd.Dir to run command in the directory
	output, err := runMd5sumCmd([]string{"-c", "test.txt.md5"}, dir)
	// Should not error (verification errors don't cause non-zero exit by default)
	_ = output
	_ = err

	result := string(output)
	if !strings.Contains(result, "test.txt: FAILED") {
		t.Errorf("expected 'test.txt: FAILED' in output, got: %s", result)
	}
}

func TestMd5sumCmdCheckBSDFormat(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	checkFile := filepath.Join(dir, "test.txt.md5")

	// Create test file
	content := "hello world"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Create checksum file in BSD format
	checksumContent := "MD5 (test.txt) = 5eb63bbbe01eeed093cb22bb8f5acdc3\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	// Use cmd.Dir to run command in the directory
	output, err := runMd5sumCmd([]string{"-c", "test.txt.md5"}, dir)
	if err != nil {
		t.Fatalf("md5sum -c BSD format failed: %v, output: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "test.txt: OK") {
		t.Errorf("expected 'test.txt: OK' in output, got: %s", result)
	}
}

func TestMd5sumCmdCheckMalformedLine(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	checkFile := filepath.Join(dir, "test.txt.md5")

	// Create test file
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Create checksum file with malformed line (only hash, no filename)
	checksumContent := "5eb63bbbe01eeed093cb22bb8f5acdc3\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	stdout, stderr, err := runMd5sumCmdFull([]string{"-c", "-w", "test.txt.md5"}, dir)
	// A hash-only line (no filename) is skipped entirely rather than
	// checked, so with -w it must warn on stderr, produce no OK/FAILED
	// line on stdout for it, and exit non-zero (checksum verification
	// found nothing valid to confirm).
	if err == nil {
		t.Fatalf("expected a malformed checksum line to cause a non-zero exit, got success")
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no OK/FAILED line for a line with no filename, got stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "improperly formatted checksum line") {
		t.Fatalf("expected a malformed-line warning on stderr, got: %q", stderr)
	}
}

func TestMd5sumCmdCheckMissingFile(t *testing.T) {
	dir := t.TempDir()
	checkFile := filepath.Join(dir, "missing.md5")

	// Create checksum file referencing non-existent file
	checksumContent := "5eb63bbbe01eeed093cb22bb8f5acdc3  nonexistent.txt\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	// Use cmd.Dir to run command in the directory
	output, err := runMd5sumCmd([]string{"-c", "missing.md5"}, dir)
	// Should complete but report the missing file
	_ = output
	_ = err
	// Verify error message is in output
	if !strings.Contains(output, "nonexistent.txt") || (!strings.Contains(output, "no such file") && !strings.Contains(output, "nonexistent")) {
		t.Errorf("expected error about missing file, got: %s", output)
	}
}

func TestMd5sumCmdCheckEmptyFile(t *testing.T) {
	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "empty.txt")
	checkFile := filepath.Join(dir, "empty.md5")

	// Create empty test file
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	// Create checksum file for empty file
	// MD5 of empty string is d41d8cd98f00b204e9800998ecf8427e
	checksumContent := "d41d8cd98f00b204e9800998ecf8427e  empty.txt\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	// Use cmd.Dir to run command in the directory
	output, err := runMd5sumCmd([]string{"-c", "empty.md5"}, dir)
	if err != nil {
		t.Fatalf("md5sum -c empty file failed: %v, output: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "empty.txt: OK") {
		t.Errorf("expected 'empty.txt: OK' in output, got: %s", result)
	}
}

// ============== QUIET MODE TESTS ==============

func TestMd5sumCmdQuietMode(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Per GNU coreutils, --quiet is only meaningful in -c (check) mode.
// In compute mode it must exit 1 with an error message.
	stdout, _, err := runMd5sumCmdFull([]string{"-q", testFile}, "")
	if err == nil {
		t.Fatalf("md5sum -q in compute mode should fail, got nil error")
	}
	if !strings.Contains(err.Error(), "--quiet") {
		t.Errorf("expected error mentioning --quiet, got: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected no stdout on -q in compute mode, got %q", stdout)
	}
}

func TestMd5sumCmdQuietModeNonExistent(t *testing.T) {
	// Per GNU coreutils, --quiet is only meaningful in -c (check) mode.
	// In compute mode it must exit 1 regardless of file existence.
	_, _, err := runMd5sumCmdFull([]string{"-q", "/nonexistent/file.txt"}, "")
	if err == nil {
		t.Fatalf("md5sum -q in compute mode should fail, got nil error")
	}
	if !strings.Contains(err.Error(), "--quiet") {
		t.Errorf("expected error mentioning --quiet, got: %v", err)
	}
}

func TestMd5sumCmdCheckQuietMode(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	checkFile := filepath.Join(dir, "test.txt.md5")

	// Create test file
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Create valid checksum file
	checksumContent := "5eb63bbbe01eeed093cb22bb8f5acdc3  test.txt\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	// Use cmd.Dir to run command in the directory
	output, err := runMd5sumCmd([]string{"-c", "-q", "test.txt.md5"}, dir)
	if err != nil {
		t.Fatalf("md5sum -c -q failed: %v, output: %s", err, output)
	}

	result := string(output)
	// In quiet mode with valid checksum, should produce no output
	if result != "" {
		t.Errorf("expected no output in quiet mode with valid checksum, got: %s", result)
	}
}

func TestMd5sumCmdCheckQuietModeInvalid(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	checkFile := filepath.Join(dir, "test.txt.md5")

	// Create test file
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Create invalid checksum file
	checksumContent := "00000000000000000000000000000000  test.txt\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	// Use cmd.Dir to run command in the directory
	stdout, stderr, err := runMd5sumCmdFull([]string{"-c", "-q", "test.txt.md5"}, dir)
	if stdout != "" || stderr != "" {
		t.Fatalf("expected quiet check failure to stay silent, stdout=%q stderr=%q", stdout, stderr)
	}
	if exitErr, ok := err.(md5sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected quiet check failure exit 1, got %T %v", err, err)
	}
}

// ============== STATUS MODE TESTS ==============

func TestMd5sumCmdCheckStatusMode(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	checkFile := filepath.Join(dir, "test.txt.md5")

	// Create test file
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Create valid checksum file
	checksumContent := "5eb63bbbe01eeed093cb22bb8f5acdc3  test.txt\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	// Use cmd.Dir to run command in the directory
	// -s (status) mode per GNU coreutils must suppress all output (stdout and stderr),
	// only the exit code conveys success/failure.
	output, err := runMd5sumCmd([]string{"-c", "-s", "test.txt.md5"}, dir)
	if err != nil {
		t.Fatalf("md5sum -c -s failed: %v, output: %s", err, output)
	}

	if len(output) != 0 {
		t.Errorf("--status must produce no output, got: %s", output)
	}
}

func TestMd5sumCmdCheckStatusModeFailure(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	checkFile := filepath.Join(dir, "test.txt.md5")

	// Create test file
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Create invalid checksum file
	checksumContent := "00000000000000000000000000000000  test.txt\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	// Use cmd.Dir to run command in the directory
	stdout, stderr, err := runMd5sumCmdFull([]string{"-c", "-s", "test.txt.md5"}, dir)
	if stdout != "" {
		t.Fatalf("--status must produce no stdout on FAILED, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for checksum mismatch, got %q", stderr)
	}
	if exitErr, ok := err.(md5sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected status-mode failure exit 1, got %T %v", err, err)
	}
}

// ============== WARN MODE TESTS ==============

func TestMd5sumCmdCheckWarnMode(t *testing.T) {
	dir := t.TempDir()
	checkFile := filepath.Join(dir, "malformed.md5")

	// Create checksum file with malformed lines
	checksumContent := "notavalidline\n5eb63bbbe01eeed093cb22bb8f5acdc3\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	// Use cmd.Dir to run command in the directory
	stdout, stderr, err := runMd5sumCmdFull([]string{"-c", "-w", "malformed.md5"}, dir)
	if stdout != "" {
		t.Fatalf("expected no stdout for malformed warn-only input, got %q", stdout)
	}
	if !strings.Contains(stderr, "improperly formatted checksum line") {
		t.Fatalf("expected malformed warning, got stderr=%q", stderr)
	}
	if exitErr, ok := err.(md5sumExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected warn mode malformed exit 1, got %T %v", err, err)
	}
}

// ============== HELP FLAG TESTS ==============

func TestMd5sumCmdHelp(t *testing.T) {
	output, err := runMd5sumCmd([]string{"--help"}, "")
	// --help does NOT cause an error because flag.ContinueOnError is used
	// and the code explicitly handles flag.ErrHelp by returning nil
	if err != nil {
		t.Fatalf("md5sum --help should not error, got: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage:") {
		t.Errorf("expected usage information, got: %s", result)
	}
	if !strings.Contains(result, "md5sum") {
		t.Errorf("expected 'md5sum' in help output, got: %s", result)
	}
}

// ============== MULTIPLE FILES IN CHECK MODE ==============

func TestMd5sumCmdCheckMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")
	checkFile := filepath.Join(dir, "check.md5")

	// Create test files
	if err := os.WriteFile(file1, []byte("hello"), 0644); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("world"), 0644); err != nil {
		t.Fatalf("write file2: %v", err)
	}

	// MD5 of "hello" is 5d41402abc4b2a76b9719d911017c592
	// MD5 of "world" is 7d793037a0760186574b0282f2f435e7
	checksumContent := "5d41402abc4b2a76b9719d911017c592  file1.txt\n7d793037a0760186574b0282f2f435e7  file2.txt\n"
	if err := os.WriteFile(checkFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("write checksum file: %v", err)
	}

	// Use cmd.Dir to run command in the directory
	output, err := runMd5sumCmd([]string{"-c", "check.md5"}, dir)
	if err != nil {
		t.Fatalf("md5sum -c multiple files failed: %v, output: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "file1.txt: OK") {
		t.Errorf("expected 'file1.txt: OK' in output, got: %s", result)
	}
	if !strings.Contains(result, "file2.txt: OK") {
		t.Errorf("expected 'file2.txt: OK' in output, got: %s", result)
	}
}

// ============== COMBINED OPTIONS TESTS ==============

func TestMd5sumCmdTagAndQuietTogether(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Per GNU coreutils, --quiet is only meaningful in -c (check) mode.
	// In compute mode it must exit 1 with an error message.
	_, err := runMd5sumCmd([]string{"--tag", "-q", testFile}, "")
	if err == nil {
		t.Fatalf("md5sum --tag -q in compute mode should fail, got nil error")
	}
	if !strings.Contains(err.Error(), "--quiet") {
		t.Errorf("expected error mentioning --quiet, got: %v", err)
	}
}

// ============== STDIN IN CHECK MODE ==============

// TestMd5sumCmdCheckModeIgnoredWithStdinAndNoFiles documents the actual
// (surprising) current contract: with no file arguments, cmd_md5sum.go's
// "no files" branch checks for stdin data *before* checking checkMode, so
// `-c` is silently ignored and stdin is treated as compute-mode input --
// it does not read a checksum list from stdin, and it does not error about
// a missing file either. This was previously asserted the opposite way
// (expected to error) and used t.Log instead of a real assertion, so the
// mismatch between the comment and the real behavior was never caught.
// TestMd5sumCmdCheckModeReadsChecksumListFromStdinWithNoFiles is a
// regression test: -c with no file operands previously ignored -c entirely
// and computed the MD5 of the stdin bytes themselves (falling into compute
// mode). GNU md5sum instead reads the checksum *list* from stdin in this
// case, matching -c FILE behavior with FILE replaced by stdin.
func TestMd5sumCmdCheckModeReadsChecksumListFromStdinWithNoFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	sum := fmt.Sprintf("%x", md5.Sum([]byte("hello")))
	checksumList := sum + "  target.txt\n"

	stdout, _, err := runMd5sumCmdWithStdinFull([]string{"-c"}, checksumList, dir)
	if err != nil {
		t.Fatalf("expected -c to successfully verify target.txt via stdin checksum list, got: %v", err)
	}
	want := "target.txt: OK\n"
	if stdout != want {
		t.Fatalf("expected %q, got %q", want, stdout)
	}
}

// TestMd5sumCmdCheckModeStdinDetectsMismatch confirms the stdin-checksum-list
// path (added above) actually verifies content rather than always reporting
// OK.
func TestMd5sumCmdCheckModeStdinDetectsMismatch(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	wrongSum := fmt.Sprintf("%x", md5.Sum([]byte("not hello")))
	checksumList := wrongSum + "  target.txt\n"

	stdout, _, err := runMd5sumCmdWithStdinFull([]string{"-c"}, checksumList, dir)
	if err == nil {
		t.Fatalf("expected a checksum mismatch via stdin list to fail, got success with stdout %q", stdout)
	}
	want := "target.txt: FAILED\n"
	if stdout != want {
		t.Fatalf("expected %q, got %q", want, stdout)
	}
}

// TestMd5sumCmdCheckModeWithoutStdinDataStillErrors preserves the original
// "no files, no stdin" contract: -c should not hang waiting on a stdin that
// was never provided data, it should print usage and error immediately.
func TestMd5sumCmdCheckModeWithoutStdinDataStillErrors(t *testing.T) {
	oldStdin := os.Stdin
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()
	os.Stdin = devNull
	defer func() { os.Stdin = oldStdin }()

	if err := Md5sumCmd([]string{"-c"}); err == nil {
		t.Fatalf("expected -c with no files and no stdin data to error")
	}
}
