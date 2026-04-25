package main

import (
	"bytes"
	"io"
	stdnet "net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"gobox/cmds/disk"
	"gobox/cmds/fs"
	"gobox/cmds/net"
	"gobox/cmds/proc"
	"gobox/cmds/text"
)

func captureOutput(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()

	origStdout := os.Stdout
	origStderr := os.Stderr
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	stdoutCh := make(chan string, 1)
	stderrCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutR)
		stdoutCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrR)
		stderrCh <- buf.String()
	}()

	runErr := fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	return <-stdoutCh, <-stderrCh, runErr
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := stdnet.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*stdnet.TCPAddr).Port
}

// ============ Cross-Platform Commands ============

func TestFindCmdHandlesFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return fs.FindCmd([]string{"-name", "*.txt", "-maxdepth", "1", dir})
	})
	if err != nil {
		t.Fatalf("FindCmd returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte(filepath.Join(dir, "file.txt"))) {
		t.Fatalf("FindCmd expected match output, got %q", stdout)
	}
}

func TestDuCmdSummary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return fs.DuCmd([]string{"-s", dir})
	})
	if err != nil {
		t.Fatalf("DuCmd returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte(dir)) {
		t.Fatalf("DuCmd expected directory summary, got %q", stdout)
	}
}

func TestXargsCmdNoRunWithNoInput(t *testing.T) {
	orig := os.Stdin
	t.Cleanup(func() { os.Stdin = orig })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_ = w.Close()
	os.Stdin = r

	if err := proc.XargsCmd([]string{"-r"}); err != nil {
		t.Fatalf("XargsCmd returned error: %v", err)
	}
}

func TestXargsCmdBasic(t *testing.T) {
	origStdin := os.Stdin
	t.Cleanup(func() { os.Stdin = origStdin })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.WriteString("alpha\nbeta\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()
	os.Stdin = r

	stdout, _, err := captureOutput(t, func() error {
		return proc.XargsCmd([]string{"echo"})
	})
	if err != nil {
		t.Fatalf("XargsCmd basic returned error: %v", err)
	}
	if stdout != "alpha beta\n" {
		t.Fatalf("XargsCmd expected appended args output, got %q", stdout)
	}
}

func TestGrepCmdBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello world\nfoo bar\nhello again"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.GrepCmd([]string{"hello", file})
	})
	if err != nil {
		t.Fatalf("GrepCmd returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("hello world")) || !bytes.Contains([]byte(stdout), []byte("hello again")) {
		t.Fatalf("GrepCmd expected matching lines, got %q", stdout)
	}
}

func TestGrepCmdInvertMatch(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello\nworld\nfoo"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.GrepCmd([]string{"-v", "hello", file})
	})
	if err != nil {
		t.Fatalf("GrepCmd -v returned error: %v", err)
	}
	if stdout != "world\nfoo\n" {
		t.Fatalf("GrepCmd -v expected filtered lines, got %q", stdout)
	}
}

func TestSedCmdSubstitute(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.SedCmd([]string{"s/hello/HELLO/", file})
	})
	if err != nil {
		t.Fatalf("SedCmd returned error: %v", err)
	}
	if stdout != "HELLO world\n" {
		t.Fatalf("SedCmd expected substitution output, got %q", stdout)
	}
}

func TestSedCmdQuietMode(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello\nworld\nhello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.SedCmd([]string{"-n", "s/hello/HELLO/p", file})
	})
	if err != nil {
		t.Fatalf("SedCmd -n returned error: %v", err)
	}
	if stdout != "HELLO\nHELLO\n" {
		t.Fatalf("SedCmd -n expected printed substitutions, got %q", stdout)
	}
}

func TestHeadCmdDefault(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.HeadCmd([]string{file})
	})
	if err != nil {
		t.Fatalf("HeadCmd returned error: %v", err)
	}
	if stdout != content+"\n" {
		t.Fatalf("HeadCmd expected file contents, got %q", stdout)
	}
}

func TestHeadCmdNLines(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.HeadCmd([]string{"-n", "2", file})
	})
	if err != nil {
		t.Fatalf("HeadCmd -n returned error: %v", err)
	}
	if stdout != "line1\nline2\n" {
		t.Fatalf("HeadCmd -n expected first 2 lines, got %q", stdout)
	}
}

func TestTailCmdDefault(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.TailCmd([]string{file})
	})
	if err != nil {
		t.Fatalf("TailCmd returned error: %v", err)
	}
	if stdout != content+"\n" {
		t.Fatalf("TailCmd expected file contents, got %q", stdout)
	}
}

func TestTailCmdNlines(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.TailCmd([]string{"-n", "2", file})
	})
	if err != nil {
		t.Fatalf("TailCmd -n returned error: %v", err)
	}
	if stdout != "line4\nline5\n" {
		t.Fatalf("TailCmd -n expected last 2 lines, got %q", stdout)
	}
}

func TestSortCmdDefault(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("cherry\napple\nbanana"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.SortCmd([]string{file})
	})
	if err != nil {
		t.Fatalf("SortCmd returned error: %v", err)
	}
	if stdout != "apple\nbanana\ncherry\n" {
		t.Fatalf("SortCmd expected sorted lines, got %q", stdout)
	}
}

func TestSortCmdNumeric(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("10\n2\n1\n20"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.SortCmd([]string{"-n", file})
	})
	if err != nil {
		t.Fatalf("SortCmd -n returned error: %v", err)
	}
	if stdout != "1\n2\n10\n20\n" {
		t.Fatalf("SortCmd -n expected numeric sort, got %q", stdout)
	}
}

func TestSortCmdReverse(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("apple\nbanana\ncherry"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.SortCmd([]string{"-r", file})
	})
	if err != nil {
		t.Fatalf("SortCmd -r returned error: %v", err)
	}
	if stdout != "cherry\nbanana\napple\n" {
		t.Fatalf("SortCmd -r expected reverse order, got %q", stdout)
	}
}

func TestUniqCmdBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("apple\napple\nbanana\ncherry\ncherry\ncherry"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.UniqCmd([]string{file})
	})
	if err != nil {
		t.Fatalf("UniqCmd returned error: %v", err)
	}
	if stdout != "apple\nbanana\ncherry\n" {
		t.Fatalf("UniqCmd expected deduped output, got %q", stdout)
	}
}

func TestUniqCmdCount(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("apple\napple\nbanana"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.UniqCmd([]string{"-c", file})
	})
	if err != nil {
		t.Fatalf("UniqCmd -c returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("2 apple")) || !bytes.Contains([]byte(stdout), []byte("1 banana")) {
		t.Fatalf("UniqCmd -c expected counted output, got %q", stdout)
	}
}

func TestWCCmdDefault(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello world\nfoo bar"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.WcCmd([]string{file})
	})
	if err != nil {
		t.Fatalf("WcCmd returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte(filepath.Base(file))) {
		t.Fatalf("WcCmd expected filename in output, got %q", stdout)
	}
}

func TestWcCmdLines(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("line1\nline2\nline3"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.WcCmd([]string{"-l", file})
	})
	if err != nil {
		t.Fatalf("WcCmd -l returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("2")) {
		t.Fatalf("WcCmd -l expected line count, got %q", stdout)
	}
}

// ============ Linux-specific Commands ============

func TestPsCmdMinimal(t *testing.T) {
	stdout, _, err := captureOutput(t, func() error {
		return proc.PsCmd([]string{"-n", "5", "-i", "0", "-sort", "pid", "-r"})
	})
	if err != nil {
		t.Fatalf("PsCmd returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("PID")) {
		t.Fatalf("PsCmd expected header output, got %q", stdout)
	}
	if !bytes.Contains([]byte(stdout), []byte(strconv.Itoa(os.Getpid()))) {
		t.Fatalf("PsCmd expected current process pid %d in output, got %q", os.Getpid(), stdout)
	}
}

func TestTopCmdSingleIteration(t *testing.T) {
	stdout, _, err := captureOutput(t, func() error {
		return proc.TopCmd([]string{"-n", "1", "-d", "0"})
	})
	if err != nil {
		t.Fatalf("TopCmd returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("PID")) {
		t.Fatalf("TopCmd expected process table output, got %q", stdout)
	}
	if !bytes.Contains([]byte(stdout), []byte(strconv.Itoa(os.Getpid()))) {
		t.Fatalf("TopCmd expected current process pid %d in output, got %q", os.Getpid(), stdout)
	}
}

func TestIostatCmdPositionals(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("iostat supported only on Linux")
	}
	stdout, _, err := captureOutput(t, func() error {
		return disk.IostatCmd([]string{"1", "1"})
	})
	if err != nil {
		t.Fatalf("IostatCmd returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("Device")) {
		t.Fatalf("IostatCmd expected device stats header, got %q", stdout)
	}
	if len(strings.Fields(stdout)) <= 6 {
		t.Fatalf("IostatCmd expected at least one device row, got %q", stdout)
	}
}

func TestIostatCmdCgroupSmoke(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("iostat supported only on Linux")
	}
	if _, err := os.Stat("/sys/fs/cgroup/io.stat"); err != nil {
		if _, err := os.Stat("/sys/fs/cgroup/blkio/blkio.throttle.io_service_bytes"); err != nil {
			if _, err := os.Stat("/sys/fs/cgroup/blkio/blkio.io_service_bytes"); err != nil {
				t.Skip("cgroup iostat files not available")
			}
		}
	}

	stdout, _, err := captureOutput(t, func() error {
		return disk.IostatCmd([]string{"--cgroup", "-n", "1"})
	})
	if err != nil {
		t.Fatalf("IostatCmd --cgroup returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("Device")) {
		t.Fatalf("IostatCmd --cgroup expected device stats header, got %q", stdout)
	}
}

func TestIostatCmdHumanReadableSmoke(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("iostat supported only on Linux")
	}
	stdout, _, err := captureOutput(t, func() error {
		return disk.IostatCmd([]string{"-H", "1", "1"})
	})
	if err != nil {
		t.Fatalf("IostatCmd -H returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("Device")) {
		t.Fatalf("IostatCmd -H expected device stats header, got %q", stdout)
	}
	if !bytes.Contains([]byte(stdout), []byte("/s")) {
		t.Fatalf("IostatCmd -H expected per-second units, got %q", stdout)
	}
}

func TestNetstatCmdRuns(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat supported only on Linux")
	}
	ln, err := stdnet.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*stdnet.TCPAddr).Port
	stdout, _, err := captureOutput(t, func() error {
		return net.NetstatCmd([]string{"-t", "-l", "-n", "-port", strconv.Itoa(port)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("Proto")) {
		t.Fatalf("NetstatCmd expected socket table header, got %q", stdout)
	}
	if !bytes.Contains([]byte(stdout), []byte("127.0.0.1:"+strconv.Itoa(port))) {
		t.Fatalf("NetstatCmd expected listener port %d in output, got %q", port, stdout)
	}
}

func TestNetstatCmdExtendedLongFlagsSmoke(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat supported only on Linux")
	}

	ln, err := stdnet.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*stdnet.TCPAddr).Port
	stdout, _, err := captureOutput(t, func() error {
		return net.NetstatCmd([]string{"--tcp", "--listening", "--programs", "--extend", "--timers", "--numeric", "--wide", "-port", strconv.Itoa(port)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd long flags returned error: %v", err)
	}
	for _, want := range []string{"PID/Program", "User", "Inode", "Timer", "127.0.0.1:" + strconv.Itoa(port)} {
		if !bytes.Contains([]byte(stdout), []byte(want)) {
			t.Fatalf("NetstatCmd long flags expected %q, got %q", want, stdout)
		}
	}
}

// ============ Network Commands ============

func TestDigCmdBasic(t *testing.T) {
	stdout, _, err := captureOutput(t, func() error {
		return net.DigCmd([]string{"+short", "localhost"})
	})
	if err != nil {
		t.Fatalf("DigCmd returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("127.0.0.1")) {
		t.Fatalf("DigCmd expected localhost A record, got %q", stdout)
	}
}

func TestDigCmdWithType(t *testing.T) {
	stdout, _, err := captureOutput(t, func() error {
		return net.DigCmd([]string{"-t", "A", "+noall", "+answer", "localhost"})
	})
	if err != nil {
		t.Fatalf("DigCmd with type returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("localhost.")) || !bytes.Contains([]byte(stdout), []byte("127.0.0.1")) {
		t.Fatalf("DigCmd with type expected answer section, got %q", stdout)
	}
}

func TestNcCmdBasic(t *testing.T) {
	ln, err := stdnet.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*stdnet.TCPAddr).Port
	if err := net.NcCmd([]string{"-z", "127.0.0.1", strconv.Itoa(port)}); err != nil {
		t.Fatalf("NcCmd scan returned error: %v", err)
	}
}

func TestCurlCmdBasic(t *testing.T) {
	server := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	ts := httptest.NewServer(server)
	defer ts.Close()

	stdout, _, err := captureOutput(t, func() error {
		return net.CurlCmd([]string{"-s", ts.URL})
	})
	if err != nil {
		t.Fatalf("CurlCmd returned error: %v", err)
	}
	if stdout != "ok" {
		t.Fatalf("CurlCmd expected body ok, got %q", stdout)
	}
}

func TestTwCmdBasic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("tw ok"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	port := freeTCPPort(t)
	http.DefaultServeMux = http.NewServeMux()
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- net.TwCmd([]string{"-p", strconv.Itoa(port), "-d", dir})
	}()

	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/"
	deadline := time.Now().Add(2 * time.Second)
	for {
		select {
		case err := <-serverErr:
			t.Fatalf("TwCmd exited early: %v", err)
		default:
		}

		resp, err := http.Get(url)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				t.Fatalf("read response: %v", readErr)
			}
			if string(body) != "tw ok" {
				t.Fatalf("TwCmd expected body %q, got %q", "tw ok", string(body))
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("TwCmd did not start in time: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestIfstatCmdSingleSample(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ifstat supported only on Linux")
	}
	stdout, _, err := captureOutput(t, func() error {
		return net.IfstatCmd([]string{"-A", "-i", "lo", "-n", "1", "-p", "1"})
	})
	if err != nil {
		t.Fatalf("IfstatCmd returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("Interface")) {
		t.Fatalf("IfstatCmd expected header output, got %q", stdout)
	}
	if !bytes.Contains([]byte(stdout), []byte("lo")) {
		t.Fatalf("IfstatCmd expected loopback interface line, got %q", stdout)
	}
}

func TestNpCmdBasic(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("np supported only on Linux")
	}

	ln, err := stdnet.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*stdnet.TCPAddr).Port
	stdout, stderr, err := captureOutput(t, func() error {
		return net.NpCmd([]string{"-tcp", "-p", strconv.Itoa(port), "-c", "1", "-W", "1", "127.0.0.1"})
	})
	if err != nil {
		t.Fatalf("NpCmd returned error: %v", err)
	}
	if out := stdout + stderr; !bytes.Contains([]byte(out), []byte("Sent=")) && !bytes.Contains([]byte(out), []byte("bytes from")) {
		t.Fatalf("NpCmd expected statistics output, got %q", out)
	}
}

// ============ Disk/System Commands ============

func TestMd5sumCmdBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return disk.Md5sumCmd([]string{file})
	})
	if err != nil {
		t.Fatalf("Md5sumCmd returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("5eb63bbbe01eeed093cb22bb8f5acdc3")) {
		t.Fatalf("Md5sumCmd expected digest output, got %q", stdout)
	}
}

func TestMd5sumCmdCheck(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	content := "hello world"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	checksum := "5eb63bbbe01eeed093cb22bb8f5acdc3"
	checkFile := filepath.Join(dir, "check.md5")
	if err := os.WriteFile(checkFile, []byte(checksum+"  "+filepath.Base(file)+"\n"), 0o644); err != nil {
		t.Fatalf("write check file: %v", err)
	}
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })
	stdout, _, err := captureOutput(t, func() error {
		return disk.Md5sumCmd([]string{"-c", checkFile})
	})
	if err != nil {
		t.Fatalf("Md5sumCmd -c returned error: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte("OK")) {
		t.Fatalf("Md5sumCmd -c expected OK status, got %q", stdout)
	}
}

func TestIoperfCmdBasic(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ioperf supported only on Linux")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "io.dat")
	stdout, _, err := captureOutput(t, func() error {
		return disk.IoperfCmd([]string{"--rw=write", "--filename=" + file, "--size=32K", "--bs=4k"})
	})
	if err != nil {
		t.Fatalf("IoperfCmd returned error: %v", err)
	}
	if _, statErr := os.Stat(file); statErr != nil {
		t.Fatalf("IoperfCmd did not create output file: %v", statErr)
	}
	if !bytes.Contains([]byte(stdout), []byte("WRITE:")) {
		t.Fatalf("IoperfCmd expected write stats, got %q", stdout)
	}
}

func TestRandCmdBasic(t *testing.T) {
	stdout, _, err := captureOutput(t, func() error {
		return text.RandCmd([]string{})
	})
	if err != nil {
		t.Fatalf("RandCmd returned error: %v", err)
	}
	if len(bytes.TrimSpace([]byte(stdout))) == 0 {
		t.Fatalf("RandCmd expected non-empty output")
	}
}

func TestRandCmdHexOutput(t *testing.T) {
	stdout, _, err := captureOutput(t, func() error {
		return text.RandCmd([]string{"-n", "16"})
	})
	if err != nil {
		t.Fatalf("RandCmd -n returned error: %v", err)
	}
	if len(bytes.TrimSpace([]byte(stdout))) != 32 {
		t.Fatalf("RandCmd -n expected 16-byte hex output, got %q", stdout)
	}
}

func TestSeqCmdBasic(t *testing.T) {
	stdout, _, err := captureOutput(t, func() error {
		return text.SeqCmd([]string{"1", "5"})
	})
	if err != nil {
		t.Fatalf("SeqCmd returned error: %v", err)
	}
	if stdout != "1\n2\n3\n4\n5\n" {
		t.Fatalf("SeqCmd expected ascending sequence, got %q", stdout)
	}
}

func TestSeqCmdWithFormat(t *testing.T) {
	stdout, _, err := captureOutput(t, func() error {
		return text.SeqCmd([]string{"-f", "%02.0f", "1", "5"})
	})
	if err != nil {
		t.Fatalf("SeqCmd -f returned error: %v", err)
	}
	if stdout != "01\n02\n03\n04\n05\n" {
		t.Fatalf("SeqCmd -f expected formatted sequence, got %q", stdout)
	}
}

func TestSeqCmdWithSeparator(t *testing.T) {
	stdout, _, err := captureOutput(t, func() error {
		return text.SeqCmd([]string{"-s", ",", "1", "3"})
	})
	if err != nil {
		t.Fatalf("SeqCmd -s returned error: %v", err)
	}
	if stdout != "1,2,3\n" {
		t.Fatalf("SeqCmd -s expected comma-separated output, got %q", stdout)
	}
}

func TestNewPlannedTextCommandsBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(file, []byte("hello\x00world"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return text.Base64Cmd([]string{"-w", "0", file})
	})
	if err != nil || strings.TrimSpace(stdout) == "" {
		t.Fatalf("Base64Cmd failed stdout=%q err=%v", stdout, err)
	}
	stdout, _, err = captureOutput(t, func() error {
		return text.HexCmd([]string{"--encode", file})
	})
	if err != nil || !strings.Contains(stdout, "68656c6c6f") {
		t.Fatalf("HexCmd --encode failed stdout=%q err=%v", stdout, err)
	}
	stdout, _, err = captureOutput(t, func() error {
		return text.StringsCmd([]string{"-n", "5", file})
	})
	if err != nil || !strings.Contains(stdout, "hello") {
		t.Fatalf("StringsCmd failed stdout=%q err=%v", stdout, err)
	}
	_, _, err = captureOutput(t, func() error {
		return text.DiffCmd([]string{file, file})
	})
	if err != nil {
		t.Fatalf("DiffCmd equal files returned error: %v", err)
	}
}

func TestNewPlannedFsProcNetDiskCommandsBasic(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("new planned command smoke tests require Linux")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout, _, err := captureOutput(t, func() error {
		return fs.StatCmd([]string{"-c", "%s", file})
	})
	if err != nil || strings.TrimSpace(stdout) != "5" {
		t.Fatalf("StatCmd failed stdout=%q err=%v", stdout, err)
	}
	if err := fs.TruncateCmd([]string{"-s", "2", file}); err != nil {
		t.Fatalf("TruncateCmd returned error: %v", err)
	}
	stdout, _, err = captureOutput(t, func() error {
		return fs.DfCmd([]string{dir})
	})
	if err != nil || !strings.Contains(stdout, "Filesystem") {
		t.Fatalf("DfCmd failed stdout=%q err=%v", stdout, err)
	}
	stdout, _, err = captureOutput(t, func() error {
		return proc.FreeCmd([]string{"-m"})
	})
	if err != nil || !strings.Contains(stdout, "Mem:") {
		t.Fatalf("FreeCmd failed stdout=%q err=%v", stdout, err)
	}
	stdout, _, err = captureOutput(t, func() error {
		return net.IpCmd([]string{"addr"})
	})
	if err != nil || !strings.Contains(stdout, "lo") {
		t.Fatalf("IpCmd failed stdout=%q err=%v", stdout, err)
	}
	stdout, _, err = captureOutput(t, func() error {
		return disk.Sha256sumCmd([]string{file})
	})
	if err != nil || !strings.Contains(stdout, "372f7e2f") {
		t.Fatalf("Sha256sumCmd failed stdout=%q err=%v", stdout, err)
	}
}
