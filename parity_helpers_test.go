package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"gobox/cmds/disk"
	"gobox/cmds/fs"
	"gobox/cmds/net"
	"gobox/cmds/proc"
	"gobox/cmds/text"
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
	ID            string
	Name          string
	GoboxArgs     []string
	NativeCommand string
	NativeArgs    []string
	Setup         func(t *testing.T, env *parityEnv)
	Stdin         string
	Normalize     func(string) string
	Assert        func(t *testing.T, gobox, native parityResult)
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

		err = invokeGobox(args)
		if rIn != nil {
			_ = rIn.Close()
		}
		_ = wOut.Close()
		_ = wErr.Close()
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		os.Stdin = oldStdin

		_, _ = io.Copy(&stdoutBuf, rOut)
		_, _ = io.Copy(&stderrBuf, rErr)
		_ = rOut.Close()
		_ = rErr.Close()

		exitCode = goboxExitCode(args[0], err)
	})

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
	if grepErr, ok := err.(text.ExitCodeError); ok && cmd == "grep" {
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
	case "ps":
		return proc.PsCmd(argv)
	case "top":
		return proc.TopCmd(argv)
	case "iostat":
		return disk.IostatCmd(argv)
	case "ioperf":
		return disk.IoperfCmd(argv)
	case "md5sum":
		return disk.Md5sumCmd(argv)
	case "netstat":
		return net.NetstatCmd(argv)
	case "xargs":
		return proc.XargsCmd(argv)
	case "grep":
		return text.GrepCmd(argv)
	case "sed":
		return text.SedCmd(argv)
	case "dig", "nslookup":
		return net.DigCmd(argv)
	case "sort":
		return text.SortCmd(argv)
	case "rand":
		return text.RandCmd(argv)
	case "head":
		return text.HeadCmd(argv)
	case "tail":
		return text.TailCmd(argv)
	case "curl":
		return net.CurlCmd(argv)
	case "wc":
		return text.WcCmd(argv)
	case "uniq":
		return text.UniqCmd(argv)
	case "nc":
		return net.NcCmd(argv)
	case "tw":
		return net.TwCmd(argv)
	case "ifstat":
		return net.IfstatCmd(argv)
	case "np":
		return net.NpCmd(argv)
	case "seq":
		return text.SeqCmd(argv)
	default:
		return fmt.Errorf("unknown command %s", cmd)
	}
}

func runNativeCLI(t *testing.T, dir string, stdin string, command string, args ...string) parityResult {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("native parity tests require unix-like environment")
	}
	path, err := exec.LookPath(command)
	if err != nil {
		t.Skipf("native command %s not found", command)
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Skipf("native command %s could not run: %v", command, err)
		}
	}
	return parityResult{Stdout: stdoutBuf.String(), Stderr: stderrBuf.String(), ExitCode: exitCode}
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
			if normalize == nil {
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
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("%s exit code mismatch: gobox=%d native=%d", tc.Name, gobox.ExitCode, native.ExitCode)
			}
			if gobox.Stdout != native.Stdout {
				t.Fatalf("%s stdout mismatch\n--- gobox ---\n%s\n--- native ---\n%s", tc.Name, gobox.Stdout, native.Stdout)
			}
			if gobox.Stderr != native.Stderr {
				t.Fatalf("%s stderr mismatch\n--- gobox ---\n%s\n--- native ---\n%s", tc.Name, gobox.Stderr, native.Stderr)
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

func collapseSpaces(s string) string {
	s = normalizeText(s)
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
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
