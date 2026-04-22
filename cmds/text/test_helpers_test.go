package text

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func captureTextCmd(t *testing.T, stdin string, fn func() error) (string, error) {
	t.Helper()
	out, _, err := captureTextCmdFull(t, stdin, fn)
	return out, err
}

func captureTextCmdFull(t *testing.T, stdin string, fn func() error) (string, string, error) {
	t.Helper()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldStdin := os.Stdin
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _ = io.WriteString(wIn, stdin)
		_ = wIn.Close()
	}()
	os.Stdout = wOut
	os.Stderr = wErr
	os.Stdin = rIn
	runErr := fn()
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	os.Stdin = oldStdin
	_ = rIn.Close()
	var buf bytes.Buffer
	var errBuf bytes.Buffer
	_, _ = io.Copy(&buf, rOut)
	_, _ = io.Copy(&errBuf, rErr)
	_ = rOut.Close()
	_ = rErr.Close()
	return buf.String(), errBuf.String(), runErr
}
