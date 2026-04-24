package text

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"
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

func runGrepCmd(args []string) (string, error) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := GrepCmd(args)

	w.Close()
	io.Copy(&buf, r)
	os.Stdout = old
	return buf.String(), err
}

func runGrepCmdWithStdin(args []string, stdinInput string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stdin = rIn

	go func() {
		wIn.WriteString(stdinInput)
		wIn.Close()
	}()

	err := GrepCmd(args)

	wOut.Close()
	io.Copy(&buf, rOut)
	os.Stdout = oldStdout
	os.Stdin = oldStdin
	return buf.String(), err
}

func runHeadCmd(args []string) (string, error) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := HeadCmd(args)

	w.Close()
	io.Copy(&buf, r)
	os.Stdout = old
	return buf.String(), err
}

func runHeadCmdWithStdin(args []string, stdinInput string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stdin = rIn

	go func() {
		wIn.WriteString(stdinInput)
		wIn.Close()
	}()

	err := HeadCmd(args)

	wOut.Close()
	io.Copy(&buf, rOut)
	os.Stdout = oldStdout
	os.Stdin = oldStdin
	return buf.String(), err
}

func runTailCmd(args []string) (string, error) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := TailCmd(args)

	w.Close()
	io.Copy(&buf, r)
	os.Stdout = old
	return buf.String(), err
}

func runTailCmdWithStdin(args []string, stdinInput string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stdin = rIn

	go func() {
		wIn.WriteString(stdinInput)
		wIn.Close()
	}()

	err := TailCmd(args)

	wOut.Close()
	io.Copy(&buf, rOut)
	os.Stdout = oldStdout
	os.Stdin = oldStdin
	return buf.String(), err
}

func runTailCmdWithTimeout(args []string, timeout time.Duration) (string, error) {
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	errCh := make(chan error, 1)
	go func() {
		errCh <- TailCmdWithContext(ctx, args)
	}()

	select {
	case err := <-errCh:
		w.Close()
		io.Copy(&buf, r)
		os.Stdout = old
		return buf.String(), err
	case <-ctx.Done():
		w.Close()
		io.Copy(&buf, r)
		os.Stdout = old
		return buf.String(), ctx.Err()
	}
}

func runSortCmd(args []string) (string, error) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := SortCmd(args)

	w.Close()
	io.Copy(&buf, r)
	os.Stdout = old
	return buf.String(), err
}

func runSortCmdWithStdin(args []string, stdinInput string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stdin = rIn

	go func() {
		wIn.WriteString(stdinInput)
		wIn.Close()
	}()

	err := SortCmd(args)

	wOut.Close()
	io.Copy(&buf, rOut)
	os.Stdout = oldStdout
	os.Stdin = oldStdin
	return buf.String(), err
}

func runUniqCmd(args []string) (string, error) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := UniqCmd(args)

	w.Close()
	io.Copy(&buf, r)
	os.Stdout = old
	return buf.String(), err
}

func runUniqCmdWithStdin(args []string, stdinInput string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stdin = rIn

	go func() {
		wIn.WriteString(stdinInput)
		wIn.Close()
	}()

	err := UniqCmd(args)

	wOut.Close()
	io.Copy(&buf, rOut)
	os.Stdout = oldStdout
	os.Stdin = oldStdin
	return buf.String(), err
}

func runWcCmd(args []string) (string, error) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := WcCmd(args)

	w.Close()
	io.Copy(&buf, r)
	os.Stdout = old
	return buf.String(), err
}

func runWcCmdWithStdin(args []string, stdinInput string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stdin = rIn

	go func() {
		wIn.WriteString(stdinInput)
		wIn.Close()
	}()

	err := WcCmd(args)

	wOut.Close()
	io.Copy(&buf, rOut)
	os.Stdout = oldStdout
	os.Stdin = oldStdin
	return buf.String(), err
}

func runSedCmd(args []string) (string, error) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := SedCmd(args)

	w.Close()
	io.Copy(&buf, r)
	os.Stdout = old
	return buf.String(), err
}

func runSedCmdWithStdin(args []string, stdinInput string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stdin = rIn

	go func() {
		wIn.WriteString(stdinInput)
		wIn.Close()
	}()

	err := SedCmd(args)

	wOut.Close()
	io.Copy(&buf, rOut)
	os.Stdout = oldStdout
	os.Stdin = oldStdin
	return buf.String(), err
}
