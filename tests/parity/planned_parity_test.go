package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gobox/cmds/proc"
)

func TestParity_NewExactTextAndChecksumCases(t *testing.T) {
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
			ID:            "CMP-001",
			Name:          "cmp equal",
			GoboxArgs:     []string{"cmp", "a", "b"},
			NativeCommand: "cmp",
			NativeArgs:    []string{"a", "b"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "a"), "same")
				writeFile(t, filepath.Join(env.Dir, "b"), "same")
			},
		},
		{
			ID:            "SHA256-001",
			Name:          "sha256sum default",
			GoboxArgs:     []string{"sha256sum", "data"},
			NativeCommand: "sha256sum",
			NativeArgs:    []string{"data"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data"), "hello")
			},
		},
		{
			ID:            "STRINGS-002",
			Name:          "strings -n",
			GoboxArgs:     []string{"strings", "-n", "5", "data.bin"},
			NativeCommand: "strings",
			NativeArgs:    []string{"-n", "5", "data.bin"},
			Setup: func(t *testing.T, env *parityEnv) {
				if err := os.WriteFile(filepath.Join(env.Dir, "data.bin"), []byte{0, 'h', 'e', 'l', 'l', 'o', 0}, 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	})
}

func TestParity_NewFsCases(t *testing.T) {
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
			ID:            "STAT-004",
			Name:          "stat format",
			GoboxArgs:     []string{"stat", "-c", "%s", "data"},
			NativeCommand: "stat",
			NativeArgs:    []string{"-c", "%s", "data"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "data"), "hello")
			},
		},
	})

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
}

func TestParity_NewStructuredLinuxCases(t *testing.T) {
	t.Run("DF-005", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "df", ".")
		native := runNativeCLI(t, env, "", "df", ".")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("df exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if !strings.Contains(gobox.Stdout, "Filesystem") || !strings.Contains(native.Stdout, "Filesystem") {
			t.Fatalf("df output missing header\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})
	t.Run("IP-001", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "ip", "addr")
		native := runNativeCLI(t, env, "", "ip", "addr")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip addr exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if !strings.Contains(gobox.Stdout, "lo") || !strings.Contains(native.Stdout, "lo") {
			t.Fatalf("ip addr output missing loopback\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})
	t.Run("FREE-001", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "free")
		native := runNativeCLI(t, env, "", "free")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("free exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if !strings.Contains(gobox.Stdout, "Mem:") || !strings.Contains(native.Stdout, "Mem:") {
			t.Fatalf("free output missing Mem row\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})
}

func TestParity_NewProcBehaviorCases(t *testing.T) {
	t.Run("TIMEOUT-001", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "timeout", "0.1s", "sleep", "2")
		native := runNativeCLI(t, env, "", "timeout", "0.1s", "sleep", "2")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("timeout exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
	})
	t.Run("WATCH-001", func(t *testing.T) {
		var out strings.Builder
		old := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = w
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
		defer cancel()
		err = proc.WatchCmdWithContext(ctx, []string{"-n", "0.05", "-t", "echo", "ok"})
		_ = w.Close()
		os.Stdout = old
		_, _ = io.Copy(&out, r)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "ok") {
			t.Fatalf("watch output missing command result: %q", out.String())
		}
	})
	t.Run("KILL-010", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "kill", "--dry-run", "-x", "sleep")
		if gobox.ExitCode != 0 {
			t.Fatalf("kill --dry-run failed: %+v", gobox)
		}
	})
	t.Run("LSOF-002", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "lsof", "-p", strconv.Itoa(os.Getpid()))
		if gobox.ExitCode == 0 && gobox.Stdout == "" {
			t.Fatalf("lsof returned no output")
		}
	})
}
