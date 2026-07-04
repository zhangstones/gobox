package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func dfHeaderAndRows(out string) ([]string, [][]string) {
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return nil, nil
	}
	header := strings.Fields(lines[0])
	rows := make([][]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		rows = append(rows, fields)
	}
	return header, rows
}

func hasNonDigitSuffix(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			return true
		}
	}
	return false
}

func expectedDFRowWidth(header []string) int {
	if len(header) >= 2 && header[len(header)-2] == "Mounted" && header[len(header)-1] == "on" {
		return len(header) - 1
	}
	return len(header)
}

// duHasHumanReadableSizeCol returns true if at least one data line has a
// human-readable suffix (K, M, G, T or KB, MB, GB, TB) in the size column.
func duHasHumanReadableSizeCol(out string) bool {
	for _, line := range nonEmptyLines(out) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		s := fields[0]
		for _, suf := range []string{"TB", "GB", "MB", "KB", "T", "G", "M", "K"} {
			if strings.HasSuffix(s, suf) {
				return true
			}
		}
	}
	return false
}

// duSizeNumeric returns true if every data line's first field is purely numeric.
func duSizeNumeric(out string) bool {
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if _, err := strconv.ParseInt(fields[0], 10, 64); err != nil {
			return false
		}
	}
	return true
}

// duTotalNumeric extracts the numeric value from the "total" line of du -c output.
func duTotalNumeric(out string) (int64, bool) {
	for _, line := range nonEmptyLines(out) {
		if strings.HasSuffix(line, "\ttotal") {
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				v, err := strconv.ParseInt(fields[0], 10, 64)
				if err == nil {
					return v, true
				}
			}
		}
	}
	return 0, false
}

// detectLocalFSDevice returns the first non-network filesystem device found in
// df output (excluding tmpfs / proc / sysfs / devtmpfs / overlay), or "".
func detectLocalFSDevice(t *testing.T) string {
	t.Helper()
	res := runGoboxCLI(t, t.TempDir(), "", "df")
	if res.ExitCode != 0 {
		return ""
	}
	for _, line := range nonEmptyLines(res.Stdout)[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		dev := fields[0]
		switch {
		case strings.Contains(dev, ":"):
			continue // NFS / remote
		case dev == "tmpfs", dev == "proc", dev == "sysfs", dev == "devtmpfs",
			dev == "cgroup", dev == "cgroup2", dev == "overlay":
			continue
		default:
			return dev
		}
	}
	return ""
}

func TestParity_FindCases(t *testing.T) {
	normFactory := func(env *parityEnv) func(string) string {
		return normalizeFindOutput(env.Dir)
	}

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
			NormalizeFactory: normFactory,
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
			NormalizeFactory: normFactory,
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
			NormalizeFactory: normFactory,
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
			NormalizeFactory: normFactory,
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
			NormalizeFactory: normFactory,
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
			NormalizeFactory: normFactory,
		},
		{
			ID:            "FIND-008",
			Name:          "find -size +1K",
			GoboxArgs:     []string{"find", "-size", "+1K", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-size", "+1k"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "big.bin"), strings.Repeat("a", 2048))
				writeFile(t, filepath.Join(env.Dir, "tree", "small.bin"), "a")
			},
			NormalizeFactory: normFactory,
		},
		{
			// FIND-008b: smaller-than size filter; use -type f so both gobox and
			// native exclude directories (gobox explicitly skips dirs for -size).
			ID:            "FIND-008b",
			Name:          "find -size -2K (smaller than)",
			GoboxArgs:     []string{"find", "-type", "f", "-size", "-2K", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-type", "f", "-size", "-2k"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "small.txt"), strings.Repeat("x", 512))
				writeFile(t, filepath.Join(env.Dir, "tree", "big.bin"), strings.Repeat("y", 4096))
			},
			NormalizeFactory: normFactory,
		},
		{
			// FIND-008c: exact zero-size match.
			ID:            "FIND-008c",
			Name:          "find -size 0 (empty files)",
			GoboxArgs:     []string{"find", "-size", "0", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-size", "0"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "empty.txt"), "")
				writeFile(t, filepath.Join(env.Dir, "tree", "nonempty.txt"), "hi")
			},
			NormalizeFactory: normFactory,
		},
		{
			ID:            "FIND-009",
			Name:          "find -type d",
			GoboxArgs:     []string{"find", "-type", "d", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-type", "d"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "sub", "a.txt"), "x")
			},
			NormalizeFactory: normFactory,
		},
		{
			// FIND-009b: -type f parity test.
			ID:            "FIND-009b",
			Name:          "find -type f",
			GoboxArgs:     []string{"find", "-type", "f", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-type", "f"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "a")
				writeFile(t, filepath.Join(env.Dir, "tree", "sub", "b.txt"), "b")
			},
			NormalizeFactory: normFactory,
		},
		{
			// FIND-path: -path glob filter parity.
			ID:            "FIND-path",
			Name:          "find -path glob",
			GoboxArgs:     []string{"find", "-path", "*/sub/*", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-path", "*/sub/*"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "sub", "deep.txt"), "d")
				writeFile(t, filepath.Join(env.Dir, "tree", "top.txt"), "t")
			},
			NormalizeFactory: normFactory,
		},
		{
			// FIND-not: -not negate parity.
			ID:            "FIND-not",
			Name:          "find -not -name",
			GoboxArgs:     []string{"find", "-not", "-name", "*.txt", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-not", "-name", "*.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "a")
				writeFile(t, filepath.Join(env.Dir, "tree", "b.log"), "b")
			},
			NormalizeFactory: normFactory,
		},
		{
			// FIND-combine: combined -name and -type predicates.
			ID:            "FIND-combine",
			Name:          "find -name -type combined",
			GoboxArgs:     []string{"find", "-name", "*.txt", "-type", "f", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-name", "*.txt", "-type", "f"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "a")
				writeFile(t, filepath.Join(env.Dir, "tree", "b.log"), "b")
				writeFile(t, filepath.Join(env.Dir, "tree", "sub", "c.txt"), "c")
				// create a directory named "d.txt" to verify -type f excludes it
				if err := os.MkdirAll(filepath.Join(env.Dir, "tree", "d.txt"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			NormalizeFactory: normFactory,
		},
		{
			// FIND-multi-root: multiple starting paths.
			ID:            "FIND-multi-root",
			Name:          "find multiple roots",
			GoboxArgs:     []string{"find", "-name", "*.txt", "dir1", "dir2"},
			NativeCommand: "find",
			NativeArgs:    []string{"dir1", "dir2", "-name", "*.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "dir1", "a.txt"), "a")
				writeFile(t, filepath.Join(env.Dir, "dir2", "b.txt"), "b")
				writeFile(t, filepath.Join(env.Dir, "dir2", "c.log"), "c")
			},
			NormalizeFactory: normFactory,
		},
	})

	// FIND-007: verify that the default (no -print) behaviour matches native find.
	// Previously this was a gobox-vs-gobox test; now it compares against native.
	t.Run("FIND-007", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "tree", "a.txt"), "a")
		gobox := runGoboxCLI(t, env, "", "find", "tree")
		native := runNativeCLI(t, env, "", "find", "tree")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("FIND-007 find tree failed: gobox=%+v native=%+v", gobox, native)
		}
		norm := normalizeFindOutput(env)
		if norm(gobox.Stdout) != norm(native.Stdout) {
			t.Fatalf("FIND-007 default find mismatch\n--- gobox ---\n%s\n--- native ---\n%s",
				gobox.Stdout, native.Stdout)
		}
	})

	// FIND-error: non-existent root should exit non-zero.
	t.Run("FIND-error", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "find", "/nonexistent_gobox_find_path")
		native := runNativeCLI(t, env, "", "find", "/nonexistent_gobox_find_path")
		if gobox.ExitCode == 0 {
			t.Fatalf("FIND-error: gobox find should exit non-zero for missing path, got 0")
		}
		if native.ExitCode == 0 {
			t.Fatalf("FIND-error: native find should exit non-zero for missing path, got 0")
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
		// Verify gobox emits human-readable suffixes in the size column.
		if !duHasHumanReadableSizeCol(gobox.Stdout) {
			t.Fatalf("du -h: gobox size column missing human-readable suffix\n%s", gobox.Stdout)
		}
		if !duHasHumanReadableSizeCol(native.Stdout) {
			t.Fatalf("du -h: native size column missing human-readable suffix\n%s", native.Stdout)
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
		// Verify size columns are numeric in default (non-human) mode.
		if !duSizeNumeric(gobox.Stdout) {
			t.Fatalf("du -s gobox size column should be numeric\n%s", gobox.Stdout)
		}
		if !duSizeNumeric(native.Stdout) {
			t.Fatalf("du -s native size column should be numeric\n%s", native.Stdout)
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
		if !duSizeNumeric(gobox.Stdout) || !duSizeNumeric(native.Stdout) {
			t.Fatalf("du -a size column should be numeric\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
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
		// Compare numeric totals; allow up to 20% delta for filesystem block-size differences.
		gTotal, gOK := duTotalNumeric(gobox.Stdout)
		nTotal, nOK := duTotalNumeric(native.Stdout)
		if gOK && nOK && gTotal > 0 && nTotal > 0 {
			diff := gTotal - nTotal
			if diff < 0 {
				diff = -diff
			}
			max := gTotal
			if nTotal > max {
				max = nTotal
			}
			if diff*100/max > 20 {
				t.Fatalf("du -c totals diverge by >20%%: gobox=%d native=%d", gTotal, nTotal)
			}
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
		if !duSizeNumeric(gobox.Stdout) || !duSizeNumeric(native.Stdout) {
			t.Fatalf("du --apparent-size size column should be numeric\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})

	// DU-sh: du -s -h combination (note: gobox does not support combined -sh shorthand).
	t.Run("DU-sh", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		gobox := runGoboxCLI(t, env, "", "du", "-s", "-h", "tree")
		native := runNativeCLI(t, env, "", "du", "-s", "-h", "tree")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("du -s -h exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		// Both outputs should include the path and a human-readable size.
		if !duHasHumanReadableSizeCol(gobox.Stdout) {
			t.Fatalf("du -s -h gobox missing human-readable size\n%s", gobox.Stdout)
		}
		if !duHasHumanReadableSizeCol(native.Stdout) {
			t.Fatalf("du -s -h native missing human-readable size\n%s", native.Stdout)
		}
		if duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du -s -h path set mismatch\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})

	// DU-multi-exclude: two --exclude patterns.
	t.Run("DU-multi-exclude", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		writeFile(t, filepath.Join(env, "tree", "skip.tmp"), "skip")
		writeFile(t, filepath.Join(env, "tree", "skip.log"), "skip")
		gobox := runGoboxCLI(t, env, "", "du", "-a", "--exclude", "*.tmp", "--exclude", "*.log", "tree")
		native := runNativeCLI(t, env, "", "du", "-a", "--exclude", "*.tmp", "--exclude", "*.log", "tree")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("du multi-exclude exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if strings.Contains(gobox.Stdout, "skip.tmp") || strings.Contains(gobox.Stdout, "skip.log") {
			t.Fatalf("du multi-exclude gobox still includes excluded files\n%s", gobox.Stdout)
		}
		if strings.Contains(native.Stdout, "skip.tmp") || strings.Contains(native.Stdout, "skip.log") {
			t.Fatalf("du multi-exclude native still includes excluded files\n%s", native.Stdout)
		}
		if duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du multi-exclude path set mismatch\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})

	// DU-error: non-existent path should exit non-zero.
	t.Run("DU-error", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxMainCLI(t, env, "", "du", "/nonexistent_gobox_du_path")
		native := runNativeCLI(t, env, "", "du", "/nonexistent_gobox_du_path")
		if gobox.ExitCode == 0 {
			t.Fatalf("DU-error: gobox du should exit non-zero for missing path, got 0")
		}
		if native.ExitCode == 0 {
			t.Fatalf("DU-error: native du should exit non-zero for missing path, got 0")
		}
		if gobox.Stderr == "" {
			t.Fatalf("DU-error: gobox du should emit stderr on error, got empty")
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
			header := strings.Fields(lines[0])
			if len(header) < 2 || header[0] != "Filesystem" || !strings.Contains(lines[0], "Mounted") {
				t.Fatalf("DF-001 %s missing df header: %q", out.name, lines[0])
			}
		}
		// Both outputs should name the same filesystem entry.
		goboxFS := strings.Fields(nonEmptyLines(res.Stdout)[1])[0]
		nativeFS := strings.Fields(nonEmptyLines(native.Stdout)[1])[0]
		if goboxFS != nativeFS {
			t.Fatalf("DF-001 filesystem name mismatch gobox=%q native=%q", goboxFS, nativeFS)
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
		// Check every data row (not just lines[1]) in both outputs for human-readable suffixes.
		for _, out := range []struct {
			name string
			text string
		}{
			{"gobox", res.Stdout},
			{"native", native.Stdout},
		} {
			dataLines := nonEmptyLines(out.text)
			if len(dataLines) < 2 {
				t.Fatalf("DF-002 %s output too short\n%s", out.name, out.text)
			}
			for _, line := range dataLines[1:] {
				foundSuffix := false
				for _, field := range strings.Fields(line) {
					if strings.HasSuffix(field, "K") || strings.HasSuffix(field, "M") || strings.HasSuffix(field, "G") || strings.HasSuffix(field, "T") ||
						strings.HasSuffix(field, "KB") || strings.HasSuffix(field, "MB") || strings.HasSuffix(field, "GB") || strings.HasSuffix(field, "TB") {
						foundSuffix = true
						break
					}
				}
				if !foundSuffix {
					t.Fatalf("DF-002 %s data row missing human-readable size suffix: %q", out.name, line)
				}
			}
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
			header := strings.Fields(lines[0])
			if len(lines) < 2 || len(header) < 6 || header[0] != "Filesystem" || !strings.Contains(lines[0], "Inodes") || !strings.Contains(lines[0], "IUse%") {
				t.Fatalf("DF-004 %s missing inode columns: %q", out.name, out.text)
			}
			fields := strings.Fields(lines[1])
			if len(fields) < 6 {
				t.Fatalf("DF-004 %s inode row too short: %q", out.name, lines[1])
			}
			// Verify the Inodes field (col 1) is a non-zero numeric.
			if v, err := strconv.ParseInt(fields[1], 10, 64); err != nil || v == 0 {
				t.Fatalf("DF-004 %s inode total count should be non-zero numeric, got %q", out.name, fields[1])
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
		if header := strings.Fields(goboxLines[0]); len(header) == 0 || header[0] != "Filesystem" {
			t.Fatalf("gobox df output missing header\ngobox=%s", gobox.Stdout)
		}
		if header := strings.Fields(nativeLines[0]); len(header) == 0 || header[0] != "Filesystem" {
			t.Fatalf("df output missing header\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		// Verify both data rows have a non-empty filesystem name.
		goboxFSName := strings.Fields(goboxLines[1])
		nativeFSName := strings.Fields(nativeLines[1])
		if len(goboxFSName) == 0 {
			t.Fatalf("df PATH gobox missing filesystem row\ngobox=%s", gobox.Stdout)
		}
		if len(nativeFSName) == 0 {
			t.Fatalf("df PATH native missing filesystem row\nnative=%s", native.Stdout)
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
				_, baseRows := dfHeaderAndRows(base.Stdout)
				header, rows := dfHeaderAndRows(res.Stdout)
				if len(rows) != 1 || len(baseRows) != 1 || len(header) < 2 {
					t.Fatalf("DF-006 expected single-path df rows\n--- base ---\n%s\n--- human ---\n%s", base.Stdout, res.Stdout)
				}
				if !hasNonDigitSuffix(rows[0][1]) || rows[0][1] == baseRows[0][1] {
					t.Fatalf("DF-006 should switch size column to SI-style units: base=%q human=%q", baseRows[0][1], rows[0][1])
				}
			case "DF-007":
				base := runGoboxCLI(t, t.TempDir(), "", "df")
				if base.ExitCode != 0 {
					t.Fatalf("DF-007 baseline failed: %+v", base)
				}
				if dfMountList(base.Stdout) == dfMountList(res.Stdout) {
					t.Fatalf("DF-007 should expose additional all-filesystem rows beyond default df\n--- base ---\n%s\n--- -a ---\n%s", base.Stdout, res.Stdout)
				}
				baseSet := make(map[string]struct{})
				for _, line := range nonEmptyLines(base.Stdout)[1:] {
					fields := strings.Fields(line)
					if len(fields) > 0 {
						baseSet[fields[0]+"|"+fields[len(fields)-1]] = struct{}{}
					}
				}
				sawHidden := false
				for _, line := range nonEmptyLines(res.Stdout)[1:] {
					fields := strings.Fields(line)
					if len(fields) == 0 {
						continue
					}
					key := fields[0] + "|" + fields[len(fields)-1]
					if _, ok := baseSet[key]; !ok {
						sawHidden = true
					}
				}
				if !sawHidden {
					t.Fatalf("DF-007 should add at least one filesystem hidden in default mode\n--- base ---\n%s\n--- all ---\n%s", base.Stdout, res.Stdout)
				}
				// Cross-compare: at least one common filesystem name should appear in both gobox and native.
				goboxMounts := dfMountSet(res.Stdout)
				nativeMounts := dfMountSet(native.Stdout)
				commonCount := 0
				for m := range goboxMounts {
					if nativeMounts[m] {
						commonCount++
					}
				}
				if commonCount == 0 {
					t.Fatalf("DF-007 gobox and native share no common filesystem names\ngobox=%s\nnative=%s", res.Stdout, native.Stdout)
				}
			case "DF-008":
				// Detect a local device at runtime instead of hardcoding a device name.
				localDev := detectLocalFSDevice(t)
				if localDev == "" {
					t.Skip("DF-008: no local (non-network) filesystem detected")
				}
				for _, line := range nonEmptyLines(res.Stdout)[1:] {
					fields := strings.Fields(line)
					if len(fields) == 0 {
						continue
					}
					if strings.Contains(fields[0], ":") {
						t.Fatalf("DF-008 should keep only local filesystems, got %q", line)
					}
				}
				if findLineWithPrefix(res.Stdout, localDev+" ") == "" && findLineWithPrefix(res.Stdout, localDev+"\t") == "" {
					t.Fatalf("DF-008 should still include detected local filesystem %q\n%s", localDev, res.Stdout)
				}
				// Cross-compare: same filesystem names in gobox and native.
				goboxMounts := dfMountSet(res.Stdout)
				nativeMounts := dfMountSet(native.Stdout)
				commonCount := 0
				for m := range goboxMounts {
					if nativeMounts[m] {
						commonCount++
					}
				}
				if commonCount == 0 {
					t.Fatalf("DF-008 gobox and native share no common local filesystem mounts\ngobox=%s\nnative=%s", res.Stdout, native.Stdout)
				}
			case "DF-009":
				for _, line := range nonEmptyLines(res.Stdout)[1:] {
					fields := strings.Fields(line)
					if len(fields) < 2 {
						t.Fatalf("DF-009 row too short: %q", line)
					}
					if fields[0] != "tmpfs" {
						t.Fatalf("DF-009 should only include tmpfs rows, got %q", line)
					}
				}
				// Cross-compare filesystem names.
				goboxMounts := dfMountSet(res.Stdout)
				nativeMounts := dfMountSet(native.Stdout)
				commonCount := 0
				for m := range goboxMounts {
					if nativeMounts[m] {
						commonCount++
					}
				}
				if len(goboxMounts) > 0 && len(nativeMounts) > 0 && commonCount == 0 {
					t.Fatalf("DF-009 gobox and native tmpfs mounts differ\ngobox=%s\nnative=%s", res.Stdout, native.Stdout)
				}
			case "DF-010":
				for _, line := range nonEmptyLines(res.Stdout)[1:] {
					fields := strings.Fields(line)
					if len(fields) < 2 {
						t.Fatalf("DF-010 row too short: %q", line)
					}
					if fields[0] == "tmpfs" {
						t.Fatalf("DF-010 should exclude tmpfs rows, got %q", line)
					}
				}
			case "DF-011":
				header, rows := dfHeaderAndRows(res.Stdout)
				if len(rows) == 0 || !strings.HasPrefix(strings.TrimSpace(nonEmptyLines(res.Stdout)[len(nonEmptyLines(res.Stdout))-1]), "total") {
					t.Fatalf("DF-011 should end with a total row\n%s", res.Stdout)
				}
				total := rows[len(rows)-1]
				if len(total) != expectedDFRowWidth(header) {
					t.Fatalf("DF-011 total row width mismatch header=%v total=%v\n%s", header, total, res.Stdout)
				}
			case "DF-012":
				header, rows := dfHeaderAndRows(res.Stdout)
				if len(rows) == 0 {
					t.Fatalf("DF-012 output too short\n%s", res.Stdout)
				}
				if len(header) < 6 || header[0] != "Filesystem" || header[1] != "1024-blocks" {
					t.Fatalf("DF-012 should render POSIX header fields, got %q", nonEmptyLines(res.Stdout)[0])
				}
				for _, row := range rows {
					if len(row) != expectedDFRowWidth(header) {
						t.Fatalf("DF-012 should keep POSIX single-line row width=%d header=%d row=%v", len(row), len(header), row)
					}
				}
			}
			for _, want := range tc.contains {
				if !strings.Contains(res.Stdout, want) || !strings.Contains(native.Stdout, want) {
					t.Fatalf("%s missing %q\n--- gobox ---\n%s\n--- native ---\n%s", tc.id, want, res.Stdout, native.Stdout)
				}
			}
		})
	}

	// DF-hT: combined -h and -T flags.
	t.Run("DF-hT", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("df cases require linux mountinfo")
		}
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "df", "-h", "-T", ".")
		native := runNativeCLI(t, env, "", "df", "-h", "-T", ".")
		if res.ExitCode != native.ExitCode {
			t.Fatalf("DF-hT exit mismatch gobox=%d native=%d\n--- gobox ---\n%s\n--- native ---\n%s",
				res.ExitCode, native.ExitCode, res.Stdout, native.Stdout)
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
				t.Fatalf("DF-hT %s output too short\n%s", out.name, out.text)
			}
			if !strings.Contains(lines[0], "Type") {
				t.Fatalf("DF-hT %s missing Type column: %q", out.name, lines[0])
			}
			// At least one data row should have a human-readable size suffix.
			foundSuffix := false
			for _, line := range lines[1:] {
				for _, field := range strings.Fields(line) {
					if strings.HasSuffix(field, "K") || strings.HasSuffix(field, "M") || strings.HasSuffix(field, "G") || strings.HasSuffix(field, "T") ||
						strings.HasSuffix(field, "KB") || strings.HasSuffix(field, "MB") || strings.HasSuffix(field, "GB") || strings.HasSuffix(field, "TB") {
						foundSuffix = true
						break
					}
				}
			}
			if !foundSuffix {
				t.Fatalf("DF-hT %s missing human-readable size suffix\n%s", out.name, out.text)
			}
		}
	})

	// DF-error: non-existent path should exit non-zero.
	t.Run("DF-error", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("df cases require linux mountinfo")
		}
		env := t.TempDir()
		gobox := runGoboxMainCLI(t, env, "", "df", "/nonexistent_gobox_df_path")
		native := runNativeCLI(t, env, "", "df", "/nonexistent_gobox_df_path")
		if gobox.ExitCode == 0 {
			t.Fatalf("DF-error gobox should exit non-zero for missing path, got 0")
		}
		if native.ExitCode == 0 {
			t.Fatalf("DF-error native should exit non-zero for missing path, got 0")
		}
		if gobox.Stderr == "" {
			t.Fatalf("DF-error gobox should emit error message on stderr")
		}
	})
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
				// gobox should produce no stdout and a non-empty stderr.
				if gobox.Stdout != "" {
					t.Fatalf("readpath -e gobox stdout should be empty, got %q", gobox.Stdout)
				}
				if gobox.Stderr == "" {
					t.Fatalf("readpath -e gobox stderr should contain an error message, got empty")
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
			// READPATH-canonicalize: long-form --canonicalize alias for -f.
			ID:            "READPATH-canonicalize",
			Name:          "readpath --canonicalize",
			GoboxArgs:     []string{"readpath", "--canonicalize", "link"},
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
			// READPATH-no-newline: long-form --no-newline alias for -n.
			ID:            "READPATH-no-newline",
			Name:          "readpath --no-newline",
			GoboxArgs:     []string{"readpath", "--no-newline", "-l", "link"},
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
					t.Fatalf("readpath --no-newline mismatch gobox=%+v native=%+v", gobox, native)
				}
			},
		},
		{
			// READPATH-multi: multiple path arguments resolved in order.
			ID:            "READPATH-multi",
			Name:          "readpath multiple paths",
			GoboxArgs:     []string{"readpath", "a", "b", "c"},
			NativeCommand: "realpath",
			NativeArgs:    []string{"a", "b", "c"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a"), "x")
				writeFile(t, filepath.Join(env.Dir, "b"), "x")
				writeFile(t, filepath.Join(env.Dir, "c"), "x")
			},
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

	// READPATH-008: -z should emit NUL delimiters in the raw output.
	t.Run("READPATH-008", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "a"), "x")
		writeFile(t, filepath.Join(env, "b"), "x")
		gobox := runGoboxCLI(t, env, "", "readpath", "-z", "-m", "a", "b")
		native := runNativeCLI(t, env, "", "realpath", "-z", "-m", "a", "b")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("READPATH-008 exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		// Verify NUL delimiter is actually present in the raw output before any normalization.
		if !strings.Contains(gobox.Stdout, "\x00") {
			t.Fatalf("READPATH-008 gobox -z output missing NUL delimiter; got %q", gobox.Stdout)
		}
		if !strings.Contains(native.Stdout, "\x00") {
			t.Fatalf("READPATH-008 native -z output missing NUL delimiter; got %q", native.Stdout)
		}
		// After stripping NUL, the resolved paths should match.
		goboxPaths := strings.Split(strings.TrimRight(gobox.Stdout, "\x00"), "\x00")
		nativePaths := strings.Split(strings.TrimRight(native.Stdout, "\x00"), "\x00")
		sort.Strings(goboxPaths)
		sort.Strings(nativePaths)
		if fmt.Sprintf("%v", goboxPaths) != fmt.Sprintf("%v", nativePaths) {
			t.Fatalf("READPATH-008 paths mismatch\ngobox=%v\nnative=%v", goboxPaths, nativePaths)
		}
	})

	// READPATH-noexist-noe: readpath on a non-existent path without -e.
	// gobox returns exit 1 (file not found via lstat), while native readlink -f
	// returns exit 0 and synthesises the absolute path. This behavioural difference
	// is documented as a bug in /tmp/bugs_fs.md.
	t.Run("READPATH-noexist-noe", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "readpath", "/nonexistent_rp_noe_path")
		if gobox.ExitCode == 0 {
			// If gobox ever starts resolving missing paths (aligning with readlink -f),
			// the stdout should be the absolute path and stderr should be empty.
			if gobox.Stdout == "" {
				t.Fatalf("READPATH-noexist-noe: gobox exit 0 but stdout empty")
			}
		} else {
			// Current behaviour: exits non-zero and writes to stderr.
			if gobox.Stderr == "" {
				t.Fatalf("READPATH-noexist-noe: gobox non-zero exit but stderr empty")
			}
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
				if got, want := statFieldValue(gobox.Stdout, "File:"), statFieldValue(native.Stdout, "File:"); got != want || got != "data" {
					t.Fatalf("stat default file field mismatch gobox=%q native=%q", got, want)
				}
				if got, want := statFieldValue(gobox.Stdout, "Size:"), statFieldValue(native.Stdout, "Size:"); got != want || got != "5" {
					t.Fatalf("stat default size field mismatch gobox=%q native=%q", got, want)
				}
				// Mode field: gobox emits Mode: NNNN; native emits Access: (NNNN/...).
				goboxMode := statFieldValue(gobox.Stdout, "Mode:")
				nativeMode := statModeFromAccess(native.Stdout)
				if goboxMode == "" {
					t.Fatalf("stat default gobox missing Mode field\n%s", gobox.Stdout)
				}
				if nativeMode == "" {
					t.Fatalf("stat default native missing mode in Access field\n%s", native.Stdout)
				}
				if goboxMode != nativeMode {
					t.Fatalf("stat default mode mismatch gobox=%q native=%q", goboxMode, nativeMode)
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
				if got, want := statFieldValue(gobox.Stdout, "File:"), statFieldValue(native.Stdout, "File:"); got != want || got != "link" {
					t.Fatalf("stat -L file field mismatch gobox=%q native=%q", got, want)
				}
				if got, want := statFieldValue(gobox.Stdout, "Size:"), statFieldValue(native.Stdout, "Size:"); got != want || got != "5" {
					t.Fatalf("stat -L size field mismatch gobox=%q native=%q", got, want)
				}
				if !strings.Contains(statLineWithPrefix(gobox.Stdout, "Size:"), "regular file") || !strings.Contains(statLineWithPrefix(native.Stdout, "Size:"), "regular file") {
					t.Fatalf("stat -L missing regular file type\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
				}
				// Mode field parity.
				goboxMode := statFieldValue(gobox.Stdout, "Mode:")
				nativeMode := statModeFromAccess(native.Stdout)
				if goboxMode != "" && nativeMode != "" && goboxMode != nativeMode {
					t.Fatalf("stat -L mode mismatch gobox=%q native=%q", goboxMode, nativeMode)
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
				if got, want := statFieldValue(statLineContaining(gobox.Stdout, "Type:"), "Type:"), statFieldValue(statLineContaining(native.Stdout, "Type:"), "Type:"); got == "" || want == "" || got != want {
					t.Fatalf("stat -f type field mismatch gobox=%q native=%q\ngobox=%s\nnative=%s", got, want, gobox.Stdout, native.Stdout)
				}
				if statLineWithPrefix(gobox.Stdout, "Block size:") == "" || statLineWithPrefix(native.Stdout, "Block size:") == "" {
					t.Fatalf("stat -f missing block size line\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
				}
			},
		},
		{
			ID:            "STAT-004",
			Name:          "stat format %s",
			GoboxArgs:     []string{"stat", "-c", "%s", "data"},
			NativeCommand: "stat",
			NativeArgs:    []string{"-c", "%s", "data"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data"), "hello")
			},
		},
		{
			// STAT-004b: %n format (filename).
			ID:            "STAT-004b",
			Name:          "stat format %n",
			GoboxArgs:     []string{"stat", "-c", "%n", "data"},
			NativeCommand: "stat",
			NativeArgs:    []string{"-c", "%n", "data"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data"), "hello")
			},
		},
		{
			// STAT-004c: %F format (file type).
			ID:            "STAT-004c",
			Name:          "stat format %F",
			GoboxArgs:     []string{"stat", "-c", "%F", "data"},
			NativeCommand: "stat",
			NativeArgs:    []string{"-c", "%F", "data"},
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
				// Both outputs should have the filename as field 0 and file size as field 1.
				// (gobox terse format: "name size octal_perm timestamp"; native has more fields.)
				gFields := strings.Fields(normalizeText(gobox.Stdout))
				nFields := strings.Fields(normalizeText(native.Stdout))
				if len(gFields) < 2 {
					t.Fatalf("stat -t gobox output too short: %q", gobox.Stdout)
				}
				if len(nFields) < 2 {
					t.Fatalf("stat -t native output too short: %q", native.Stdout)
				}
				// Field 0: filename.
				if gFields[0] != nFields[0] {
					t.Fatalf("stat -t filename mismatch gobox=%q native=%q", gFields[0], nFields[0])
				}
				// Field 1: file size in bytes.
				if gFields[1] != nFields[1] {
					t.Fatalf("stat -t file size mismatch gobox=%q native=%q", gFields[1], nFields[1])
				}
			},
		},
		{
			// STAT-multi: multiple file arguments (all existing).
			ID:            "STAT-multi",
			Name:          "stat multiple files",
			GoboxArgs:     []string{"stat", "-c", "%n:%s", "a", "b"},
			NativeCommand: "stat",
			NativeArgs:    []string{"-c", "%n:%s", "a", "b"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a"), "hi")
				writeFile(t, filepath.Join(env.Dir, "b"), "world")
			},
		},
	})

	// STAT-error: missing path should exit non-zero.
	t.Run("STAT-error", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxMainCLI(t, env, "", "stat", "/nonexistent_gobox_stat_path")
		native := runNativeCLI(t, env, "", "stat", "/nonexistent_gobox_stat_path")
		if gobox.ExitCode == 0 {
			t.Fatalf("STAT-error gobox should exit non-zero for missing path, got 0")
		}
		if native.ExitCode == 0 {
			t.Fatalf("STAT-error native should exit non-zero for missing path, got 0")
		}
		if gobox.Stderr == "" {
			t.Fatalf("STAT-error gobox should emit error on stderr")
		}
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
		// Verify content: truncated to 2 bytes should contain "he".
		goboxContent, err := os.ReadFile(filepath.Join(env, "gobox"))
		if err != nil {
			t.Fatalf("read gobox file: %v", err)
		}
		nativeContent, err := os.ReadFile(filepath.Join(env, "native"))
		if err != nil {
			t.Fatalf("read native file: %v", err)
		}
		if string(goboxContent) != "he" {
			t.Fatalf("truncate gobox content wrong: got %q want %q", goboxContent, "he")
		}
		if string(goboxContent) != string(nativeContent) {
			t.Fatalf("truncate content mismatch gobox=%q native=%q", goboxContent, nativeContent)
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
		// Silently skip missing files: no stderr expected.
		if gobox.Stderr != "" {
			t.Fatalf("truncate -c on missing file should not emit stderr, got %q", gobox.Stderr)
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
		// Success case: no error output.
		if gobox.Stderr != "" {
			t.Fatalf("truncate -r stderr should be empty on success, got %q", gobox.Stderr)
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

	// TRUNCATE-shrink: -s -N shrinks the file by N bytes.
	t.Run("TRUNCATE-shrink", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "gobox"), "hello world") // 11 bytes
		writeFile(t, filepath.Join(env, "native"), "hello world")
		gobox := runGoboxCLI(t, env, "", "truncate", "-s", "-3", "gobox")
		native := runNativeCLI(t, env, "", "truncate", "-s", "-3", "native")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("truncate shrink exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		gi, err := os.Stat(filepath.Join(env, "gobox"))
		if err != nil {
			t.Fatal(err)
		}
		ni, err := os.Stat(filepath.Join(env, "native"))
		if err != nil {
			t.Fatal(err)
		}
		if gi.Size() != ni.Size() {
			t.Fatalf("truncate shrink size mismatch gobox=%d native=%d", gi.Size(), ni.Size())
		}
		// 11 - 3 = 8 bytes expected.
		if gi.Size() != 8 {
			t.Fatalf("truncate shrink expected 8 bytes, got %d", gi.Size())
		}
	})

	// TRUNCATE-multi: multiple file arguments set to the same size.
	t.Run("TRUNCATE-multi", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "f1"), "hello")
		writeFile(t, filepath.Join(env, "f2"), "world123")
		writeFile(t, filepath.Join(env, "f3"), "ab")
		res := runGoboxCLI(t, env, "", "truncate", "-s", "0", "f1", "f2", "f3")
		if res.ExitCode != 0 {
			t.Fatalf("truncate multi exit %d: %+v", res.ExitCode, res)
		}
		for _, name := range []string{"f1", "f2", "f3"} {
			fi, err := os.Stat(filepath.Join(env, name))
			if err != nil {
				t.Fatalf("stat %s: %v", name, err)
			}
			if fi.Size() != 0 {
				t.Fatalf("truncate multi: %s should be 0 bytes, got %d", name, fi.Size())
			}
		}
	})

	// TRUNCATE-error: invalid size argument should produce non-zero exit and stderr.
	t.Run("TRUNCATE-error", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "f"), "data")
		gobox := runGoboxMainCLI(t, env, "", "truncate", "-s", "invalid_size", "f")
		native := runNativeCLI(t, env, "", "truncate", "-s", "invalid_size", "f")
		if gobox.ExitCode == 0 {
			t.Fatalf("TRUNCATE-error gobox should exit non-zero for invalid size, got 0")
		}
		if native.ExitCode == 0 {
			t.Fatalf("TRUNCATE-error native should exit non-zero for invalid size, got 0")
		}
		if gobox.Stderr == "" {
			t.Fatalf("TRUNCATE-error gobox should emit error message on stderr")
		}
		if native.Stderr == "" {
			t.Fatalf("TRUNCATE-error native should emit error message on stderr")
		}
	})

	// TRUNCATE-extend-content: extended region should be zero-padded.
	t.Run("TRUNCATE-extend-content", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "f"), "a")
		if err := os.Truncate(filepath.Join(env, "f"), 4); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(env, "f"))
		if err != nil {
			t.Fatal(err)
		}
		if len(content) != 4 {
			t.Fatalf("extended file should be 4 bytes, got %d", len(content))
		}
		for i := 1; i < 4; i++ {
			if content[i] != 0 {
				t.Fatalf("extended byte %d should be zero, got %d", i, content[i])
			}
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

// dfMountSet returns the set of mount-point strings from df output (column = last field).
func dfMountSet(out string) map[string]bool {
	lines := nonEmptyLines(out)
	if len(lines) < 2 {
		return nil
	}
	s := make(map[string]bool)
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		s[fields[len(fields)-1]] = true
	}
	return s
}

func statLineWithPrefix(out, prefix string) string {
	for _, line := range nonEmptyLines(out) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			return trimmed
		}
	}
	return ""
}

func statLineContaining(out, needle string) string {
	for _, line := range nonEmptyLines(out) {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, needle) {
			return trimmed
		}
	}
	return ""
}

func statFieldValue(lineOrOut, label string) string {
	line := lineOrOut
	if !strings.Contains(line, label) || strings.Contains(line, "\n") {
		line = statLineWithPrefix(lineOrOut, label)
		if line == "" {
			line = statLineContaining(lineOrOut, label)
		}
	}
	if line == "" {
		return ""
	}
	rest := strings.TrimSpace(strings.SplitN(line, label, 2)[1])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], "\"")
}

// statModeFromAccess extracts the four-digit octal mode from a native stat
// "Access: (NNNN/-rw-...)" line, e.g. returns "0644".
func statModeFromAccess(out string) string {
	for _, line := range nonEmptyLines(out) {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Access: (") {
			continue
		}
		// Format: Access: (0644/-rw-r--r--)  Uid: ...
		rest := strings.TrimPrefix(trimmed, "Access: (")
		slash := strings.Index(rest, "/")
		if slash < 0 {
			continue
		}
		return rest[:slash]
	}
	return ""
}
