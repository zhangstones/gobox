package text

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBase64EncodeDecode(t *testing.T) {
	out, err := captureTextCmd(t, "hello", func() error {
		return Base64Cmd([]string{"-w", "0"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "aGVsbG8=" {
		t.Fatalf("unexpected encoded output %q", out)
	}
	decoded, err := captureTextCmd(t, out, func() error {
		return Base64Cmd([]string{"-d"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "hello" {
		t.Fatalf("unexpected decoded output %q", decoded)
	}
}

func TestBase64CmdOptionsWrapOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return Base64Cmd([]string{"-w", "4", input})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "aGVs\nbG8g\nd29y\nbGQ=\n" {
		t.Fatalf("unexpected wrapped output %q", out)
	}

}

func TestBase64CmdOptionsLongOptionAliases(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "aG!!Vs", func() error {
		return Base64Cmd([]string{"--decode", "--ignore-garbage", "--wrap", "0"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hel" {
		t.Fatalf("unexpected long option output %q", out)
	}

}

func TestBase64CmdOptionsIgnoreGarbage(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "aG!!Vs\nbG8=", func() error {
		return Base64Cmd([]string{"-d", "-i"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Fatalf("unexpected ignored-garbage decode %q", out)
	}

}

func TestBase64CmdOptionsDecodeWhitespace(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "aG Vs\nbG8=", func() error {
		return Base64Cmd([]string{"-d"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Fatalf("unexpected whitespace decode %q", out)
	}

}

func TestBase64CmdOptionsGarbageWithoutIgnoreFails(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "aG!!Vs", func() error {
		return Base64Cmd([]string{"-d"})
	}); err == nil {
		t.Fatal("expected dirty decode to fail without -i")
	}

}

func TestBase64CmdOptionsOutputFile(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(dir, "out.b64")
	out, err := captureTextCmd(t, "", func() error {
		return Base64Cmd([]string{"-w", "0", "-o", outFile, input})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected no stdout for -o, got %q", out)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "aGVsbG8gd29ybGQ=" {
		t.Fatalf("unexpected output file %q", data)
	}

}

func TestBase64CmdOptionsOutputFileOverwrites(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(dir, "overwrite.b64")
	if err := os.WriteFile(outFile, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := captureTextCmd(t, "hi", func() error {
		return Base64Cmd([]string{"-w", "0", "-o", outFile})
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "aGk=" {
		t.Fatalf("expected overwritten file, got %q", data)
	}

}

func TestBase64CmdOptionsOutputPathDirectoryErrors(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := captureTextCmdFull(t, "hi", func() error {
		return Base64Cmd([]string{"-o", dir})
	})
	if err == nil {
		t.Fatal("expected output directory error")
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected no output on create error, stdout=%q stderr=%q", stdout, stderr)
	}

}

func TestBase64CmdOptionsInvalidWrapFlag(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return Base64Cmd([]string{"-w", "bad", input})
	}); err == nil {
		t.Fatal("expected invalid wrap flag error")
	}

}

func TestBase64CmdOptionsInvalidDecode(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "!!!!", func() error {
		return Base64Cmd([]string{"-d"})
	}); err == nil {
		t.Fatal("expected invalid base64 error")
	}

}

func TestBase64CmdOptionsMultipleFilesConcatenate(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("!"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureTextCmd(t, "", func() error {
		return Base64Cmd([]string{"-w", "0", a, b})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "aGkh" {
		t.Fatalf("unexpected multi-file output %q", out)
	}

}

func TestBase64CmdOptionsEmptyInput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return Base64Cmd([]string{"-w", "0"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}

}

func TestBase64CmdOptionsMissingFile(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input")
	if err := os.WriteFile(input, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return Base64Cmd([]string{filepath.Join(dir, "missing")})
	}); err == nil {
		t.Fatal("expected missing file error")
	}

}
