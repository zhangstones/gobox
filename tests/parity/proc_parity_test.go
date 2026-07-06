package main

import (
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gobox/cmds/proc"
)

func columnIndex(line, name string) int {
	for i, field := range strings.Fields(line) {
		if field == name {
			return i
		}
	}
	return -1
}

func rowFieldsByPID(out string, pid string) ([]string, bool) {
	for _, line := range nonEmptyLines(out)[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == pid {
			return fields, true
		}
	}
	return nil, false
}

func extractColumnByPID(out string, pid string, idx int) (string, bool) {
	fields, ok := rowFieldsByPID(out, pid)
	if !ok || idx < 0 || idx >= len(fields) {
		return "", false
	}
	return fields[idx], true
}

func topHeaderIndex(out string) int {
	lines := nonEmptyLines(out)
	for i, line := range lines {
		if strings.Contains(line, "PID") && strings.Contains(strings.ToUpper(line), "COMMAND") {
			return i
		}
	}
	return -1
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func topProcessLines(out string) []string {
	headerIdx := topHeaderIndex(out)
	if headerIdx < 0 {
		return nil
	}
	lines := nonEmptyLines(out)
	if headerIdx+1 >= len(lines) {
		return nil
	}
	return lines[headerIdx+1:]
}

func runWatchCapture(t *testing.T, timeout time.Duration, args ...string) string {
	t.Helper()
	var out strings.Builder
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err = proc.WatchCmdWithContext(ctx, args)
	_ = w.Close()
	os.Stdout = old
	_, _ = io.Copy(&out, r)
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func watchPayloadCount(out, payload string) int {
	count := 0
	for _, line := range nonEmptyLines(out) {
		if strings.TrimSpace(line) == payload {
			count++
		}
	}
	return count
}

func freeRowFields(t *testing.T, out, prefix string) []string {
	t.Helper()
	line := findLineWithPrefix(out, prefix)
	if line == "" {
		t.Fatalf("missing %s row\n%s", prefix, out)
	}
	fields := strings.Fields(line)
	if len(fields) < 4 {
		t.Fatalf("%s row too short: %q", prefix, line)
	}
	return fields
}

func holdLockedOSThreads(t *testing.T, n int) func() {
	t.Helper()
	if n <= 0 {
		return func() {}
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			defer wg.Done()
			<-stop
		}()
	}
	time.Sleep(150 * time.Millisecond)
	return func() {
		close(stop)
		wg.Wait()
	}
}

func lsofHeaderAndRows(out string) (string, []string) {
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return "", nil
	}
	return lines[0], lines[1:]
}

func lsofFindRow(rows []string, needle string) string {
	for _, line := range rows {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func TestParity_XargsCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{
			ID:            "XARGS-001",
			Name:          "xargs -I",
			GoboxArgs:     []string{"xargs", "-I", "{}", "echo", "pre:{}:post"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-I", "{}", "echo", "pre:{}:post"},
			Stdin:         "abc\n",
		},
		{
			ID:            "XARGS-002",
			Name:          "xargs -i",
			GoboxArgs:     []string{"xargs", "-i{}", "echo", "pre:{}:post"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-i{}", "echo", "pre:{}:post"},
			Stdin:         "abc\n",
		},
		{
			ID:            "XARGS-003",
			Name:          "xargs -d",
			GoboxArgs:     []string{"xargs", "-d", ",", "echo"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-d", ",", "echo"},
			Stdin:         "a,b,c",
		},
		{
			ID:            "XARGS-004",
			Name:          "xargs -n",
			GoboxArgs:     []string{"xargs", "-n", "2", "echo"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-n", "2", "echo"},
			Stdin:         "a\nb\nc\n",
		},
		{
			ID:            "XARGS-006",
			Name:          "xargs -r",
			GoboxArgs:     []string{"xargs", "-r", "echo"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-r", "echo"},
			Stdin:         "",
		},
		{
			ID:            "XARGS-007",
			Name:          "xargs -t",
			GoboxArgs:     []string{"xargs", "-t", "echo"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-t", "echo"},
			Stdin:         "a\n",
			Normalize:     normalizeText,
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("xargs -t exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
				}
				if gobox.Stderr != native.Stderr {
					t.Fatalf("xargs -t stderr mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stderr, native.Stderr)
				}
				if gobox.Stdout != native.Stdout {
					t.Fatalf("xargs -t stdout mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
				}
			},
		},
		{
			ID:            "XARGS-008",
			Name:          "xargs -v alias",
			GoboxArgs:     []string{"xargs", "-v", "echo"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-t", "echo"},
			Stdin:         "a\n",
			Normalize:     normalizeText,
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("xargs -v exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
				}
				if gobox.Stderr != native.Stderr {
					t.Fatalf("xargs -v stderr mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stderr, native.Stderr)
				}
				if gobox.Stdout != native.Stdout {
					t.Fatalf("xargs -v stdout mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
				}
			},
		},
	})

	t.Run("XARGS-005", func(t *testing.T) {
		env := t.TempDir()
		script := filepath.Join(env, "emit.sh")
		writeFile(t, script, "#!/bin/sh\nprintf '%s\\n' \"$1\"\n")
		if err := os.Chmod(script, 0o755); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		gobox := runGoboxCLI(t, env, "a\nb\nc\n", "xargs", "-P", "2", "-n", "1", "./emit.sh")
		native := runNativeCLI(t, env, "a\nb\nc\n", "xargs", "-P", "2", "-n", "1", "./emit.sh")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("xargs -P exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if !sameStringSet(gobox.Stdout, native.Stdout) {
			t.Fatalf("xargs -P output set mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
	})
}

func TestParity_TopCases(t *testing.T) {
	assertTopBatchOutput := func(t *testing.T, label string, res parityResult) {
		t.Helper()
		if res.ExitCode != 0 {
			t.Fatalf("%s failed: %+v", label, res)
		}
		if strings.Contains(res.Stdout, "\x1b[H\x1b[2J") {
			t.Fatalf("%s emitted clear-screen sequence in batch output: %q", label, res.Stdout)
		}
		lines := nonEmptyLines(res.Stdout)
		if len(lines) < 7 {
			t.Fatalf("%s expected at least summary + process table lines, got %q", label, res.Stdout)
		}
		for _, want := range []string{"top - ", "Tasks:", "%Cpu(s):", "MiB Mem :", "MiB Swap:"} {
			if line := findLineWithPrefix(res.Stdout, want); line == "" {
				t.Fatalf("%s missing summary line %q\n%s", label, want, res.Stdout)
			}
		}
		headerIdx := topHeaderIndex(res.Stdout)
		if headerIdx < 0 {
			t.Fatalf("%s missing process table header\n%s", label, res.Stdout)
		}
		header := strings.ToUpper(nonEmptyLines(res.Stdout)[headerIdx])
		for _, want := range []string{"PID", "%CPU"} {
			if !strings.Contains(header, want) {
				t.Fatalf("%s missing process field %q in header %q\n%s", label, want, header, res.Stdout)
			}
		}
		hasMemoryField := strings.Contains(header, "%MEM") || strings.Contains(header, "RSS") || strings.Contains(header, "VIRT") || strings.Contains(header, "VMS")
		if !hasMemoryField {
			t.Fatalf("%s missing memory-related field\n%s", label, res.Stdout)
		}
	}

	t.Run("TOP-001", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "top", "-b", "-n", "1", "-d", "0")
		native := runNativeCLI(t, env, "", "top", "-b", "-n", "1", "-d", "0")
		if res.ExitCode != native.ExitCode {
			t.Fatalf("top -d mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		assertTopBatchOutput(t, "gobox top baseline", res)
		assertTopBatchOutput(t, "native top baseline", native)
	})
	for _, tc := range []struct {
		id         string
		args       []string
		nativeArgs []string
	}{
		{"TOP-003", []string{"top", "-b", "-n", "1", "-d", "0", "-r"}, []string{"-b", "-n", "1", "-d", "0"}},
		{"TOP-004", []string{"top", "-b", "-n", "1", "-d", "0", "--sort", "pid"}, []string{"-b", "-n", "1", "-d", "0"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			env := t.TempDir()
			base := runGoboxCLI(t, env, "", "top", "-b", "-n", "1", "-d", "0")
			res := runGoboxCLI(t, env, "", tc.args...)
			native := runNativeCLI(t, env, "", "top", tc.nativeArgs...)
			if res.ExitCode != native.ExitCode || topHeaderIndex(res.Stdout) < 0 || topHeaderIndex(native.Stdout) < 0 {
				t.Fatalf("%s mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", tc.id, res, native)
			}
			if base.ExitCode != 0 {
				t.Fatalf("%s base top failed: %+v", tc.id, base)
			}
			basePIDs := extractLeadingInts(topProcessLines(base.Stdout))
			resPIDs := extractLeadingInts(topProcessLines(res.Stdout))
			if len(basePIDs) == 0 || len(resPIDs) == 0 {
				t.Fatalf("%s expected process rows in both baseline and variant\n--- base ---\n%s\n--- variant ---\n%s", tc.id, base.Stdout, res.Stdout)
			}
			switch tc.id {
			case "TOP-003":
				// top default sort is by %CPU (descending); -r must reverse
				// that to ascending. Comparing row counts or PID sets across
				// two separate top invocations is inherently racy on a live
				// host (process churn between the two snapshots), so verify
				// each invocation's own %CPU column monotonicity instead --
				// that is a property of a single sort call and doesn't
				// depend on the two snapshots agreeing on which processes
				// exist (TEST-DESIGN.md §11.2: sort params must be verified
				// by real column value, not row/PID-set comparison).
				baseHeaderIdx := topHeaderIndex(base.Stdout)
				resHeaderIdx := topHeaderIndex(res.Stdout)
				baseLines := nonEmptyLines(base.Stdout)
				resLines := nonEmptyLines(res.Stdout)
				// gobox's top wraps the active sort column's header token in
				// brackets (e.g. "[%CPU]"); strip that so the plain "%CPU"
				// header lookup below matches regardless of which column is
				// currently the active sort key.
				unbracketHeader := func(h string) string {
					return strings.NewReplacer("[%CPU]", "%CPU", "[%MEM]", "%MEM", "[PID]", "PID").Replace(h)
				}
				baseCPU := extractFloatColumn(t, unbracketHeader(baseLines[baseHeaderIdx]), baseLines[baseHeaderIdx+1:], "%CPU")
				resCPU := extractFloatColumn(t, unbracketHeader(resLines[resHeaderIdx]), resLines[resHeaderIdx+1:], "%CPU")
				if len(baseCPU) < 2 || len(resCPU) < 2 {
					t.Skip("TOP-003: not enough distinct %CPU values in this environment to prove sort direction")
				}
				assertMonotonicFloat(t, baseCPU, true)
				assertMonotonicFloat(t, resCPU, false)
			case "TOP-004":
				assertMonotonic(t, resPIDs, true)
			}
		})
	}

	t.Run("TOP-002", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "top", "-b", "-n", "1", "-d", "0")
		native := runNativeCLI(t, env.Dir, "", "top", "-b", "-n", "1", "-d", "0")
		if res.ExitCode != native.ExitCode {
			t.Fatalf("top -n 1 mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		assertTopBatchOutput(t, "gobox top -n 1", res)
		assertTopBatchOutput(t, "native top -n 1", native)
		if strings.Contains(res.Stdout, "\x1b[H\x1b[2J") || strings.Contains(native.Stdout, "\x1b[H\x1b[2J") {
			t.Fatalf("top -n 1 should not clear screen in batch mode\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
	})

	for _, tc := range []struct {
		id       string
		args     []string
		contains []string
	}{
		{"TOP-005", []string{"top", "-b", "-n", "1", "-d", "0"}, []string{"PID"}},
		{"TOP-006", []string{"top", "-b", "-n", "1", "-d", "0", "-p", strconv.Itoa(os.Getpid())}, []string{strconv.Itoa(os.Getpid())}},
		{"TOP-007", []string{"top", "-b", "-n", "1", "-d", "0", "-u", strconv.Itoa(os.Getuid())}, []string{"PID"}},
		{"TOP-008", []string{"top", "-b", "-n", "1", "-d", "0", "-H"}, []string{"PID"}},
		{"TOP-009", []string{"top", "-b", "-n", "1", "-d", "0", "-i"}, []string{"PID"}},
		{"TOP-010", []string{"top", "-b", "-n", "1", "-d", "0", "-c"}, []string{"PID"}},
		{"TOP-011", []string{"top", "-b", "-n", "1", "-d", "0", "-o", "PID"}, []string{"PID"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			env := t.TempDir()
			base := runGoboxCLI(t, env, "", "top", "-b", "-n", "1", "-d", "0")
			res := runGoboxCLI(t, env, "", tc.args...)
			nativeArgs := append([]string{}, tc.args[1:]...)
			native := runNativeCLI(t, env, "", "top", nativeArgs...)
			if res.ExitCode != 0 {
				t.Fatalf("%s failed: %+v", tc.id, res)
			}
			if native.ExitCode != 0 {
				t.Fatalf("%s native top failed: %+v", tc.id, native)
			}
			if strings.Contains(res.Stdout, "\x1b[H\x1b[2J") {
				t.Fatalf("%s emitted clear-screen sequence in batch output: %q", tc.id, res.Stdout)
			}
			processLines := topProcessLines(res.Stdout)
			if tc.id != "TOP-009" && len(processLines) == 0 {
				t.Fatalf("%s expected at least one process row\n%s", tc.id, res.Stdout)
			}
			if tc.id != "TOP-005" && tc.id != "TOP-007" && tc.id != "TOP-008" && base.ExitCode == 0 && base.Stdout == res.Stdout {
				t.Fatalf("%s did not change gobox output relative to baseline\n--- base ---\n%s\n--- variant ---\n%s", tc.id, base.Stdout, res.Stdout)
			}
			switch tc.id {
			case "TOP-005":
				assertTopBatchOutput(t, tc.id, res)
			case "TOP-006":
				pids := extractLeadingInts(processLines)
				if len(pids) != 1 || pids[0] != os.Getpid() {
					t.Fatalf("%s should keep only target pid %d, got %v\n%s", tc.id, os.Getpid(), pids, res.Stdout)
				}
			case "TOP-007":
				currentUser, err := user.LookupId(strconv.Itoa(os.Getuid()))
				if err != nil {
					t.Fatalf("lookup current user: %v", err)
				}
				headerLine := nonEmptyLines(res.Stdout)[topHeaderIndex(res.Stdout)]
				userIdx := columnIndex(headerLine, "USER")
				if userIdx < 0 {
					t.Fatalf("%s missing USER column in header: %q", tc.id, headerLine)
				}
				for _, line := range processLines {
					fields := strings.Fields(line)
					if len(fields) <= userIdx || fields[userIdx] != currentUser.Username {
						t.Fatalf("%s returned row for unexpected user: %q", tc.id, line)
					}
				}
			case "TOP-009":
				idleCmd := exec.Command("sleep", "30")
				if err := idleCmd.Start(); err != nil {
					t.Fatalf("start idle process: %v", err)
				}
				defer stopCmd(idleCmd)
				filterBase := runGoboxCLI(t, env, "", "top", "-b", "-n", "1", "-d", "0", "-p", strconv.Itoa(idleCmd.Process.Pid))
				filtered := runGoboxCLI(t, env, "", "top", "-b", "-n", "1", "-d", "0", "-i", "-p", strconv.Itoa(idleCmd.Process.Pid))
				if len(topProcessLines(filterBase.Stdout)) == 0 {
					t.Fatalf("%s baseline -p output should include idle target\n%s", tc.id, filterBase.Stdout)
				}
				if len(topProcessLines(filtered.Stdout)) >= len(topProcessLines(filterBase.Stdout)) {
					t.Fatalf("%s should hide idle pid %d from filtered output\n--- base ---\n%s\n--- filtered ---\n%s", tc.id, idleCmd.Process.Pid, filterBase.Stdout, filtered.Stdout)
				}
			case "TOP-008":
				releaseThreads := holdLockedOSThreads(t, 3)
				defer releaseThreads()
				threaded := runGoboxCLI(t, env, "", "top", "-b", "-n", "1", "-d", "0", "-H", "-p", strconv.Itoa(os.Getpid()))
				assertTopBatchOutput(t, tc.id, threaded)
				got := extractLeadingInts(topProcessLines(threaded.Stdout))
				if len(got) < 2 {
					t.Fatalf("%s should expose multiple thread IDs for the current process, got %v\n%s", tc.id, got, threaded.Stdout)
				}
				seenNonMainThread := false
				for _, tid := range got {
					if tid != os.Getpid() {
						seenNonMainThread = true
						break
					}
				}
				if !seenNonMainThread {
					t.Fatalf("%s should render at least one non-main thread ID, got %v\n%s", tc.id, got, threaded.Stdout)
				}
			case "TOP-010":
				markerCmd := startMarkerProcess(t, "top-longcmd")
				defer stopCmd(markerCmd)
				shortOut := runGoboxCLI(t, env, "", "top", "-b", "-n", "1", "-d", "0", "-p", strconv.Itoa(markerCmd.Process.Pid))
				longOut := runGoboxCLI(t, env, "", "top", "-b", "-n", "1", "-d", "0", "-c", "-p", strconv.Itoa(markerCmd.Process.Pid))
				shortRow, ok := rowFieldsByPID(shortOut.Stdout, strconv.Itoa(markerCmd.Process.Pid))
				if !ok {
					t.Fatalf("%s baseline -p output missing marker pid %d\n%s", tc.id, markerCmd.Process.Pid, shortOut.Stdout)
				}
				longRow, ok := rowFieldsByPID(longOut.Stdout, strconv.Itoa(markerCmd.Process.Pid))
				if !ok {
					t.Fatalf("%s -c output missing marker pid %d\n%s", tc.id, markerCmd.Process.Pid, longOut.Stdout)
				}
				if strings.Join(longRow, " ") == strings.Join(shortRow, " ") {
					t.Fatalf("%s should expose a fuller command line than baseline\n--- base ---\n%s\n--- long ---\n%s", tc.id, shortOut.Stdout, longOut.Stdout)
				}
				longHeaderLine := nonEmptyLines(longOut.Stdout)[topHeaderIndex(longOut.Stdout)]
				cmdIdx := -1
				for i, f := range strings.Fields(longHeaderLine) {
					if f == "COMMAND" || f == "CMD" {
						cmdIdx = i
						break
					}
				}
				if cmdIdx < 0 {
					t.Fatalf("%s missing CMD/COMMAND column in header: %q", tc.id, longHeaderLine)
				}
				cmdPart := strings.Join(longRow[cmdIdx:], " ")
				if !strings.Contains(cmdPart, "30") {
					t.Fatalf("%s CMD field should contain sleep duration '30'\n--- long ---\n%s", tc.id, longOut.Stdout)
				}
			case "TOP-011":
				pids := extractLeadingInts(processLines)
				assertMonotonic(t, pids, true)
				nativePIDs := extractLeadingInts(topProcessLines(native.Stdout))
				assertMonotonic(t, nativePIDs, true)
			}
			for _, want := range tc.contains {
				if !strings.Contains(res.Stdout, want) {
					t.Fatalf("%s missing %q in %q", tc.id, want, res.Stdout)
				}
			}
		})
	}
}

func TestParity_PsCases(t *testing.T) {
	t.Run("PS-001", func(t *testing.T) {
		// The core assertion for PS-001 is that the *default* CMD column
		// (i.e. without -o overriding the column set) renders comm-style
		// (short executable name), matching native `ps -e`'s default
		// convention -- not the full argv/cmdline. startMarkerProcess uses
		// `exec -a parity-ps-001 sleep 30`: this only rewrites argv[0], so
		// /proc/<pid>/comm (and hence gobox's comm-style CMD) stays "sleep",
		// while the full cmdline would read "parity-ps-001 30". Passing
		// `-o pid,cmd` here would defeat the point, since the "cmd" output
		// field always renders full cmdline (see cmds/proc renderPSField),
		// so we must NOT pass -o for this case.
		markerCmd := startMarkerProcess(t, "parity-ps-001")
		defer stopCmd(markerCmd)
		env := t.TempDir()
		markerPID := strconv.Itoa(markerCmd.Process.Pid)
		// -n is intentionally omitted (0 = show all): a fixed cap like -n 50
		// would non-deterministically exclude the marker PID depending on
		// host process count and sort order, unrelated to what -e/CMD
		// formatting actually tests.
		gobox := runGoboxCLI(t, env, "", "ps", "-e", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-e", "-o", "pid,comm")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps -e failed gobox=%+v native=%+v", gobox, native)
		}
		goboxLines := nonEmptyLines(gobox.Stdout)
		nativeLines := nonEmptyLines(native.Stdout)
		if len(goboxLines) < 2 || len(nativeLines) < 2 {
			t.Fatalf("ps -e output too short\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
		goboxCmd, ok := extractColumnByPIDHeader(gobox.Stdout, markerPID, "CMD")
		if !ok {
			t.Fatalf("gobox ps -e missing marker pid %s\n%s", markerPID, gobox.Stdout)
		}
		nativeComm, ok := extractColumnByPIDHeader(native.Stdout, markerPID, "COMMAND")
		if !ok {
			t.Fatalf("native ps -e -o pid,comm missing marker pid %s\n%s", markerPID, native.Stdout)
		}
		if goboxCmd != nativeComm {
			t.Fatalf("ps -e default CMD column should match native comm-style rendering for marker pid %s\ngobox=%q\nnative comm=%q", markerPID, goboxCmd, nativeComm)
		}
		if strings.Contains(goboxCmd, "parity-ps-001") {
			t.Fatalf("ps -e default CMD column should be comm-style (executable name), not the rewritten argv0 %q: got %q", "parity-ps-001", goboxCmd)
		}
	})

	t.Run("PS-002", func(t *testing.T) {
		requireNativeCommand(t, "ps")
		env := &parityEnv{Dir: t.TempDir()}
		// -n is intentionally omitted (0 = show all): a fixed cap like -n 5
		// would non-deterministically exclude the current test process
		// (looked up by its real PID below) depending on host process count
		// and sort order.
		base := runGoboxCLI(t, env.Dir, "", "ps", "-i", "1")
		gobox := runGoboxCLI(t, env.Dir, "", "ps", "-f", "-i", "1")
		native := runNativeCLI(t, env.Dir, "", "ps", "-f")
		if base.ExitCode != 0 || gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps command failed base=%+v gobox=%+v native=%+v", base, gobox, native)
		}
		if base.Stdout == gobox.Stdout {
			t.Fatalf("ps -f did not change output\n--- base ---\n%s\n--- -f ---\n%s", base.Stdout, gobox.Stdout)
		}
		goboxLines := nonEmptyLines(gobox.Stdout)
		nativeLines := nonEmptyLines(native.Stdout)
		if len(goboxLines) < 2 || len(nativeLines) < 2 {
			t.Fatalf("ps -f expected header plus rows\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
		if !strings.Contains(goboxLines[0], "PPID") || !strings.Contains(nativeLines[0], "PPID") {
			t.Fatalf("ps -f missing PPID headers\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
		for _, want := range []string{"UID", "PID", "PPID", "CMD"} {
			if !strings.Contains(goboxLines[0], want) {
				t.Fatalf("gobox ps -f header missing %q: %q", want, goboxLines[0])
			}
			if !strings.Contains(nativeLines[0], want) {
				t.Fatalf("native ps -f header missing %q: %q", want, nativeLines[0])
			}
		}
		for _, line := range goboxLines[1:] {
			if len(strings.Fields(line)) < 7 {
				t.Fatalf("gobox ps -f row too short: %q", line)
			}
		}
		// Verify PPID actually carries a real parent PID rather than merely
		// existing as a column name (TEST-DESIGN.md §11.3): the current test
		// process's own PPID must match os.Getppid() in both gobox and
		// native output.
		pid := strconv.Itoa(os.Getpid())
		wantPPID := strconv.Itoa(os.Getppid())
		goboxPPID, ok := extractColumnByPIDHeader(gobox.Stdout, pid, "PPID")
		if !ok {
			t.Fatalf("gobox ps -f missing current process pid %s\n%s", pid, gobox.Stdout)
		}
		if goboxPPID != wantPPID {
			t.Fatalf("gobox ps -f PPID column should carry the real parent pid: got %q want %q", goboxPPID, wantPPID)
		}
		nativePPID, ok := extractColumnByPIDHeader(native.Stdout, pid, "PPID")
		if !ok {
			t.Fatalf("native ps -f missing current process pid %s\n%s", pid, native.Stdout)
		}
		if nativePPID != wantPPID {
			t.Fatalf("native ps -f PPID column should carry the real parent pid: got %q want %q", nativePPID, wantPPID)
		}
	})

	t.Run("PS-003", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "-i", "1", "-n", "2")
		if res.ExitCode != 0 {
			t.Fatalf("ps -i failed: %+v", res)
		}
		lines := nonEmptyLines(res.Stdout)
		if len(lines) < 2 {
			t.Fatalf("ps -i output too short (expected header + data row): %q", res.Stdout)
		}
		if !strings.Contains(lines[0], "PID") {
			t.Fatalf("ps -i header missing PID: %q", lines[0])
		}
		firstDataFields := strings.Fields(lines[1])
		if len(firstDataFields) == 0 {
			t.Fatalf("ps -i first data row is empty: %q", res.Stdout)
		}
		if _, err := strconv.Atoi(firstDataFields[0]); err != nil {
			t.Fatalf("ps -i first data row first field not a numeric PID: %q in %q", firstDataFields[0], lines[1])
		}
	})

	t.Run("PS-004", func(t *testing.T) {
		markerCmd := startMarkerProcess(t, "parity-ps-trunc")
		defer stopCmd(markerCmd)
		pid := strconv.Itoa(markerCmd.Process.Pid)
		const maxCmd = 8
		shortRes := runGoboxCLI(t, t.TempDir(), "", "ps", "--full", "parity-ps-trunc", "-f", "-n", "1", "--maxcmd", strconv.Itoa(maxCmd), "-i", "1")
		wideRes := runGoboxCLI(t, t.TempDir(), "", "ps", "--full", "parity-ps-trunc", "-f", "-n", "1", "-ww", "-i", "1")
		if shortRes.ExitCode != 0 || wideRes.ExitCode != 0 {
			t.Fatalf("ps truncation failed short=%+v wide=%+v", shortRes, wideRes)
		}
		shortCmd, ok := extractColumnByPIDHeader(shortRes.Stdout, pid, "CMD")
		if !ok {
			t.Fatalf("ps --maxcmd missing marker pid %s\n%s", pid, shortRes.Stdout)
		}
		// truncateString(s, 8) yields exactly 8 runes (5 kept + "...") when
		// the source is longer than 8, so the truncated length must exactly
		// equal --maxcmd's value, not merely be "shorter than baseline".
		if len(shortCmd) != maxCmd {
			t.Fatalf("ps --maxcmd %d should truncate CMD to exactly %d characters, got %d: %q", maxCmd, maxCmd, len(shortCmd), shortCmd)
		}
		if !strings.HasSuffix(shortCmd, "...") {
			t.Fatalf("ps --maxcmd %d truncated CMD should end with an ellipsis, got %q", maxCmd, shortCmd)
		}
		wideCmd, ok := extractColumnByPIDHeader(wideRes.Stdout, pid, "CMD")
		if !ok {
			t.Fatalf("ps -ww missing marker pid %s\n%s", pid, wideRes.Stdout)
		}
		if len(wideCmd) <= len(shortCmd) {
			t.Fatalf("expected truncated output to be shorter\nshort=%q\nwide=%q", shortCmd, wideCmd)
		}
	})

	t.Run("PS-005", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "-n", "2", "-i", "1")
		if res.ExitCode != 0 {
			t.Fatalf("ps -n failed: %+v", res)
		}
		// The host always has at least 2 live processes (this test binary
		// plus at least its parent/init), so -n 2 must truncate to exactly
		// header + 2 rows -- not 0 or 1, proving truncation actually
		// happened rather than merely staying under an upper bound.
		if lines := nonEmptyLines(res.Stdout); len(lines) != 3 {
			t.Fatalf("ps -n 2 expected header + exactly 2 rows (3 lines), got %d lines: %q", len(lines), res.Stdout)
		}
	})

	t.Run("PS-006", func(t *testing.T) {
		markerCmd := startMarkerProcess(t, "parity-ps-filter-123")
		defer stopCmd(markerCmd)
		pattern := "parity-ps-filter-[0-9]+"
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "--full", pattern, "-f", "-n", "5", "-ww", "-i", "1")
		native := runNativeCLI(t, t.TempDir(), "", "pgrep", "-f", pattern)
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps --full failed gobox=%+v native=%+v", res, native)
		}
		goboxLines := nonEmptyLines(res.Stdout)
		if len(goboxLines) < 2 {
			t.Fatalf("ps --full expected header plus matching rows\n%s", res.Stdout)
		}
		headerFields := strings.Fields(goboxLines[0])
		pidIdx := -1
		for i, field := range headerFields {
			if field == "PID" {
				pidIdx = i
				break
			}
		}
		if pidIdx < 0 {
			t.Fatalf("ps --full missing PID column\n%s", res.Stdout)
		}
		nativePIDs := make(map[string]bool)
		for _, line := range nonEmptyLines(native.Stdout) {
			nativePIDs[strings.TrimSpace(line)] = true
		}
		markerPID := strconv.Itoa(markerCmd.Process.Pid)
		if !nativePIDs[markerPID] {
			t.Fatalf("pgrep -f did not return marker pid %s\n%s", markerPID, native.Stdout)
		}
		foundMarker := false
		for _, line := range goboxLines[1:] {
			fields := strings.Fields(line)
			if len(fields) <= pidIdx {
				continue
			}
			if !strings.Contains(line, "parity-ps-filter-123") {
				t.Fatalf("ps --full returned non-matching row %q", line)
			}
			if fields[pidIdx] == markerPID {
				foundMarker = true
			}
		}
		if !foundMarker {
			t.Fatalf("ps --full missing marker pid %s\n%s", markerPID, res.Stdout)
		}
	})

	t.Run("PS-007", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "--sort", "pid", "-r", "-n", "5", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-e", "--sort", "-pid", "-o", "pid")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps -r failed gobox=%+v native=%+v", res, native)
		}
		pids := extractLeadingInts(nonEmptyLines(res.Stdout)[1:])
		assertMonotonic(t, pids, true)
		nativePIDs := extractLeadingInts(nonEmptyLines(native.Stdout)[1:])
		assertMonotonic(t, nativePIDs, true)
	})

	t.Run("PS-008", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "--sort", "pid", "-n", "5", "-i", "1")
		if res.ExitCode != 0 {
			t.Fatalf("ps --sort failed: %+v", res)
		}
		pids := extractLeadingInts(nonEmptyLines(res.Stdout)[1:])
		assertMonotonic(t, pids, false)
	})

	t.Run("PS-009", func(t *testing.T) {
		// A bare, non-tty baseline already renders unlimited command width
		// (see cmds/proc/cmd_ps.go's non-tty maxCmd=0 auto-relaxation), so
		// contrasting -ww against a flagless baseline can never actually
		// exercise -ww's truncation-disabling effect under this test
		// harness. Force a small explicit --maxcmd on both runs so -ww's
		// override behavior is genuinely observable.
		markerCmd := startMarkerProcess(t, "parity-ps-wide-case")
		defer stopCmd(markerCmd)
		pid := strconv.Itoa(markerCmd.Process.Pid)
		const maxCmd = 8
		base := runGoboxCLI(t, t.TempDir(), "", "ps", "--full", "parity-ps-wide-case", "-f", "-n", "1", "--maxcmd", strconv.Itoa(maxCmd), "-i", "1")
		wide := runGoboxCLI(t, t.TempDir(), "", "ps", "--full", "parity-ps-wide-case", "-f", "-n", "1", "--maxcmd", strconv.Itoa(maxCmd), "-ww", "-i", "1")
		if base.ExitCode != 0 || wide.ExitCode != 0 {
			t.Fatalf("ps -ww failed base=%+v wide=%+v", base, wide)
		}
		baseCmd, ok := extractColumnByPIDHeader(base.Stdout, pid, "CMD")
		if !ok {
			t.Fatalf("ps -f missing marker pid %s in --maxcmd baseline\n%s", pid, base.Stdout)
		}
		wideCmd, ok := extractColumnByPIDHeader(wide.Stdout, pid, "CMD")
		if !ok {
			t.Fatalf("ps -f missing marker pid %s in -ww output\n%s", pid, wide.Stdout)
		}
		if len(baseCmd) != maxCmd {
			t.Fatalf("--maxcmd %d baseline should truncate to exactly %d characters, got %d: %q", maxCmd, maxCmd, len(baseCmd), baseCmd)
		}
		if strings.HasSuffix(wideCmd, "...") {
			t.Fatalf("ps -ww should override --maxcmd truncation entirely (no ellipsis), got %q", wideCmd)
		}
		if len(wideCmd) <= len(baseCmd) {
			t.Fatalf("ps -ww should not shorten command output relative to the truncated baseline\nbase=%q\nwide=%q", baseCmd, wideCmd)
		}
		if !strings.Contains(wideCmd, "parity-ps-wide-case") {
			t.Fatalf("ps -ww should preserve full command marker, got %q", wideCmd)
		}
	})

	t.Run("PS-010", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		// "cmd" is placed last in the field list (matching real ps -o
		// convention) because it is the only variable-width field that can
		// contain embedded spaces (the current test binary's own command
		// line, e.g. "-test.testlogfile=..."); putting it before %CPU/%MEM
		// would break the position-based `strings.Fields` column extraction
		// used by extractColumnByPIDHeader below, misattributing a CMD
		// fragment to the %CPU/%MEM columns.
		fields := "pid,ppid,pcpu,pmem,cmd"
		res := runGoboxCLI(t, env.Dir, "", "ps", "-o", fields, "-n", "3", "-i", "1")
		native := runNativeCLI(t, env.Dir, "", "ps", "-o", fields)
		if res.ExitCode != native.ExitCode {
			t.Fatalf("ps -o exit mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		goboxLines := nonEmptyLines(res.Stdout)
		nativeLines := nonEmptyLines(native.Stdout)
		if len(goboxLines) < 2 || len(nativeLines) < 2 {
			t.Fatalf("ps -o expected header plus at least one row\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
		wantHeader := []string{"PID", "PPID", "%CPU", "%MEM", "CMD"}
		if got := strings.Fields(goboxLines[0]); len(got) < len(wantHeader) || strings.Join(got[:len(wantHeader)], " ") != strings.Join(wantHeader, " ") {
			t.Fatalf("gobox ps -o header mismatch: got %q want prefix %q", goboxLines[0], strings.Join(wantHeader, " "))
		}
		if got := strings.Fields(nativeLines[0]); len(got) < len(wantHeader) || strings.Join(got[:len(wantHeader)], " ") != strings.Join(wantHeader, " ") {
			t.Fatalf("native ps -o header mismatch: got %q want prefix %q", nativeLines[0], strings.Join(wantHeader, " "))
		}
		for _, line := range goboxLines[1:] {
			if len(strings.Fields(line)) < len(wantHeader) {
				t.Fatalf("gobox ps -o row does not contain all requested fields: %q", line)
			}
		}
		for _, line := range nativeLines[1:] {
			if len(strings.Fields(line)) < len(wantHeader) {
				t.Fatalf("native ps -o row does not contain all requested fields: %q", line)
			}
		}

		// Verify column VALUES are semantically correct, not merely that
		// the column exists and the row has enough fields (TEST-DESIGN.md
		// §11.3): PPID must carry the real parent pid of the current test
		// process, and %CPU/%MEM must be sane numeric percentages rather
		// than e.g. CMD text shifted into the wrong column.
		pid := strconv.Itoa(os.Getpid())
		wantPPID := strconv.Itoa(os.Getppid())
		selfRes := runGoboxCLI(t, env.Dir, "", "ps", "-o", fields, "-p", pid, "-i", "1")
		if selfRes.ExitCode != 0 {
			t.Fatalf("ps -o -p self failed: %+v", selfRes)
		}
		ppid, ok := extractColumnByPIDHeader(selfRes.Stdout, pid, "PPID")
		if !ok {
			t.Fatalf("ps -o missing current process pid %s\n%s", pid, selfRes.Stdout)
		}
		if ppid != wantPPID {
			t.Fatalf("ps -o PPID column should carry the real parent pid: got %q want %q", ppid, wantPPID)
		}
		for _, headerName := range []string{"%CPU", "%MEM"} {
			val, ok := extractColumnByPIDHeader(selfRes.Stdout, pid, headerName)
			if !ok {
				t.Fatalf("ps -o missing %s column for current pid %s\n%s", headerName, pid, selfRes.Stdout)
			}
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				t.Fatalf("ps -o %s column should be numeric, got %q", headerName, val)
			}
			// %CPU can legitimately exceed 100 for a multi-threaded process
			// on a multi-core host (standard ps/top semantics: 100% per
			// core), so its sane upper bound scales with NumCPU; %MEM is a
			// share of total system memory and cannot exceed 100.
			upperBound := 100.0
			if headerName == "%CPU" {
				upperBound = 100.0 * float64(runtime.NumCPU())
			}
			if f < 0 || f > upperBound {
				t.Fatalf("ps -o %s column out of sane percentage range: %v (bound %v)", headerName, f, upperBound)
			}
		}

		invalid := runGoboxMainCLI(t, env.Dir, "", "ps", "-o", "pid,notafield", "-n", "3", "-i", "1")
		if invalid.ExitCode == 0 || !strings.Contains(invalid.Stderr, "unsupported output fields") {
			t.Fatalf("ps -o should reject unsupported fields, got %+v", invalid)
		}
	})

	t.Run("PS-011", func(t *testing.T) {
		markerCmd := startExactNameProcess(t, "pscomm")
		defer stopCmd(markerCmd)
		pattern := "pscomm"
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "--comm", pattern, "-o", "pid,comm", "-n", "5", "-ww", "-i", "1")
		native := runNativeCLI(t, t.TempDir(), "", "pgrep", "-x", pattern)
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps --comm failed gobox=%+v native=%+v", res, native)
		}
		markerPID := strconv.Itoa(markerCmd.Process.Pid)
		if _, ok := extractColumnByPID(res.Stdout, markerPID, 1); !ok {
			t.Fatalf("ps --comm missing marker pid %s\n%s", markerPID, res.Stdout)
		}
		for _, line := range nonEmptyLines(res.Stdout)[1:] {
			fields := strings.Fields(line)
			if len(fields) < 2 || fields[1] != pattern {
				t.Fatalf("ps --comm returned non-matching row %q", line)
			}
		}
	})

	t.Run("PS-012", func(t *testing.T) {
		env := t.TempDir()
		pid := strconv.Itoa(os.Getpid())
		res := runGoboxCLI(t, env, "", "ps", "-A", "-o", "pid,cmd", "-ww", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-A", "-o", "pid,cmd")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps -A mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		if got, want := strings.Fields(nonEmptyLines(res.Stdout)[0]), strings.Fields(nonEmptyLines(native.Stdout)[0]); !reflect.DeepEqual(got, want) {
			t.Fatalf("ps -A header mismatch\ngobox=%v\nnative=%v", got, want)
		}
		// -A must not just print a matching header: it must actually return
		// data rows, and specifically must include the current test process
		// among the "all processes" it claims to enumerate (TEST-DESIGN.md
		// §10: a silently empty -A implementation must fail this test).
		resLines := nonEmptyLines(res.Stdout)
		if len(resLines) < 2 {
			t.Fatalf("ps -A produced no data rows\n%s", res.Stdout)
		}
		if _, ok := rowFieldsByPID(res.Stdout, pid); !ok {
			t.Fatalf("ps -A should include the current test process pid %s\n%s", pid, res.Stdout)
		}
		nativeLines := nonEmptyLines(native.Stdout)
		if len(nativeLines) < 2 {
			t.Fatalf("native ps -A produced no data rows\n%s", native.Stdout)
		}
		if _, ok := rowFieldsByPID(native.Stdout, pid); !ok {
			t.Fatalf("native ps -A should include the current test process pid %s\n%s", pid, native.Stdout)
		}
	})

	t.Run("PS-013", func(t *testing.T) {
		pid := strconv.Itoa(os.Getpid())
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "ps", "-p", pid, "-i", "1")
		res := runGoboxCLI(t, env, "", "ps", "-F", "-p", pid, "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-F", "-p", pid)
		if base.ExitCode != 0 || res.ExitCode != native.ExitCode {
			t.Fatalf("ps -F mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		if base.Stdout == res.Stdout {
			t.Fatalf("ps -F did not change output\n--- base ---\n%s\n--- -F ---\n%s", base.Stdout, res.Stdout)
		}
		goboxLines := nonEmptyLines(res.Stdout)
		nativeLines := nonEmptyLines(native.Stdout)
		if len(goboxLines) < 2 || len(nativeLines) < 2 {
			t.Fatalf("ps -F expected header plus target row\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
		goboxPIDIdx := columnIndex(goboxLines[0], "PID")
		nativePIDIdx := columnIndex(nativeLines[0], "PID")
		if goboxPIDIdx < 0 || nativePIDIdx < 0 {
			t.Fatalf("ps -F missing PID column\ngobox=%q\nnative=%q", goboxLines[0], nativeLines[0])
		}
		foundGoboxPID := false
		for _, line := range goboxLines[1:] {
			fields := strings.Fields(line)
			if len(fields) > goboxPIDIdx && fields[goboxPIDIdx] == pid {
				foundGoboxPID = true
				break
			}
		}
		if !foundGoboxPID {
			t.Fatalf("gobox ps -F missing target pid\n%s", res.Stdout)
		}
		foundNativePID := false
		for _, line := range nativeLines[1:] {
			fields := strings.Fields(line)
			if len(fields) > nativePIDIdx && fields[nativePIDIdx] == pid {
				foundNativePID = true
				break
			}
		}
		if !foundNativePID {
			t.Fatalf("native ps -F missing target pid\n%s", native.Stdout)
		}
		if got := strings.Fields(goboxLines[0]); len(got) < 8 || got[0] != "UID" || got[1] != "PID" || got[2] != "PPID" || got[len(got)-1] != "CMD" {
			t.Fatalf("gobox ps -F header shape mismatch: %q", goboxLines[0])
		}
		if got := strings.Fields(nativeLines[0]); len(got) < 8 || got[0] != "UID" || got[1] != "PID" || got[2] != "PPID" || got[len(got)-1] != "CMD" {
			t.Fatalf("native ps -F header shape mismatch: %q", nativeLines[0])
		}
		if len(strings.Fields(goboxLines[1])) < 8 {
			t.Fatalf("gobox ps -F target row does not contain full-format columns: %q", goboxLines[1])
		}
	})

	t.Run("PS-014", func(t *testing.T) {
		// Deliberately do NOT also pass -p here: -p alone already narrows
		// the result set to a single row, which would make -u's own
		// filtering effect completely unobserved (a no-op -u implementation
		// would still pass). Exercise -u in isolation instead.
		uid := strconv.Itoa(os.Getuid())
		pid := strconv.Itoa(os.Getpid())
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "-u", uid, "-o", "pid,user", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-u", uid)
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps -u mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		if _, ok := rowFieldsByPID(res.Stdout, pid); !ok {
			t.Fatalf("gobox ps -u missing target pid %s\n%s", pid, res.Stdout)
		}
		if !strings.Contains(native.Stdout, pid) {
			t.Fatalf("native ps -u baseline missing target pid %s\n%s", pid, native.Stdout)
		}
		currentUser, err := user.LookupId(uid)
		if err != nil {
			t.Fatalf("lookup user by uid %s: %v", uid, err)
		}
		for _, line := range nonEmptyLines(res.Stdout)[1:] {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				t.Fatalf("ps -u row too short: %q", line)
			}
			if fields[1] != uid && fields[1] != currentUser.Username {
				t.Fatalf("ps -u returned row for unexpected user %q (want uid=%s or username=%s): %q", fields[1], uid, currentUser.Username, line)
			}
		}
	})

	t.Run("PS-015", func(t *testing.T) {
		pid := strconv.Itoa(os.Getpid())
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "-p", pid, "-o", "pid", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-p", pid, "-o", "pid")
		if res.ExitCode != native.ExitCode {
			t.Fatalf("ps -p mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		if _, ok := rowFieldsByPID(res.Stdout, pid); !ok {
			t.Fatalf("gobox ps -p missing target pid %s\n%s", pid, res.Stdout)
		}
		if _, ok := rowFieldsByPID(native.Stdout, pid); !ok {
			t.Fatalf("native ps -p missing target pid %s\n%s", pid, native.Stdout)
		}
		if got := extractLeadingInts(nonEmptyLines(res.Stdout)[1:]); len(got) != 1 || got[0] != os.Getpid() {
			t.Fatalf("ps -p should keep only target pid, got %v\n%s", got, res.Stdout)
		}
	})

	t.Run("PS-016", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("ps -C test reads /proc/self/comm")
		}
		data, err := os.ReadFile("/proc/self/comm")
		if err != nil {
			t.Skipf("cannot read comm: %v", err)
		}
		comm := strings.TrimSpace(string(data))
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "-C", comm, "-o", "pid,comm", "-n", "20", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-C", comm, "-o", "pid,comm")
		if res.ExitCode != native.ExitCode {
			t.Fatalf("ps -C mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		selfPID := strconv.Itoa(os.Getpid())
		if _, ok := rowFieldsByPID(res.Stdout, selfPID); !ok {
			t.Fatalf("gobox ps -C missing current process pid %s\n%s", selfPID, res.Stdout)
		}
		if _, ok := rowFieldsByPID(native.Stdout, selfPID); !ok {
			t.Fatalf("native ps -C missing current process pid %s\n%s", selfPID, native.Stdout)
		}
		// Like PS-011/PS-015, every returned row must satisfy the filter --
		// not just the target row's presence, otherwise an unfiltered -C
		// implementation that returns all processes would also pass because
		// the current process's own comm happens to appear in the result.
		for _, line := range nonEmptyLines(res.Stdout)[1:] {
			fields := strings.Fields(line)
			if len(fields) < 2 || fields[1] != comm {
				t.Fatalf("gobox ps -C returned non-matching row %q (want comm=%q)", line, comm)
			}
		}
		for _, line := range nonEmptyLines(native.Stdout)[1:] {
			fields := strings.Fields(line)
			if len(fields) < 2 || fields[1] != comm {
				t.Fatalf("native ps -C returned non-matching row %q (want comm=%q)", line, comm)
			}
		}
	})

	t.Run("PS-017", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ps", "--sort", "-pid", "-n", "5", "-i", "1")
		native := runNativeCLI(t, env, "", "ps", "-e", "--sort", "-pid", "-o", "pid")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps --sort failed gobox=%+v native=%+v", res, native)
		}
		pids := extractLeadingInts(nonEmptyLines(res.Stdout)[1:])
		assertMonotonic(t, pids, true)

		invalid := runGoboxMainCLI(t, env, "", "ps", "--sort", "nosuchfield", "-n", "5", "-i", "1")
		if invalid.ExitCode == 0 || !strings.Contains(invalid.Stderr, "unsupported sort field") {
			t.Fatalf("ps --sort should reject unsupported fields, got %+v", invalid)
		}
		// --sort cpu must be verified against the real %CPU values, not by
		// checking for the literal substring "PID" in the header
		// (TEST-DESIGN.md §11.2 explicitly forbids this weak pattern for
		// %CPU/%MEM columns).
		cpuSort := runGoboxCLI(t, env, "", "ps", "--sort", "cpu", "-n", "20", "-i", "1")
		if cpuSort.ExitCode != 0 {
			t.Fatalf("ps --sort cpu failed: %+v", cpuSort)
		}
		cpuLines := nonEmptyLines(cpuSort.Stdout)
		if len(cpuLines) < 2 {
			t.Fatalf("ps --sort cpu produced no data rows: %q", cpuSort.Stdout)
		}
		cpuVals := extractFloatColumn(t, cpuLines[0], cpuLines[1:], "%CPU")
		if len(cpuVals) < 2 {
			t.Fatalf("ps --sort cpu expected at least two parseable %%CPU values, got %v\n%s", cpuVals, cpuSort.Stdout)
		}
		assertMonotonicFloat(t, cpuVals, false)
	})

	t.Run("PS-018", func(t *testing.T) {
		env := t.TempDir()
		goboxDefault := runGoboxCLI(t, env, "", "ps", "-n", "2", "-i", "1")
		goboxAux := runGoboxCLI(t, env, "", "ps", "aux", "-n", "2", "-i", "1")
		goboxU := runGoboxCLI(t, env, "", "ps", "u", "-n", "2", "-i", "1")
		goboxAX := runGoboxCLI(t, env, "", "ps", "ax", "-n", "2", "-i", "1")
		nativeDefault := runNativeCLI(t, env, "", "ps")
		nativeAux := runNativeCLI(t, env, "", "ps", "aux")
		nativeU := runNativeCLI(t, env, "", "ps", "u")
		nativeAX := runNativeCLI(t, env, "", "ps", "ax")
		if goboxDefault.ExitCode != 0 || goboxAux.ExitCode != 0 || goboxU.ExitCode != 0 || goboxAX.ExitCode != 0 || nativeDefault.ExitCode != 0 || nativeAux.ExitCode != 0 || nativeAX.ExitCode != 0 {
			t.Fatalf("bsd ps behavior baseline failed goboxDefault=%+v goboxAux=%+v goboxU=%+v goboxAX=%+v nativeDefault=%+v nativeAux=%+v nativeU=%+v nativeAX=%+v", goboxDefault, goboxAux, goboxU, goboxAX, nativeDefault, nativeAux, nativeU, nativeAX)
		}
		if strings.Contains(goboxDefault.Stdout, "USER") || strings.Contains(nativeDefault.Stdout, "USER") {
			t.Fatalf("default ps unexpectedly looks like aux\n--- gobox default ---\n%s\n--- native default ---\n%s", goboxDefault.Stdout, nativeDefault.Stdout)
		}
		if got, want := strings.Fields(nonEmptyLines(goboxAux.Stdout)[0])[:4], strings.Fields(nonEmptyLines(nativeAux.Stdout)[0])[:4]; !reflect.DeepEqual(got, want) {
			t.Fatalf("ps aux header mismatch\ngobox=%v\nnative=%v\n--- gobox ---\n%s\n--- native ---\n%s", got, want, goboxAux.Stdout, nativeAux.Stdout)
		}
		if nativeU.ExitCode == 0 {
			if got, want := strings.Fields(nonEmptyLines(goboxU.Stdout)[0])[:4], strings.Fields(nonEmptyLines(nativeU.Stdout)[0])[:4]; !reflect.DeepEqual(got, want) {
				t.Fatalf("ps u header mismatch\ngobox=%v\nnative=%v\n--- gobox ---\n%s\n--- native ---\n%s", got, want, goboxU.Stdout, nativeU.Stdout)
			}
		}
		if strings.Contains(goboxAX.Stdout, "USER") || strings.Contains(nativeAX.Stdout, "USER") {
			t.Fatalf("ps ax should not imply BSD user format\n--- gobox ---\n%s\n--- native ---\n%s", goboxAX.Stdout, nativeAX.Stdout)
		}
		if got, want := strings.Fields(nonEmptyLines(goboxAX.Stdout)[0]), strings.Fields(nonEmptyLines(nativeAX.Stdout)[0]); !reflect.DeepEqual(got, want) {
			t.Fatalf("ps ax header mismatch\ngobox=%v\nnative=%v\n--- gobox ---\n%s\n--- native ---\n%s", got, want, goboxAX.Stdout, nativeAX.Stdout)
		}
	})

	// PS-018 above only exercises the combined BSD tokens (aux/u/ax). Per
	// TEST-DESIGN.md §14.4, an aggregate BSD-style token must not be treated
	// as a single opaque switch: the individual letters 'a' and 'x' each
	// need their own case proving an independently observable effect.
	t.Run("PS-018-a", func(t *testing.T) {
		env := t.TempDir()
		// -n is intentionally omitted (0 = show all): a fixed cap like -n 50
		// would non-deterministically exclude the current test process
		// depending on host process count and sort order.
		def := runGoboxCLI(t, env, "", "ps", "-o", "pid", "-i", "1")
		withA := runGoboxCLI(t, env, "", "ps", "a", "-o", "pid", "-i", "1")
		if def.ExitCode != 0 || withA.ExitCode != 0 {
			t.Fatalf("bsd 'a' baseline failed default=%+v a=%+v", def, withA)
		}
		selfPID := strconv.Itoa(os.Getpid())
		if _, ok := rowFieldsByPID(def.Stdout, selfPID); ok {
			t.Skip("current test process unexpectedly has a controlling tty in this environment; cannot distinguish 'a' from default via non-tty visibility here")
		}
		if _, ok := rowFieldsByPID(withA.Stdout, selfPID); ok {
			t.Fatalf("'a' alone (without 'x') must still require a controlling tty and must not surface the non-tty current process %s\n%s", selfPID, withA.Stdout)
		}
		if len(nonEmptyLines(withA.Stdout)) < len(nonEmptyLines(def.Stdout)) {
			t.Fatalf("'a' should not shrink the tty-attached result set relative to default\n--- default ---\n%s\n--- a ---\n%s", def.Stdout, withA.Stdout)
		}
	})

	t.Run("PS-018-x", func(t *testing.T) {
		env := t.TempDir()
		// -n is intentionally omitted (0 = show all); see PS-018-a comment.
		def := runGoboxCLI(t, env, "", "ps", "-o", "pid", "-i", "1")
		withX := runGoboxCLI(t, env, "", "ps", "x", "-o", "pid", "-i", "1")
		if def.ExitCode != 0 || withX.ExitCode != 0 {
			t.Fatalf("bsd 'x' baseline failed default=%+v x=%+v", def, withX)
		}
		selfPID := strconv.Itoa(os.Getpid())
		if _, ok := rowFieldsByPID(withX.Stdout, selfPID); !ok {
			t.Fatalf("'x' alone (without 'a') should include the current (non-tty) test process %s among same-user processes\n%s", selfPID, withX.Stdout)
		}
	})

	t.Run("PS-021", func(t *testing.T) {
		res := runGoboxMainCLI(t, t.TempDir(), "", "ps", "--full", ".*", "--comm", ".*", "-i", "1")
		if res.ExitCode == 0 {
			t.Fatalf("ps --full --comm should fail when both filters are given, got %+v", res)
		}
		if !strings.Contains(res.Stderr, "--full") && !strings.Contains(res.Stderr, "--comm") && !strings.Contains(res.Stderr, "cannot") {
			t.Fatalf("ps --full --comm error message missing expected content: %q", res.Stderr)
		}
	})

	// PS-019 and PS-020 test ps's `--long` and `--hide-idle` flags. They were
	// previously (mis-)placed under TestParity_FreeCases, which meant
	// `go test ./tests/parity -run TestParity_PsCases -v` silently skipped
	// them. Relocated here to keep the CLAUDE.md-documented targeted test
	// command traceable to actual ps coverage.
	t.Run("PS-019", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "--long", "-n", "5", "-i", "1")
		native := runNativeCLI(t, t.TempDir(), "", "ps", "-l")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps --long failed gobox=%+v native=%+v", res, native)
		}
		goboxLines := nonEmptyLines(res.Stdout)
		nativeLines := nonEmptyLines(native.Stdout)
		if len(goboxLines) < 2 || len(nativeLines) < 2 {
			t.Fatalf("ps --long expected header plus rows\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
		for _, want := range []string{"PID", "PPID", "TTY", "TIME", "CMD"} {
			if !strings.Contains(goboxLines[0], want) {
				t.Fatalf("gobox ps --long header missing %q: %q", want, goboxLines[0])
			}
		}
		hasStat := false
		for _, f := range strings.Fields(goboxLines[0]) {
			if f == "STAT" {
				hasStat = true
				break
			}
		}
		if !hasStat {
			t.Fatalf("gobox ps --long header missing STAT field: %q", goboxLines[0])
		}
		if !strings.Contains(nativeLines[0], "PID") || !strings.Contains(nativeLines[0], "PPID") {
			t.Fatalf("native ps -l baseline missing expected long columns: %q", nativeLines[0])
		}
	})

	t.Run("PS-020", func(t *testing.T) {
		idleCmd := exec.Command("sleep", "30")
		if err := idleCmd.Start(); err != nil {
			t.Fatalf("start idle process: %v", err)
		}
		defer stopCmd(idleCmd)

		pid := strconv.Itoa(idleCmd.Process.Pid)
		base := runGoboxCLI(t, t.TempDir(), "", "ps", "-p", pid, "-o", "pid,pcpu,cmd", "-i", "200", "-ww")
		filtered := runGoboxCLI(t, t.TempDir(), "", "ps", "-p", pid, "-o", "pid,pcpu,cmd", "--hide-idle", "-i", "200", "-ww")
		if base.ExitCode != 0 || filtered.ExitCode != 0 {
			t.Fatalf("ps --hide-idle failed base=%+v filtered=%+v", base, filtered)
		}
		if _, ok := rowFieldsByPID(base.Stdout, pid); !ok {
			t.Fatalf("ps baseline missing idle pid %s\n%s", pid, base.Stdout)
		}
		if _, ok := rowFieldsByPID(filtered.Stdout, pid); ok {
			t.Fatalf("ps --hide-idle should remove idle pid %s\n--- base ---\n%s\n--- filtered ---\n%s", pid, base.Stdout, filtered.Stdout)
		}
	})

	// PS-022/PS-023 cover the "no matching process" edge case for -p/-C,
	// which had no coverage at all. Native `ps -p <nonexistent>` and
	// `ps -C <nonexistent>` both print header-only output and exit 1; this
	// mirrors that expectation for gobox. NOTE: as of this writing gobox's
	// ps returns exit 0 with an empty result set instead of exit 1 -- see
	// BUGS.md candidate reported alongside this test suite change. Per this
	// project's bug-fixing protocol, the strict/correct assertion is kept
	// here rather than weakened to match the current (arguably incorrect)
	// gobox behavior.
	t.Run("PS-022", func(t *testing.T) {
		pid := findUnusedPID(t)
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "-p", strconv.Itoa(pid), "-o", "pid", "-i", "1")
		native := runNativeCLI(t, t.TempDir(), "", "ps", "-p", strconv.Itoa(pid))
		if lines := nonEmptyLines(res.Stdout); len(lines) != 1 {
			t.Fatalf("ps -p <nonexistent pid %d> should print header only with no data rows, got %d lines: %q", pid, len(lines), res.Stdout)
		}
		if res.ExitCode != native.ExitCode {
			t.Fatalf("ps -p <nonexistent pid %d> exit code should match native's no-match convention: gobox=%d native=%d\ngobox=%q\nnative=%q", pid, res.ExitCode, native.ExitCode, res.Stdout, native.Stdout)
		}
	})

	t.Run("PS-023", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("ps -C test reads /proc/self/comm")
		}
		comm := "no-such-comm-xyz-parity"
		res := runGoboxCLI(t, t.TempDir(), "", "ps", "-C", comm, "-o", "pid", "-i", "1")
		native := runNativeCLI(t, t.TempDir(), "", "ps", "-C", comm)
		if lines := nonEmptyLines(res.Stdout); len(lines) != 1 {
			t.Fatalf("ps -C <nonexistent comm> should print header only with no data rows, got %d lines: %q", len(lines), res.Stdout)
		}
		if res.ExitCode != native.ExitCode {
			t.Fatalf("ps -C <nonexistent comm> exit code should match native's no-match convention: gobox=%d native=%d\ngobox=%q\nnative=%q", res.ExitCode, native.ExitCode, res.Stdout, native.Stdout)
		}
	})
}

func TestParity_LsofCases(t *testing.T) {
	requireNativeCommand(t, "lsof")

	t.Run("LSOF-001", func(t *testing.T) {
		env := t.TempDir()
		markerName := "lsof-default-marker.txt"
		f, err := os.Create(filepath.Join(env, markerName))
		if err != nil {
			t.Fatalf("create marker file: %v", err)
		}
		defer f.Close()
		gobox := runGoboxCLI(t, env, "", "lsof")
		native := runNativeCLI(t, env, "", "lsof")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("lsof exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		header, rows := lsofHeaderAndRows(gobox.Stdout)
		if len(rows) == 0 {
			t.Fatalf("lsof default output should include header plus rows\ngobox=%s", gobox.Stdout)
		}
		for _, want := range []string{"COMMAND", "PID", "FD", "NAME"} {
			if !strings.Contains(header, want) {
				t.Fatalf("lsof default header missing %q: %q", want, header)
			}
		}
		pid := strconv.Itoa(os.Getpid())
		if !strings.Contains(gobox.Stdout, pid) || !strings.Contains(native.Stdout, pid) {
			t.Fatalf("lsof missing current pid\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		if !strings.Contains(gobox.Stdout, markerName) || !strings.Contains(native.Stdout, markerName) {
			t.Fatalf("lsof default output should include a controlled opened file\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		if row := lsofFindRow(rows, markerName); row == "" || !strings.Contains(row, pid) {
			t.Fatalf("lsof default output missing controlled file row for current pid\ngobox=%s", gobox.Stdout)
		}
	})

	t.Run("LSOF-002", func(t *testing.T) {
		env := t.TempDir()
		pid := strconv.Itoa(os.Getpid())
		gobox := runGoboxCLI(t, env, "", "lsof", "-p", pid)
		native := runNativeCLI(t, env, "", "lsof", "-p", pid)
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, pid) || !strings.Contains(native.Stdout, pid) {
			t.Fatalf("lsof -p mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
		for _, line := range nonEmptyLines(gobox.Stdout)[1:] {
			fields := strings.Fields(line)
			if len(fields) < 2 || fields[1] != pid {
				t.Fatalf("lsof -p leaked non-target pid row %q", line)
			}
		}
	})

	t.Run("LSOF-003", func(t *testing.T) {
		cmd := startExactNameProcess(t, "lsofcmd")
		defer stopCmd(cmd)
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof", "-c", "lsofcmd")
		native := runNativeCLI(t, env, "", "lsof", "-c", "lsofcmd")
		if gobox.ExitCode != native.ExitCode || lsofFindRow(nonEmptyLines(gobox.Stdout), "lsofcmd") == "" || lsofFindRow(nonEmptyLines(native.Stdout), "lsofcmd") == "" {
			t.Fatalf("lsof -c mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
		targetPID := strconv.Itoa(cmd.Process.Pid)
		if !strings.Contains(gobox.Stdout, targetPID) || !strings.Contains(native.Stdout, targetPID) {
			t.Fatalf("lsof -c missing target pid %s\ngobox=%s\nnative=%s", targetPID, gobox.Stdout, native.Stdout)
		}
		_, rows := lsofHeaderAndRows(gobox.Stdout)
		foundTargetPID := false
		for _, line := range rows {
			fields := strings.Fields(line)
			if len(fields) < 2 || fields[0] != "lsofcmd" {
				t.Fatalf("lsof -c leaked non-matching row %q", line)
			}
			if fields[1] == targetPID {
				foundTargetPID = true
			}
		}
		if !foundTargetPID {
			t.Fatalf("lsof -c missing row with target pid %s\ngobox=%s", targetPID, gobox.Stdout)
		}
	})

	t.Run("LSOF-004", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "lsof")
		gobox := runGoboxCLI(t, env, "", "lsof", "-i")
		native := runNativeCLI(t, env, "", "lsof", "-i")
		if base.ExitCode != 0 || gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "TCP") || !strings.Contains(native.Stdout, "TCP") {
			t.Fatalf("lsof -i mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
		if base.Stdout == gobox.Stdout {
			t.Fatalf("lsof -i should narrow output relative to default lsof\n--- base ---\n%s\n--- -i ---\n%s", base.Stdout, gobox.Stdout)
		}
		targetPort := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		if !strings.Contains(gobox.Stdout, targetPort) {
			t.Fatalf("lsof -i should include the target listener port %s\n%s", targetPort, gobox.Stdout)
		}
		_, rows := lsofHeaderAndRows(gobox.Stdout)
		foundPort := false
		for _, line := range rows {
			if strings.Contains(line, "TCP") {
				if strings.Contains(line, targetPort) {
					foundPort = true
				}
				continue
			}
			if strings.Contains(line, "UDP") {
				continue
			}
			if strings.Contains(strings.ToLower(line), "unix") {
				t.Fatalf("lsof -i leaked unexpected non-network row %q", line)
			}
		}
		if !foundPort {
			t.Fatalf("lsof -i missing target listener row\n%s", gobox.Stdout)
		}
	})

	t.Run("LSOF-005", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof", "-iTCP")
		native := runNativeCLI(t, env, "", "lsof", "-iTCP")
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "TCP") || !strings.Contains(native.Stdout, "TCP") {
			t.Fatalf("lsof -iTCP mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
		_, rows := lsofHeaderAndRows(gobox.Stdout)
		foundPort := false
		for _, line := range rows {
			if strings.Contains(line, "UDP") {
				t.Fatalf("lsof -iTCP should exclude udp rows\ngobox=%s", gobox.Stdout)
			}
			if strings.Contains(line, "TCP") && strings.Contains(line, port) {
				foundPort = true
			}
		}
		if !foundPort {
			t.Fatalf("lsof -iTCP should preserve the tcp listener\ngobox=%s", gobox.Stdout)
		}
	})

	t.Run("LSOF-006", func(t *testing.T) {
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		port := strconv.Itoa(conn.LocalAddr().(*net.UDPAddr).Port)
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof", "-iUDP")
		native := runNativeCLI(t, env, "", "lsof", "-iUDP")
		if gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "UDP") || !strings.Contains(native.Stdout, "UDP") {
			t.Fatalf("lsof -iUDP mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
		_, rows := lsofHeaderAndRows(gobox.Stdout)
		foundPort := false
		for _, line := range rows {
			if strings.Contains(line, "TCP") {
				t.Fatalf("lsof -iUDP should exclude tcp rows\ngobox=%s", gobox.Stdout)
			}
			if strings.Contains(line, "UDP") && strings.Contains(line, port) {
				foundPort = true
			}
		}
		if !foundPort {
			t.Fatalf("lsof -iUDP should preserve the udp socket\ngobox=%s", gobox.Stdout)
		}
	})

	t.Run("LSOF-007", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "lsof", "-i")
		gobox := runGoboxCLI(t, env, "", "lsof", "-i", ":"+port)
		native := runNativeCLI(t, env, "", "lsof", "-i", ":"+port)
		if base.ExitCode != 0 || gobox.ExitCode != native.ExitCode || !strings.Contains(gobox.Stdout, "TCP") || !strings.Contains(native.Stdout, ":"+port) {
			t.Fatalf("lsof -i :PORT mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
		if !strings.Contains(gobox.Stdout, port) {
			t.Fatalf("lsof -i :PORT missing filtered port %s\n%s", port, gobox.Stdout)
		}
		_, baseRows := lsofHeaderAndRows(base.Stdout)
		_, rows := lsofHeaderAndRows(gobox.Stdout)
		if len(rows) > len(baseRows) {
			t.Fatalf("lsof -i :PORT should not enlarge the bare -i result set\n--- base ---\n%s\n--- filtered ---\n%s", base.Stdout, gobox.Stdout)
		}
		for _, line := range rows {
			if !strings.Contains(line, port) {
				t.Fatalf("lsof -i :PORT leaked non-target row %q", line)
			}
		}
	})

	t.Run("LSOF-008", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "lsof", "-i")
		gobox := runGoboxCLI(t, env, "", "lsof", "-n", "-i")
		if base.ExitCode != 0 || gobox.ExitCode != 0 {
			t.Fatalf("lsof -n mismatch base=%+v gobox=%+v", base, gobox)
		}
		if base.Stdout != gobox.Stdout {
			t.Fatalf("lsof -n should be a compatibility no-op because gobox output is already numeric\n--- base ---\n%s\n--- -n ---\n%s", base.Stdout, gobox.Stdout)
		}
	})

	t.Run("LSOF-009", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "lsof", "-i", ":"+port)
		gobox := runGoboxCLI(t, env, "", "lsof", "-P", "-i", ":"+port)
		if base.ExitCode != 0 || gobox.ExitCode != 0 {
			t.Fatalf("lsof -P mismatch base=%+v gobox=%+v", base, gobox)
		}
		if base.Stdout != gobox.Stdout {
			t.Fatalf("lsof -P should be a compatibility no-op because gobox already renders numeric ports\n--- base ---\n%s\n--- -P ---\n%s", base.Stdout, gobox.Stdout)
		}
		if !strings.Contains(gobox.Stdout, ":"+port) {
			t.Fatalf("lsof -P missing numeric port %s\n%s", port, gobox.Stdout)
		}
	})

	t.Run("LSOF-010", func(t *testing.T) {
		env := t.TempDir()
		pid := strconv.Itoa(os.Getpid())
		gobox := runGoboxCLI(t, env, "", "lsof", "-t", "-p", pid)
		native := runNativeCLI(t, env, "", "lsof", "-t", "-p", pid)
		if gobox.ExitCode != native.ExitCode || normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("lsof -t mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("LSOF-011", func(t *testing.T) {
		env := t.TempDir()
		f, err := os.Create(filepath.Join(env, "open.txt"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		gobox := runGoboxCLI(t, env, "", "lsof", "open.txt")
		native := runNativeCLI(t, env, "", "lsof", "open.txt")
		if gobox.ExitCode != native.ExitCode || lsofFindRow(nonEmptyLines(gobox.Stdout), "open.txt") == "" || lsofFindRow(nonEmptyLines(native.Stdout), "open.txt") == "" {
			t.Fatalf("lsof FILE mismatch\ngobox=%+v\nnative=%+v", gobox, native)
		}
		pid := strconv.Itoa(os.Getpid())
		if !strings.Contains(gobox.Stdout, pid) || !strings.Contains(native.Stdout, pid) {
			t.Fatalf("lsof FILE missing current pid %s\ngobox=%s\nnative=%s", pid, gobox.Stdout, native.Stdout)
		}
		_, rows := lsofHeaderAndRows(gobox.Stdout)
		for _, line := range rows {
			if !strings.Contains(line, "open.txt") {
				t.Fatalf("lsof FILE leaked non-target row %q", line)
			}
		}
	})

}

func TestParity_FreeCases(t *testing.T) {
	t.Run("FREE-001", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "free")
		native := runNativeCLI(t, env, "", "free")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("free exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		goboxMem := findLineWithPrefix(gobox.Stdout, "Mem:")
		nativeMem := findLineWithPrefix(native.Stdout, "Mem:")
		if goboxMem == "" || nativeMem == "" {
			t.Fatalf("free output missing Mem row\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		if len(strings.Fields(goboxMem)) < 6 || len(strings.Fields(nativeMem)) < 6 {
			t.Fatalf("free Mem row missing expected columns\ngobox=%s\nnative=%s", goboxMem, nativeMem)
		}
		if findLineWithPrefix(gobox.Stdout, "Swap:") == "" || findLineWithPrefix(native.Stdout, "Swap:") == "" {
			t.Fatalf("free output missing Swap row\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		// Header must contain "total"
		goboxLines := nonEmptyLines(gobox.Stdout)
		if len(goboxLines) < 1 || !strings.Contains(goboxLines[0], "total") {
			t.Fatalf("free header missing 'total' keyword: %q", gobox.Stdout)
		}
		// Mem: data columns 1-3 must be parseable integers
		goboxMemFields := strings.Fields(goboxMem)
		for _, i := range []int{1, 2, 3} {
			if i >= len(goboxMemFields) {
				t.Fatalf("free Mem row too short (need column %d): %q", i, goboxMem)
			}
			if _, err := strconv.ParseInt(goboxMemFields[i], 10, 64); err != nil {
				t.Fatalf("free Mem row column %d not parseable as int: %q in %q", i, goboxMemFields[i], goboxMem)
			}
		}
	})
	t.Run("FREE-002", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "free")
		gobox := runGoboxCLI(t, env, "", "free", "-h")
		native := runNativeCLI(t, env, "", "free", "-h")
		if base.ExitCode != 0 || gobox.ExitCode != native.ExitCode {
			t.Fatalf("free -h exit mismatch base=%+v gobox=%+v native=%+v", base, gobox, native)
		}
		humanSuffixes := []string{"Ki", "Mi", "Gi", "Ti", "KB", "MB", "GB", "TB"}
		goboxMem := findLineWithPrefix(gobox.Stdout, "Mem:")
		nativeMem := findLineWithPrefix(native.Stdout, "Mem:")
		if !containsAny(goboxMem, humanSuffixes) || !containsAny(nativeMem, humanSuffixes) {
			t.Fatalf("free -h missing human units\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		goboxSwap := findLineWithPrefix(gobox.Stdout, "Swap:")
		if goboxSwap == "" {
			t.Fatalf("free -h missing Swap row\n%s", gobox.Stdout)
		}
		if !containsAny(goboxSwap, humanSuffixes) {
			// Allow zero-only swap rows (no suffix when all values are 0)
			swapFields := strings.Fields(goboxSwap)
			allZero := true
			for _, f := range swapFields[1:] {
				if f != "0" && f != "0B" {
					allZero = false
					break
				}
			}
			if !allZero {
				t.Fatalf("free -h Swap row missing human-readable suffix: %q", goboxSwap)
			}
		}
	})
	t.Run("FREE-003", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "free")
		gobox := runGoboxCLI(t, env, "", "free", "-m")
		native := runNativeCLI(t, env, "", "free", "-m")
		if base.ExitCode != 0 || gobox.ExitCode != native.ExitCode || findLineWithPrefix(gobox.Stdout, "Mem:") == "" || findLineWithPrefix(native.Stdout, "Mem:") == "" {
			t.Fatalf("free -m mismatch gobox=%+v native=%+v", gobox, native)
		}
		baseFields := freeRowFields(t, base.Stdout, "Mem:")
		goboxMem := findLineWithPrefix(gobox.Stdout, "Mem:")
		if containsAny(goboxMem, []string{"KB", "MB", "GB", "TB", "Ki", "Mi", "Gi", "Ti"}) {
			t.Fatalf("free -m Mem row should stay numeric without human unit suffixes\n%s", gobox.Stdout)
		}
		fields := strings.Fields(goboxMem)
		if len(fields) < 6 {
			t.Fatalf("free -m Mem row missing numeric columns\n%s", gobox.Stdout)
		}
		for i, field := range fields[1:] {
			got, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				t.Fatalf("free -m Mem row should stay numeric, got %q in %q", field, goboxMem)
			}
			baseKiB, err := strconv.ParseUint(baseFields[i+1], 10, 64)
			if err != nil {
				t.Fatalf("free baseline row should stay numeric, got %q in %q", baseFields[i+1], base.Stdout)
			}
			if want := baseKiB / 1024; got != want {
				t.Fatalf("free -m should convert KiB to MiB at column %d: got=%d want=%d\nbase=%s\nmib=%s", i+1, got, want, base.Stdout, gobox.Stdout)
			}
		}
	})
	t.Run("FREE-004", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "free")
		gobox := runGoboxCLI(t, env, "", "free", "-g")
		native := runNativeCLI(t, env, "", "free", "-g")
		if base.ExitCode != 0 || gobox.ExitCode != native.ExitCode || findLineWithPrefix(gobox.Stdout, "Mem:") == "" || findLineWithPrefix(native.Stdout, "Mem:") == "" {
			t.Fatalf("free -g mismatch gobox=%+v native=%+v", gobox, native)
		}
		baseFields := freeRowFields(t, base.Stdout, "Mem:")
		goboxMem := findLineWithPrefix(gobox.Stdout, "Mem:")
		if containsAny(goboxMem, []string{"KB", "MB", "GB", "TB", "Ki", "Mi", "Gi", "Ti"}) {
			t.Fatalf("free -g Mem row should stay numeric without human unit suffixes\n%s", gobox.Stdout)
		}
		fields := strings.Fields(goboxMem)
		if len(fields) < 6 {
			t.Fatalf("free -g Mem row missing numeric columns\n%s", gobox.Stdout)
		}
		for i, field := range fields[1:] {
			got, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				t.Fatalf("free -g Mem row should stay numeric, got %q in %q", field, goboxMem)
			}
			baseKiB, err := strconv.ParseUint(baseFields[i+1], 10, 64)
			if err != nil {
				t.Fatalf("free baseline row should stay numeric, got %q in %q", baseFields[i+1], base.Stdout)
			}
			if want := baseKiB / 1024 / 1024; got != want {
				t.Fatalf("free -g should convert KiB to GiB at column %d: got=%d want=%d\nbase=%s\ngib=%s", i+1, got, want, base.Stdout, gobox.Stdout)
			}
		}
	})
	t.Run("FREE-005", func(t *testing.T) {
		env := t.TempDir()
		start := time.Now()
		gobox := runGoboxCLI(t, env, "", "free", "-s", "1", "-c", "2")
		native := runNativeCLI(t, env, "", "free", "-s", "1", "-c", "2")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("free -s/-c exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if strings.Count(gobox.Stdout, "Mem:") < 2 || strings.Count(native.Stdout, "Mem:") < 2 {
			t.Fatalf("free -s/-c expected repeated samples\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		if time.Since(start) < time.Second {
			t.Fatalf("free -s/-c should wait for the second sample, elapsed=%s\ngobox=%s", time.Since(start), gobox.Stdout)
		}
		// Each Mem: sample row must have at least 5 parseable integer data columns
		for _, line := range nonEmptyLines(gobox.Stdout) {
			if !strings.HasPrefix(strings.TrimSpace(line), "Mem:") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 6 {
				t.Fatalf("free -s/-c Mem: row has fewer than 5 data columns: %q", line)
			}
			for i := 1; i <= 5; i++ {
				if _, err := strconv.ParseInt(fields[i], 10, 64); err != nil {
					t.Fatalf("free -s/-c Mem: column %d not parseable as int: %q in %q", i, fields[i], line)
				}
			}
		}
	})

	t.Run("FREE-006", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "free")
		gobox := runGoboxCLI(t, env, "", "free", "-b")
		if base.ExitCode != 0 || gobox.ExitCode != 0 || findLineWithPrefix(gobox.Stdout, "Mem:") == "" {
			t.Fatalf("free -b mismatch base=%+v gobox=%+v", base, gobox)
		}
		baseFields := freeRowFields(t, base.Stdout, "Mem:")
		goboxMem := findLineWithPrefix(gobox.Stdout, "Mem:")
		if containsAny(goboxMem, []string{"KB", "MB", "GB", "TB", "Ki", "Mi", "Gi", "Ti"}) {
			t.Fatalf("free -b Mem row should stay numeric without human unit suffixes\n%s", gobox.Stdout)
		}
		fields := strings.Fields(goboxMem)
		if len(fields) < 6 {
			t.Fatalf("free -b Mem row missing numeric columns\n%s", gobox.Stdout)
		}
		for i, field := range fields[1:] {
			got, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				t.Fatalf("free -b Mem row should stay numeric, got %q in %q", field, goboxMem)
			}
			baseKiB, err := strconv.ParseUint(baseFields[i+1], 10, 64)
			if err != nil {
				t.Fatalf("free baseline row should stay numeric, got %q in %q", baseFields[i+1], base.Stdout)
			}
			if want := baseKiB * 1024; got != want {
				t.Fatalf("free -b should convert KiB to bytes at column %d: got=%d want=%d\nbase=%s\nbytes=%s", i+1, got, want, base.Stdout, gobox.Stdout)
			}
		}
	})

	t.Run("FREE-007", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "free")
		gobox := runGoboxCLI(t, env, "", "free", "-k")
		if base.ExitCode != 0 || gobox.ExitCode != 0 || findLineWithPrefix(gobox.Stdout, "Mem:") == "" {
			t.Fatalf("free -k mismatch base=%+v gobox=%+v", base, gobox)
		}
		baseFields := freeRowFields(t, base.Stdout, "Mem:")
		goboxFields := freeRowFields(t, gobox.Stdout, "Mem:")
		for i := range baseFields[1:] {
			if baseFields[i+1] != goboxFields[i+1] {
				t.Fatalf("free -k (default unit) should match default output at column %d: base=%q gobox=%q", i+1, baseFields[i+1], goboxFields[i+1])
			}
		}
	})
}

func TestParity_TimeoutCases(t *testing.T) {
	t.Run("TIMEOUT-001", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "timeout", "0.1s", "sleep", "2")
		native := runNativeCLI(t, env, "", "timeout", "0.1s", "sleep", "2")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("timeout exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if gobox.ExitCode != 124 {
			t.Fatalf("timeout should exit with 124 when command is killed by timeout, got %d", gobox.ExitCode)
		}
	})
	t.Run("TIMEOUT-002", func(t *testing.T) {
		env := t.TempDir()
		goboxMarker := filepath.Join(env, "gobox-int")
		nativeMarker := filepath.Join(env, "native-int")
		script := "trap 'echo INT > \"$1\"; exit 0' INT; while true; do sleep 1; done"
		gobox := runGoboxCLI(t, env, "", "timeout", "-s", "INT", "0.1s", "sh", "-c", script, "sh", goboxMarker)
		native := runNativeCLI(t, env, "", "timeout", "-s", "INT", "0.1s", "sh", "-c", script, "sh", nativeMarker)
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("timeout -s exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		goboxData, err := os.ReadFile(goboxMarker)
		if err != nil {
			t.Fatalf("read gobox marker: %v", err)
		}
		nativeData, err := os.ReadFile(nativeMarker)
		if err != nil {
			t.Fatalf("read native marker: %v", err)
		}
		if strings.TrimSpace(string(goboxData)) != "INT" || strings.TrimSpace(string(nativeData)) != "INT" {
			t.Fatalf("timeout -s marker mismatch gobox=%q native=%q", string(goboxData), string(nativeData))
		}
	})
	t.Run("TIMEOUT-003", func(t *testing.T) {
		env := t.TempDir()
		start := time.Now()
		gobox := runGoboxCLI(t, env, "", "timeout", "-k", "0.1s", "0.1s", "sh", "-c", "trap '' TERM; while true; do sleep 1; done")
		goboxElapsed := time.Since(start)
		start = time.Now()
		native := runNativeCLI(t, env, "", "sh", "-c", "timeout -k 0.1s 0.1s sh -c 'trap \"\" TERM; while true; do sleep 1; done'; exit $?")
		nativeElapsed := time.Since(start)
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("timeout -k exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if goboxElapsed < 180*time.Millisecond || nativeElapsed < 180*time.Millisecond {
			t.Fatalf("timeout -k should honor grace period gobox=%s native=%s", goboxElapsed, nativeElapsed)
		}
	})
	t.Run("TIMEOUT-004", func(t *testing.T) {
		env := t.TempDir()
		script := "trap 'exit 7' TERM; while true; do sleep 1; done"
		gobox := runGoboxCLI(t, env, "", "timeout", "--preserve-status", "0.1s", "sh", "-c", script)
		native := runNativeCLI(t, env, "", "timeout", "--preserve-status", "0.1s", "sh", "-c", script)
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("timeout --preserve-status exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if gobox.ExitCode != 7 {
			t.Fatalf("timeout --preserve-status should keep child exit 7, got %+v", gobox)
		}
	})
	t.Run("TIMEOUT-005", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "timeout", "0.1s", "sleep", "1")
		native := runNativeCLI(t, env, "", "timeout", "0.1s", "sleep", "1")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("timeout suffix exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if gobox.ExitCode != 124 {
			t.Fatalf("timeout should exit with 124 when command is killed by timeout, got %d", gobox.ExitCode)
		}
	})
}

// TestParity_WatchCases deliberately does not compare against native `watch`
// (docs/TEST-CASES.md lists it as the baseline for WATCH-001..004). native
// `watch` clears/redraws the screen using terminal control sequences that
// depend on being attached to a real pty; reliably capturing and comparing
// that behavior would require a pty-backed test harness this suite doesn't
// have. These cases instead directly invoke proc.WatchCmdWithContext and
// assert gobox's own documented cadence/title/append contract (repeated
// execution, refresh cadence scaling with -n, title suppression under -t,
// no clear-screen sequences, --append scrolling) -- strong self-consistency
// evidence per TEST-DESIGN.md §2.5's "explain, don't silently downgrade"
// requirement, even though it isn't a native comparison.
func TestParity_WatchCases(t *testing.T) {
	t.Run("WATCH-001", func(t *testing.T) {
		out := runWatchCapture(t, 120*time.Millisecond, "-n", "0.05", "-t", "echo", "ok")
		if count := watchPayloadCount(out, "ok"); count < 2 {
			t.Fatalf("watch should execute command repeatedly, got %d payload lines in %q", count, out)
		}
		if strings.Contains(out, "\x1b[H\x1b[J") {
			t.Fatalf("watch should avoid clear-screen sequences when stdout is not a tty, got %q", out)
		}
	})
	t.Run("WATCH-001-title", func(t *testing.T) {
		// Without -t, watch should emit the "Every N.Ns: [cmd]" title line
		out := runWatchCapture(t, 120*time.Millisecond, "-n", "0.05", "echo", "ok")
		if !strings.Contains(out, "Every") {
			t.Fatalf("watch without -t should emit title line containing 'Every', got %q", out)
		}
	})
	t.Run("WATCH-002", func(t *testing.T) {
		runWatch := func(interval string, timeout time.Duration) int {
			var out strings.Builder
			old := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdout = w
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err = proc.WatchCmdWithContext(ctx, []string{"-n", interval, "-t", "echo", "tick"})
			_ = w.Close()
			os.Stdout = old
			_, _ = io.Copy(&out, r)
			if err != nil {
				t.Fatal(err)
			}
			return strings.Count(out.String(), "tick")
		}
		fast := runWatch("0.03", 220*time.Millisecond)
		slow := runWatch("0.09", 220*time.Millisecond)
		if fast <= slow {
			t.Fatalf("watch -n cadence mismatch fast=%d slow=%d", fast, slow)
		}
		if fast < 4 || slow < 2 {
			t.Fatalf("watch -n produced too few refreshes fast=%d slow=%d", fast, slow)
		}
	})
	t.Run("WATCH-003", func(t *testing.T) {
		out := runWatchCapture(t, 120*time.Millisecond, "-n", "0.05", "-t", "echo", "ok")
		if strings.Contains(out, "Every ") {
			t.Fatalf("watch -t title suppression mismatch: %q", out)
		}
		for _, line := range nonEmptyLines(out) {
			if strings.TrimSpace(line) != "ok" {
				t.Fatalf("watch -t should emit command payload only, got line %q in %q", line, out)
			}
		}
	})
	t.Run("WATCH-004", func(t *testing.T) {
		out := runWatchCapture(t, 120*time.Millisecond, "-n", "0.05", "-t", "--append", "echo", "ok")
		if strings.Contains(out, "\x1b[H\x1b[J") {
			t.Fatalf("watch --append should keep scrolling output without clearing the screen, got %q", out)
		}
		if count := watchPayloadCount(out, "ok"); count < 2 {
			t.Fatalf("watch --append should still execute repeatedly, got %d payload lines in %q", count, out)
		}
	})
}

func TestParity_KillCases(t *testing.T) {
	t.Run("KILL-010", func(t *testing.T) {
		env := t.TempDir()
		name := "pkdry" + strconv.FormatInt(time.Now().UnixNano()%100000000, 10)
		cmd := startExactNameProcess(t, name)
		defer stopCmd(cmd)
		gobox := runGoboxCLI(t, env, "", "kill", "--dry-run", "-x", name)
		if gobox.ExitCode != 0 {
			t.Fatalf("kill --dry-run failed: %+v", gobox)
		}
		out := gobox.Stdout + gobox.Stderr
		if !strings.Contains(out, name) || !strings.Contains(out, strconv.Itoa(cmd.Process.Pid)) {
			t.Fatalf("kill --dry-run should print the matched process name and pid, got %q", out)
		}
		requireAlive(t, cmd)
	})
	for _, tc := range []struct {
		id     string
		reason string
	}{
		{"KILL-001", ""},
		{"KILL-002", ""},
		{"KILL-003", ""},
		{"KILL-004", ""},
		{"KILL-005", ""},
		{"KILL-006", ""},
		{"KILL-007", ""},
		{"KILL-008", ""},
		{"KILL-009", ""},
	} {
		t.Run(tc.id, func(t *testing.T) {
			switch tc.id {
			case "KILL-001":
				cmd := exec.Command("sleep", "30")
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", strconv.Itoa(cmd.Process.Pid)); res.ExitCode != 0 {
					t.Fatalf("kill default TERM failed: %+v", res)
				}
				if err := waitForExit(t, cmd, time.Second); err != nil {
					_ = cmd.Process.Kill()
					t.Fatal(err)
				}
			case "KILL-002":
				gobox := runGoboxCLI(t, t.TempDir(), "", "kill", "-l")
				native := runNativeCLI(t, t.TempDir(), "", "kill", "-l")
				for _, want := range []string{"HUP", "INT", "KILL", "TERM"} {
					if !strings.Contains(gobox.Stdout, want) || !strings.Contains(native.Stdout, want) {
						t.Fatalf("kill -l missing %q\ngobox=%q\nnative=%q", want, gobox.Stdout, native.Stdout)
					}
				}
				if out := normalizeText(runGoboxCLI(t, t.TempDir(), "", "kill", "-l", "TERM").Stdout); out != "15" {
					t.Fatalf("kill -l TERM mismatch: %q", out)
				}
				if out := normalizeText(runGoboxCLI(t, t.TempDir(), "", "kill", "-l", "15").Stdout); out != "TERM" {
					t.Fatalf("kill -l 15 mismatch: %q", out)
				}
			case "KILL-003":
				cmd := exec.Command("sleep", "30")
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-s", "TERM", strconv.Itoa(cmd.Process.Pid)); res.ExitCode != 0 {
					t.Fatalf("kill -s TERM failed: %+v", res)
				}
				if err := waitForExit(t, cmd, time.Second); err != nil {
					_ = cmd.Process.Kill()
					t.Fatal(err)
				}
			case "KILL-004":
				cmd := exec.Command("sleep", "30")
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-KILL", strconv.Itoa(cmd.Process.Pid)); res.ExitCode != 0 {
					t.Fatalf("kill -KILL failed: %+v", res)
				}
				if err := waitForExit(t, cmd, time.Second); err != nil {
					_ = cmd.Process.Kill()
					t.Fatal(err)
				}
			case "KILL-005":
				marker := "pkfull-" + strconv.FormatInt(time.Now().UnixNano(), 10)
				cmd := exec.Command("sh", "-c", "sleep 30 & wait", marker)
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				time.Sleep(100 * time.Millisecond)
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-f", marker); res.ExitCode != 0 {
					stopCmd(cmd)
					t.Fatalf("kill -f failed: %+v", res)
				}
				if err := waitForExit(t, cmd, time.Second); err != nil {
					stopCmd(cmd)
					t.Fatal(err)
				}
			case "KILL-006":
				name := "pkx" + strconv.FormatInt(time.Now().UnixNano()%100000000, 10)
				cmd := startExactNameProcess(t, name)
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-x", name); res.ExitCode != 0 {
					stopCmd(cmd)
					t.Fatalf("kill -x failed: %+v", res)
				}
				if err := waitForExit(t, cmd, time.Second); err != nil {
					stopCmd(cmd)
					t.Fatal(err)
				}
			case "KILL-007":
				parent := exec.Command("sh", "-c", "sleep 30 & wait")
				if err := parent.Start(); err != nil {
					t.Fatal(err)
				}
				defer stopCmd(parent)
				ppid := parent.Process.Pid
				// Collect child PIDs before killing, retrying with backoff
				// instead of silently treating a transient empty read as "no
				// postcondition to check" -- that would let a no-op -P pass
				// undetected whenever this /proc read raced ahead of the child
				// registering.
				childrenFile := "/proc/" + strconv.Itoa(ppid) + "/task/" + strconv.Itoa(ppid) + "/children"
				var childPIDs []int
				for attempt := 0; attempt < 20 && len(childPIDs) == 0; attempt++ {
					time.Sleep(50 * time.Millisecond)
					childData, err := os.ReadFile(childrenFile)
					if err != nil {
						continue
					}
					for _, pidStr := range strings.Fields(string(childData)) {
						if v, parseErr := strconv.Atoi(pidStr); parseErr == nil {
							childPIDs = append(childPIDs, v)
						}
					}
				}
				if len(childPIDs) == 0 {
					t.Fatalf("kill -P: could not observe any child PID under %s after retrying; cannot verify -P postcondition", childrenFile)
				}
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-P", strconv.Itoa(ppid)); res.ExitCode != 0 {
					t.Fatalf("kill -P failed: %+v", res)
				}
				// Verify child processes are no longer alive.
				time.Sleep(200 * time.Millisecond)
				for _, childPID := range childPIDs {
					if _, statErr := os.Stat("/proc/" + strconv.Itoa(childPID)); statErr == nil {
						t.Fatalf("kill -P: child process %d still alive after kill", childPID)
					}
				}
			case "KILL-008":
				marker := "pknew-" + strconv.FormatInt(time.Now().UnixNano(), 10)
				oldest := exec.Command("sh", "-c", "sleep 30 & wait", marker+"-1")
				if err := oldest.Start(); err != nil {
					t.Fatal(err)
				}
				defer stopCmd(oldest)
				time.Sleep(1200 * time.Millisecond)
				newest := exec.Command("sh", "-c", "sleep 30 & wait", marker+"-2")
				if err := newest.Start(); err != nil {
					t.Fatal(err)
				}
				defer stopCmd(newest)
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-n", "-f", marker); res.ExitCode != 0 {
					t.Fatalf("kill -n failed: %+v", res)
				}
				if err := waitForExit(t, newest, time.Second); err != nil {
					t.Fatal(err)
				}
				requireAlive(t, oldest)
			case "KILL-009":
				marker := "pkold-" + strconv.FormatInt(time.Now().UnixNano(), 10)
				oldest := exec.Command("sh", "-c", "sleep 30 & wait", marker+"-1")
				if err := oldest.Start(); err != nil {
					t.Fatal(err)
				}
				defer stopCmd(oldest)
				time.Sleep(1200 * time.Millisecond)
				newest := exec.Command("sh", "-c", "sleep 30 & wait", marker+"-2")
				if err := newest.Start(); err != nil {
					t.Fatal(err)
				}
				defer stopCmd(newest)
				if res := runGoboxCLI(t, t.TempDir(), "", "kill", "-o", "-f", marker); res.ExitCode != 0 {
					t.Fatalf("kill -o failed: %+v", res)
				}
				if err := waitForExit(t, oldest, time.Second); err != nil {
					t.Fatal(err)
				}
				requireAlive(t, newest)
			default:
				t.Fatalf("unexpected case %s", tc.id)
			}
		})
	}
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func findLineWithPrefix(s, prefix string) string {
	for _, line := range nonEmptyLines(s) {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return line
		}
	}
	return ""
}

// splitRowPreservingLastColumn splits a fixed-width table row into exactly
// numCols fields, joining any overflow tokens into the final column. This is
// required because ps/top render CMD/COMMAND as an unpadded trailing column
// that may itself contain internal spaces (e.g. a marker process argv),
// which would otherwise be shattered by a naive strings.Fields split.
func splitRowPreservingLastColumn(line string, numCols int) []string {
	fields := strings.Fields(line)
	if numCols <= 0 || len(fields) <= numCols {
		return fields
	}
	result := append([]string{}, fields[:numCols-1]...)
	result = append(result, strings.Join(fields[numCols-1:], " "))
	return result
}

// columnByHeaderRows extracts, for each data row, the value found at the
// column named headerName (as located in headerLine). It returns the
// resolved column index (-1 if headerName was not found in headerLine).
func columnByHeaderRows(headerLine string, rows []string, headerName string) ([]string, int) {
	headerFields := strings.Fields(headerLine)
	numCols := len(headerFields)
	idx := -1
	for i, f := range headerFields {
		if f == headerName {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, -1
	}
	var vals []string
	for _, line := range rows {
		fields := splitRowPreservingLastColumn(line, numCols)
		if len(fields) <= idx {
			continue
		}
		vals = append(vals, fields[idx])
	}
	return vals, idx
}

// columnByPIDFromRows locates the data row whose PID column equals pid and
// returns the value of the headerName column on that row, honoring a
// possibly multi-word trailing column (e.g. CMD).
func columnByPIDFromRows(headerLine string, rows []string, pid, headerName string) (string, bool) {
	headerFields := strings.Fields(headerLine)
	numCols := len(headerFields)
	pidIdx := -1
	targetIdx := -1
	for i, f := range headerFields {
		if f == "PID" {
			pidIdx = i
		}
		if f == headerName {
			targetIdx = i
		}
	}
	if pidIdx < 0 || targetIdx < 0 {
		return "", false
	}
	for _, line := range rows {
		fields := splitRowPreservingLastColumn(line, numCols)
		if len(fields) <= pidIdx || len(fields) <= targetIdx {
			continue
		}
		if fields[pidIdx] == pid {
			return fields[targetIdx], true
		}
	}
	return "", false
}

// extractColumnByPIDHeader is a convenience wrapper over columnByPIDFromRows
// for plain ps-style output where line 0 is the header and the remaining
// lines are data rows.
func extractColumnByPIDHeader(out, pid, headerName string) (string, bool) {
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return "", false
	}
	return columnByPIDFromRows(lines[0], lines[1:], pid, headerName)
}

// extractFloatColumn parses the named column from every data row as a
// float64, skipping rows where the column is missing or non-numeric. Callers
// must supply the header line and the data rows explicitly since top's
// output has summary lines preceding the process table header.
func extractFloatColumn(t *testing.T, headerLine string, rows []string, headerName string) []float64 {
	t.Helper()
	vals, idx := columnByHeaderRows(headerLine, rows, headerName)
	if idx < 0 {
		t.Fatalf("column %q not found in header %q", headerName, headerLine)
	}
	var out []float64
	for _, v := range vals {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			continue
		}
		out = append(out, f)
	}
	return out
}

// assertMonotonicFloat verifies vals are sorted (ascending or descending)
// using true numeric comparison rather than lexical/text comparison, per
// TEST-DESIGN.md §11.2 for %CPU/%MEM-style columns. Ties are permitted.
func assertMonotonicFloat(t *testing.T, vals []float64, descending bool) {
	t.Helper()
	const eps = 1e-9
	for i := 1; i < len(vals); i++ {
		if descending {
			if vals[i-1] < vals[i]-eps {
				t.Fatalf("expected descending order, got %v", vals)
			}
		} else if vals[i-1] > vals[i]+eps {
			t.Fatalf("expected ascending order, got %v", vals)
		}
	}
}

// findUnusedPID returns a PID that is (with overwhelming probability) not
// currently assigned to any process, for exercising "no matching process"
// edge cases. It scans a sparse sequence of candidates and requires that
// /proc/<candidate> not exist, per TEST-DESIGN.md §13 (rely on controlled,
// observable state rather than guessing a single hardcoded PID).
func findUnusedPID(t *testing.T) int {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("unused-pid probing relies on /proc")
	}
	for candidate := 1000003; candidate < 4000000; candidate += 104729 {
		if _, err := os.Stat("/proc/" + strconv.Itoa(candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
	t.Fatal("could not find an unused pid for negative-case testing")
	return 0
}

// isDigitByte reports whether b is an ASCII digit.
func isDigitByte(b byte) bool { return b >= '0' && b <= '9' }

// containsPortToken reports whether line contains port as a standalone
// numeric token (not as a substring of a larger number such as a PID or a
// different port). This avoids false positives like port "80" matching
// inside PID "1180".
func containsPortToken(line, port string) bool {
	from := 0
	for {
		i := strings.Index(line[from:], port)
		if i < 0 {
			return false
		}
		pos := from + i
		beforeOK := pos == 0 || !isDigitByte(line[pos-1])
		end := pos + len(port)
		afterOK := end >= len(line) || !isDigitByte(line[end])
		if beforeOK && afterOK {
			return true
		}
		from = pos + 1
	}
}
