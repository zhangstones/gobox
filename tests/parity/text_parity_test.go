package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	})

	t.Run("HEAD-004", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "head", "-h")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Usage") {
			t.Fatalf("head -h failed: %+v", res)
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
		for _, want := range []string{"diff -r left/sub/a.txt right/sub/a.txt", "Only in right/sub: b.txt"} {
			if !strings.Contains(res.Stdout, want) || !strings.Contains(native.Stdout, want) {
				t.Fatalf("diff -r missing %q\n--- gobox ---\n%s\n--- native ---\n%s", want, res.Stdout, native.Stdout)
			}
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

func TestParity_TailCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "TAIL-001", Name: "tail -n", GoboxArgs: []string{"tail", "-n", "2", "input.txt"}, NativeCommand: "tail", NativeArgs: []string{"-n", "2", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "1\n2\n3\n") }},
		{ID: "TAIL-005", Name: "tail -q", GoboxArgs: []string{"tail", "-q", "-n", "1", "a.txt", "b.txt"}, NativeCommand: "tail", NativeArgs: []string{"-q", "-n", "1", "a.txt", "b.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "a.txt"), "a1\na2\n")
			writeFile(t, filepath.Join(env.Dir, "b.txt"), "b1\nb2\n")
		}},
	})

	t.Run("TAIL-002", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "follow.log")
		writeFile(t, file, "base\n")
		gobox := runTailGoboxFollow(t, env, []string{"-n", "0", "-f", "follow.log"}, func() {
			appendFile(t, file, "gobox-follow\n")
		}, 1600*time.Millisecond)
		native := runNativeFollow(t, env, "tail", []string{"-n", "0", "-f", "follow.log"}, func() {
			appendFile(t, file, "native-follow\n")
		}, 1600*time.Millisecond)
		if !strings.Contains(gobox.Stdout, "gobox-follow") || !strings.Contains(native.Stdout, "native-follow") {
			t.Fatalf("tail -f did not follow append\ngobox=%+v\nnative=%+v", gobox, native)
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
	})
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
		{ID: "GREP-008", Name: "grep -q", GoboxArgs: []string{"grep", "-q", "foo", "input.txt"}, NativeCommand: "grep", NativeArgs: []string{"-q", "foo", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") }, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("grep -q exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
			}
		}},
		{ID: "GREP-009", Name: "grep -r", GoboxArgs: []string{"grep", "-r", "foo", "tree"}, NativeCommand: "grep", NativeArgs: []string{"-r", "foo", "tree"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "foo\n")
			writeFile(t, filepath.Join(env.Dir, "tree", "sub", "b.txt"), "bar\nfoo\n")
		}, Normalize: collapseSpaces},
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
		}, Normalize: collapseSpaces},
		{ID: "GREP-015", Name: "grep --exclude-dir", GoboxArgs: []string{"grep", "-r", "--exclude-dir=skip", "foo", "tree"}, NativeCommand: "grep", NativeArgs: []string{"-r", "--exclude-dir=skip", "foo", "tree"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "tree", "keep", "a.txt"), "foo\n")
			writeFile(t, filepath.Join(env.Dir, "tree", "skip", "b.txt"), "foo\n")
		}, Normalize: collapseSpaces},
		{ID: "GREP-016", Name: "grep -l", GoboxArgs: []string{"grep", "-l", "foo", "a.txt", "b.txt"}, NativeCommand: "grep", NativeArgs: []string{"-l", "foo", "a.txt", "b.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "a.txt"), "foo\n")
			writeFile(t, filepath.Join(env.Dir, "b.txt"), "bar\n")
		}},
		{ID: "GREP-017", Name: "grep -L", GoboxArgs: []string{"grep", "-L", "foo", "a.txt", "b.txt"}, NativeCommand: "grep", NativeArgs: []string{"-L", "foo", "a.txt", "b.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "a.txt"), "foo\n")
			writeFile(t, filepath.Join(env.Dir, "b.txt"), "bar\n")
		}},
	})

	t.Run("GREP-005", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "input.txt"), "foo\nbar\n")
		res := runGoboxCLI(t, env, "", "grep", "--line-buffered", "foo", "input.txt")
		if res.ExitCode != 0 || normalizeText(res.Stdout) != "foo" {
			t.Fatalf("grep --line-buffered failed: %+v", res)
		}
	})
}

func TestParity_SedCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "SED-001", Name: "sed -n", GoboxArgs: []string{"sed", "-n", "p", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"-n", "p", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\n") }},
		{ID: "SED-002", Name: "sed -i", GoboxArgs: []string{"sed", "-i.bak", "s/foo/bar/", "input.txt"}, NativeCommand: "sed", NativeArgs: []string{"-i.bak", "s/foo/bar/", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") }, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("sed -i exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
			}
		}},
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
	})

	t.Run("SED-005", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "sed", "-h")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Usage") {
			t.Fatalf("sed -h failed: %+v", res)
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
		}},
		{ID: "SORT-010", Name: "sort -o", GoboxArgs: []string{"sort", "-o", "out.txt", "input.txt"}, NativeCommand: "sort", NativeArgs: []string{"-o", "native.txt", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "b\na\n") }, Assert: func(t *testing.T, gobox, native parityResult) {
			g, _ := os.ReadFile("out.txt")
			n, _ := os.ReadFile("native.txt")
			if normalizeText(string(g)) != normalizeText(string(n)) {
				t.Fatalf("sort -o file output mismatch\n%s\n%s", string(g), string(n))
			}
		}},
	})

	t.Run("SORT-008", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nb\nc\n")
		result := runGoboxCLI(t, env.Dir, "", "sort", "-R", "input.txt")
		if result.ExitCode != 0 {
			t.Fatalf("sort -R failed: %+v", result)
		}
		lines := strings.Split(normalizeText(result.Stdout), "\n")
		sortStrings := append([]string(nil), lines...)
		if len(sortStrings) != 3 {
			t.Fatalf("expected 3 lines from sort -R, got %v", lines)
		}
		for _, want := range []string{"a", "b", "c"} {
			found := false
			for _, got := range sortStrings {
				if got == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("sort -R output missing %s: %v", want, sortStrings)
			}
		}
	})

	t.Run("SORT-011", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "input.txt"), "b\x00a\x00")
		res := runGoboxCLI(t, env, "", "sort", "-z", "input.txt")
		if res.ExitCode != 0 || res.Stdout != "a\x00b\x00" {
			t.Fatalf("sort -z failed: %+v", res)
		}
	})
}

func TestParity_UniqCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "UNIQ-001", Name: "uniq -c", GoboxArgs: []string{"uniq", "-c", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-c", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n") }},
		{ID: "UNIQ-002", Name: "uniq -d", GoboxArgs: []string{"uniq", "-d", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-d", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n") }},
		{ID: "UNIQ-003", Name: "uniq -u", GoboxArgs: []string{"uniq", "-u", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-u", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n") }},
		{ID: "UNIQ-004", Name: "uniq -i", GoboxArgs: []string{"uniq", "-i", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-i", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "A\na\nb\n") }},
		{ID: "UNIQ-005", Name: "uniq -w", GoboxArgs: []string{"uniq", "-w", "2", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-w", "2", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "input.txt"), "ab1\nab2\nxy1\n")
		}},
		{ID: "UNIQ-006", Name: "uniq -f", GoboxArgs: []string{"uniq", "-f", "1", "input.txt"}, NativeCommand: "uniq", NativeArgs: []string{"-f", "1", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) {
			writeFile(t, filepath.Join(env.Dir, "input.txt"), "x a\ny a\nz b\n")
		}},
	})
}

func TestParity_WcCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{ID: "WC-001", Name: "wc -l", GoboxArgs: []string{"wc", "-l", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"-l", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nb\n") }, Normalize: collapseSpaces},
		{ID: "WC-002", Name: "wc -w", GoboxArgs: []string{"wc", "-w", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"-w", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a b\n") }, Normalize: collapseSpaces},
		{ID: "WC-003", Name: "wc -c", GoboxArgs: []string{"wc", "-c", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"-c", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "abc") }, Normalize: collapseSpaces},
		{ID: "WC-004", Name: "wc -m", GoboxArgs: []string{"wc", "-m", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"-m", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "你好a") }, Normalize: collapseSpaces},
		{ID: "WC-005", Name: "wc -L", GoboxArgs: []string{"wc", "-L", "input.txt"}, NativeCommand: "wc", NativeArgs: []string{"-L", "input.txt"}, Setup: func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nlonger\n") }, Normalize: collapseSpaces},
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
