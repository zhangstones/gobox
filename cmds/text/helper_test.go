package text

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"
)

// runGrepCmd runs GrepCmd with args and captures stdout
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

// runGrepCmdWithStdin runs GrepCmd with stdin input and captures stdout
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

// runHeadCmd runs HeadCmd with args and captures stdout
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

// runHeadCmdWithStdin runs HeadCmd with stdin input and captures stdout
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

// runTailCmd runs TailCmd with args and captures stdout
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

// runTailCmdWithStdin runs TailCmd with stdin input and captures stdout
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

// runTailCmdWithTimeout runs TailCmd with context timeout for follow mode tests
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

	// Wait for either completion or timeout
	select {
	case err := <-errCh:
		w.Close()
		io.Copy(&buf, r)
		os.Stdout = old
		return buf.String(), err
	case <-ctx.Done():
		// Timeout - return what we have
		w.Close()
		io.Copy(&buf, r)
		os.Stdout = old
		return buf.String(), ctx.Err()
	}
}

// runSortCmd runs SortCmd with args and captures stdout
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

// runSortCmdWithStdin runs SortCmd with stdin input and captures stdout
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

// runUniqCmd runs UniqCmd with args and captures stdout
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

// runUniqCmdWithStdin runs UniqCmd with stdin input and captures stdout
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

// runWcCmd runs WcCmd with args and captures stdout
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

// runWcCmdWithStdin runs WcCmd with stdin input and captures stdout
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

// runSedCmd runs SedCmd with args and captures stdout
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

// runSedCmdWithStdin runs SedCmd with stdin input and captures stdout
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
