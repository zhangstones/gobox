package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestParity_FindSubset(t *testing.T) {
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
}

func TestParity_DuCases(t *testing.T) {
	if _, err := exec.LookPath("du"); err != nil {
		t.Skip("native du not found")
	}
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
}

func TestParity_FsLightweightCases(t *testing.T) {
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
