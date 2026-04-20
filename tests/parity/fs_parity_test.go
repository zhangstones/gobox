package main

import (
	"os"
	"os/exec"
	"path/filepath"
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
	})

	cases := []parityCase{
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
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("find -maxdepth exit mismatch")
				}
				if strings.Contains(gobox.Stdout, "b.txt") {
					t.Fatalf("find -maxdepth leaked deep file: %q", gobox.Stdout)
				}
			},
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
			Assert: func(t *testing.T, gobox, native parityResult) {
				if strings.Contains(gobox.Stdout, "tree\n") || strings.HasSuffix(strings.TrimSpace(gobox.Stdout), "tree") {
					t.Fatalf("find -mindepth included root: %q", gobox.Stdout)
				}
			},
		},
		{
			ID:            "FIND-005",
			Name:          "find -mtime",
			GoboxArgs:     []string{"find", "-mtime", "+1h", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-mtime", "+1h"},
			Setup: func(t *testing.T, env *parityEnv) {
				p := filepath.Join(env.Dir, "tree", "old.txt")
				writeFile(t, p, "x")
				old := time.Now().Add(-2 * time.Hour)
				if err := os.Chtimes(p, old, old); err != nil {
					t.Fatal(err)
				}
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if !strings.Contains(gobox.Stdout, "old.txt") {
					t.Fatalf("find -mtime missing old.txt")
				}
			},
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
			Assert: func(t *testing.T, gobox, native parityResult) {
				if !strings.Contains(gobox.Stdout, "a.log") || strings.Contains(gobox.Stdout, "b.txt") {
					t.Fatalf("find -name mismatch: %q", gobox.Stdout)
				}
			},
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
			Assert: func(t *testing.T, gobox, native parityResult) {
				if !strings.Contains(gobox.Stdout, "big.bin") || strings.Contains(gobox.Stdout, "small.bin") {
					t.Fatalf("find -size mismatch: %q", gobox.Stdout)
				}
			},
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
			Assert: func(t *testing.T, gobox, native parityResult) {
				if strings.Contains(gobox.Stdout, "a.txt") {
					t.Fatalf("find -type d included file: %q", gobox.Stdout)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			env := &parityEnv{Dir: t.TempDir()}
			if tc.Setup != nil {
				tc.Setup(t, env)
			}
			gobox := runGoboxCLI(t, env.Dir, "", tc.GoboxArgs...)
			if gobox.ExitCode != 0 {
				t.Fatalf("%s gobox failed: %+v", tc.ID, gobox)
			}
			if tc.Assert != nil {
				tc.Assert(t, gobox, parityResult{})
			}
		})
	}
}

func TestParity_DuCases(t *testing.T) {
	if _, err := exec.LookPath("du"); err != nil {
		t.Skip("native du not found")
	}
	env := &parityEnv{Dir: t.TempDir()}
	writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), strings.Repeat("a", 128))
	writeFile(t, filepath.Join(env.Dir, "tree", "sub", "b.txt"), strings.Repeat("b", 256))
	gobox := runGoboxCLI(t, env.Dir, "", "du", "-s", "tree")
	native := runNativeCLI(t, env.Dir, "", "du", "-s", "tree")
	if gobox.ExitCode != native.ExitCode {
		t.Fatalf("du -s exit mismatch")
	}
	if !strings.Contains(gobox.Stdout, "tree") {
		t.Fatalf("du output missing tree: %q", gobox.Stdout)
	}
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

	t.Run("DU-002", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "tree", "a.txt"), strings.Repeat("a", 128))
		writeFile(t, filepath.Join(env, "tree", "sub", "b.txt"), strings.Repeat("b", 256))
		res := runGoboxCLI(t, env, "", "du", "-s", "tree")
		if res.ExitCode != 0 {
			t.Fatalf("du -s failed: %+v", res)
		}
		lines := nonEmptyLines(res.Stdout)
		if len(lines) != 1 || !strings.Contains(lines[0], "tree") {
			t.Fatalf("du -s expected single summary line, got %q", res.Stdout)
		}
	})

	t.Run("DU-001", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "tree", "a.txt"), strings.Repeat("a", 128))
		res := runGoboxCLI(t, env, "", "du", "-h", "tree")
		if res.ExitCode != 0 || !strings.Contains(res.Stdout, "tree/a.txt") {
			t.Fatalf("du -h failed: %+v", res)
		}
	})
}
