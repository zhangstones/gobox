package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParity_TextCommands(t *testing.T) {
	cases := []parityCase{
		{
			ID:            "HEAD-001",
			Name:          "head -n",
			GoboxArgs:     []string{"head", "-n", "2", "input.txt"},
			NativeCommand: "head",
			NativeArgs:    []string{"-n", "2", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "1\n2\n3\n")
			},
		},
		{
			ID:            "HEAD-002",
			Name:          "head -c",
			GoboxArgs:     []string{"head", "-c", "5", "input.txt"},
			NativeCommand: "head",
			NativeArgs:    []string{"-c", "5", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "abcdef\n")
			},
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
			ID:            "TAIL-001",
			Name:          "tail -n",
			GoboxArgs:     []string{"tail", "-n", "2", "input.txt"},
			NativeCommand: "tail",
			NativeArgs:    []string{"-n", "2", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "1\n2\n3\n")
			},
		},
		{
			ID:            "TAIL-005",
			Name:          "tail -q",
			GoboxArgs:     []string{"tail", "-q", "-n", "1", "a.txt", "b.txt"},
			NativeCommand: "tail",
			NativeArgs:    []string{"-q", "-n", "1", "a.txt", "b.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a.txt"), "a1\na2\n")
				writeFile(t, filepath.Join(env.Dir, "b.txt"), "b1\nb2\n")
			},
		},
		{
			ID:            "GREP-001",
			Name:          "grep -E",
			GoboxArgs:     []string{"grep", "-E", "foo|bar", "input.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-E", "foo|bar", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\nbaz\nbar\n")
			},
		},
		{
			ID:            "GREP-002",
			Name:          "grep -F",
			GoboxArgs:     []string{"grep", "-F", "a.b", "input.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-F", "a.b", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "a.b\naxb\n")
			},
		},
		{
			ID:            "GREP-003",
			Name:          "grep -c",
			GoboxArgs:     []string{"grep", "-c", "foo", "input.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-c", "foo", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\nfoo\nbar\n")
			},
		},
		{
			ID:            "GREP-004",
			Name:          "grep -i",
			GoboxArgs:     []string{"grep", "-i", "foo", "input.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-i", "foo", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "Foo\nbar\n")
			},
		},
		{
			ID:            "GREP-006",
			Name:          "grep -n",
			GoboxArgs:     []string{"grep", "-n", "foo", "input.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-n", "foo", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "bar\nfoo\n")
			},
		},
		{
			ID:            "GREP-007",
			Name:          "grep -o",
			GoboxArgs:     []string{"grep", "-o", "foo", "input.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-o", "foo", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo foo\n")
			},
		},
		{
			ID:            "GREP-008",
			Name:          "grep -q",
			GoboxArgs:     []string{"grep", "-q", "foo", "input.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-q", "foo", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n")
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("grep -q exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
				}
			},
		},
		{
			ID:            "GREP-009",
			Name:          "grep -r",
			GoboxArgs:     []string{"grep", "-r", "foo", "tree"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-r", "foo", "tree"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "foo\n")
				writeFile(t, filepath.Join(env.Dir, "tree", "sub", "b.txt"), "bar\nfoo\n")
			},
			Normalize: collapseSpaces,
		},
		{
			ID:            "GREP-010",
			Name:          "grep -v",
			GoboxArgs:     []string{"grep", "-v", "foo", "input.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-v", "foo", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\nbar\n")
			},
		},
		{
			ID:            "GREP-011",
			Name:          "grep -A",
			GoboxArgs:     []string{"grep", "-A", "1", "foo", "input.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-A", "1", "foo", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "x\nfoo\na\nb\n")
			},
		},
		{
			ID:            "GREP-012",
			Name:          "grep -B",
			GoboxArgs:     []string{"grep", "-B", "1", "foo", "input.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-B", "1", "foo", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "x\ny\nfoo\nz\n")
			},
		},
		{
			ID:            "GREP-013",
			Name:          "grep -C",
			GoboxArgs:     []string{"grep", "-C", "1", "foo", "input.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-C", "1", "foo", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "x\nfoo\nz\n") },
		},
		{
			ID:            "GREP-014",
			Name:          "grep --include",
			GoboxArgs:     []string{"grep", "-r", "--include=*.log", "foo", "tree"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-r", "--include=*.log", "foo", "tree"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "a.log"), "foo\n")
				writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "foo\n")
			},
			Normalize: collapseSpaces,
		},
		{
			ID:            "GREP-015",
			Name:          "grep --exclude-dir",
			GoboxArgs:     []string{"grep", "-r", "--exclude-dir=skip", "foo", "tree"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-r", "--exclude-dir=skip", "foo", "tree"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "keep", "a.txt"), "foo\n")
				writeFile(t, filepath.Join(env.Dir, "tree", "skip", "b.txt"), "foo\n")
			},
			Normalize: collapseSpaces,
		},
		{
			ID:            "GREP-016",
			Name:          "grep -l",
			GoboxArgs:     []string{"grep", "-l", "foo", "a.txt", "b.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-l", "foo", "a.txt", "b.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a.txt"), "foo\n")
				writeFile(t, filepath.Join(env.Dir, "b.txt"), "bar\n")
			},
		},
		{
			ID:            "GREP-017",
			Name:          "grep -L",
			GoboxArgs:     []string{"grep", "-L", "foo", "a.txt", "b.txt"},
			NativeCommand: "grep",
			NativeArgs:    []string{"-L", "foo", "a.txt", "b.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a.txt"), "foo\n")
				writeFile(t, filepath.Join(env.Dir, "b.txt"), "bar\n")
			},
		},
		{
			ID:            "SED-001",
			Name:          "sed -n",
			GoboxArgs:     []string{"sed", "-n", "p", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"-n", "p", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\n") },
		},
		{
			ID:            "SED-003",
			Name:          "sed -e",
			GoboxArgs:     []string{"sed", "-e", "s/foo/bar/", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"-e", "s/foo/bar/", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") },
		},
		{
			ID:            "SED-004",
			Name:          "sed -f",
			GoboxArgs:     []string{"sed", "-f", "script.sed", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"-f", "script.sed", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "script.sed"), "s/foo/bar/\n")
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n")
			},
		},
		{
			ID:            "SED-006",
			Name:          "sed substitute",
			GoboxArgs:     []string{"sed", "s/foo/bar/", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"s/foo/bar/", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") },
		},
		{
			ID:            "SED-007",
			Name:          "sed d",
			GoboxArgs:     []string{"sed", "d", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"d", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") },
		},
		{
			ID:            "SED-008",
			Name:          "sed p",
			GoboxArgs:     []string{"sed", "p", "input.txt"},
			NativeCommand: "sed",
			NativeArgs:    []string{"p", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "foo\n") },
		},
		{
			ID:            "SORT-001",
			Name:          "sort -n",
			GoboxArgs:     []string{"sort", "-n", "input.txt"},
			NativeCommand: "sort",
			NativeArgs:    []string{"-n", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "10\n2\n1\n") },
		},
		{
			ID:            "SORT-002",
			Name:          "sort -r",
			GoboxArgs:     []string{"sort", "-r", "input.txt"},
			NativeCommand: "sort",
			NativeArgs:    []string{"-r", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nc\nb\n") },
		},
		{
			ID:            "SORT-003",
			Name:          "sort -k",
			GoboxArgs:     []string{"sort", "-k", "2", "input.txt"},
			NativeCommand: "sort",
			NativeArgs:    []string{"-k", "2", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a 2\nb 1\n") },
		},
		{
			ID:            "SORT-004",
			Name:          "sort -t",
			GoboxArgs:     []string{"sort", "-t", ",", "-k", "2", "input.txt"},
			NativeCommand: "sort",
			NativeArgs:    []string{"-t", ",", "-k", "2", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a,2\nb,1\n") },
		},
		{
			ID:            "SORT-005",
			Name:          "sort -u",
			GoboxArgs:     []string{"sort", "-u", "input.txt"},
			NativeCommand: "sort",
			NativeArgs:    []string{"-u", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n") },
		},
		{
			ID:            "SORT-006",
			Name:          "sort -M",
			GoboxArgs:     []string{"sort", "-M", "input.txt"},
			NativeCommand: "sort",
			NativeArgs:    []string{"-M", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "Feb\nJan\n") },
		},
		{
			ID:            "SORT-007",
			Name:          "sort -h",
			GoboxArgs:     []string{"sort", "-h", "input.txt"},
			NativeCommand: "sort",
			NativeArgs:    []string{"-h", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "2K\n512\n1M\n") },
		},
		{
			ID:            "SORT-009",
			Name:          "sort -c",
			GoboxArgs:     []string{"sort", "-c", "input.txt"},
			NativeCommand: "sort",
			NativeArgs:    []string{"-c", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "b\na\n") },
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("sort -c exit mismatch %d != %d", gobox.ExitCode, native.ExitCode)
				}
			},
		},
		{
			ID:            "SORT-010",
			Name:          "sort -o",
			GoboxArgs:     []string{"sort", "-o", "out.txt", "input.txt"},
			NativeCommand: "sort",
			NativeArgs:    []string{"-o", "native.txt", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "b\na\n") },
			Assert: func(t *testing.T, gobox, native parityResult) {
				g, _ := os.ReadFile("out.txt")
				n, _ := os.ReadFile("native.txt")
				if normalizeText(string(g)) != normalizeText(string(n)) {
					t.Fatalf("sort -o file output mismatch\n%s\n%s", string(g), string(n))
				}
			},
		},
		{
			ID:            "UNIQ-001",
			Name:          "uniq -c",
			GoboxArgs:     []string{"uniq", "-c", "input.txt"},
			NativeCommand: "uniq",
			NativeArgs:    []string{"-c", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n") },
		},
		{
			ID:            "UNIQ-002",
			Name:          "uniq -d",
			GoboxArgs:     []string{"uniq", "-d", "input.txt"},
			NativeCommand: "uniq",
			NativeArgs:    []string{"-d", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n") },
		},
		{
			ID:            "UNIQ-003",
			Name:          "uniq -u",
			GoboxArgs:     []string{"uniq", "-u", "input.txt"},
			NativeCommand: "uniq",
			NativeArgs:    []string{"-u", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\na\nb\n") },
		},
		{
			ID:            "UNIQ-004",
			Name:          "uniq -i",
			GoboxArgs:     []string{"uniq", "-i", "input.txt"},
			NativeCommand: "uniq",
			NativeArgs:    []string{"-i", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "A\na\nb\n") },
		},
		{
			ID:            "UNIQ-005",
			Name:          "uniq -w",
			GoboxArgs:     []string{"uniq", "-w", "2", "input.txt"},
			NativeCommand: "uniq",
			NativeArgs:    []string{"-w", "2", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "ab1\nab2\nxy1\n")
			},
		},
		{
			ID:            "UNIQ-006",
			Name:          "uniq -f",
			GoboxArgs:     []string{"uniq", "-f", "1", "input.txt"},
			NativeCommand: "uniq",
			NativeArgs:    []string{"-f", "1", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "x a\ny a\nz b\n")
			},
		},
		{
			ID:            "WC-001",
			Name:          "wc -l",
			GoboxArgs:     []string{"wc", "-l", "input.txt"},
			NativeCommand: "wc",
			NativeArgs:    []string{"-l", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nb\n") },
			Normalize:     collapseSpaces,
		},
		{
			ID:            "WC-002",
			Name:          "wc -w",
			GoboxArgs:     []string{"wc", "-w", "input.txt"},
			NativeCommand: "wc",
			NativeArgs:    []string{"-w", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a b\n") },
			Normalize:     collapseSpaces,
		},
		{
			ID:            "WC-003",
			Name:          "wc -c",
			GoboxArgs:     []string{"wc", "-c", "input.txt"},
			NativeCommand: "wc",
			NativeArgs:    []string{"-c", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "abc") },
			Normalize:     collapseSpaces,
		},
		{
			ID:            "WC-004",
			Name:          "wc -m",
			GoboxArgs:     []string{"wc", "-m", "input.txt"},
			NativeCommand: "wc",
			NativeArgs:    []string{"-m", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "你好a") },
			Normalize:     collapseSpaces,
		},
		{
			ID:            "WC-005",
			Name:          "wc -L",
			GoboxArgs:     []string{"wc", "-L", "input.txt"},
			NativeCommand: "wc",
			NativeArgs:    []string{"-L", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "a\nlonger\n") },
			Normalize:     collapseSpaces,
		},
		{
			ID:            "XARGS-001",
			Name:          "xargs -I",
			GoboxArgs:     []string{"xargs", "-I", "{}", "echo", "pre:{}:post"},
			NativeCommand: "xargs",
			NativeArgs:    []string{"-I", "{}", "echo", "pre:{}:post"},
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
			},
		},
		{
			ID:            "MD5-002",
			Name:          "md5sum --tag",
			GoboxArgs:     []string{"md5sum", "--tag", "input.txt"},
			NativeCommand: "md5sum",
			NativeArgs:    []string{"--tag", "input.txt"},
			Setup:         func(t *testing.T, env *parityEnv) { writeFile(t, filepath.Join(env.Dir, "input.txt"), "hello") },
		},
		{
			ID:            "MD5-001",
			Name:          "md5sum --check",
			GoboxArgs:     []string{"md5sum", "--check", "checksums.md5"},
			NativeCommand: "md5sum",
			NativeArgs:    []string{"--check", "checksums.md5"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "hello")
				res := runNativeCLI(t, env.Dir, "", "md5sum", "input.txt")
				writeFile(t, filepath.Join(env.Dir, "checksums.md5"), normalizeText(res.Stdout)+"\n")
			},
			Normalize: normalizeText,
		},
		{
			ID:            "MD5-003",
			Name:          "md5sum --quiet",
			GoboxArgs:     []string{"md5sum", "--quiet", "--check", "checksums.md5"},
			NativeCommand: "md5sum",
			NativeArgs:    []string{"--quiet", "--check", "checksums.md5"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "hello")
				res := runNativeCLI(t, env.Dir, "", "md5sum", "input.txt")
				writeFile(t, filepath.Join(env.Dir, "checksums.md5"), normalizeText(res.Stdout)+"\n")
			},
			Normalize: normalizeText,
		},
		{
			ID:            "MD5-004",
			Name:          "md5sum --status",
			GoboxArgs:     []string{"md5sum", "--status", "--check", "checksums.md5"},
			NativeCommand: "md5sum",
			NativeArgs:    []string{"--status", "--check", "checksums.md5"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "hello")
				res := runNativeCLI(t, env.Dir, "", "md5sum", "input.txt")
				writeFile(t, filepath.Join(env.Dir, "checksums.md5"), normalizeText(res.Stdout)+"\n")
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("md5sum --status exit mismatch %d != %d", gobox.ExitCode, native.ExitCode)
				}
			},
		},
		{
			ID:            "SEQ-001",
			Name:          "seq basic",
			GoboxArgs:     []string{"seq", "5"},
			NativeCommand: "seq",
			NativeArgs:    []string{"5"},
		},
		{
			ID:            "SEQ-002",
			Name:          "seq format",
			GoboxArgs:     []string{"seq", "-f", "%02g", "5"},
			NativeCommand: "seq",
			NativeArgs:    []string{"-f", "%02g", "5"},
		},
		{
			ID:            "SEQ-003",
			Name:          "seq separator",
			GoboxArgs:     []string{"seq", "-s", ",", "3"},
			NativeCommand: "seq",
			NativeArgs:    []string{"-s", ",", "3"},
		},
	}
	runExactParityCases(t, cases)
}

func TestParity_SortRandomContract(t *testing.T) {
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
}
