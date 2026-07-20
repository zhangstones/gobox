package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"gobox/cmds/base"
	"gobox/cmds/disk"
	"gobox/cmds/fs"
	netcmd "gobox/cmds/net"
	"gobox/cmds/proc"
	textcmd "gobox/cmds/text"
)

type parityResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type parityEnv struct {
	Dir string
}

type parityCase struct {
	ID               string
	Name             string
	GoboxArgs        []string
	NativeCommand    string
	NativeArgs       []string
	Setup            func(t *testing.T, env *parityEnv)
	Stdin            string
	Normalize        func(string) string
	NormalizeFactory func(env *parityEnv) func(string) string
	Assert           func(t *testing.T, gobox, native parityResult)
}

var parityExecMu sync.Mutex

func withTempChdir(t *testing.T, dir string, fn func()) {
	t.Helper()
	parityExecMu.Lock()
	defer parityExecMu.Unlock()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()
	fn()
}

func runGoboxCLI(t *testing.T, dir string, stdin string, args ...string) parityResult {
	t.Helper()
	var stdoutBuf, stderrBuf bytes.Buffer
	var exitCode int

	withTempChdir(t, dir, func() {
		oldStdout := os.Stdout
		oldStderr := os.Stderr
		oldStdin := os.Stdin
		rOut, wOut, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe stdout: %v", err)
		}
		rErr, wErr, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe stderr: %v", err)
		}
		var rIn *os.File
		if stdin != "" {
			rIn, wIn, err := os.Pipe()
			if err != nil {
				t.Fatalf("pipe stdin: %v", err)
			}
			go func() {
				_, _ = io.WriteString(wIn, stdin)
				_ = wIn.Close()
			}()
			os.Stdin = rIn
		}
		os.Stdout = wOut
		os.Stderr = wErr
		defer func() {
			os.Stdout = oldStdout
			os.Stderr = oldStderr
			os.Stdin = oldStdin
		}()

		// Drain pipes concurrently to prevent deadlock when output exceeds the
		// pipe buffer (64 KB on Linux).
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = io.Copy(&stdoutBuf, rOut) }()
		go func() { defer wg.Done(); _, _ = io.Copy(&stderrBuf, rErr) }()

		err = invokeGobox(args)
		if rIn != nil {
			_ = rIn.Close()
		}
		_ = wOut.Close()
		_ = wErr.Close()

		wg.Wait()
		_ = rOut.Close()
		_ = rErr.Close()

		exitCode = goboxExitCode(args[0], err)
	})

	return parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode}
}

func runGoboxMainCLI(t *testing.T, dir string, stdin string, args ...string) parityResult {
	t.Helper()
	var stdoutBuf, stderrBuf bytes.Buffer
	var exitCode int

	withTempChdir(t, dir, func() {
		oldStdout := os.Stdout
		oldStderr := os.Stderr
		oldStdin := os.Stdin
		rOut, wOut, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe stdout: %v", err)
		}
		rErr, wErr, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe stderr: %v", err)
		}
		var rIn *os.File
		if stdin != "" {
			rIn, wIn, err := os.Pipe()
			if err != nil {
				t.Fatalf("pipe stdin: %v", err)
			}
			go func() {
				_, _ = io.WriteString(wIn, stdin)
				_ = wIn.Close()
			}()
			os.Stdin = rIn
		}
		os.Stdout = wOut
		os.Stderr = wErr
		defer func() {
			os.Stdout = oldStdout
			os.Stderr = oldStderr
			os.Stdin = oldStdin
		}()

		// Drain pipes concurrently to prevent deadlock when output exceeds the
		// pipe buffer (64 KB on Linux), matching runGoboxCLI's protection.
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = io.Copy(&stdoutBuf, rOut) }()
		go func() { defer wg.Done(); _, _ = io.Copy(&stderrBuf, rErr) }()

		err = invokeGobox(args)
		if err != nil {
			cmd := args[0]
			type cliErrorSilencer interface{ SuppressCLIError() bool }
			type cliExitCoder interface{ ExitCode() int }
			if exitErr, ok := err.(cliExitCoder); ok {
				if silencer, ok := err.(cliErrorSilencer); !ok || !silencer.SuppressCLIError() {
					fmt.Fprintln(os.Stderr, cmd+":", err)
				}
				exitCode = exitErr.ExitCode()
			} else {
				if silencer, ok := err.(cliErrorSilencer); !ok || !silencer.SuppressCLIError() {
					fmt.Fprintln(os.Stderr, cmd+":", err)
				}
				exitCode = 2
			}
		}

		if rIn != nil {
			_ = rIn.Close()
		}
		_ = wOut.Close()
		_ = wErr.Close()

		wg.Wait()
		_ = rOut.Close()
		_ = rErr.Close()
	})

	return parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode}
}

func runGoboxSubprocess(t *testing.T, dir string, args []string, timeout time.Duration) parityResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=TestParityHelperProcess", "--"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOBOX_PARITY_HELPER=1")

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil && ctx.Err() == nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("gobox subprocess failed: %v", err)
		}
	}
	return parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode}
}

func goboxExitCode(cmd string, err error) int {
	if err == nil {
		return 0
	}
	type exitCoder interface {
		ExitCode() int
	}
	if exitErr, ok := err.(exitCoder); ok {
		return exitErr.ExitCode()
	}
	if grepErr, ok := err.(textcmd.ExitCodeError); ok && cmd == "grep" {
		return int(grepErr)
	}
	return 2
}

func invokeGobox(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command")
	}
	cmd := args[0]
	argv := args[1:]
	switch cmd {
	case "find":
		return fs.FindCmd(argv)
	case "du":
		return fs.DuCmd(argv)
	case "df":
		return fs.DfCmd(argv)
	case "readpath":
		return fs.ReadpathCmd(argv)
	case "stat":
		return fs.StatCmd(argv)
	case "truncate":
		return fs.TruncateCmd(argv)
	case "ps":
		return proc.PsCmd(argv)
	case "top":
		return proc.TopCmd(argv)
	case "free":
		return proc.FreeCmd(argv)
	case "iostat":
		return disk.IostatCmd(argv)
	case "ioperf":
		return disk.IoperfCmd(argv)
	case "md5sum":
		return disk.Md5sumCmd(argv)
	case "sha256sum":
		return disk.Sha256sumCmd(argv)
	case "netstat":
		return netcmd.NetstatCmd(argv)
	case "ip":
		return netcmd.IpCmd(argv)
	case "xargs":
		return proc.XargsCmd(argv)
	case "kill":
		return proc.KillCmd(argv)
	case "lsof":
		return proc.LsofCmd(argv)
	case "watch":
		return proc.WatchCmd(argv)
	case "timeout":
		return proc.TimeoutCmd(argv)
	case "grep":
		return textcmd.GrepCmd(argv)
	case "sed":
		return textcmd.SedCmd(argv)
	case "dig":
		return netcmd.DigCmd(argv)
	case "nslookup":
		return netcmd.NslookupCmd(argv)
	case "sort":
		return textcmd.SortCmd(argv)
	case "rand":
		return textcmd.RandCmd(argv)
	case "head":
		return textcmd.HeadCmd(argv)
	case "tail":
		return textcmd.TailCmd(argv)
	case "curl":
		return netcmd.CurlCmd(argv)
	case "wc":
		return textcmd.WcCmd(argv)
	case "hex":
		return textcmd.HexCmd(argv)
	case "base64":
		return textcmd.Base64Cmd(argv)
	case "strings":
		return textcmd.StringsCmd(argv)
	case "diff":
		return textcmd.DiffCmd(argv)
	case "uniq":
		return textcmd.UniqCmd(argv)
	case "nc":
		return netcmd.NcCmd(argv)
	case "tw":
		return netcmd.TwCmd(argv)
	case "ifstat":
		return netcmd.IfstatCmd(argv)
	case "np":
		return netcmd.NpCmd(argv)
	case "seq":
		return textcmd.SeqCmd(argv)
	case "alias":
		aliasCmd, ok := base.Lookup("alias")
		if !ok {
			return fmt.Errorf("alias command not registered")
		}
		return aliasCmd.Run(argv, os.Stdout)
	default:
		return fmt.Errorf("unknown command %s", cmd)
	}
}

func runGoboxCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "missing command")
		return 1
	}
	err := invokeGobox(args)
	if err == nil {
		return 0
	}
	cmd := args[0]
	type cliErrorSilencer interface{ SuppressCLIError() bool }
	type cliExitCoder interface{ ExitCode() int }
	if exitErr, ok := err.(cliExitCoder); ok {
		if silencer, ok := err.(cliErrorSilencer); !ok || !silencer.SuppressCLIError() {
			fmt.Fprintln(os.Stderr, cmd+":", err)
		}
		return exitErr.ExitCode()
	}
	if silencer, ok := err.(cliErrorSilencer); !ok || !silencer.SuppressCLIError() {
		fmt.Fprintln(os.Stderr, cmd+":", err)
	}
	return 2
}

func TestParityHelperProcess(t *testing.T) {
	if os.Getenv("GOBOX_PARITY_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Exit(runGoboxCommand(os.Args[i+1:]))
		}
	}
	os.Exit(1)
}

func runNativeCLI(t *testing.T, dir string, stdin string, command string, args ...string) parityResult {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("native parity tests require unix-like environment")
	}
	path := requireNativeCommand(t, command)
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("native command %s could not run: %v", command, err)
		}
	}
	return parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode}
}

func requireNativeCommand(t *testing.T, command string) string {
	t.Helper()
	path, err := exec.LookPath(command)
	if err != nil {
		t.Skipf("native command %s not found in PATH, skipping native parity comparison", command)
	}
	return path
}

func runExactParityCases(t *testing.T, cases []parityCase) {
	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			env := &parityEnv{Dir: t.TempDir()}
			if tc.Setup != nil {
				tc.Setup(t, env)
			}
			gobox := runGoboxCLI(t, env.Dir, tc.Stdin, tc.GoboxArgs...)
			native := runNativeCLI(t, env.Dir, tc.Stdin, tc.NativeCommand, tc.NativeArgs...)
			normalize := tc.Normalize
			if tc.NormalizeFactory != nil {
				normalize = tc.NormalizeFactory(env)
			} else if normalize == nil {
				normalize = normalizeText
			}
			gobox.Stdout = normalize(gobox.Stdout)
			gobox.Stderr = normalize(gobox.Stderr)
			native.Stdout = normalize(native.Stdout)
			native.Stderr = normalize(native.Stderr)
			if tc.Assert != nil {
				tc.Assert(t, gobox, native)
				return
			}
			cmdline := fmt.Sprintf("[%s/%s] gobox args=%v native=%s %v", tc.ID, tc.Name, tc.GoboxArgs, tc.NativeCommand, tc.NativeArgs)
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("%s exit code mismatch: gobox=%d native=%d", cmdline, gobox.ExitCode, native.ExitCode)
			}
			if gobox.Stdout != native.Stdout {
				t.Fatalf("%s stdout mismatch\n--- gobox ---\n%s\n--- native ---\n%s", cmdline, gobox.Stdout, native.Stdout)
			}
			if gobox.Stderr != native.Stderr {
				t.Fatalf("%s stderr mismatch\n--- gobox ---\n%s\n--- native ---\n%s", cmdline, gobox.Stderr, native.Stderr)
			}
		})
	}
}

func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// collapseSpaces collapses runs of horizontal whitespace within each line to a
// single space (to normalize column-width noise) while preserving line
// boundaries, so it cannot mask a regression that merges or splits lines.
func collapseSpaces(s string) string {
	s = normalizeText(s)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.Join(lines, "\n")
}

func normalizeFindOutput(base string) func(string) string {
	return func(s string) string {
		s = normalizeText(s)
		if s == "" {
			return s
		}
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			line = strings.ReplaceAll(line, filepath.Clean(base)+string(os.PathSeparator), "")
			line = strings.TrimPrefix(line, filepath.Clean(base))
			line = strings.TrimPrefix(line, string(os.PathSeparator))
			lines[i] = filepath.ToSlash(line)
		}
		sort.Strings(lines)
		return strings.Join(lines, "\n")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// sortedLines normalizes whitespace and sorts lines alphabetically.
// Use as a Normalize func when line ordering may vary (e.g. grep -r).
func sortedLines(s string) string {
	return strings.Join(sortedNonEmptyLines(s), "\n")
}

func sameStringSet(a, b string) bool {
	al := nonEmptyLines(a)
	bl := nonEmptyLines(b)
	sort.Strings(al)
	sort.Strings(bl)
	return strings.Join(al, "\n") == strings.Join(bl, "\n")
}

func nonEmptyLines(s string) []string {
	s = normalizeText(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func lastLine(s string) string {
	lines := nonEmptyLines(s)
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

func startMarkerProcess(t *testing.T, marker string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("bash", "-lc", "exec -a "+marker+" sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start marker process: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	return cmd
}

func waitForExit(t *testing.T, cmd *exec.Cmd, timeout time.Duration) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if _, ok := err.(*exec.ExitError); ok {
			return nil
		}
		return err
	case <-time.After(timeout):
		return fmt.Errorf("process %d did not exit within %s", cmd.Process.Pid, timeout)
	}
}

func requireAlive(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd == nil || cmd.Process == nil {
		t.Fatal("missing process handle")
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("process %d is not alive: %v", cmd.Process.Pid, err)
	}
}

func startExactNameProcess(t *testing.T, name string) *exec.Cmd {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.Symlink("/usr/bin/sleep", path); err != nil {
		t.Fatalf("create sleep symlink: %v", err)
	}
	cmd := exec.Command(path, "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start exact-name process: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	return cmd
}

func stopCmd(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

func extractLeadingInts(lines []string) []int {
	var vals []int
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		v, err := strconv.Atoi(fields[0])
		if err == nil {
			vals = append(vals, v)
		}
	}
	return vals
}

func extractNetstatPIDs(out string) []int {
	var vals []int
	for _, line := range nonEmptyLines(out)[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		last := fields[len(fields)-1]
		pidStr, _, _ := strings.Cut(last, "/")
		if pidStr == "-" {
			continue
		}
		v, err := strconv.Atoi(pidStr)
		if err == nil {
			vals = append(vals, v)
		}
	}
	return vals
}

func assertMonotonic(t *testing.T, vals []int, descending bool) {
	t.Helper()
	// A slice of 0-2 elements is trivially "sorted" regardless of whether
	// the sort actually ran, so it proves nothing. Require enough rows to
	// make ordering a meaningful signal, and fail loudly instead of
	// silently passing when the fixture didn't produce them.
	if len(vals) < 3 {
		t.Fatalf("assertMonotonic requires at least 3 values to meaningfully prove ordering, got %d: %v", len(vals), vals)
	}
	for i := 1; i < len(vals); i++ {
		if descending {
			if vals[i-1] < vals[i] {
				t.Fatalf("expected descending order, got %v", vals)
			}
		} else if vals[i-1] > vals[i] {
			t.Fatalf("expected ascending order, got %v", vals)
		}
	}
}

func runTailGoboxFollow(t *testing.T, dir string, args []string, action func(), timeout time.Duration) parityResult {
	t.Helper()
	parityExecMu.Lock()
	defer parityExecMu.Unlock()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	var stdoutBuf, stderrBuf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stdout = wOut
	os.Stderr = wErr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- textcmd.TailCmdWithContext(ctx, args)
	}()
	time.Sleep(150 * time.Millisecond)
	if action != nil {
		action()
	}
	err = <-errCh

	_ = wOut.Close()
	_ = wErr.Close()
	_, _ = io.Copy(&stdoutBuf, rOut)
	_, _ = io.Copy(&stderrBuf, rErr)
	_ = rOut.Close()
	_ = rErr.Close()

	exitCode := 0
	if err != nil && err != context.DeadlineExceeded {
		exitCode = 2
	}
	return parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode}
}

type ncListenResult struct {
	Server       parityResult
	ClientOutput string
	ClientErr    error
}

func runGoboxNCListen(t *testing.T, port, serverStdin, clientInput string, timeout time.Duration) ncListenResult {
	t.Helper()
	parityExecMu.Lock()
	defer parityExecMu.Unlock()

	var stdoutBuf, stderrBuf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldStdin := os.Stdin
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	_, _ = io.WriteString(wIn, serverStdin)
	_ = wIn.Close()
	os.Stdout = wOut
	os.Stderr = wErr
	os.Stdin = rIn
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		os.Stdin = oldStdin
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- netcmd.NcCmdWithContext(ctx, []string{"-l", port})
	}()
	time.Sleep(150 * time.Millisecond)
	clientOutput, clientErr := exchangeWithNCListener(port, clientInput, serverStdin)
	err = <-errCh

	_ = rIn.Close()
	_ = wOut.Close()
	_ = wErr.Close()
	_, _ = io.Copy(&stdoutBuf, rOut)
	_, _ = io.Copy(&stderrBuf, rErr)
	_ = rOut.Close()
	_ = rErr.Close()

	exitCode := 0
	if err != nil && err != context.DeadlineExceeded {
		exitCode = 2
	}
	return ncListenResult{
		Server:       parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode},
		ClientOutput: clientOutput,
		ClientErr:    clientErr,
	}
}

func runNativeNCListen(t *testing.T, port, serverStdin, clientInput string, timeout time.Duration) ncListenResult {
	t.Helper()
	path := requireNativeCommand(t, "nc")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-l", port)
	cmd.Stdin = strings.NewReader(serverStdin)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start native nc -l: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	clientOutput, clientErr := exchangeWithNCListener(port, clientInput, serverStdin)
	err := cmd.Wait()

	exitCode := 0
	if err != nil && ctx.Err() == nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 2
		}
	}
	return ncListenResult{
		Server:       parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode},
		ClientOutput: clientOutput,
		ClientErr:    clientErr,
	}
}

func exchangeWithNCListener(port, clientInput, wantOutput string) (string, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, clientInput); err != nil {
		return "", err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.CloseWrite()
	}
	var out strings.Builder
	buf := make([]byte, 256)
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(out.String(), wantOutput) {
		_ = conn.SetReadDeadline(deadline)
		n, err := conn.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if err != nil {
			if strings.Contains(out.String(), wantOutput) || err == io.EOF {
				break
			}
			return out.String(), err
		}
	}
	return out.String(), nil
}

func defaultIPv4Gateway(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		t.Skipf("default IPv4 gateway unavailable: %v", err)
	}
	fields := strings.Fields(string(out))
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "via" && net.ParseIP(fields[i+1]) != nil {
			return fields[i+1]
		}
	}
	t.Skipf("default IPv4 gateway unavailable in route output: %q", string(out))
	return ""
}

func runNativeFollow(t *testing.T, dir, command string, args []string, action func(), timeout time.Duration) parityResult {
	t.Helper()
	path := requireNativeCommand(t, command)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start native %s: %v", command, err)
	}
	time.Sleep(150 * time.Millisecond)
	if action != nil {
		action()
	}
	err := cmd.Wait()
	exitCode := 0
	if err != nil && ctx.Err() == nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 2
		}
	}
	return parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("append open %s: %v", path, err)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		t.Fatalf("append write %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("append close %s: %v", path, err)
	}
}

func startTCPEchoServer(t *testing.T, addr string) (string, string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if strings.Contains(addr, "::1") {
			t.Skipf("IPv6 loopback unavailable: %v", err)
		}
		t.Fatalf("listen tcp %s: %v", addr, err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split listener addr: %v", err)
	}
	return host, port, func() {
		_ = ln.Close()
		<-done
	}
}

func startDelayedCloseServer(t *testing.T, delay time.Duration) (string, string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen delayed close: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				time.Sleep(delay)
				_ = c.Close()
			}(conn)
		}
	}()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		t.Fatalf("split delayed close addr: %v", err)
	}
	return host, port, func() {
		_ = ln.Close()
		<-done
	}
}

func startTCPRemotePortRecorder(t *testing.T) (string, string, <-chan int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen remote port recorder: %v", err)
	}
	remotePorts := make(chan int, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			remotePorts <- addr.Port
		}
	}()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		t.Fatalf("split remote port recorder addr: %v", err)
	}
	return host, port, remotePorts, func() {
		_ = ln.Close()
		<-done
	}
}

func atoiForTest(t *testing.T, s string) int {
	t.Helper()
	v, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("atoi %q: %v", s, err)
	}
	return v
}

func startUDPReceiver(t *testing.T) (net.Addr, <-chan string) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	done := make(chan string, 1)
	go func() {
		defer conn.Close()
		buf := make([]byte, 1024)
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			done <- ""
			return
		}
		_, _ = conn.WriteTo(buf[:n], addr)
		done <- string(buf[:n])
	}()
	return conn.LocalAddr(), done
}

func closedTCPPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for closed port: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		t.Fatalf("split closed port addr: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("close temp listener: %v", err)
	}
	return port
}

func startLocalDNSServer(t *testing.T, ip string) (string, string, func()) {
	t.Helper()
	udpConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen dns udp: %v", err)
	}
	host, port, err := net.SplitHostPort(udpConn.LocalAddr().String())
	if err != nil {
		_ = udpConn.Close()
		t.Fatalf("split dns udp addr: %v", err)
	}
	tcpLn, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		_ = udpConn.Close()
		t.Fatalf("listen dns tcp: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 512)
		for {
			n, addr, err := udpConn.ReadFrom(buf)
			if err != nil {
				return
			}
			resp := buildDNSAResponse(buf[:n], net.ParseIP(ip).To4())
			_, _ = udpConn.WriteTo(resp, addr)
		}
	}()

	tcpDone := make(chan struct{})
	go func() {
		defer close(tcpDone)
		for {
			conn, err := tcpLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var lenBuf [2]byte
				if _, err := io.ReadFull(c, lenBuf[:]); err != nil {
					return
				}
				msgLen := int(binary.BigEndian.Uint16(lenBuf[:]))
				msg := make([]byte, msgLen)
				if _, err := io.ReadFull(c, msg); err != nil {
					return
				}
				resp := buildDNSAResponse(msg, net.ParseIP(ip).To4())
				binary.BigEndian.PutUint16(lenBuf[:], uint16(len(resp)))
				_, _ = c.Write(lenBuf[:])
				_, _ = c.Write(resp)
			}(conn)
		}
	}()

	return host, port, func() {
		_ = udpConn.Close()
		_ = tcpLn.Close()
		<-done
		<-tcpDone
	}
}

// buildDNSNXDOMAINResponse builds a DNS response with RCODE=3 (NXDOMAIN) and no answer.
func buildDNSNXDOMAINResponse(query []byte) []byte {
	if len(query) < 12 {
		return nil
	}
	qEnd := 12
	for qEnd < len(query) {
		labelLen := int(query[qEnd])
		qEnd++
		if labelLen == 0 {
			break
		}
		qEnd += labelLen
	}
	if qEnd+4 > len(query) {
		return nil
	}
	question := query[12 : qEnd+4]
	resp := make([]byte, 0, 12+len(question))
	resp = append(resp, query[0], query[1]) // transaction ID
	resp = append(resp, 0x81, 0x83)        // flags: QR=1, RD=1, RA=1, RCODE=3 (NXDOMAIN)
	resp = append(resp, 0x00, 0x01)        // QDCOUNT=1
	resp = append(resp, 0x00, 0x00)        // ANCOUNT=0 (no answers)
	resp = append(resp, 0x00, 0x00)        // NSCOUNT=0
	resp = append(resp, 0x00, 0x00)        // ARCOUNT=0
	resp = append(resp, question...)
	return resp
}

// startLocalNXDOMAINServer starts a local DNS server that returns NXDOMAIN for all queries.
func startLocalNXDOMAINServer(t *testing.T) (string, string, func()) {
	t.Helper()
	udpConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen nxdomain dns udp: %v", err)
	}
	host, port, err := net.SplitHostPort(udpConn.LocalAddr().String())
	if err != nil {
		_ = udpConn.Close()
		t.Fatalf("split nxdomain dns addr: %v", err)
	}
	tcpLn, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		_ = udpConn.Close()
		t.Fatalf("listen nxdomain dns tcp: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 512)
		for {
			n, addr, err := udpConn.ReadFrom(buf)
			if err != nil {
				return
			}
			resp := buildDNSNXDOMAINResponse(buf[:n])
			_, _ = udpConn.WriteTo(resp, addr)
		}
	}()

	tcpDone := make(chan struct{})
	go func() {
		defer close(tcpDone)
		for {
			conn, err := tcpLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var lenBuf [2]byte
				if _, err := io.ReadFull(c, lenBuf[:]); err != nil {
					return
				}
				msgLen := int(binary.BigEndian.Uint16(lenBuf[:]))
				msg := make([]byte, msgLen)
				if _, err := io.ReadFull(c, msg); err != nil {
					return
				}
				resp := buildDNSNXDOMAINResponse(msg)
				binary.BigEndian.PutUint16(lenBuf[:], uint16(len(resp)))
				_, _ = c.Write(lenBuf[:])
				_, _ = c.Write(resp)
			}(conn)
		}
	}()

	return host, port, func() {
		_ = udpConn.Close()
		_ = tcpLn.Close()
		<-done
		<-tcpDone
	}
}

func buildDNSAResponse(query []byte, ip net.IP) []byte {
	if len(query) < 12 || len(ip) != 4 {
		return nil
	}
	qEnd := 12
	for qEnd < len(query) {
		labelLen := int(query[qEnd])
		qEnd++
		if labelLen == 0 {
			break
		}
		qEnd += labelLen
	}
	if qEnd+4 > len(query) {
		return nil
	}
	question := query[12 : qEnd+4]
	resp := make([]byte, 0, 12+len(question)+16)
	resp = append(resp, query[0], query[1])
	resp = append(resp, 0x81, 0x80)
	resp = append(resp, 0x00, 0x01)
	resp = append(resp, 0x00, 0x01)
	resp = append(resp, 0x00, 0x00)
	resp = append(resp, 0x00, 0x00)
	resp = append(resp, question...)
	resp = append(resp, 0xc0, 0x0c)
	resp = append(resp, 0x00, 0x01)
	resp = append(resp, 0x00, 0x01)
	resp = append(resp, 0x00, 0x00, 0x00, 0x3c)
	resp = append(resp, 0x00, 0x04)
	resp = append(resp, ip...)
	return resp
}
