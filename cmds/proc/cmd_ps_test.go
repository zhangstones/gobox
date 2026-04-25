package proc

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func captureProcOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()

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
	defer rOut.Close()
	defer rErr.Close()

	os.Stdout = wOut
	os.Stderr = wErr
	runErr := fn()
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, rOut)
	_, _ = io.Copy(&buf, rErr)
	return buf.String(), runErr
}

func TestPsCmdFullFormatShowsExecutable(t *testing.T) {
	output, err := captureProcOutput(t, func() error {
		return PsCmd([]string{"-f", "-n", "3", "-i", "1"})
	})
	if err != nil {
		t.Fatalf("PsCmd failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and at least one process line, got %q", output)
	}
	for _, want := range []string{"UID", "PID", "PPID", "CMD"} {
		if !strings.Contains(lines[0], want) {
			t.Fatalf("expected full-format header with %s, got %q", want, lines[0])
		}
	}
	if len(strings.Fields(lines[1])) < 8 {
		t.Fatalf("expected full-format row with core columns, got %q", lines[1])
	}
}

func TestPsCmdHelpPrefersCanonicalFlags(t *testing.T) {
	output, err := captureProcOutput(t, func() error {
		return PsCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("PsCmd help failed: %v", err)
	}
	for _, want := range []string{"--sort FIELD", "-maxcmd N", "-ww", "Compatibility:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected help to contain %q, got %q", want, output)
		}
	}
	for _, unwanted := range []string{"-sort string", "  -long\n"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("expected help to hide %q, got %q", unwanted, output)
		}
	}
}

func TestRenderPSCommandSanitizesNewlines(t *testing.T) {
	got := renderPSCommand("bash -c \"printf 'a\\nb'\"\nnext", "", 0)
	if strings.Contains(got, "\n") || strings.Contains(got, "\r") {
		t.Fatalf("expected sanitized single-line command, got %q", got)
	}
	if !strings.Contains(got, "printf") || !strings.Contains(got, "next") {
		t.Fatalf("expected sanitized command to retain content, got %q", got)
	}
}

func TestFitPSRowsToWidthTruncatesLastColumn(t *testing.T) {
	headers := []string{"UID", "PID", "PPID", "C", "STIME", "TTY", "TIME", "CMD"}
	rows := [][]string{{"root", "123", "1", "0", "12:00", "pts/0", "00:00", "bash -c this-is-a-very-long-command-line"}}
	fitted := fitPSRowsToWidth(headers, rows, 40)
	if len(fitted) != 1 || fitted[0][7] == rows[0][7] {
		t.Fatalf("expected command column truncation, got %q", fitted)
	}
	if !strings.HasSuffix(fitted[0][7], "...") {
		t.Fatalf("expected ellipsis after width truncation, got %q", fitted[0][7])
	}
}

func TestFitPSRowsToWidthLeavesNonTTYRowsUntouched(t *testing.T) {
	headers := []string{"UID", "PID", "PPID", "C", "STIME", "TTY", "TIME", "CMD"}
	rows := [][]string{{"root", "123", "1", "0", "12:00", "pts/0", "00:00", "bash -c this-is-a-very-long-command-line"}}
	fitted := fitPSRowsToWidth(headers, rows, 0)
	if len(fitted) != 1 || fitted[0][7] != rows[0][7] {
		t.Fatalf("expected non-tty/default width path to preserve full command, got %q", fitted)
	}
}

func TestPsCmdLengthLimitAppliesWithoutTTY(t *testing.T) {
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	filter := filepath.Base(exePath)

	output, err := captureProcOutput(t, func() error {
		return PsCmd([]string{"-full", filter, "-n", "1", "-maxcmd", "8", "-i", "1", "--sort", "pid", "-r"})
	})
	if err != nil {
		t.Fatalf("PsCmd failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected truncated process output, got %q", output)
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		t.Fatalf("unexpected process row format: %q", lines[1])
	}
	cmd := strings.Join(fields[4:], " ")
	if len([]rune(cmd)) > 8 {
		t.Fatalf("expected command to respect -maxcmd 8, got %q", cmd)
	}
	if !strings.Contains(cmd, "...") {
		t.Fatalf("expected truncated command with ellipsis, got %q", cmd)
	}
}

func TestNormalizePSArgsKeepsLegacyMaxCmdAlias(t *testing.T) {
	args, bsdAux := normalizePSArgs([]string{"-l", "12", "--sort", "pid"})
	if bsdAux {
		t.Fatalf("unexpected aux detection")
	}
	got := strings.Join(args, " ")
	if got != "-maxcmd 12 --sort pid" {
		t.Fatalf("expected legacy -l N to normalize to -maxcmd N, got %q", got)
	}
}

func TestTopCmdNonTTYOutputDoesNotEmitClearScreen(t *testing.T) {
	output, err := captureProcOutput(t, func() error {
		return TopCmd([]string{"-n", "1", "-d", "0"})
	})
	if err != nil {
		t.Fatalf("TopCmd failed: %v", err)
	}
	if strings.Contains(output, "\x1b[H\x1b[2J") {
		t.Fatalf("expected no clear-screen escape in non-tty output, got %q", output)
	}
	for _, want := range []string{"PID", "%CPU", "CMD"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected top output to include %s in the process header, got %q", want, output)
		}
	}
}

func TestPsCmdNameFilterMatchesPgrepStyleRegex(t *testing.T) {
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	base := filepath.Base(exePath)
	if len(base) < 3 {
		t.Fatalf("unexpected executable name %q", base)
	}
	filter := regexp.QuoteMeta(base[:3]) + ".*"

	output, err := captureProcOutput(t, func() error {
		return PsCmd([]string{"-full", filter, "-n", "20", "-i", "1"})
	})
	if err != nil {
		t.Fatalf("PsCmd failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least one matching process, got %q", output)
	}
	found := false
	for _, line := range lines[1:] {
		if strings.Contains(line, base[:3]) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected at least one row to match regex %q, got %q", filter, output)
	}
}

func TestTruncateString(t *testing.T) {
	if got := truncateString("hello", 0); got != "hello" {
		t.Fatalf("expected no truncation, got %q", got)
	}
	if got := truncateString("hello", 3); got != "hel" {
		t.Fatalf("expected hard truncation to 3, got %q", got)
	}
	if got := truncateString("hello", 4); got != "h..." {
		t.Fatalf("expected ellipsis truncation, got %q", got)
	}
	if got := truncateString("hi", 5); got != "hi" {
		t.Fatalf("expected no truncation, got %q", got)
	}
}

func TestPsCmdWideWideDisablesTruncation(t *testing.T) {
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	filter := filepath.Base(exePath)

	output, err := captureProcOutput(t, func() error {
		return PsCmd([]string{"-full", filter, "-n", "1", "-maxcmd", "4", "-ww", "-i", "1", "--sort", "pid", "-r"})
	})
	if err != nil {
		t.Fatalf("PsCmd failed: %v", err)
	}
	if strings.Contains(output, "...") {
		t.Fatalf("expected -ww to disable truncation, got %q", output)
	}
}

func TestPsCmdCustomOutputFields(t *testing.T) {
	output, err := captureProcOutput(t, func() error {
		return PsCmd([]string{"-o", "pid,ppid,cmd,pcpu,pmem", "-n", "1", "-i", "1"})
	})
	if err != nil {
		t.Fatalf("PsCmd failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected custom output header and row, got %q", output)
	}
	header := lines[0]
	for _, field := range []string{"PID", "PPID", "CMD", "%CPU", "%MEM"} {
		if !strings.Contains(header, field) {
			t.Fatalf("expected custom header to contain %s, got %q", field, header)
		}
	}
}

func TestPrintCustomPSAlignsColumns(t *testing.T) {
	infos := []procInfo{
		{pid: 7, ppid: 1, user: "root", cpu: 1.2, rss: 4096, vsize: 8192, cmdline: "sleep 10", start: time.Unix(0, 0)},
		{pid: 12345, ppid: 999, user: "verylongusername", cpu: 12.3, rss: 10240, vsize: 20480, cmdline: "a much longer command", start: time.Unix(0, 0)},
	}
	out, err := captureProcOutput(t, func() error {
		printCustomPS(infos, []string{"user", "pid", "ppid", "args"}, 0, 0, 0)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %q", out)
	}
	headerPID := strings.Index(lines[0], "PID")
	row1PID := strings.Index(lines[1], "7")
	row2PID := strings.Index(lines[2], "12345")
	if headerPID == -1 || row1PID != headerPID || row2PID != headerPID {
		t.Fatalf("expected PID column alignment, got %q", out)
	}
}

func TestPsCmdGNUCompatibilityFiltersAndFields(t *testing.T) {
	pid := os.Getpid()
	output, err := captureProcOutput(t, func() error {
		return PsCmd([]string{"-A", "-p", strconv.Itoa(pid), "-o", "pid,ppid,stat,etime,time,rss,vsz,args", "--sort", "-pid", "-ww", "-i", "1"})
	})
	if err != nil {
		t.Fatalf("PsCmd failed: %v", err)
	}
	if !strings.Contains(output, strconv.Itoa(pid)) {
		t.Fatalf("expected filtered output to contain pid %d, got %q", pid, output)
	}
	for _, field := range []string{"PID", "PPID", "STAT", "ELAPSED", "TIME", "RSS", "VSZ", "CMD"} {
		if !strings.Contains(output, field) {
			t.Fatalf("expected custom header to contain %s, got %q", field, output)
		}
	}
}

func TestPsCmdBSDStyleAux(t *testing.T) {
	output, err := captureProcOutput(t, func() error {
		return PsCmd([]string{"aux", "-n", "1", "-i", "1"})
	})
	if err != nil {
		t.Fatalf("PsCmd failed: %v", err)
	}
	for _, field := range []string{"USER", "PID", "%CPU", "%MEM", "VSZ", "RSS", "TTY", "STAT", "START", "TIME", "CMD"} {
		if !strings.Contains(output, field) {
			t.Fatalf("expected aux header to contain %s, got %q", field, output)
		}
	}
}

func TestTopCmdBatchFiltersAndSorts(t *testing.T) {
	output, err := captureProcOutput(t, func() error {
		return TopCmd([]string{"-b", "-H", "-n", "1", "-d", "0", "-p", strconv.Itoa(os.Getpid()), "-o", "%cpu", "-c"})
	})
	if err != nil {
		t.Fatalf("TopCmd failed: %v", err)
	}
	if strings.Contains(output, "\x1b[H\x1b[2J") {
		t.Fatalf("expected batch output without clear-screen escape, got %q", output)
	}
	if !strings.Contains(output, strconv.Itoa(os.Getpid())) {
		t.Fatalf("expected top output to include current pid, got %q", output)
	}
}
