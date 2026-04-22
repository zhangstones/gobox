package fs

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func captureFsCmd(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	out, _, err := captureFsCmdFull(t, fn)
	return out, err
}

func captureFsCmdFull(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()
	old := os.Stdout
	oldErr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	os.Stderr = wErr
	runErr := fn()
	_ = w.Close()
	_ = wErr.Close()
	os.Stdout = old
	os.Stderr = oldErr
	var buf bytes.Buffer
	var errBuf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_, _ = io.Copy(&errBuf, rErr)
	return buf.String(), errBuf.String(), runErr
}
