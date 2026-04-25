package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestParity_FindCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{
			ID:            "FIND-001",
			Name:          "find -atime",
			GoboxArgs:     []string{"find", "-atime", "+1", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-atime", "+1"},
			Setup: func(t *testing.T, env *parityEnv) {
				oldFile := filepath.Join(env.Dir, "tree", "old.txt")
				recentFile := filepath.Join(env.Dir, "tree", "recent.txt")
				writeFile(t, oldFile, "old")
				writeFile(t, recentFile, "recent")
				old := time.Now().Add(-49 * time.Hour)
				if err := os.Chtimes(oldFile, old, old); err != nil {
					t.Fatal(err)
				}
			},
			Normalize: normalizeFindOutput(filepath.Join(t.TempDir(), "unused")),
		},
		{
			ID:            "FIND-002",
			Name:          "find -empty",
			GoboxArgs:     []string{"find", "-empty", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-empty"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "empty.txt"), "")
				writeFile(t, filepath.Join(env.Dir, "tree", "full.txt"), "x")
				if err := os.MkdirAll(filepath.Join(env.Dir, "tree", "emptydir"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			Normalize: normalizeFindOutput(filepath.Join(t.TempDir(), "unused")),
		},
		{
			ID:            "FIND-003",
			Name:          "find -maxdepth",
			GoboxArgs:     []string{"find", "-maxdepth", "1", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-maxdepth", "1"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "a")
				writeFile(t, filepath.Join(env.Dir, "tree", "sub", "b.txt"), "b")
			},
			Normalize: normalizeFindOutput(filepath.Join(t.TempDir(), "unused")),
		},
		{
			ID:            "FIND-004",
			Name:          "find -mindepth",
			GoboxArgs:     []string{"find", "-mindepth", "1", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-mindepth", "1"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "a")
			},
			Normalize: normalizeFindOutput(filepath.Join(t.TempDir(), "unused")),
		},
		{
			ID:            "FIND-005",
			Name:          "find -mtime +1 day",
			GoboxArgs:     []string{"find", "-mtime", "+1", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-mtime", "+1"},
			Setup: func(t *testing.T, env *parityEnv) {
				p := filepath.Join(env.Dir, "tree", "old.txt")
				writeFile(t, p, "x")
				old := time.Now().Add(-49 * time.Hour)
				if err := os.Chtimes(p, old, old); err != nil {
					t.Fatal(err)
				}
			},
			Normalize: normalizeFindOutput(filepath.Join(t.TempDir(), "unused")),
		},
		{
			ID:            "FIND-006",
			Name:          "find -name",
			GoboxArgs:     []string{"find", "-name", "*.log", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-name", "*.log"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "a.log"), "x")
				writeFile(t, filepath.Join(env.Dir, "tree", "b.txt"), "x")
			},
			Normalize: normalizeFindOutput(filepath.Join(t.TempDir(), "unused")),
		},
		{
			ID:            "FIND-008",
			Name:          "find -size",
			GoboxArgs:     []string{"find", "-size", "+1K", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-size", "+1k"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "big.bin"), strings.Repeat("a", 2048))
				writeFile(t, filepath.Join(env.Dir, "tree", "small.bin"), "a")
			},
			Normalize: normalizeFindOutput(filepath.Join(t.TempDir(), "unused")),
		},
		{
			ID:            "FIND-009",
			Name:          "find -type",
			GoboxArgs:     []string{"find", "-type", "d", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-type", "d"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "sub", "a.txt"), "x")
			},
			Normalize: normalizeFindOutput(filepath.Join(t.TempDir(), "unused")),
		},
	})

	t.Run("FIND-007", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "tree", "a.txt"), "a")
		defaultRes := runGoboxCLI(t, env, "", "find", "tree")
		explicitRes := runGoboxCLI(t, env, "", "find", "-print", "tree")
		if defaultRes.ExitCode != 0 || explicitRes.ExitCode != 0 {
			t.Fatalf("find default/-print failed: default=%+v explicit=%+v", defaultRes, explicitRes)
		}
		if normalizeText(defaultRes.Stdout) != normalizeText(explicitRes.Stdout) {
			t.Fatalf("find default vs -print mismatch\n%s\n%s", defaultRes.Stdout, explicitRes.Stdout)
		}
	})
}

func TestParity_DuCases(t *testing.T) {
	requireNativeCommand(t, "du")
	setupTree := func(dir string) {
		writeFile(t, filepath.Join(dir, "tree", "a.txt"), strings.Repeat("a", 128))
		writeFile(t, filepath.Join(dir, "tree", "sub", "b.txt"), strings.Repeat("b", 256))
	}

	t.Run("DU-001", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		gobox := runGoboxCLI(t, env, "", "du", "-h", "tree")
		native := runNativeCLI(t, env, "", "du", "-h", "tree")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("du -h exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du -h path set mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("DU-002", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		gobox := runGoboxCLI(t, env, "", "du", "-s", "tree")
		native := runNativeCLI(t, env, "", "du", "-s", "tree")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("du -s exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du -s path set mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("DU-003", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		gobox := runGoboxCLI(t, env, "", "du", "-a", "tree")
		native := runNativeCLI(t, env, "", "du", "-a", "tree")
		if gobox.ExitCode != native.ExitCode || duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du -a mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", gobox, native)
		}
	})

	t.Run("DU-004", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		gobox := runGoboxCLI(t, env, "", "du", "-c", "tree")
		native := runNativeCLI(t, env, "", "du", "-c", "tree")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("du -c mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", gobox, native)
		}
		goboxLines := nonEmptyLines(gobox.Stdout)
		nativeLines := nonEmptyLines(native.Stdout)
		if len(goboxLines) < 2 || len(nativeLines) < 2 {
			t.Fatalf("du -c expected entries plus total line\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
		if !strings.HasSuffix(goboxLines[len(goboxLines)-1], "\ttotal") || !strings.HasSuffix(nativeLines[len(nativeLines)-1], "\ttotal") {
			t.Fatalf("du -c should end with a total line\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
		if duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du -c path set mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("DU-005", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		gobox := runGoboxCLI(t, env, "", "du", "-d", "0", "tree")
		native := runNativeCLI(t, env, "", "du", "-d", "0", "tree")
		if gobox.ExitCode != native.ExitCode || duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du -d 0 mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", gobox, native)
		}
		longOpt := runGoboxCLI(t, env, "", "du", "--max-depth", "0", "tree")
		if duPathSet(gobox.Stdout) != duPathSet(longOpt.Stdout) {
			t.Fatalf("du -d and --max-depth differ\n-d=%q\n--max-depth=%q", gobox.Stdout, longOpt.Stdout)
		}
	})

	t.Run("DU-006", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		writeFile(t, filepath.Join(env, "tree", "skip.tmp"), "skip")
		gobox := runGoboxCLI(t, env, "", "du", "-a", "--exclude", "*.tmp", "tree")
		native := runNativeCLI(t, env, "", "du", "-a", "--exclude", "*.tmp", "tree")
		if gobox.ExitCode != native.ExitCode || strings.Contains(gobox.Stdout, "skip.tmp") || strings.Contains(native.Stdout, "skip.tmp") || duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du --exclude mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", gobox, native)
		}
	})

	t.Run("DU-007", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		gobox := runGoboxCLI(t, env, "", "du", "-x", "tree")
		native := runNativeCLI(t, env, "", "du", "-x", "tree")
		if gobox.ExitCode != native.ExitCode || duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du -x mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", gobox, native)
		}
	})

	t.Run("DU-008", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		gobox := runGoboxCLI(t, env, "", "du", "--apparent-size", "-s", "tree")
		native := runNativeCLI(t, env, "", "du", "--apparent-size", "-s", "tree")
		if gobox.ExitCode != native.ExitCode || duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du --apparent-size mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", gobox, native)
		}
	})
}

func TestParity_DfCases(t *testing.T) {
	t.Run("DF-001", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("df cases require linux mountinfo")
		}
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "df", ".")
		native := runNativeCLI(t, env, "", "df", ".")
		if res.ExitCode != native.ExitCode {
			t.Fatalf("DF-001 exit mismatch gobox=%d native=%d\n--- gobox ---\n%s\n--- native ---\n%s", res.ExitCode, native.ExitCode, res.Stdout, native.Stdout)
		}
		for _, out := range []struct {
			name string
			text string
		}{
			{"gobox", res.Stdout},
			{"native", native.Stdout},
		} {
			lines := nonEmptyLines(out.text)
			if len(lines) < 2 {
				t.Fatalf("DF-001 %s output too short: %q", out.name, out.text)
			}
			if !strings.Contains(lines[0], "Filesystem") || !strings.Contains(lines[0], "Mounted") {
				t.Fatalf("DF-001 %s missing df header: %q", out.name, lines[0])
			}
		}
	})

	t.Run("DF-002", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("df cases require linux mountinfo")
		}
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "df", "-h", ".")
		native := runNativeCLI(t, env, "", "df", "-h", ".")
		if res.ExitCode != native.ExitCode {
			t.Fatalf("DF-002 exit mismatch gobox=%d native=%d\n--- gobox ---\n%s\n--- native ---\n%s", res.ExitCode, native.ExitCode, res.Stdout, native.Stdout)
		}
		if !dfHasHumanReadableRow(res.Stdout) || !dfHasHumanReadableRow(native.Stdout) {
			t.Fatalf("DF-002 expected human-readable row\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
	})

	t.Run("DF-003", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("df cases require linux mountinfo")
		}
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "df", "-T", ".")
		native := runNativeCLI(t, env, "", "df", "-T", ".")
		if res.ExitCode != native.ExitCode {
			t.Fatalf("DF-003 exit mismatch gobox=%d native=%d\n--- gobox ---\n%s\n--- native ---\n%s", res.ExitCode, native.ExitCode, res.Stdout, native.Stdout)
		}
		for _, out := range []struct {
			name string
			text string
		}{
			{"gobox", res.Stdout},
			{"native", native.Stdout},
		} {
			lines := nonEmptyLines(out.text)
			if len(lines) < 2 || !strings.Contains(lines[0], "Type") {
				t.Fatalf("DF-003 %s missing Type column: %q", out.name, out.text)
			}
			fields := strings.Fields(lines[1])
			if len(fields) < 2 || fields[1] == "-" {
				t.Fatalf("DF-003 %s missing fs type row: %q", out.name, lines[1])
			}
		}
	})

	t.Run("DF-004", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("df cases require linux mountinfo")
		}
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "df", "-i", ".")
		native := runNativeCLI(t, env, "", "df", "-i", ".")
		if res.ExitCode != native.ExitCode {
			t.Fatalf("DF-004 exit mismatch gobox=%d native=%d\n--- gobox ---\n%s\n--- native ---\n%s", res.ExitCode, native.ExitCode, res.Stdout, native.Stdout)
		}
		for _, out := range []struct {
			name string
			text string
		}{
			{"gobox", res.Stdout},
			{"native", native.Stdout},
		} {
			lines := nonEmptyLines(out.text)
			if len(lines) < 2 || !strings.Contains(lines[0], "Inodes") || !strings.Contains(lines[0], "IUse%") {
				t.Fatalf("DF-004 %s missing inode columns: %q", out.name, out.text)
			}
			fields := strings.Fields(lines[1])
			if len(fields) < 6 {
				t.Fatalf("DF-004 %s inode row too short: %q", out.name, lines[1])
			}
		}
	})

	t.Run("DF-005", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("df cases require linux mountinfo")
		}
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "df", ".")
		native := runNativeCLI(t, env, "", "df", ".")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("df exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		goboxLines := nonEmptyLines(gobox.Stdout)
		nativeLines := nonEmptyLines(native.Stdout)
		if len(goboxLines) != 2 || len(nativeLines) != 2 {
			t.Fatalf("df PATH should render exactly header + one filesystem row\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		if !strings.Contains(goboxLines[0], "Filesystem") || !strings.Contains(nativeLines[0], "Filesystem") {
			t.Fatalf("df output missing header\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		if strings.Fields(goboxLines[1])[0] == "" || strings.Fields(nativeLines[1])[0] == "" {
			t.Fatalf("df PATH missing filesystem row\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})

	for _, tc := range []struct {
		id       string
		args     []string
		contains []string
	}{
		{"DF-006", []string{"df", "-H", "."}, []string{"Filesystem", "Size"}},
		{"DF-007", []string{"df", "-a"}, []string{"Filesystem"}},
		{"DF-008", []string{"df", "-l"}, []string{"Filesystem"}},
		{"DF-009", []string{"df", "-t", "tmpfs"}, []string{"Filesystem"}},
		{"DF-010", []string{"df", "-x", "tmpfs"}, []string{"Filesystem"}},
		{"DF-011", []string{"df", "--total"}, []string{"total"}},
		{"DF-012", []string{"df", "-P", "."}, []string{"Filesystem", "1024-blocks"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			if runtime.GOOS != "linux" {
				t.Skip("df cases require linux mountinfo")
			}
			res := runGoboxCLI(t, t.TempDir(), "", tc.args...)
			native := runNativeCLI(t, t.TempDir(), "", tc.args[0], tc.args[1:]...)
			if res.ExitCode != native.ExitCode {
				t.Fatalf("%s exit mismatch gobox=%d native=%d\n--- gobox ---\n%s\n--- native ---\n%s", tc.id, res.ExitCode, native.ExitCode, res.Stdout, native.Stdout)
			}
			switch tc.id {
			case "DF-006":
				base := runGoboxCLI(t, t.TempDir(), "", "df", ".")
				if base.ExitCode != 0 {
					t.Fatalf("DF-006 baseline failed: %+v", base)
				}
				if base.Stdout == res.Stdout {
					t.Fatalf("DF-006 should change output relative to default df\n--- base ---\n%s\n--- -H ---\n%s", base.Stdout, res.Stdout)
				}
			case "DF-007", "DF-008", "DF-009", "DF-010":
				if dfMountList(res.Stdout) != dfMountList(native.Stdout) {
					t.Fatalf("%s mount-set mismatch\n--- gobox ---\n%s\n--- native ---\n%s", tc.id, res.Stdout, native.Stdout)
				}
				if tc.id == "DF-007" {
					base := runGoboxCLI(t, t.TempDir(), "", "df")
					if base.ExitCode != 0 {
						t.Fatalf("DF-007 baseline failed: %+v", base)
					}
					if dfMountList(base.Stdout) == dfMountList(res.Stdout) {
						t.Fatalf("DF-007 should expose additional all-filesystem rows beyond default df\n--- base ---\n%s\n--- -a ---\n%s", base.Stdout, res.Stdout)
					}
				}
			case "DF-011":
				lines := nonEmptyLines(res.Stdout)
				if len(lines) < 2 || !strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "total") {
					t.Fatalf("DF-011 should end with a total row\n%s", res.Stdout)
				}
			case "DF-012":
				lines := nonEmptyLines(res.Stdout)
				if len(lines) < 2 {
					t.Fatalf("DF-012 output too short\n%s", res.Stdout)
				}
				header := strings.Fields(lines[0])
				if len(header) < 6 || header[0] != "Filesystem" || header[1] != "1024-blocks" {
					t.Fatalf("DF-012 should render POSIX header fields, got %q", lines[0])
				}
			}
			for _, want := range tc.contains {
				if !strings.Contains(res.Stdout, want) || !strings.Contains(native.Stdout, want) {
					t.Fatalf("%s missing %q\n--- gobox ---\n%s\n--- native ---\n%s", tc.id, want, res.Stdout, native.Stdout)
				}
			}
		})
	}
}

func TestParity_ReadpathCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{
			ID:            "READPATH-001",
			Name:          "readpath default",
			GoboxArgs:     []string{"readpath", "data"},
			NativeCommand: "realpath",
			NativeArgs:    []string{"data"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data"), "x")
			},
		},
		{
			ID:            "READPATH-002",
			Name:          "readpath -f",
			GoboxArgs:     []string{"readpath", "-f", "link"},
			NativeCommand: "readlink",
			NativeArgs:    []string{"-f", "link"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "target"), "x")
				if err := os.Symlink("target", filepath.Join(env.Dir, "link")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			ID:            "READPATH-003",
			Name:          "readpath -e",
			GoboxArgs:     []string{"readpath", "-e", "missing"},
			NativeCommand: "realpath",
			NativeArgs:    []string{"-e", "missing"},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode == 0 || native.ExitCode == 0 {
					t.Fatalf("readpath -e expected failure gobox=%+v native=%+v", gobox, native)
				}
			},
		},
		{
			ID:            "READPATH-004",
			Name:          "readpath -m",
			GoboxArgs:     []string{"readpath", "-m", "missing/path"},
			NativeCommand: "realpath",
			NativeArgs:    []string{"-m", "missing/path"},
		},
		{
			ID:            "READPATH-005",
			Name:          "readpath readlink",
			GoboxArgs:     []string{"readpath", "-l", "link"},
			NativeCommand: "readlink",
			NativeArgs:    []string{"link"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "target"), "x")
				if err := os.Symlink("target", filepath.Join(env.Dir, "link")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			ID:            "READPATH-006",
			Name:          "readpath -n",
			GoboxArgs:     []string{"readpath", "-n", "-l", "link"},
			NativeCommand: "readlink",
			NativeArgs:    []string{"-n", "link"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "target"), "x")
				if err := os.Symlink("target", filepath.Join(env.Dir, "link")); err != nil {
					t.Fatal(err)
				}
			},
			Normalize: func(s string) string { return s },
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode || gobox.Stdout != native.Stdout {
					t.Fatalf("readpath -n mismatch gobox=%+v native=%+v", gobox, native)
				}
			},
		},
		{
			ID:            "READPATH-008",
			Name:          "readpath -z",
			GoboxArgs:     []string{"readpath", "-z", "-m", "a", "b"},
			NativeCommand: "realpath",
			NativeArgs:    []string{"-z", "-m", "a", "b"},
		},
	})

	t.Run("READPATH-007", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxMainCLI(t, env, "", "readpath", "-q", "-e", "missing")
		native := runNativeCLI(t, env, "", "realpath", "-q", "-e", "missing")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("readpath -q exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if gobox.Stdout != "" || gobox.Stderr != "" {
			t.Fatalf("readpath -q should stay silent on missing path, got stdout=%q stderr=%q", gobox.Stdout, gobox.Stderr)
		}
		if native.Stdout != "" {
			t.Fatalf("native realpath -q unexpectedly wrote stdout: %q", native.Stdout)
		}
	})
}

func TestParity_StatCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
		{
			ID:            "STAT-001",
			Name:          "stat default",
			GoboxArgs:     []string{"stat", "data"},
			NativeCommand: "stat",
			NativeArgs:    []string{"data"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data"), "hello")
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("stat default exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
				}
				for _, want := range []string{"File:", "Size:"} {
					if !strings.Contains(gobox.Stdout, want) || !strings.Contains(native.Stdout, want) {
						t.Fatalf("stat default missing %q\ngobox=%s\nnative=%s", want, gobox.Stdout, native.Stdout)
					}
				}
			},
		},
		{
			ID:            "STAT-002",
			Name:          "stat -L",
			GoboxArgs:     []string{"stat", "-L", "link"},
			NativeCommand: "stat",
			NativeArgs:    []string{"-L", "link"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "target"), "hello")
				if err := os.Symlink("target", filepath.Join(env.Dir, "link")); err != nil {
					t.Fatal(err)
				}
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("stat -L exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
				}
				if !strings.Contains(gobox.Stdout, "File: link") || !strings.Contains(native.Stdout, "File: link") {
					t.Fatalf("stat -L missing file header\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
				}
				if !strings.Contains(gobox.Stdout, "Size: 5") || !strings.Contains(native.Stdout, "Size: 5") {
					t.Fatalf("stat -L missing size\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
				}
				if !strings.Contains(gobox.Stdout, "regular file") || !strings.Contains(native.Stdout, "regular file") {
					t.Fatalf("stat -L missing regular file type\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
				}
			},
		},
		{
			ID:            "STAT-003",
			Name:          "stat -f",
			GoboxArgs:     []string{"stat", "-f", "."},
			NativeCommand: "stat",
			NativeArgs:    []string{"-f", "."},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("stat -f exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
				}
				if !strings.Contains(gobox.Stdout, "Block") || !strings.Contains(native.Stdout, "Block") {
					t.Fatalf("stat -f missing filesystem fields\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
				}
			},
		},
		{
			ID:            "STAT-004",
			Name:          "stat format",
			GoboxArgs:     []string{"stat", "-c", "%s", "data"},
			NativeCommand: "stat",
			NativeArgs:    []string{"-c", "%s", "data"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data"), "hello")
			},
		},
		{
			ID:            "STAT-005",
			Name:          "stat -t",
			GoboxArgs:     []string{"stat", "-t", "data"},
			NativeCommand: "stat",
			NativeArgs:    []string{"-t", "data"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data"), "hello")
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("stat -t exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
				}
				if len(strings.Fields(normalizeText(gobox.Stdout))) < 4 || len(strings.Fields(normalizeText(native.Stdout))) < 4 {
					t.Fatalf("stat -t terse output too short\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
				}
			},
		},
	})
}

func TestParity_TruncateCases(t *testing.T) {
	t.Run("TRUNCATE-001", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "gobox"), "hello")
		writeFile(t, filepath.Join(env, "native"), "hello")
		gobox := runGoboxCLI(t, env, "", "truncate", "-s", "2", "gobox")
		native := runNativeCLI(t, env, "", "truncate", "-s", "2", "native")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("truncate exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		gi, _ := os.Stat(filepath.Join(env, "gobox"))
		ni, _ := os.Stat(filepath.Join(env, "native"))
		if gi.Size() != ni.Size() {
			t.Fatalf("truncate size mismatch gobox=%d native=%d", gi.Size(), ni.Size())
		}
	})

	t.Run("TRUNCATE-002", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "truncate", "-c", "-s", "2", "missing")
		native := runNativeCLI(t, env, "", "truncate", "-c", "-s", "2", "missing")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("truncate -c exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if _, err := os.Stat(filepath.Join(env, "missing")); !os.IsNotExist(err) {
			t.Fatalf("truncate -c unexpectedly created file")
		}
	})

	t.Run("TRUNCATE-003", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "ref"), "12345")
		writeFile(t, filepath.Join(env, "gobox"), "a")
		writeFile(t, filepath.Join(env, "native"), "a")
		gobox := runGoboxCLI(t, env, "", "truncate", "-r", "ref", "gobox")
		native := runNativeCLI(t, env, "", "truncate", "-r", "ref", "native")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("truncate -r exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		gi, _ := os.Stat(filepath.Join(env, "gobox"))
		ni, _ := os.Stat(filepath.Join(env, "native"))
		if gi.Size() != ni.Size() {
			t.Fatalf("truncate -r size mismatch gobox=%d native=%d", gi.Size(), ni.Size())
		}
	})

	t.Run("TRUNCATE-004", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "truncate", "-s", "1K", "gobox")
		native := runNativeCLI(t, env, "", "truncate", "-s", "1K", "native")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("truncate suffix exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		gi, _ := os.Stat(filepath.Join(env, "gobox"))
		ni, _ := os.Stat(filepath.Join(env, "native"))
		if gi.Size() != ni.Size() {
			t.Fatalf("truncate suffix size mismatch gobox=%d native=%d", gi.Size(), ni.Size())
		}
	})

	t.Run("TRUNCATE-005", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "gobox"), "1234")
		writeFile(t, filepath.Join(env, "native"), "1234")
		gobox := runGoboxCLI(t, env, "", "truncate", "-s", "+2", "gobox")
		native := runNativeCLI(t, env, "", "truncate", "-s", "+2", "native")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("truncate relative exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		gi, _ := os.Stat(filepath.Join(env, "gobox"))
		ni, _ := os.Stat(filepath.Join(env, "native"))
		if gi.Size() != ni.Size() {
			t.Fatalf("truncate relative size mismatch gobox=%d native=%d", gi.Size(), ni.Size())
		}
	})
}

func duPathSet(out string) string {
	lines := nonEmptyLines(out)
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		paths = append(paths, fields[len(fields)-1])
	}
	sort.Strings(paths)
	return strings.Join(paths, "\n")
}

func dfHasHumanReadableRow(out string) bool {
	lines := nonEmptyLines(out)
	if len(lines) < 2 {
		return false
	}
	for _, field := range strings.Fields(lines[1]) {
		if strings.HasSuffix(field, "K") || strings.HasSuffix(field, "M") || strings.HasSuffix(field, "G") || strings.HasSuffix(field, "T") ||
			strings.HasSuffix(field, "KB") || strings.HasSuffix(field, "MB") || strings.HasSuffix(field, "GB") || strings.HasSuffix(field, "TB") {
			return true
		}
	}
	return false
}

func dfMountList(out string) string {
	lines := nonEmptyLines(out)
	if len(lines) < 2 {
		return ""
	}
	mounts := make([]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		mounts = append(mounts, fields[len(fields)-1])
	}
	sort.Strings(mounts)
	return strings.Join(mounts, "\n")
}
