package text

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStringsExtractsPrintableText(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	if err := os.WriteFile(file, []byte{0, 'h', 'e', 'l', 'l', 'o', 0}, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{"-n", "5", file})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Fatalf("unexpected strings output %q", out)
	}
}

func TestStringsCmdOptionsDefaultMinimum(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{file})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Fatalf("unexpected default strings output %q", out)
	}

}

func TestStringsCmdOptionsFlushesStringAtEof(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	eofFile := filepath.Join(dir, "eof")
	if err := os.WriteFile(eofFile, []byte{0, 'e', 'n', 'd', 'i', 'n', 'g'}, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{"-n", "6", eofFile})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "ending" {
		t.Fatalf("unexpected EOF string output %q", out)
	}

}

func TestStringsCmdOptionsFilenamePrefix(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{"-n", "3", "-f", file})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, file+": abc") || !strings.Contains(out, file+": hello") {
		t.Fatalf("unexpected filename-prefixed output %q", out)
	}

}

func TestStringsCmdOptionsMultipleFilePrefixes(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	other := filepath.Join(dir, "other")
	if err := os.WriteFile(other, []byte{0, 'w', 'o', 'r', 'l', 'd', 0}, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{"-n", "5", "-f", file, other})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, file+": hello") || !strings.Contains(out, other+": world") {
		t.Fatalf("unexpected multi-file prefix output %q", out)
	}

}

func TestStringsCmdOptionsOffsetDecimal(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{"-n", "5", "-t", "d", file})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "5 hello") {
		t.Fatalf("unexpected decimal offset output %q", out)
	}

}

func TestStringsCmdOptionsStdin(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "\x00stdin-text\x00", func() error {
		return StringsCmd([]string{"-n", "5"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "stdin-text" {
		t.Fatalf("unexpected stdin strings output %q", out)
	}

}

func TestStringsCmdOptionsOffsetOctalAndHex(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	offsetFile := filepath.Join(dir, "offset")
	if err := os.WriteFile(offsetFile, append(bytes.Repeat([]byte{0}, 16), []byte("hello")...), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{"-n", "5", "-t", "x", offsetFile})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "10 hello") {
		t.Fatalf("unexpected hex offset output %q", out)
	}
	out, err = captureTextCmd(t, "", func() error {
		return StringsCmd([]string{"-n", "5", "-t", "o", offsetFile})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "20 hello") {
		t.Fatalf("unexpected octal offset output %q", out)
	}

}

func TestStringsCmdOptionsInvalidMinimumLength(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{"-n", "bad", file})
	}); err == nil {
		t.Fatal("expected invalid minimum length error")
	}

}

func TestStringsCmdOptionsInvalidOffsetBase(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{"-t", "z", file})
	}); err == nil {
		t.Fatal("expected invalid offset base error")
	}

}

func TestStringsCmdOptionsMinLengthFiltersAll(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{"-n", "20", file})
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("expected no strings, got %q", out)
	}

}

func TestStringsCmdOptionsMissingFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{filepath.Join(dir, "missing")})
	}); err == nil {
		t.Fatal("expected missing file error")
	}

}

func TestStringsCmdOptionsDashAAccepted(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bin")
	data := []byte{0, 'a', 'b', 'c', 0, 'h', 'e', 'l', 'l', 'o', 0}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTextCmd(t, "", func() error {
		return StringsCmd([]string{"-a", "-n", "5", file})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Fatalf("unexpected -a output %q", out)
	}

}
