package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestParity_HeadCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{
			ID:            "HEAD-001",
			Name:          "head -n",
			GoboxArgs:     []string{"head", "-n", "2", "input.txt"},
			NativeCommand: "head",
			NativeArgs:    []string{"-n", "2", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "1\n2\n3\n") },
		},
		{
			ID:            "HEAD-002",
			Name:          "head -c",
			GoboxArgs:     []string{"head", "-c", "5", "input.txt"},
			NativeCommand: "head",
			NativeArgs:    []string{"-c", "5", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "abcdef\n") },
		},
		{
			ID:            "HEAD-003",
			Name:          "head -q",
			GoboxArgs:     []string{"head", "-q", "a.txt", "b.txt"},
			NativeCommand: "head",
			NativeArgs:    []string{"-q", "a.txt", "b.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a.txt"), "a1\na2\n")
				writeFile(t, filepath.Join(env.Dir, "b.txt"), "b1\nb2\n")
			},
		},
		{
			ID:            "HEAD-005",
			Name:          "head stdin",
			GoboxArgs:     []string{"head", "-n", "2"},
			NativeCommand: "head",
			NativeArgs:    []string{"-n", "2"},
			Stdin:         "line1\nline2\nline3\n",
		},
		{
			ID:            "HEAD-001b",
			Name:          "head -n empty file",
			GoboxArgs:     []string{"head", "-n", "2", "empty.txt"},
			NativeCommand: "head",
			NativeArgs:    []string{"-n", "2", "empty.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "empty.txt"), "") },
		},
	})

	t.Run("HEAD-004", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "head", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("head -h failed: %+v", res)
		}
		if strings.TrimSpace(res.Stderr) != "" {
			t.Fatalf("head -h should not write stderr, got %q", res.Stderr)
		}
		// Assert every flag head actually supports (per cmds/text/cmd_head.go's
		// printHeadUsage: -n, -c, -q, -h), not just a couple of substrings, so
		// stale/incomplete help text is caught.
		for _, want := range []string{"Usage: gobox head", "Options:", "-n NUM", "-c NUM", "-q, --quiet", "-h, --help", "Examples:"} {
			if !strings.Contains(res.Stdout, want) {
				t.Fatalf("head -h missing %q in %q", want, res.Stdout)
			}
		}
	})
}

func TestParity_DiffCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{
			ID:            "DIFF-001",
			Name:          "diff default",
			GoboxArgs:     []string{"diff", "a.txt", "b.txt"},
			NativeCommand: "diff",
			NativeArgs:    []string{"a.txt", "b.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a.txt"), "one\ntwo\n")
				writeFile(t, filepath.Join(env.Dir, "b.txt"), "one\nTWO\n")
			},
		},
		{
			ID:            "DIFF-002",
			Name:          "diff -u",
			GoboxArgs:     []string{"diff", "-u", "a.txt", "b.txt"},
			NativeCommand: "diff",
			NativeArgs:    []string{"-u", "a.txt", "b.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a.txt"), "one\ntwo\n")
				writeFile(t, filepath.Join(env.Dir, "b.txt"), "one\nTWO\n")
			},
			Normalize: normalizeUnifiedDiffHeaders,
		},
		{
			ID:            "DIFF-003",
			Name:          "diff -q",
			GoboxArgs:     []string{"diff", "-q", "a.txt", "b.txt"},
			NativeCommand: "diff",
			NativeArgs:    []string{"-q", "a.txt", "b.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a.txt"), "left\n")
				writeFile(t, filepath.Join(env.Dir, "b.txt"), "right\n")
			},
		},
		{
			ID:            "DIFF-005",
			Name:          "diff -N",
			GoboxArgs:     []string{"diff", "-N", "missing.txt", "b.txt"},
			NativeCommand: "diff",
			NativeArgs:    []string{"-N", "missing.txt", "b.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "b.txt"), "created\n") },
		},
		{
			ID:            "DIFF-006",
			Name:          "diff --strip-trailing-cr",
			GoboxArgs:     []string{"diff", "--strip-trailing-cr", "a.txt", "b.txt"},
			NativeCommand: "diff",
			NativeArgs:    []string{"--strip-trailing-cr", "a.txt", "b.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a.txt"), "one\r\ntwo\r\n")
				writeFile(t, filepath.Join(env.Dir, "b.txt"), "one\ntwo\n")
			},
		},
		{
			ID:            "DIFF-007",
			Name:          "diff stdin",
			GoboxArgs:     []string{"diff", "a.txt", "-"},
			NativeCommand: "diff",
			NativeArgs:    []string{"a.txt", "-"},
			Stdin:         "same\n",
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "a.txt"), "same\n") },
		},
		{
			ID:            "DIFF-008",
			Name:          "diff binary",
			GoboxArgs:     []string{"diff", "a.bin", "b.bin"},
			NativeCommand: "diff",
			NativeArgs:    []string{"a.bin", "b.bin"},
			Setup: func(t *testing.T, env *parityEnv) {
				if err := os.WriteFile(filepath.Join(env.Dir, "a.bin"), []byte{0, 1, 2}, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(env.Dir, "b.bin"), []byte{0, 1, 3}, 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			ID:            "DIFF-009",
			Name:          "diff equal",
			GoboxArgs:     []string{"diff", "a", "b"},
			NativeCommand: "diff",
			NativeArgs:    []string{"a", "b"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a"), "same")
				writeFile(t, filepath.Join(env.Dir, "b"), "same")
			},
		},
		{
			ID:            "DIFF-002b",
			Name:          "diff --unified (long form)",
			GoboxArgs:     []string{"diff", "--unified", "a.txt", "b.txt"},
			NativeCommand: "diff",
			NativeArgs:    []string{"--unified", "a.txt", "b.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a.txt"), "one\ntwo\n")
				writeFile(t, filepath.Join(env.Dir, "b.txt"), "one\nTWO\n")
			},
			Normalize: normalizeUnifiedDiffHeaders,
		},
		{
			ID:            "DIFF-003b",
			Name:          "diff --brief (long form)",
			GoboxArgs:     []string{"diff", "--brief", "a.txt", "b.txt"},
			NativeCommand: "diff",
			NativeArgs:    []string{"--brief", "a.txt", "b.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a.txt"), "left\n")
				writeFile(t, filepath.Join(env.Dir, "b.txt"), "right\n")
			},
		},
		{
			ID:            "DIFF-005b",
			Name:          "diff --new-file (long form)",
			GoboxArgs:     []string{"diff", "--new-file", "missing.txt", "b.txt"},
			NativeCommand: "diff",
			NativeArgs:    []string{"--new-file", "missing.txt", "b.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "b.txt"), "created\n") },
		},
	})

	t.Run("DIFF-010", func(t *testing.T) {
		// Two non-adjacent changes: line 1 and line 20 of a 25-line file.
		// Gobox must cover all changes.  If it emits fewer hunks than native,
		// that is documented as an implementation bug but the test still passes.
		var aLines, bLines []string
		for i := 1; i <= 25; i++ {
			aLines = append(aLines, fmt.Sprintf("line%d", i))
			bLine := fmt.Sprintf("line%d", i)
			if i == 1 || i == 20 {
				bLine = fmt.Sprintf("CHANGED%d", i)
			}
			bLines = append(bLines, bLine)
		}
		aContent := strings.Join(aLines, "\n") + "\n"
		bContent := strings.Join(bLines, "\n") + "\n"
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "a.txt"), aContent)
		writeFile(t, filepath.Join(env, "b.txt"), bContent)

		gobox := runGoboxCLI(t, env, "", "diff", "-u", "a.txt", "b.txt")
		native := runNativeCLI(t, env, "", "diff", "-u", "a.txt", "b.txt")

		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("DIFF-010 exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		// Gobox must surface both change regions.
		if !strings.Contains(gobox.Stdout, "CHANGED1") || !strings.Contains(gobox.Stdout, "CHANGED20") {
			t.Fatalf("DIFF-010: gobox output does not cover both change regions\n%s", gobox.Stdout)
		}
		// Count @@ hunk headers.
		goboxHunks := 0
		for _, line := range strings.Split(gobox.Stdout, "\n") {
			if strings.HasPrefix(line, "@@") {
				goboxHunks++
			}
		}
		nativeHunks := 0
		for _, line := range strings.Split(native.Stdout, "\n") {
			if strings.HasPrefix(line, "@@") {
				nativeHunks++
			}
		}
		if goboxHunks != nativeHunks {
			t.Fatalf("DIFF-010: gobox diff -u produced %d @@-hunk(s) but native produced %d for a "+
				"25-line file with non-adjacent changes at lines 1 and 20; gobox must split hunks the "+
				"same way native does instead of merging separate change regions into one hunk\ngobox:\n%s\nnative:\n%s",
				goboxHunks, nativeHunks, gobox.Stdout, native.Stdout)
		}
		// Hunk count alone doesn't prove the hunks cover the same lines: gobox
		// could produce the right count of hunks with different @@ start/length
		// numbers than native. Parse the "@@ -a,b +c,d @@" headers from both
		// and assert the actual (a,b,c,d) tuples match line-for-line.
		goboxHeaders := extractUnifiedHunkHeaders(t, gobox.Stdout)
		nativeHeaders := extractUnifiedHunkHeaders(t, native.Stdout)
		if len(goboxHeaders) != len(nativeHeaders) {
			t.Fatalf("DIFF-010: parsed %d gobox hunk header(s) but %d native hunk header(s)\ngobox:\n%s\nnative:\n%s",
				len(goboxHeaders), len(nativeHeaders), gobox.Stdout, native.Stdout)
		}
		for i := range goboxHeaders {
			if goboxHeaders[i] != nativeHeaders[i] {
				t.Fatalf("DIFF-010: hunk header #%d mismatch: gobox=%q native=%q\ngobox:\n%s\nnative:\n%s",
					i, goboxHeaders[i], nativeHeaders[i], gobox.Stdout, native.Stdout)
			}
		}
	})

	t.Run("DIFF-004", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "left", "z.txt"), "same\n")
		writeFile(t, filepath.Join(env, "left", "sub", "a.txt"), "old\n")
		writeFile(t, filepath.Join(env, "right", "z.txt"), "same\n")
		writeFile(t, filepath.Join(env, "right", "sub", "a.txt"), "new\n")
		writeFile(t, filepath.Join(env, "right", "sub", "b.txt"), "extra\n")
		res := runGoboxCLI(t, env, "", "diff", "-r", "left", "right")
		native := runNativeCLI(t, env, "", "diff", "-r", "left", "right")
		if res.ExitCode != native.ExitCode {
			t.Fatalf("diff -r exit mismatch gobox=%d native=%d\n--- gobox ---\n%s\n--- native ---\n%s", res.ExitCode, native.ExitCode, res.Stdout, native.Stdout)
		}
		if got, want := strings.Join(sortedNonEmptyLines(res.Stdout), "\n"), strings.Join(sortedNonEmptyLines(native.Stdout), "\n"); got != want {
			t.Fatalf("diff -r line-set mismatch\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
	})
}

func normalizeUnifiedDiffHeaders(s string) string {
	s = normalizeText(s)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				lines[i] = strings.Join(fields[:2], " ")
			}
		}
	}
	return strings.Join(lines, "\n")
}

// extractUnifiedHunkHeaders parses each "@@ -a,b +c,d @@" line in a unified
// diff and returns the four numbers as a normalized "a,b,c,d" string per
// hunk (the trailing "@@ [context]" text, if any, is ignored). A missing
// ",b" or ",d" (length 1) is normalized to "b=1"/"d=1" per unified diff
// convention, matching how both gobox and native diff behave.
func extractUnifiedHunkHeaders(t *testing.T, s string) []string {
	t.Helper()
	var headers []string
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(line, "@@") {
			continue
		}
		var aStart, aLen, cStart, cLen int
		aLen, cLen = 1, 1
		fields := strings.Fields(line)
		if len(fields) < 3 {
			t.Fatalf("extractUnifiedHunkHeaders: malformed hunk header %q", line)
		}
		if _, err := fmt.Sscanf(fields[1], "-%d,%d", &aStart, &aLen); err != nil {
			if _, err2 := fmt.Sscanf(fields[1], "-%d", &aStart); err2 != nil {
				t.Fatalf("extractUnifiedHunkHeaders: cannot parse %q in %q", fields[1], line)
			}
		}
		if _, err := fmt.Sscanf(fields[2], "+%d,%d", &cStart, &cLen); err != nil {
			if _, err2 := fmt.Sscanf(fields[2], "+%d", &cStart); err2 != nil {
				t.Fatalf("extractUnifiedHunkHeaders: cannot parse %q in %q", fields[2], line)
			}
		}
		headers = append(headers, fmt.Sprintf("%d,%d,%d,%d", aStart, aLen, cStart, cLen))
	}
	return headers
}

func sortedNonEmptyLines(s string) []string {
	lines := nonEmptyLines(s)
	sort.Strings(lines)
	return lines
}

func TestParity_TailCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "TAIL-001", Name: "tail -n", GoboxArgs: []string{"tail", "-n", "2", "input.txt"}, NativeCommand: "tail", NativeArgs: []string{"-n", "2", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "1\n2\n3\n") }},
		{ID: "TAIL-005", Name: "tail -q", GoboxArgs: []string{"tail", "-q", "-n", "1", "a.txt", "b.txt"}, NativeCommand: "tail", NativeArgs: []string{"-q", "-n", "1", "a.txt", "b.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "a.txt"), "a1\na2\n")
			writeFile(t, filepath.Join(env.Dir, "b.txt"), "b1\nb2\n")
		}},
		{ID: "TAIL-001b", Name: "tail -n empty file", GoboxArgs: []string{"tail", "-n", "2", "empty.txt"}, NativeCommand: "tail", NativeArgs: []string{"-n", "2", "empty.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "empty.txt"), "") }},
	})

	t.Run("TAIL-002", func(t *testing.T) {
		env := t.TempDir()
		// Use separate files with identical content so both runners see the same append.
		// This is a true parity test: same input, outputs must match.
		fileG := filepath.Join(env, "follow-gobox.log")
		fileN := filepath.Join(env, "follow-native.log")
		writeFile(t, fileG, "base\n")
		writeFile(t, fileN, "base\n")
		gobox := runTailGoboxFollow(t, env, []string{"-n", "0", "-f", "follow-gobox.log"}, func() {
			appendFile(t, fileG, "same-follow\n")
		}, 1600*time.Millisecond)
		native := runNativeFollow(t, env, "tail", []string{"-n", "0", "-f", "follow-native.log"}, func() {
			appendFile(t, fileN, "same-follow\n")
		}, 1600*time.Millisecond)
		if !strings.Contains(gobox.Stdout, "same-follow") || !strings.Contains(native.Stdout, "same-follow") {
			t.Fatalf("tail -f did not follow append\ngobox=%+v\nnative=%+v", gobox, native)
		}
		if strings.Contains(gobox.Stdout, "base") || strings.Contains(native.Stdout, "base") {
			t.Fatalf("tail -n 0 -f should not replay baseline content\ngobox=%+v\nnative=%+v", gobox, native)
		}
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("tail -f output parity mismatch\ngobox: %q\nnative: %q", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("TAIL-003", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "rotate.log")
		writeFile(t, file, "base\n")
		action := func(content string) func() {
			return func() {
				if err := os.Rename(file, filepath.Join(env, "rotate.log.1")); err != nil {
					t.Fatalf("rename: %v", err)
				}
				writeFile(t, file, content+"\n")
			}
		}
		gobox := runTailGoboxFollow(t, env, []string{"-n", "1", "--follow=name", "-s", "0.1", "rotate.log"}, action("gobox-rotated"), 1200*time.Millisecond)
		writeFile(t, file, "base\n")
		native := runNativeFollow(t, env, "tail", []string{"-n", "1", "--follow=name", "-s", "0.1", "rotate.log"}, action("native-rotated"), 1200*time.Millisecond)
		if !strings.Contains(gobox.Stdout, "gobox-rotated") || !strings.Contains(native.Stdout, "native-rotated") {
			t.Fatalf("tail --follow=name did not follow rotation\ngobox=%+v\nnative=%+v", gobox, native)
		}
		if lastLine(gobox.Stdout) != "gobox-rotated" || lastLine(native.Stdout) != "native-rotated" {
			t.Fatalf("tail --follow=name should end on replacement file content\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("TAIL-004", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "delayed.log")
		gobox := runTailGoboxFollow(t, env, []string{"-n", "1", "--retry", "--follow=name", "-s", "0.1", "delayed.log"}, func() {
			writeFile(t, file, "gobox-created\n")
		}, 900*time.Millisecond)
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove delayed file: %v", err)
		}
		native := runNativeFollow(t, env, "tail", []string{"-n", "1", "--retry", "--follow=name", "-s", "0.1", "delayed.log"}, func() {
			writeFile(t, file, "native-created\n")
		}, 900*time.Millisecond)
		if !strings.Contains(gobox.Stdout, "gobox-created") || !strings.Contains(native.Stdout, "native-created") {
			t.Fatalf("tail --retry did not read delayed file\ngobox=%+v\nnative=%+v", gobox, native)
		}
		if lastLine(gobox.Stdout) != "gobox-created" || lastLine(native.Stdout) != "native-created" {
			t.Fatalf("tail --retry should end on newly created file content\ngobox=%+v\nnative=%+v", gobox, native)
		}
	})

	t.Run("TAIL-006", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "sleep.log")
		writeFile(t, file, "base\n")
		gobox := runTailGoboxFollow(t, env, []string{"-n", "0", "-f", "-s", "0.1", "sleep.log"}, func() {
			appendFile(t, file, "gobox-sleep\n")
		}, 800*time.Millisecond)
		native := runNativeFollow(t, env, "tail", []string{"-n", "0", "-f", "-s", "0.1", "sleep.log"}, func() {
			appendFile(t, file, "native-sleep\n")
		}, 800*time.Millisecond)
		if !strings.Contains(gobox.Stdout, "gobox-sleep") || !strings.Contains(native.Stdout, "native-sleep") {
			t.Fatalf("tail -s did not poll appended content\ngobox=%+v\nnative=%+v", gobox, native)
		}
		if strings.Contains(gobox.Stdout, "base") || strings.Contains(native.Stdout, "base") {
			t.Fatalf("tail -n 0 -f -s should not replay baseline content\ngobox=%+v\nnative=%+v", gobox, native)
		}

		// Prove that -s actually controls the polling cadence rather than being a
		// no-op: with a fast interval, gobox should observe most of a burst of
		// incremental appends within a bounded window; with a slow interval whose
		// period exceeds that window, it should observe far fewer (ideally none),
		// since the next poll tick will not have fired yet.
		countUpdates := func(sleepArg string, overallTimeout time.Duration) int {
			cEnv := t.TempDir()
			cFile := filepath.Join(cEnv, "cadence.log")
			writeFile(t, cFile, "base\n")
			res := runTailGoboxFollow(t, cEnv, []string{"-n", "0", "-f", "-s", sleepArg, "cadence.log"}, func() {
				for i := 0; i < 5; i++ {
					appendFile(t, cFile, fmt.Sprintf("m%d\n", i))
					time.Sleep(120 * time.Millisecond)
				}
			}, overallTimeout)
			return len(nonEmptyLines(res.Stdout))
		}
		fastCount := countUpdates("0.05", 1200*time.Millisecond)
		slowCount := countUpdates("2", 1200*time.Millisecond)
		if fastCount == 0 {
			t.Fatalf("TAIL-006: fast -s interval (0.05s) observed no appended lines within the window; cannot prove cadence changed")
		}
		if slowCount >= fastCount {
			t.Fatalf("TAIL-006: -s does not appear to change polling cadence: slow interval (2s) observed %d line(s), fast interval (0.05s) observed %d line(s); expected slow < fast", slowCount, fastCount)
		}
	})

	t.Run("TAIL-007", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "pid.log")
		writeFile(t, file, "base\n")
		child := exec.Command("sleep", "0.5")
		if err := child.Start(); err != nil {
			t.Fatalf("start child: %v", err)
		}
		childDone := make(chan struct{})
		go func() {
			_ = child.Wait()
			close(childDone)
		}()
		start := time.Now()
		gobox := runTailGoboxFollow(t, env, []string{"-n", "0", "-f", "-s", "0.1", fmt.Sprintf("--pid=%d", child.Process.Pid), "pid.log"}, func() {
			appendFile(t, file, "pid-follow\n")
		}, 2*time.Second)
		<-childDone
		if gobox.ExitCode != 0 || time.Since(start) > 2200*time.Millisecond {
			t.Fatalf("tail --pid did not exit after pid ended: %+v elapsed=%s", gobox, time.Since(start))
		}
		if !strings.Contains(gobox.Stdout, "pid-follow") {
			t.Fatalf("tail --pid did not emit appended content: %+v", gobox)
		}
		if strings.Contains(gobox.Stdout, "base") {
			t.Fatalf("tail --pid with -n 0 should not replay baseline content: %+v", gobox)
		}
	})
}

// startGoboxStreamCmd starts gobox as a real subprocess (reusing the
// TestParityHelperProcess harness) with piped stdin/stdout left open, so
// streaming/incremental behavior can be observed while the process is still
// running rather than only after it exits.
func startGoboxStreamCmd(t *testing.T, dir string, args []string) (*exec.Cmd, io.WriteCloser, *bufio.Reader) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=TestParityHelperProcess", "--"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOBOX_PARITY_HELPER=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("gobox stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("gobox stdout pipe: %v", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("start gobox stream subprocess: %v", err)
	}
	return cmd, stdin, bufio.NewReader(stdout)
}

// startNativeStreamCmd starts a native command with piped stdin/stdout left
// open, mirroring startGoboxStreamCmd so the two can be compared.
func startNativeStreamCmd(t *testing.T, dir, command string, args []string) (*exec.Cmd, io.WriteCloser, *bufio.Reader) {
	t.Helper()
	path := requireNativeCommand(t, command)
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("native stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("native stdout pipe: %v", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("start native stream subprocess: %v", err)
	}
	return cmd, stdin, bufio.NewReader(stdout)
}

// readLineWithTimeout attempts to read a single line within timeout. It
// returns ("", false) if no line arrives in time, which is the expected
// outcome for a fully-buffered process that hasn't hit EOF yet.
func readLineWithTimeout(r *bufio.Reader, timeout time.Duration) (string, bool) {
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := r.ReadString('\n')
		ch <- result{line, err}
	}()
	select {
	case res := <-ch:
		if res.line == "" && res.err != nil {
			return "", false
		}
		return res.line, true
	case <-time.After(timeout):
		return "", false
	}
}

func TestParity_GrepCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "GREP-001", Name: "grep -E", GoboxArgs: []string{"grep", "-E", "foo|bar", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-E", "foo|bar", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\nbaz\nbar\n")
		}},
		{ID: "GREP-002", Name: "grep -F", GoboxArgs: []string{"grep", "-F", "a.b", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-F", "a.b", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a.b\naxb\n") }},
		{ID: "GREP-003", Name: "grep -c", GoboxArgs: []string{"grep", "-c", "foo", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-c", "foo", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\nfoo\nbar\n")
		}},
		{ID: "GREP-004", Name: "grep -i", GoboxArgs: []string{"grep", "-i", "foo", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-i", "foo", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "Foo\nbar\n") }},
		{ID: "GREP-006", Name: "grep -n", GoboxArgs: []string{"grep", "-n", "foo", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-n", "foo", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "bar\nfoo\n") }},
		{ID: "GREP-007", Name: "grep -o", GoboxArgs: []string{"grep", "-o", "foo", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-o", "foo", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo foo\n") }},
		{ID: "GREP-008", Name: "grep -q match", GoboxArgs: []string{"grep", "-q", "foo", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-q", "foo", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") }, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("grep -q exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
			}
			// -q must suppress all stdout output.
			if gobox.Stdout != "" {
				t.Fatalf("grep -q should produce no stdout, got %q", gobox.Stdout)
			}
		}},
		{ID: "GREP-008b", Name: "grep -q no match", GoboxArgs: []string{"grep", "-q", "notfound", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-q", "notfound", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") }, Assert: func(t *testing.T, gobox, native parityResult) {
			// No-match should exit 1, no stdout, no stderr.
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("grep -q no-match exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
			}
			if gobox.ExitCode != 1 {
				t.Fatalf("grep -q no-match: expected exit 1, got %d", gobox.ExitCode)
			}
			if gobox.Stdout != "" {
				t.Fatalf("grep -q no-match should produce no stdout, got %q", gobox.Stdout)
			}
			if strings.TrimSpace(gobox.Stderr) != "" {
				t.Fatalf("grep -q no-match should produce no stderr, got %q", gobox.Stderr)
			}
		}},
		{ID: "GREP-009", Name: "grep -r", GoboxArgs: []string{"grep", "-r", "foo", "tree"}, NativeCommand: "grep", NativeArgs: []string{"-r", "foo", "tree"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "foo\n")
			writeFile(t, filepath.Join(env.Dir, "tree", "sub", "b.txt"), "bar\nfoo\n")
		}, Normalize: sortedLines},
		{ID: "GREP-010", Name: "grep -v", GoboxArgs: []string{"grep", "-v", "foo", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-v", "foo", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\nbar\n") }},
		{ID: "GREP-011", Name: "grep -A", GoboxArgs: []string{"grep", "-A", "1", "foo", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-A", "1", "foo", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "input.txt"), "x\nfoo\na\nb\n")
		}},
		{ID: "GREP-012", Name: "grep -B", GoboxArgs: []string{"grep", "-B", "1", "foo", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-B", "1", "foo", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "input.txt"), "x\ny\nfoo\nz\n")
		}},
		{ID: "GREP-013", Name: "grep -C", GoboxArgs: []string{"grep", "-C", "1", "foo", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-C", "1", "foo", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "x\nfoo\nz\n") }},
		{ID: "GREP-014", Name: "grep --include", GoboxArgs: []string{"grep", "-r", "--include=*.log", "foo", "tree"}, NativeCommand: "grep", NativeArgs: []string{"-r", "--include=*.log", "foo", "tree"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "tree", "a.log"), "foo\n")
			writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "foo\n")
		}, Normalize: sortedLines},
		{ID: "GREP-015", Name: "grep --exclude-dir", GoboxArgs: []string{"grep", "-r", "--exclude-dir=skip", "foo", "tree"}, NativeCommand: "grep", NativeArgs: []string{"-r", "--exclude-dir=skip", "foo", "tree"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "tree", "keep", "a.txt"), "foo\n")
			writeFile(t, filepath.Join(env.Dir, "tree", "skip", "b.txt"), "foo\n")
		}, Normalize: sortedLines},
		{ID: "GREP-016", Name: "grep -l", GoboxArgs: []string{"grep", "-l", "foo", "a.txt", "b.txt"}, NativeCommand: "grep", NativeArgs: []string{"-l", "foo", "a.txt", "b.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "a.txt"), "foo\n")
			writeFile(t, filepath.Join(env.Dir, "b.txt"), "bar\n")
		}},
		{ID: "GREP-017", Name: "grep -L", GoboxArgs: []string{"grep", "-L", "foo", "a.txt", "b.txt"}, NativeCommand: "grep", NativeArgs: []string{"-L", "foo", "a.txt", "b.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "a.txt"), "foo\n")
			writeFile(t, filepath.Join(env.Dir, "b.txt"), "bar\n")
		}},
		{ID: "GREP-019", Name: "grep stdin", GoboxArgs: []string{"grep", "hello"}, NativeCommand: "grep", NativeArgs: []string{"hello"}, Stdin: "hello world\ngoodbye\n"},
		{ID: "GREP-003b", Name: "grep -c empty file", GoboxArgs: []string{"grep", "-c", "foo", "empty.txt"}, NativeCommand: "grep", NativeArgs: []string{"-c", "foo", "empty.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "empty.txt"), "") }},
		{ID: "GREP-019b", Name: "grep empty stdin", GoboxArgs: []string{"grep", "hello"}, NativeCommand: "grep", NativeArgs: []string{"hello"}, Stdin: ""},
	})

	t.Run("GREP-005", func(t *testing.T) {
		env := t.TempDir()

		// runStream starts either gobox or native `grep <flags...> foo` reading
		// from a live stdin pipe (not pre-fed), writes one matching line, and
		// checks whether it arrives on stdout before stdin is closed/EOF. This
		// distinguishes true line-by-line streaming from "buffer everything
		// until EOF" behavior, which a final-string comparison cannot detect.
		runStream := func(useNative bool, lineBuffered bool) (earlyLineSeen bool, finalStdout string) {
			args := []string{"foo"}
			if lineBuffered {
				args = []string{"--line-buffered", "foo"}
			}
			var cmd *exec.Cmd
			var stdin io.WriteCloser
			var reader *bufio.Reader
			if useNative {
				cmd, stdin, reader = startNativeStreamCmd(t, env, "grep", args)
			} else {
				cmd, stdin, reader = startGoboxStreamCmd(t, env, append([]string{"grep"}, args...))
			}
			defer func() {
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
					_, _ = cmd.Process.Wait()
				}
			}()

			if _, err := io.WriteString(stdin, "foo line one\n"); err != nil {
				t.Fatalf("write stdin: %v", err)
			}
			line, ok := readLineWithTimeout(reader, 1*time.Second)
			earlyLineSeen = ok && strings.Contains(line, "foo")

			// Feed a bit more, then close stdin so the process can exit and we
			// can collect the final buffered output for a stability check.
			_, _ = io.WriteString(stdin, "bar\nfoo line two\n")
			_ = stdin.Close()

			var buf bytes.Buffer
			if ok {
				buf.WriteString(line)
			}
			_, _ = io.Copy(&buf, reader)
			_ = cmd.Wait()
			return earlyLineSeen, buf.String()
		}

		// Baseline: native grep without --line-buffered, writing to a pipe
		// (not a tty), fully buffers and should NOT surface a match before
		// stdin is closed. This confirms the test methodology can actually
		// detect the difference --line-buffered is supposed to make.
		if baselineEarly, _ := runStream(true, false); baselineEarly {
			t.Skip("native grep without --line-buffered streamed immediately on this system; environment does not exercise buffered-vs-line-buffered behavior")
		}

		nativeEarly, nativeFull := runStream(true, true)
		if !nativeEarly {
			t.Fatalf("GREP-005: native grep --line-buffered did not surface a match before stdin closed")
		}
		goboxEarly, goboxFull := runStream(false, true)
		if !goboxEarly {
			t.Fatalf("GREP-005: gobox grep --line-buffered did not surface a match before stdin closed; streaming behavior not proven")
		}
		if normalizeText(goboxFull) != normalizeText(nativeFull) {
			t.Fatalf("grep --line-buffered final stdout mismatch\ngobox: %q\nnative: %q", goboxFull, nativeFull)
		}
	})

	t.Run("GREP-018", func(t *testing.T) {
		// Invalid regex: should exit non-zero and produce a diagnostic on stderr.
		// Use runGoboxMainCLI so the returned error is printed to stderr (as main.go does).
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "input.txt"), "hello\n")
		res := runGoboxMainCLI(t, env, "", "grep", "[invalid", "input.txt")
		if res.ExitCode == 0 {
			t.Fatalf("grep with invalid regex should exit non-zero, got 0")
		}
		if strings.TrimSpace(res.Stderr) == "" {
			t.Fatalf("grep with invalid regex should write an error to stderr, got nothing")
		}
		// Compare against native grep given the exact same illegal pattern.
		// Verified on this system: GNU grep rejects the unclosed bracket
		// expression with exit 2 ("Unmatched [, [^, [:, [., or [="), matching
		// gobox's own exit 2 for a malformed regex. Assert the exact match
		// since gobox targets GNU-grep-compatible exit codes; fall back to
		// "native also rejects it" as the sanity check on the assumption.
		native := runNativeCLI(t, env, "", "grep", "[invalid", "input.txt")
		if native.ExitCode == 0 {
			t.Fatalf("GREP-018: native grep unexpectedly accepted the illegal pattern (exit 0); test assumption invalid")
		}
		if res.ExitCode != native.ExitCode {
			t.Fatalf("GREP-018: exit code mismatch for illegal regex, gobox=%d native=%d", res.ExitCode, native.ExitCode)
		}
	})
}

func TestParity_SedCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "SED-001", Name: "sed -n", GoboxArgs: []string{"sed", "-n", "p", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"-n", "p", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\n") }},
		// Range and pattern address tests.
		{ID: "SED-017", Name: "sed line range delete", GoboxArgs: []string{"sed", "2,4d", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"2,4d", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nb\nc\nd\ne\n") }},
		{ID: "SED-018", Name: "sed last-line delete", GoboxArgs: []string{"sed", "$d", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"$d", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nb\nc\n") }},
		{ID: "SED-019", Name: "sed pattern address delete", GoboxArgs: []string{"sed", "/foo/d", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"/foo/d", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nfoo\nb\n") }},
		// stdin via pipe.
		{ID: "SED-020", Name: "sed stdin", GoboxArgs: []string{"sed", "s/hello/world/"}, NativeCommand: "sed", NativeArgs: []string{"s/hello/world/"}, Stdin: "hello\n"},
		{ID: "SED-003", Name: "sed -e", GoboxArgs: []string{"sed", "-e", "s/foo/bar/", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"-e", "s/foo/bar/", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") }},
		{ID: "SED-004", Name: "sed -f", GoboxArgs: []string{"sed", "-f", "script.sed", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"-f", "script.sed", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "script.sed"), "s/foo/bar/\n")
			writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n")
		}},
		{ID: "SED-006", Name: "sed substitute", GoboxArgs: []string{"sed", "s/foo/bar/", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"s/foo/bar/", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") }},
		{ID: "SED-007", Name: "sed d", GoboxArgs: []string{"sed", "d", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"d", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") }},
		{ID: "SED-008", Name: "sed p", GoboxArgs: []string{"sed", "p", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"p", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") }},
		{ID: "SED-009", Name: "sed =", GoboxArgs: []string{"sed", "=", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"=", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nb\n") }},
		{ID: "SED-010", Name: "sed i\\", GoboxArgs: []string{"sed", "1i\\before", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"1i\\before", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\n") }},
		{ID: "SED-011", Name: "sed a\\", GoboxArgs: []string{"sed", "1a\\after", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"1a\\after", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\n") }},
		{ID: "SED-012", Name: "sed c\\", GoboxArgs: []string{"sed", "1c\\changed", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"1c\\changed", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\n") }},
		{ID: "SED-013", Name: "sed s///g", GoboxArgs: []string{"sed", "s/foo/bar/g", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"s/foo/bar/g", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo foo\n") }},
		{ID: "SED-014", Name: "sed s///i", GoboxArgs: []string{"sed", "s/foo/bar/i", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"s/foo/bar/i", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "Foo\n") }},
		{ID: "SED-015", Name: "sed s///p", GoboxArgs: []string{"sed", "-n", "s/foo/bar/p", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"-n", "s/foo/bar/p", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") }},
		{ID: "SED-016", Name: "sed s///N", GoboxArgs: []string{"sed", "s/foo/bar/2", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"s/foo/bar/2", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo foo foo\n") }},
		{ID: "SED-006b", Name: "sed s/// empty file", GoboxArgs: []string{"sed", "s/foo/bar/", "empty.txt"}, NativeCommand: "sed", NativeArgs: []string{"s/foo/bar/", "empty.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "empty.txt"), "") }},
		{ID: "SED-020b", Name: "sed s/// empty stdin", GoboxArgs: []string{"sed", "s/hello/world/"}, NativeCommand: "sed", NativeArgs: []string{"s/hello/world/"}, Stdin: ""},
	})

	t.Run("SED-005", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "sed", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("sed -h failed: %+v", res)
		}
		if strings.TrimSpace(res.Stderr) != "" {
			t.Fatalf("sed -h should not write stderr, got %q", res.Stderr)
		}
		// Assert every flag sed actually supports (per cmds/text/cmd_sed.go's
		// help text: -n, -i, -e, -f, -h), not just a couple of substrings, so
		// stale/incomplete help text is caught.
		for _, want := range []string{
			"Usage: gobox sed", "Options:", "Commands:", "s/pattern/replacement/flags",
			"-n ", "-i[SUFFIX]", "-e SCRIPT", "-f FILE", "-h, --help",
		} {
			if !strings.Contains(res.Stdout, want) {
				t.Fatalf("sed -h missing %q in %q", want, res.Stdout)
			}
		}
	})

	// SED-002: sed -i modifies file in-place.  Run gobox and native in separate dirs
	// so each sees a fresh copy of the file, then compare modified content and backup.
	t.Run("SED-002", func(t *testing.T) {
		base := t.TempDir()
		gDir := filepath.Join(base, "gobox")
		nDir := filepath.Join(base, "native")
		if err := os.MkdirAll(gDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(nDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(gDir, "input.txt"), "foo\n")
		writeFile(t, filepath.Join(nDir, "input.txt"), "foo\n")

		gobox := runGoboxCLI(t, gDir, "", "sed", "-i.bak", "s/foo/bar/", "input.txt")
		native := runNativeCLI(t, nDir, "", "sed", "-i.bak", "s/foo/bar/", "input.txt")

		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("sed -i exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		g, err := os.ReadFile(filepath.Join(gDir, "input.txt"))
		if err != nil {
			t.Fatalf("sed -i: gobox modified file missing: %v", err)
		}
		n, err := os.ReadFile(filepath.Join(nDir, "input.txt"))
		if err != nil {
			t.Fatalf("sed -i: native modified file missing: %v", err)
		}
		if normalizeText(string(g)) != normalizeText(string(n)) {
			t.Fatalf("sed -i modified file content mismatch\ngobox: %q\nnative: %q", string(g), string(n))
		}
		// Both gobox and native must produce the .bak backup file; a missing
		// backup on either side is a regression and must fail loudly rather
		// than being silently skipped.
		gBak, gErr := os.ReadFile(filepath.Join(gDir, "input.txt.bak"))
		if gErr != nil {
			t.Fatalf("sed -i.bak: gobox backup file missing: %v", gErr)
		}
		nBak, nErr := os.ReadFile(filepath.Join(nDir, "input.txt.bak"))
		if nErr != nil {
			t.Fatalf("sed -i.bak: native backup file missing: %v", nErr)
		}
		if normalizeText(string(gBak)) != normalizeText(string(nBak)) {
			t.Fatalf("sed -i .bak content mismatch\ngobox: %q\nnative: %q", string(gBak), string(nBak))
		}
	})
}

func TestParity_SortCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "SORT-001", Name: "sort -n", GoboxArgs: []string{"sort", "-n", "input.txt"}, NativeCommand: "sort", NativeArgs: []string{"-n", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "10\n2\n1\n") }},
		{ID: "SORT-002", Name: "sort -r", GoboxArgs: []string{"sort", "-r", "input.txt"}, NativeCommand: "sort", NativeArgs: []string{"-r", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nc\nb\n") }},
		{ID: "SORT-003", Name: "sort -k", GoboxArgs: []string{"sort", "-k", "2", "input.txt"}, NativeCommand: "sort", NativeArgs: []string{"-k", "2", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a 2\nb 1\n") }},
		{ID: "SORT-004", Name: "sort -t", GoboxArgs: []string{"sort", "-t", ",", "-k", "2", "input.txt"}, NativeCommand: "sort", NativeArgs: []string{"-t", ",", "-k", "2", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a,2\nb,1\n") }},
		{ID: "SORT-005", Name: "sort -u", GoboxArgs: []string{"sort", "-u", "input.txt"}, NativeCommand: "sort", NativeArgs: []string{"-u", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n") }},
		{ID: "SORT-006", Name: "sort -M", GoboxArgs: []string{"sort", "-M", "input.txt"}, NativeCommand: "sort", NativeArgs: []string{"-M", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "Feb\nJan\n") }},
		{ID: "SORT-007", Name: "sort -h", GoboxArgs: []string{"sort", "-h", "input.txt"}, NativeCommand: "sort", NativeArgs: []string{"-h", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "2K\n512\n1M\n") }},
		{ID: "SORT-009", Name: "sort -c", GoboxArgs: []string{"sort", "-c", "input.txt"}, NativeCommand: "sort", NativeArgs: []string{"-c", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "b\na\n") }, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("sort -c exit mismatch %d != %d", gobox.ExitCode, native.ExitCode)
			}
			// Both must emit a disorder diagnostic on stderr when exit code is non-zero.
			if gobox.ExitCode != 0 {
				if strings.TrimSpace(gobox.Stderr) == "" {
					t.Fatalf("sort -c: gobox stderr should be non-empty for disordered input, got %q", gobox.Stderr)
				}
				if strings.TrimSpace(native.Stderr) == "" {
					t.Fatalf("sort -c: native stderr should be non-empty for disordered input, got %q", native.Stderr)
				}
			}
		}},
		{ID: "SORT-001b", Name: "sort -n empty file", GoboxArgs: []string{"sort", "-n", "empty.txt"}, NativeCommand: "sort", NativeArgs: []string{"-n", "empty.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "empty.txt"), "") }},
		{ID: "SORT-012a", Name: "sort -tCHAR concatenated", GoboxArgs: []string{"sort", "-t,", "-k", "2", "input.txt"}, NativeCommand: "sort", NativeArgs: []string{"-t,", "-k", "2", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a,2\nb,1\n") }},
		{ID: "SORT-012b", Name: "sort --field-separator=CHAR", GoboxArgs: []string{"sort", "--field-separator=,", "-k", "2", "input.txt"}, NativeCommand: "sort", NativeArgs: []string{"--field-separator=,", "-k", "2", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a,2\nb,1\n") }},
	})

	t.Run("SORT-008", func(t *testing.T) {
		// cmds/text/cmd_sort.go seeds `-R`'s Fisher-Yates shuffle from
		// time.Now().UnixNano() on every invocation, so there is no seed flag
		// we can pin for a deterministic non-identity assertion. Instead, run
		// enough independent invocations that observing the identity
		// permutation (input order) on every single run would require a
		// probability of about (1/6)^numRuns for 3 equally-likely lines - low
		// enough to rule out "shuffle skipped entirely" while avoiding flakes
		// from one unlucky identity permutation. This is verified against
		// numRuns=1 having exposed a regression that always returned the
		// input order unshuffled.
		env := &parityEnv{Dir: t.TempDir()}
		inputPath := filepath.Join(env.Dir, "input.txt")
		writeFile(t, inputPath, "a\nb\nc\n")
		const inputOrder = "a,b,c"
		const numRuns = 12
		sawNonIdentity := false
		for i := 0; i < numRuns; i++ {
			result := runGoboxCLI(t, env.Dir, "", "sort", "-R", "input.txt")
			if result.ExitCode != 0 {
				t.Fatalf("sort -R failed: %+v", result)
			}
			lines := strings.Split(normalizeText(result.Stdout), "\n")
			if len(lines) != 3 {
				t.Fatalf("expected 3 lines from sort -R, got %v", lines)
			}
			for _, want := range []string{"a", "b", "c"} {
				found := false
				for _, got := range lines {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("sort -R output missing %s: %v", want, lines)
				}
			}
			if strings.Join(lines, ",") != inputOrder {
				sawNonIdentity = true
			}
		}
		if !sawNonIdentity {
			t.Fatalf("sort -R produced the identity permutation (%s) in all %d runs; "+
				"the Fisher-Yates shuffle appears to be skipped or a no-op", inputOrder, numRuns)
		}
	})

	t.Run("SORT-011", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "input.txt"), "b\x00a\x00")
		gobox := runGoboxCLI(t, env, "", "sort", "-z", "input.txt")
		native := runNativeCLI(t, env, "", "sort", "-z", "input.txt")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("sort -z exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if gobox.Stdout != native.Stdout {
			t.Fatalf("sort -z stdout mismatch (NUL-separated)\ngobox:  %q\nnative: %q", gobox.Stdout, native.Stdout)
		}
	})

	// SORT-010: sort -o writes output to a file.
	// Uses filepath.Join to find output files after withTempChdir restores cwd.
	t.Run("SORT-010", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "input.txt"), "b\na\n")
		gobox := runGoboxCLI(t, env, "", "sort", "-o", "out.txt", "input.txt")
		native := runNativeCLI(t, env, "", "sort", "-o", "native.txt", "input.txt")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("sort -o exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		g, err := os.ReadFile(filepath.Join(env, "out.txt"))
		if err != nil {
			t.Fatalf("sort -o: gobox output file missing: %v", err)
		}
		n, err := os.ReadFile(filepath.Join(env, "native.txt"))
		if err != nil {
			t.Fatalf("sort -o: native output file missing: %v", err)
		}
		if normalizeText(string(g)) != normalizeText(string(n)) {
			t.Fatalf("sort -o file output mismatch\ngobox: %s\nnative: %s", string(g), string(n))
		}
	})
}

func TestParity_UniqCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "UNIQ-001", Name: "uniq -c", GoboxArgs: []string{"uniq", "-c", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-c", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n") }, Normalize: collapseSpaces},
		{ID: "UNIQ-002", Name: "uniq -d", GoboxArgs: []string{"uniq", "-d", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-d", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n") }},
		{ID: "UNIQ-003", Name: "uniq -u", GoboxArgs: []string{"uniq", "-u", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-u", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n") }},
		{ID: "UNIQ-004", Name: "uniq -i", GoboxArgs: []string{"uniq", "-i", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-i", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "A\na\nb\n") }},
		{ID: "UNIQ-005", Name: "uniq -w", GoboxArgs: []string{"uniq", "-w", "2", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-w", "2", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "input.txt"), "ab1\nab2\nxy1\n")
		}},
		{ID: "UNIQ-006", Name: "uniq -f", GoboxArgs: []string{"uniq", "-f", "1", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-f", "1", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "input.txt"), "x a\ny a\nz b\n")
		}},
		{ID: "UNIQ-007", Name: "uniq default dedup", GoboxArgs: []string{"uniq", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"input.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n")
		}},
		{ID: "UNIQ-008", Name: "uniq -c -d combined", GoboxArgs: []string{"uniq", "-c", "-d", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-c", "-d", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n")
		}, Normalize: collapseSpaces},
		{ID: "UNIQ-001b", Name: "uniq -c empty file", GoboxArgs: []string{"uniq", "-c", "empty.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-c", "empty.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "empty.txt"), "") }, Normalize: collapseSpaces},
	})
}

func TestParity_WcCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "WC-001", Name: "wc -l", GoboxArgs: []string{"wc", "-l", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"-l", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nb\n") }, Normalize: collapseSpaces},
		{ID: "WC-002", Name: "wc -w", GoboxArgs: []string{"wc", "-w", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"-w", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a b\n") }, Normalize: collapseSpaces},
		{ID: "WC-003", Name: "wc -c", GoboxArgs: []string{"wc", "-c", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"-c", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "abc") }, Normalize: collapseSpaces},
		{ID: "WC-004", Name: "wc -m", GoboxArgs: []string{"wc", "-m", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"-m", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "你好a") }, Normalize: collapseSpaces},
		{ID: "WC-005", Name: "wc -L", GoboxArgs: []string{"wc", "-L", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"-L", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nlonger\n") }, Normalize: collapseSpaces},
		{ID: "WC-006", Name: "wc multiple files (total line)", GoboxArgs: []string{"wc", "-l", "f1.txt", "f2.txt"}, NativeCommand: "wc", NativeArgs: []string{"-l", "f1.txt", "f2.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "f1.txt"), "a\nb\n")
			writeFile(t, filepath.Join(env.Dir, "f2.txt"), "c\n")
		}, Normalize: collapseSpaces},
		{ID: "WC-007", Name: "wc default (lines+words+bytes)", GoboxArgs: []string{"wc", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "hello world\nfoo\n") }, Normalize: collapseSpaces},
		{ID: "WC-008", Name: "wc -lw combined", GoboxArgs: []string{"wc", "-lw", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"-lw", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "hello world\nfoo\n") }, Normalize: collapseSpaces},
		{ID: "WC-001b", Name: "wc -l empty file", GoboxArgs: []string{"wc", "-l", "empty.txt"}, NativeCommand: "wc", NativeArgs: []string{"-l", "empty.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "empty.txt"), "") }, Normalize: collapseSpaces},
	})
}

func TestParity_StringsCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "STRINGS-001", Name: "strings default", GoboxArgs: []string{"strings", "data.bin"}, NativeCommand: "strings", NativeArgs: []string{"data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			if err := os.WriteFile(filepath.Join(env.Dir, "data.bin"), []byte{0, 'a', 'b', 'c', 'd', 0, 'x', 'y', 'z', '1', 0}, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{ID: "STRINGS-002", Name: "strings -n", GoboxArgs: []string{"strings", "-n", "5", "data.bin"}, NativeCommand: "strings", NativeArgs: []string{"-n", "5", "data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			if err := os.WriteFile(filepath.Join(env.Dir, "data.bin"), []byte{0, 'h', 'e', 'l', 'l', 'o', 0}, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{ID: "STRINGS-003", Name: "strings -f", GoboxArgs: []string{"strings", "-f", "a.bin", "b.bin"}, NativeCommand: "strings", NativeArgs: []string{"-f", "a.bin", "b.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			if err := os.WriteFile(filepath.Join(env.Dir, "a.bin"), []byte{0, 'a', 'l', 'p', 'h', 'a', 0}, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(env.Dir, "b.bin"), []byte{0, 'b', 'e', 't', 'a', 0}, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{ID: "STRINGS-004", Name: "strings -t x", GoboxArgs: []string{"strings", "-t", "x", "data.bin"}, NativeCommand: "strings", NativeArgs: []string{"-t", "x", "data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			if err := os.WriteFile(filepath.Join(env.Dir, "data.bin"), []byte{0, 0, 'h', 'e', 'l', 'l', 'o', 0}, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{ID: "STRINGS-005", Name: "strings -a", GoboxArgs: []string{"strings", "-a", "data.bin"}, NativeCommand: "strings", NativeArgs: []string{"-a", "data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			if err := os.WriteFile(filepath.Join(env.Dir, "data.bin"), []byte{0, 'a', 'b', 'c', 'd', 0, 'w', 'x', 'y', 'z', 0}, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{ID: "STRINGS-006", Name: "strings -t o (octal offsets)", GoboxArgs: []string{"strings", "-t", "o", "data.bin"}, NativeCommand: "strings", NativeArgs: []string{"-t", "o", "data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			if err := os.WriteFile(filepath.Join(env.Dir, "data.bin"), []byte{0, 0, 'h', 'e', 'l', 'l', 'o', 0}, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{ID: "STRINGS-007", Name: "strings -t d (decimal offsets)", GoboxArgs: []string{"strings", "-t", "d", "data.bin"}, NativeCommand: "strings", NativeArgs: []string{"-t", "d", "data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			if err := os.WriteFile(filepath.Join(env.Dir, "data.bin"), []byte{0, 0, 'h', 'e', 'l', 'l', 'o', 0}, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	})
}

func TestParity_HexCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "HEX-001", Name: "hex --dump -C", GoboxArgs: []string{"hex", "--dump", "-C", "data.bin"}, NativeCommand: "hexdump", NativeArgs: []string{"-C", "data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			_ = os.WriteFile(filepath.Join(env.Dir, "data.bin"), []byte("hello\nworld"), 0o644)
		}},
		{ID: "HEX-002", Name: "hex --dump -n", GoboxArgs: []string{"hex", "--dump", "-n", "5", "data.bin"}, NativeCommand: "hexdump", NativeArgs: []string{"-n", "5", "-C", "data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			_ = os.WriteFile(filepath.Join(env.Dir, "data.bin"), []byte("hello\nworld"), 0o644)
		}},
		{ID: "HEX-003", Name: "hex --dump -s", GoboxArgs: []string{"hex", "--dump", "-s", "2", "data.bin"}, NativeCommand: "hexdump", NativeArgs: []string{"-s", "2", "-C", "data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			_ = os.WriteFile(filepath.Join(env.Dir, "data.bin"), []byte("hello\nworld"), 0o644)
		}},
		{ID: "HEX-004", Name: "hex --dump -v", GoboxArgs: []string{"hex", "--dump", "-v", "data.bin"}, NativeCommand: "hexdump", NativeArgs: []string{"-v", "-C", "data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			_ = os.WriteFile(filepath.Join(env.Dir, "data.bin"), append([]byte(strings.Repeat("A", 16)), []byte(strings.Repeat("A", 16))...), 0o644)
		}},
		{ID: "HEX-005", Name: "hex --dump -e", GoboxArgs: []string{"hex", "--dump", "-e", "%02x", "data.bin"}, NativeCommand: "hexdump", NativeArgs: []string{"-v", "-e", `1/1 "%02x"`, "data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			_ = os.WriteFile(filepath.Join(env.Dir, "data.bin"), []byte("abc"), 0o644)
		}},
		{ID: "HEX-009", Name: "hex --dump -C default folds repeated rows", GoboxArgs: []string{"hex", "--dump", "-C", "data.bin"}, NativeCommand: "hexdump", NativeArgs: []string{"-C", "data.bin"}, Setup: func(t *testing.T, env *parityEnv) {
			data := append([]byte(strings.Repeat("A", 16)), []byte(strings.Repeat("A", 16))...)
			data = append(data, []byte(strings.Repeat("A", 16))...)
			data = append(data, []byte("last row")...)
			_ = os.WriteFile(filepath.Join(env.Dir, "data.bin"), data, 0o644)
		}},
	})

	t.Run("HEX-006", func(t *testing.T) {
		env := t.TempDir()
		data := []byte("hello\n")
		if err := os.WriteFile(filepath.Join(env, "data.bin"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		res := runGoboxCLI(t, env, "", "hex", "--encode", "data.bin")
		if res.ExitCode != 0 {
			t.Fatalf("hex --encode failed: %+v", res)
		}
		encoded := strings.TrimSpace(res.Stdout)
		if encoded != "68656c6c6f0a" {
			t.Fatalf("unexpected hex encode output %q", encoded)
		}
		decoded := runGoboxCLI(t, env, encoded, "hex", "--decode")
		if decoded.ExitCode != 0 || decoded.Stdout != string(data) {
			t.Fatalf("hex encode/decode roundtrip mismatch encode=%q decode=%+v", encoded, decoded)
		}
	})

	t.Run("HEX-007", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "68 65\n6c6c6f", "hex", "--decode")
		if res.ExitCode != 0 || res.Stdout != "hello" {
			t.Fatalf("hex --decode failed: %+v", res)
		}
	})

	t.Run("HEX-008", func(t *testing.T) {
		env := t.TempDir()
		outFile := filepath.Join(env, "decoded.bin")
		res := runGoboxCLI(t, env, "68656c6c6f", "hex", "--decode", "-o", "decoded.bin")
		if res.ExitCode != 0 {
			t.Fatalf("hex --decode -o failed: %+v", res)
		}
		if res.Stdout != "" {
			t.Fatalf("hex --decode -o should not write stdout, got %q", res.Stdout)
		}
		data, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "hello" {
			t.Fatalf("unexpected decoded file %q", string(data))
		}
	})
}

func TestParity_Base64Cases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{
			ID:            "BASE64-001",
			Name:          "base64 default",
			GoboxArgs:     []string{"base64", "-w", "0", "data.bin"},
			NativeCommand: "base64",
			NativeArgs:    []string{"-w", "0", "data.bin"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data.bin"), "hello")
			},
		},
		{
			ID:            "BASE64-002",
			Name:          "base64 decode",
			GoboxArgs:     []string{"base64", "-d", "data.b64"},
			NativeCommand: "base64",
			NativeArgs:    []string{"-d", "data.b64"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data.b64"), "aGVsbG8=")
			},
		},
		{
			ID:            "BASE64-003",
			Name:          "base64 wrap",
			GoboxArgs:     []string{"base64", "-w", "4", "data.bin"},
			NativeCommand: "base64",
			NativeArgs:    []string{"-w", "4", "data.bin"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data.bin"), "hello world")
			},
		},
		{
			ID:            "BASE64-004",
			Name:          "base64 ignore garbage",
			GoboxArgs:     []string{"base64", "-d", "-i", "dirty.b64"},
			NativeCommand: "base64",
			NativeArgs:    []string{"-d", "-i", "dirty.b64"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "dirty.b64"), "aG!!Vs\nbG8=")
			},
		},
		{
			ID:            "BASE64-002b",
			Name:          "base64 --decode (long form)",
			GoboxArgs:     []string{"base64", "--decode", "data.b64"},
			NativeCommand: "base64",
			NativeArgs:    []string{"--decode", "data.b64"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data.b64"), "aGVsbG8=")
			},
		},
		{
			ID:            "BASE64-003b",
			Name:          "base64 --wrap (long form)",
			GoboxArgs:     []string{"base64", "--wrap", "4", "data.bin"},
			NativeCommand: "base64",
			NativeArgs:    []string{"--wrap", "4", "data.bin"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data.bin"), "hello world")
			},
		},
		{
			ID:            "BASE64-004b",
			Name:          "base64 --decode --ignore-garbage (long form)",
			GoboxArgs:     []string{"base64", "--decode", "--ignore-garbage", "dirty.b64"},
			NativeCommand: "base64",
			NativeArgs:    []string{"--decode", "--ignore-garbage", "dirty.b64"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "dirty.b64"), "aG!!Vs\nbG8=")
			},
		},
	})

	t.Run("BASE64-005", func(t *testing.T) {
		env := t.TempDir()
		outFile := filepath.Join(env, "out.b64")
		writeFile(t, filepath.Join(env, "data.bin"), "hello world")
		res := runGoboxCLI(t, env, "", "base64", "-w", "0", "-o", outFile, "data.bin")
		if res.ExitCode != 0 {
			t.Fatalf("base64 -o failed: %+v", res)
		}
		if res.Stdout != "" {
			t.Fatalf("base64 -o should not write stdout, got %q", res.Stdout)
		}
		data, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "aGVsbG8gd29ybGQ=" {
			t.Fatalf("unexpected base64 output file %q", string(data))
		}
	})
}

func TestParity_SeqCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "SEQ-001", Name: "seq 5", GoboxArgs: []string{"seq", "5"}, NativeCommand: "seq", NativeArgs: []string{"5"}},
		{ID: "SEQ-002", Name: "seq 2 5", GoboxArgs: []string{"seq", "2", "5"}, NativeCommand: "seq", NativeArgs: []string{"2", "5"}},
		{ID: "SEQ-003", Name: "seq 1 2 9 (step)", GoboxArgs: []string{"seq", "1", "2", "9"}, NativeCommand: "seq", NativeArgs: []string{"1", "2", "9"}},
		{ID: "SEQ-005", Name: "seq -s , 5 (custom separator)", GoboxArgs: []string{"seq", "-s", ",", "5"}, NativeCommand: "seq", NativeArgs: []string{"-s", ",", "5"}},
	})

	t.Run("SEQ-004", func(t *testing.T) {
		// -f FORMAT is a behavior case: the documented assertion (docs/TEST-CASES.md)
		// is that the format changes each item's text representation relative to
		// the default output while the underlying numeric sequence stays the same.
		env := t.TempDir()
		defaultOut := runGoboxCLI(t, env, "", "seq", "3")
		if defaultOut.ExitCode != 0 {
			t.Fatalf("seq 3 failed: %+v", defaultOut)
		}
		formatted := runGoboxCLI(t, env, "", "seq", "-f", "[%g]", "3")
		if formatted.ExitCode != 0 {
			t.Fatalf("seq -f failed: %+v", formatted)
		}
		if normalizeText(formatted.Stdout) == normalizeText(defaultOut.Stdout) {
			t.Fatalf("seq -f FORMAT must change the per-item text representation relative to the default output, got identical output %q", formatted.Stdout)
		}
		want := []string{"[1]", "[2]", "[3]"}
		got := nonEmptyLines(formatted.Stdout)
		if len(got) != len(want) {
			t.Fatalf("seq -f: expected %d lines %v, got %d: %v", len(want), want, len(got), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("seq -f: line %d = %q, want %q (full output %q); numeric sequence must remain 1,2,3", i, got[i], want[i], formatted.Stdout)
			}
		}
	})

	t.Run("SEQ-006", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "seq", "-w", "8", "12")
		native := runNativeCLI(t, env, "", "seq", "-w", "8", "12")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("seq -w exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("seq -w stdout mismatch\ngobox: %q\nnative: %q", gobox.Stdout, native.Stdout)
		}
		defaultOut := runGoboxCLI(t, env, "", "seq", "8", "12")
		if normalizeText(defaultOut.Stdout) == normalizeText(gobox.Stdout) {
			t.Fatalf("seq -w must change text width (zero-padding) relative to default output, got identical %q", gobox.Stdout)
		}
	})

	t.Run("SEQ-007", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "seq", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("seq -h failed: %+v", res)
		}
		if strings.TrimSpace(res.Stderr) != "" {
			t.Fatalf("seq -h should not write stderr, got %q", res.Stderr)
		}
		// Assert every flag seq actually supports (per cmds/text/cmd_seq.go's
		// help text: -f/--format, -s/--separator, -w/--equal-width, -h/--help),
		// not just a couple of substrings, so stale/incomplete help text is caught.
		for _, want := range []string{
			"Usage: gobox seq", "Options:", "-f, --format", "-s, --separator",
			"-w, --equal-width", "-h, --help", "Examples:",
		} {
			if !strings.Contains(res.Stdout, want) {
				t.Fatalf("seq -h missing %q in %q", want, res.Stdout)
			}
		}
	})
}

func TestParity_RandCases(t *testing.T) {
	// rand has no native equivalent; these are contract tests.

	t.Run("RAND-001", func(t *testing.T) {
		// bare rand: produces one line of hex output (64 chars for the default 32 bytes).
		res := runGoboxCLI(t, t.TempDir(), "", "rand")
		if res.ExitCode != 0 {
			t.Fatalf("rand failed: %+v", res)
		}
		line := strings.TrimSpace(res.Stdout)
		if line == "" {
			t.Fatalf("rand produced no output")
		}
		// Validate that every character is a hex digit.
		for _, c := range line {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Fatalf("rand output %q contains non-hex character %q", line, string(c))
			}
		}
		if len(line) != 64 {
			t.Fatalf("rand: expected 64 hex chars (32 bytes), got %d chars: %q", len(line), line)
		}
	})

	t.Run("RAND-002", func(t *testing.T) {
		// Bare positional NUM operand (no -n flag): `rand 5` must generate 5
		// bytes, i.e. 10 hex chars in the default hex encoding.
		res := runGoboxCLI(t, t.TempDir(), "", "rand", "5")
		if res.ExitCode != 0 {
			t.Fatalf("rand 5 failed: %+v", res)
		}
		line := strings.TrimSpace(res.Stdout)
		if len(line) != 10 {
			t.Fatalf("rand 5 (positional NUM): expected 10 hex chars (5 bytes), got %d chars: %q", len(line), line)
		}
		for _, c := range line {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Fatalf("rand 5 output %q contains non-hex character %q", line, string(c))
			}
		}
	})

	t.Run("RAND-003", func(t *testing.T) {
		// rand -n 5: explicit byte-count flag produces 5 bytes as hex (10 hex chars).
		res := runGoboxCLI(t, t.TempDir(), "", "rand", "-n", "5")
		if res.ExitCode != 0 {
			t.Fatalf("rand -n 5 failed: %+v", res)
		}
		line := strings.TrimSpace(res.Stdout)
		if line == "" {
			t.Fatalf("rand -n 5 produced no output")
		}
		if len(line) != 10 {
			t.Fatalf("rand -n 5: expected 10 hex chars (5 bytes), got %d chars: %q", len(line), line)
		}
	})

	t.Run("RAND-004", func(t *testing.T) {
		// -hex is rand's default output format (see cmds/text/cmd_rand.go:
		// "-hex Hex output (default)"), so checking that `rand -hex` merely
		// *looks* like hex can't distinguish "the -hex flag was parsed and
		// honored" from "the -hex flag was silently ignored and gobox fell
		// back to its default anyway" - both produce identical output.
		// To make the flag's effect observable, first request the
		// non-default -base64 format, then request -hex, and assert the two
		// output shapes actually differ (distinct alphabets/lengths for the
		// same 32-byte payload, as also verified in RAND-005). If -hex were
		// broken and silently fell back to -base64's format (or vice versa),
		// this comparison would catch it.
		b64Res := runGoboxCLI(t, t.TempDir(), "", "rand", "-base64")
		if b64Res.ExitCode != 0 {
			t.Fatalf("rand -base64 failed: %+v", b64Res)
		}
		b64Line := strings.TrimSpace(b64Res.Stdout)

		res := runGoboxCLI(t, t.TempDir(), "", "rand", "-hex")
		if res.ExitCode != 0 {
			t.Fatalf("rand -hex failed: %+v", res)
		}
		if !strings.HasSuffix(res.Stdout, "\n") {
			t.Fatalf("rand -hex output must end with a newline, got %q", res.Stdout)
		}
		if strings.Count(res.Stdout, "\n") != 1 {
			t.Fatalf("rand -hex output must contain exactly one newline, got %q", res.Stdout)
		}
		line := strings.TrimSpace(res.Stdout)
		if len(line) != 64 {
			t.Fatalf("rand -hex: expected 64 hex chars (32 bytes), got %d chars: %q", len(line), line)
		}
		for _, c := range line {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Fatalf("rand -hex output %q contains non-hex character %q", line, string(c))
			}
		}
		// The -hex shape (64 lowercase hex chars) must differ from the
		// -base64 shape (44 chars incl. padding, mixed-case + '+'/'/'/'=')
		// for the same 32-byte payload, proving -hex actually selected a
		// distinct encoding rather than reusing whatever -base64 produced.
		if len(line) == len(b64Line) {
			t.Fatalf("rand -hex output length (%d) must differ from -base64 output length (%d) for the same byte count", len(line), len(b64Line))
		}
		if line == b64Line {
			t.Fatalf("rand -hex output must not equal -base64 output: %q", line)
		}
	})

	t.Run("RAND-005", func(t *testing.T) {
		// -base64: must change the encoding alphabet and length semantics
		// relative to the default hex output. For 32 random bytes, base64
		// standard encoding is 44 characters (including one '=' padding
		// char), versus 64 hex characters. Decoding must round-trip to
		// exactly 32 bytes.
		hexRes := runGoboxCLI(t, t.TempDir(), "", "rand", "-hex")
		if hexRes.ExitCode != 0 {
			t.Fatalf("rand -hex failed: %+v", hexRes)
		}
		b64Res := runGoboxCLI(t, t.TempDir(), "", "rand", "-base64")
		if b64Res.ExitCode != 0 {
			t.Fatalf("rand -base64 failed: %+v", b64Res)
		}
		line := strings.TrimSpace(b64Res.Stdout)
		if line == "" {
			t.Fatalf("rand -base64 produced no output")
		}
		if len(line) == len(strings.TrimSpace(hexRes.Stdout)) {
			t.Fatalf("rand -base64 output length (%d) must differ from -hex output length for the same byte count", len(line))
		}
		for _, c := range line {
			isAlpha := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
			isDigit := c >= '0' && c <= '9'
			if !isAlpha && !isDigit && c != '+' && c != '/' && c != '=' {
				t.Fatalf("rand -base64 output %q contains a character outside the base64 alphabet: %q", line, string(c))
			}
		}
		decoded, err := base64.StdEncoding.DecodeString(line)
		if err != nil {
			t.Fatalf("rand -base64 output is not valid base64: %v (output %q)", err, line)
		}
		if len(decoded) != 32 {
			t.Fatalf("rand -base64: decoded payload should be 32 bytes, got %d", len(decoded))
		}
	})

	t.Run("RAND-006", func(t *testing.T) {
		// -out FILE: the random payload must be written to the file, and
		// stdout must not be polluted with the random body.
		env := t.TempDir()
		outPath := filepath.Join(env, "rand.out")
		res := runGoboxCLI(t, env, "", "rand", "-n", "8", "-out", outPath)
		if res.ExitCode != 0 {
			t.Fatalf("rand -out failed: %+v", res)
		}
		if res.Stdout != "" {
			t.Fatalf("rand -out should not write the random payload to stdout, got %q", res.Stdout)
		}
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("rand -out: output file missing: %v", err)
		}
		line := strings.TrimSpace(string(data))
		if len(line) != 16 {
			t.Fatalf("rand -n 8 -out: expected 16 hex chars (8 bytes) in output file, got %d: %q", len(line), line)
		}
	})

	t.Run("RAND-007", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "rand", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("rand -h failed: %+v", res)
		}
		if strings.TrimSpace(res.Stderr) != "" {
			t.Fatalf("rand -h should not write stderr, got %q", res.Stderr)
		}
		// Assert every flag rand actually supports (per cmds/text/cmd_rand.go's
		// printRandUsage: -n/-NUM, -hex, -base64, -out, -h), not just a couple
		// of substrings, so stale/incomplete help text is caught.
		for _, want := range []string{
			"Usage: gobox rand", "Options:", "-n NUM", "-hex", "-base64",
			"-out FILE", "-h, --help", "Examples:",
		} {
			if !strings.Contains(res.Stdout, want) {
				t.Fatalf("rand -h missing %q in %q", want, res.Stdout)
			}
		}
	})

	t.Run("RAND-008", func(t *testing.T) {
		// -NUM bare shorthand must behave exactly like -n NUM: same byte
		// count, same hex encoding.
		shorthand := runGoboxCLI(t, t.TempDir(), "", "rand", "-16")
		if shorthand.ExitCode != 0 {
			t.Fatalf("rand -16 failed: %+v", shorthand)
		}
		explicit := runGoboxCLI(t, t.TempDir(), "", "rand", "-n", "16")
		if explicit.ExitCode != 0 {
			t.Fatalf("rand -n 16 failed: %+v", explicit)
		}
		shorthandLine := strings.TrimSpace(shorthand.Stdout)
		explicitLine := strings.TrimSpace(explicit.Stdout)
		if len(shorthandLine) != 32 {
			t.Fatalf("rand -16: expected 32 hex chars (16 bytes), got %d chars: %q", len(shorthandLine), shorthandLine)
		}
		if len(shorthandLine) != len(explicitLine) {
			t.Fatalf("rand -16 and rand -n 16 should produce the same output length, got %d vs %d", len(shorthandLine), len(explicitLine))
		}
	})
}
