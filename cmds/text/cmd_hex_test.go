package text

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHexEncodeDecode(t *testing.T) {
	out, err := captureTextCmd(t, "hello", func() error {
		return HexCmd([]string{"--encode"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "68656c6c6f" {
		t.Fatalf("unexpected hex output %q", out)
	}
	decoded, err := captureTextCmd(t, out, func() error {
		return HexCmd([]string{"--decode"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "hello" {
		t.Fatalf("unexpected decoded output %q", decoded)
	}
}

func TestHexCmdOptionsDumpOffsetAndLength(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--dump", "-s", "2", "-n", "3", input})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "00000002  63 64 65                                          |cde|\n00000005\n" {
		t.Fatalf("unexpected dump output %q", out)
	}

}

func TestHexCmdOptionsDumpCanonicalShortLine(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--dump", "-C", input})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "00000000  61 62 63 64 65 66                                 |abcdef|\n00000006\n" {
		t.Fatalf("unexpected canonical dump %q", out)
	}

}

func TestHexCmdOptionsDumpZeroLength(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--dump", "-n", "0", input})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "00000000" {
		t.Fatalf("unexpected zero length dump %q", out)
	}

}

func TestHexCmdOptionsFormatHex(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--dump", "-e", "%02x", input})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "616263646566" {
		t.Fatalf("unexpected format hex output %q", out)
	}

}

func TestHexCmdOptionsFormatDecimal(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--dump", "-n", "3", "-e", "%u", input})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "97 98 99" {
		t.Fatalf("unexpected decimal output %q", out)
	}

}

func TestHexCmdOptionsVerboseRepeatedContent(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	repeated := filepath.Join(dir, "repeated")
	if err := os.WriteFile(repeated, bytes.Repeat([]byte("a"), 32), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--dump", "-v", repeated})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "|aaaaaaaaaaaaaaaa|") != 2 {
		t.Fatalf("expected repeated rows without folding, got %q", out)
	}

}

func TestHexCmdOptionsDecodeOutputFile(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(dir, "decoded")
	stdout, err := captureTextCmd(t, "6869", func() error {
		return HexCmd([]string{"--decode", "-o", outFile})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hi" {
		t.Fatalf("unexpected decoded file %q", data)
	}

}

func TestHexCmdOptionsDecodeIgnoresWhitespace(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "68 65\n6c6c6f", func() error {
		return HexCmd([]string{"--decode"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Fatalf("unexpected whitespace hex decode %q", out)
	}

}

func TestHexCmdOptionsEncodeEmptyInput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--encode"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "\n" {
		t.Fatalf("expected empty hex line, got %q", out)
	}

}

func TestHexCmdOptionsMissingInputFile(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--encode", filepath.Join(dir, "missing")})
	}); err == nil {
		t.Fatal("expected missing input error")
	}

}

func TestHexCmdOptionsModeRequired(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{input})
	}); err == nil {
		t.Fatal("expected missing mode error")
	}

}

func TestHexCmdOptionsMultipleModesRejected(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--encode", "--decode", input})
	}); err == nil {
		t.Fatal("expected multiple mode error")
	}

}

func TestHexCmdOptionsOffsetBeyondEof(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--dump", "-s", "100", input})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "00000064" {
		t.Fatalf("unexpected beyond EOF dump %q", out)
	}

}

func TestHexCmdOptionsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--dump", "-e", "%04x", input})
	}); err == nil {
		t.Fatal("expected unsupported format error")
	}

}

func TestHexCmdOptionsInvalidNumericFlags(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--dump", "-n", "bad", input})
	}); err == nil {
		t.Fatal("expected invalid length flag error")
	}
	if _, err := captureTextCmd(t, "", func() error {
		return HexCmd([]string{"--dump", "-s", "bad", input})
	}); err == nil {
		t.Fatal("expected invalid offset flag error")
	}

}

func TestHexCmdOptionsInvalidDecode(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bin")
	if err := os.WriteFile(input, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "abc", func() error {
		return HexCmd([]string{"--decode"})
	}); err == nil {
		t.Fatal("expected invalid hex decode error")
	}

}
