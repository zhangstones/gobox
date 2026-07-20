package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"gobox/cmds/base"
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

// dfCol names a column index into a df output row, for cross-comparison
// error messages in assertDfColumnsMatch.
type dfCol struct {
	idx  int
	name string
}

// dfIntFieldToFloat parses a plain integer df column (e.g. the 1K-blocks/
// Used/Available columns of default `df`) as a float64 for use with
// assertDfColumnsMatch.
func dfIntFieldToFloat(s string) (float64, bool) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return float64(v), true
}

// dfHumanFieldToBytes converts a df -h formatted size field such as "3.6G",
// "512K", or a bare "0" into an approximate byte count, so gobox's and
// native's human-readable df output can be cross-compared numerically
// instead of merely checked for "has some K/M/G/T suffix".
func dfHumanFieldToBytes(s string) (float64, bool) {
	i := len(s)
	for i > 0 && (s[i-1] < '0' || s[i-1] > '9') && s[i-1] != '.' {
		i--
	}
	numPart := s[:i]
	unitPart := s[i:]
	v, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, false
	}
	if unitPart == "" {
		return v, true
	}
	mult := map[byte]float64{
		'K': 1 << 10, 'M': 1 << 20, 'G': 1 << 30, 'T': 1 << 40, 'P': 1 << 50,
	}
	m, ok := mult[unitPart[0]]
	if !ok {
		return 0, false
	}
	return v * m, true
}

// assertDfColumnsMatch parses the named columns of matching gobox/native df
// rows with parse and asserts the resulting values agree within a relative
// tolerance. Exact equality is too strict: the two invocations query live
// filesystem state sequentially (not atomically together), so concurrent
// host activity can shift Used/Available slightly between the two calls,
// and human-readable values are additionally already rounded to one
// significant decimal by both implementations.
func assertDfColumnsMatch(t *testing.T, label string, goboxFields, nativeFields []string, cols []dfCol, parse func(string) (float64, bool), tolerance float64) {
	t.Helper()
	for _, c := range cols {
		if c.idx >= len(goboxFields) || c.idx >= len(nativeFields) {
			t.Fatalf("%s: %s column missing gobox=%v native=%v", label, c.name, goboxFields, nativeFields)
		}
		gv, gok := parse(goboxFields[c.idx])
		nv, nok := parse(nativeFields[c.idx])
		if !gok || !nok {
			t.Fatalf("%s: %s column not parsable gobox=%q native=%q", label, c.name, goboxFields[c.idx], nativeFields[c.idx])
		}
		diff := gv - nv
		if diff < 0 {
			diff = -diff
		}
		max := gv
		if nv > max {
			max = nv
		}
		if max > 0 && diff/max > tolerance {
			t.Fatalf("%s: %s mismatch beyond tolerance: gobox=%q(%v bytes) native=%q(%v bytes)", label, c.name, goboxFields[c.idx], gv, nativeFields[c.idx], nv)
		}
	}
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

// mountTmpfsAt attempts to mount a tmpfs filesystem at dir, constructing a
// genuine cross-filesystem boundary for -x/-l parity fixtures. This requires
// CAP_SYS_ADMIN (root). If the mount cannot be established (e.g. running
// unprivileged in a sandboxed CI runner), it returns false so the caller can
// skip with a precise reason instead of silently degrading to a same-fs
// fixture that could never prove the parameter's effect.
func mountTmpfsAt(t *testing.T, dir string) bool {
	t.Helper()
	if runtime.GOOS != "linux" {
		return false
	}
	if _, err := exec.LookPath("mount"); err != nil {
		return false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := exec.Command("mount", "-t", "tmpfs", "-o", "size=4m", "tmpfs", dir).Run(); err != nil {
		return false
	}
	t.Cleanup(func() {
		_ = exec.Command("umount", dir).Run()
	})
	return true
}

// remoteDfTypes mirrors the set gobox's df -l treats as non-local
// (cmds/fs/cmd_df.go isLocalDfType).
var remoteDfTypes = map[string]bool{
	"nfs": true, "nfs4": true, "cifs": true, "smbfs": true,
	"sshfs": true, "fuse.sshfs": true, "9p": true, "afs": true,
	"ceph": true, "glusterfs": true,
}

// findRemoteMount scans /proc/self/mountinfo for a mount whose filesystem
// type is one of the "remote" types df -l is supposed to exclude. Returns
// ("", "") if no such mount is present in this environment (which is the
// common case in a sandboxed container with no NFS/CIFS/9p mounts).
func findRemoteMount(t *testing.T) (fsType, target string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		return "", ""
	}
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		sep := -1
		for i, f := range fields {
			if f == "-" {
				sep = i
				break
			}
		}
		if sep < 0 || sep+2 >= len(fields) || len(fields) < 5 {
			continue
		}
		typ := fields[sep+1]
		if remoteDfTypes[typ] {
			return typ, fields[4]
		}
	}
	return "", ""
}

// duSizeEntry is a parsed "<size><unit> <path>" du output row. unit is 0 for
// plain byte/block counts (non-human mode) or the first letter of a
// human-readable suffix (K/M/G/T), independent of whether the full suffix is
// spelled "K" (native GNU du) or "KB" (gobox) -- that cosmetic difference is
// not the thing under test here.
type duSizeEntry struct {
	value float64
	unit  byte
}

// duPathSizeEntries maps each reported path to its parsed size, for
// per-entry numeric comparison between gobox and native du output.
func duPathSizeEntries(out string) map[string]duSizeEntry {
	entries := make(map[string]duSizeEntry)
	for _, line := range nonEmptyLines(out) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sizeField := fields[0]
		path := fields[len(fields)-1]
		i := len(sizeField)
		for i > 0 && (sizeField[i-1] < '0' || sizeField[i-1] > '9') && sizeField[i-1] != '.' {
			i--
		}
		numPart := sizeField[:i]
		unitPart := sizeField[i:]
		v, err := strconv.ParseFloat(numPart, 64)
		if err != nil {
			continue
		}
		var unit byte
		if len(unitPart) > 0 {
			unit = unitPart[0]
		}
		entries[path] = duSizeEntry{value: v, unit: unit}
	}
	return entries
}

// assertDuSizesMatch verifies that every path common to both gobox and
// native du output reports a numerically equivalent size, not merely the
// same path set. Raw block-count sizes are allowed a tight tolerance to
// absorb filesystem block-size rounding; human-readable sizes (already
// rounded to one decimal by both implementations) must match exactly once
// the cosmetic "K" vs "KB" unit-suffix difference is normalized away.
func assertDuSizesMatch(t *testing.T, label, goboxOut, nativeOut string) {
	t.Helper()
	gEntries := duPathSizeEntries(goboxOut)
	nEntries := duPathSizeEntries(nativeOut)
	if len(gEntries) == 0 || len(nEntries) == 0 {
		t.Fatalf("%s: no parsable per-entry sizes\n--- gobox ---\n%s\n--- native ---\n%s", label, goboxOut, nativeOut)
	}
	matched := 0
	for path, g := range gEntries {
		n, ok := nEntries[path]
		if !ok {
			continue
		}
		matched++
		if g.unit != n.unit {
			t.Fatalf("%s: size unit mismatch for %q: gobox=%q native=%q", label, path, string(g.unit), string(n.unit))
		}
		if g.unit == 0 {
			diff := g.value - n.value
			if diff < 0 {
				diff = -diff
			}
			max := g.value
			if n.value > max {
				max = n.value
			}
			if max > 0 && diff/max > 0.10 {
				t.Fatalf("%s: size mismatch for %q: gobox=%v native=%v (>10%% apart)", label, path, g.value, n.value)
			}
		} else if g.value != n.value {
			t.Fatalf("%s: human-readable size mismatch for %q: gobox=%v%c native=%v%c", label, path, g.value, g.unit, n.value, n.unit)
		}
	}
	if matched == 0 {
		t.Fatalf("%s: gobox and native reported no common paths to compare sizes for\n--- gobox ---\n%s\n--- native ---\n%s", label, goboxOut, nativeOut)
	}
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

	// FIND-symlink-broken: a dangling symlink should be listed (not
	// followed/errored) by default find semantics.
	t.Run("FIND-symlink-broken", func(t *testing.T) {
		env := t.TempDir()
		if err := os.MkdirAll(filepath.Join(env, "tree"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("does-not-exist", filepath.Join(env, "tree", "dangling")); err != nil {
			t.Fatal(err)
		}
		gobox := runGoboxCLI(t, env, "", "find", "tree")
		native := runNativeCLI(t, env, "", "find", "tree")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("FIND-symlink-broken exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		norm := normalizeFindOutput(env)
		if norm(gobox.Stdout) != norm(native.Stdout) {
			t.Fatalf("FIND-symlink-broken mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
		if !strings.Contains(gobox.Stdout, "dangling") {
			t.Fatalf("FIND-symlink-broken: dangling symlink should still be listed\n%s", gobox.Stdout)
		}
	})

	// FIND-permission-denied: an unreadable subdirectory should be reported
	// (skipped with a diagnostic), not crash the traversal.
	//
	// This can only be exercised meaningfully as a non-root user: root
	// bypasses Unix DAC permission checks, so chmod 000 has no effect on
	// what root itself can traverse. When the test process runs as root
	// (the default in this sandboxed environment) we skip with that
	// specific reason rather than asserting a false negative.
	t.Run("FIND-permission-denied", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("FIND-permission-denied: unix permission bits not applicable on windows")
		}
		if os.Geteuid() == 0 {
			t.Skip("FIND-permission-denied: running as root, which bypasses Unix permission checks; cannot construct a real permission-denied directory in this environment")
		}
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "tree", "visible.txt"), "x")
		writeFile(t, filepath.Join(env, "tree", "locked", "hidden.txt"), "x")
		if err := os.Chmod(filepath.Join(env, "tree", "locked"), 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(env, "tree", "locked"), 0o755) })
		gobox := runGoboxCLI(t, env, "", "find", "tree")
		native := runNativeCLI(t, env, "", "find", "tree")
		if gobox.ExitCode == 0 {
			t.Fatalf("FIND-permission-denied: gobox should report a non-zero exit when a subdirectory is unreadable")
		}
		if native.ExitCode == 0 {
			t.Fatalf("FIND-permission-denied: native find should also report a non-zero exit for the unreadable directory")
		}
		if !strings.Contains(gobox.Stdout, "visible.txt") {
			t.Fatalf("FIND-permission-denied: gobox should still list readable siblings\n%s", gobox.Stdout)
		}
		if gobox.Stderr == "" {
			t.Fatalf("FIND-permission-denied: gobox should emit a diagnostic for the unreadable directory")
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
		assertDuSizesMatch(t, "DU-001", gobox.Stdout, native.Stdout)
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
		assertDuSizesMatch(t, "DU-002", gobox.Stdout, native.Stdout)
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
		assertDuSizesMatch(t, "DU-003", gobox.Stdout, native.Stdout)
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
		// Compare numeric totals. gobox and native both derive du sizes from
		// st.Blocks*512 on the same filesystem (cmds/fs/cmd_du.go duFileSize),
		// so for this ~128/256-byte two-file fixture the totals should be
		// identical modulo directory-entry block rounding; 10% is already a
		// generous margin (previously 20%, which would let a 15%-wrong
		// calculation through undetected).
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
			if diff*100/max > 10 {
				t.Fatalf("du -c totals diverge by >10%%: gobox=%d native=%d", gTotal, nTotal)
			}
		}
		assertDuSizesMatch(t, "DU-004", gobox.Stdout, native.Stdout)
	})

	t.Run("DU-c-s", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		gobox := runGoboxCLI(t, env, "", "du", "-c", "-s", "tree")
		native := runNativeCLI(t, env, "", "du", "-c", "-s", "tree")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("du -c -s exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		goboxLines := nonEmptyLines(gobox.Stdout)
		nativeLines := nonEmptyLines(native.Stdout)
		// -s collapses to a single summary row per argument, so -c -s should
		// produce exactly two lines: the tree summary and the total (which,
		// for a single argument, duplicates the summary value).
		if len(goboxLines) != 2 || len(nativeLines) != 2 {
			t.Fatalf("du -c -s should combine to summary+total (2 lines)\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
		if duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du -c -s path set mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
		assertDuSizesMatch(t, "DU-c-s", gobox.Stdout, native.Stdout)
	})

	t.Run("DU-c-a", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		gobox := runGoboxCLI(t, env, "", "du", "-c", "-a", "tree")
		native := runNativeCLI(t, env, "", "du", "-c", "-a", "tree")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("du -c -a exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		goboxLines := nonEmptyLines(gobox.Stdout)
		nativeLines := nonEmptyLines(native.Stdout)
		if !strings.HasSuffix(goboxLines[len(goboxLines)-1], "\ttotal") || !strings.HasSuffix(nativeLines[len(nativeLines)-1], "\ttotal") {
			t.Fatalf("du -c -a should end with a total line\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
		// -a combined with -c must still list every file (not just directories).
		if !strings.Contains(gobox.Stdout, "a.txt") || !strings.Contains(gobox.Stdout, "b.txt") {
			t.Fatalf("du -c -a should list per-file entries\n%s", gobox.Stdout)
		}
		if duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du -c -a path set mismatch\n--- gobox ---\n%s\n--- native ---\n%s", gobox.Stdout, native.Stdout)
		}
		assertDuSizesMatch(t, "DU-c-a", gobox.Stdout, native.Stdout)
	})

	// DU-005: exercise depth 0, 1 and 2 against a three-level tree/sub/subsub
	// fixture (a dedicated local fixture, not the shared two-level setupTree,
	// since depth 2 needs a third level to differ meaningfully from depth 1),
	// so a truncation bug at depth >=1 (e.g. an off-by-one that always
	// behaves like depth 0, or one that never truncates at all) would be
	// caught. Note: default `du` (without -a) only ever lists directories,
	// never plain files, so assertions below check for directory rows
	// (matched as exact trailing path components, not substrings, since
	// "sub" is itself a substring of "subsub").
	t.Run("DU-005", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "tree", "a.txt"), strings.Repeat("a", 128))
		writeFile(t, filepath.Join(env, "tree", "sub", "b.txt"), strings.Repeat("b", 256))
		writeFile(t, filepath.Join(env, "tree", "sub", "subsub", "c.txt"), strings.Repeat("c", 256))

		hasDirRow := func(stdout, suffix string) bool {
			for _, line := range nonEmptyLines(stdout) {
				fields := strings.Fields(line)
				if len(fields) < 2 {
					continue
				}
				path := filepath.ToSlash(fields[len(fields)-1])
				if path == suffix || strings.HasSuffix(path, "/"+suffix) {
					return true
				}
			}
			return false
		}

		var rowCounts []int
		for _, depth := range []string{"0", "1", "2"} {
			gobox := runGoboxCLI(t, env, "", "du", "-d", depth, "tree")
			native := runNativeCLI(t, env, "", "du", "-d", depth, "tree")
			if gobox.ExitCode != native.ExitCode || duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
				t.Fatalf("du -d %s mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", depth, gobox, native)
			}
			longOpt := runGoboxCLI(t, env, "", "du", "--max-depth", depth, "tree")
			if duPathSet(gobox.Stdout) != duPathSet(longOpt.Stdout) {
				t.Fatalf("du -d and --max-depth differ at depth %s\n-d=%q\n--max-depth=%q", depth, gobox.Stdout, longOpt.Stdout)
			}
			rowCounts = append(rowCounts, len(nonEmptyLines(gobox.Stdout)))
			switch depth {
			case "0":
				if hasDirRow(gobox.Stdout, "sub") || hasDirRow(gobox.Stdout, "subsub") {
					t.Fatalf("du -d 0 should only report the root entry, got\n%s", gobox.Stdout)
				}
			case "1":
				if hasDirRow(gobox.Stdout, "subsub") {
					t.Fatalf("du -d 1 should not descend into tree/sub/subsub, got\n%s", gobox.Stdout)
				}
				if !hasDirRow(gobox.Stdout, "sub") {
					t.Fatalf("du -d 1 should include tree/sub, got\n%s", gobox.Stdout)
				}
			case "2":
				if !hasDirRow(gobox.Stdout, "subsub") {
					t.Fatalf("du -d 2 should include tree/sub/subsub, got\n%s", gobox.Stdout)
				}
			}
		}
		// The row count must strictly grow as depth increases from 0 to 2 for
		// this fixture -- proves -d actually changes the truncation depth
		// rather than being a no-op that happens to pass the path-set check.
		if !(rowCounts[0] < rowCounts[1] && rowCounts[1] < rowCounts[2]) {
			t.Fatalf("du -d row counts should strictly increase with depth, got %v", rowCounts)
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
		assertDuSizesMatch(t, "DU-006", gobox.Stdout, native.Stdout)
	})

	// DU-007: -x must skip a subtree that lives on a different filesystem.
	// A same-filesystem fixture can never prove this (an implementation that
	// completely ignores -x would still pass), so this mounts a real tmpfs
	// at tree/mnt to construct a genuine cross-filesystem boundary.
	t.Run("DU-007", func(t *testing.T) {
		env := t.TempDir()
		setupTree(env)
		mountDir := filepath.Join(env, "tree", "mnt")
		if !mountTmpfsAt(t, mountDir) {
			t.Skip("DU-007: cannot mount tmpfs to construct a genuine cross-filesystem fixture (requires CAP_SYS_ADMIN); without crossing a real mount boundary, -x cannot be proven to have any effect")
		}
		writeFile(t, filepath.Join(mountDir, "crossfs.txt"), strings.Repeat("z", 4096))

		// Sanity-check the fixture: without -x, the cross-fs file must be
		// visible, otherwise the mount didn't actually take effect.
		baseline := runGoboxCLI(t, env, "", "du", "-a", "tree")
		if !strings.Contains(baseline.Stdout, "crossfs.txt") {
			t.Fatalf("DU-007 fixture invalid: baseline du -a should include the cross-fs file\n%s", baseline.Stdout)
		}

		gobox := runGoboxCLI(t, env, "", "du", "-x", "-a", "tree")
		native := runNativeCLI(t, env, "", "du", "-x", "-a", "tree")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("du -x exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if duPathSet(gobox.Stdout) != duPathSet(native.Stdout) {
			t.Fatalf("du -x mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", gobox, native)
		}
		// The whole point of -x: the mounted subtree must now be excluded.
		if strings.Contains(gobox.Stdout, "crossfs.txt") {
			t.Fatalf("du -x should exclude the mounted subtree, but gobox still reports it\n%s", gobox.Stdout)
		}
		if strings.Contains(native.Stdout, "crossfs.txt") {
			t.Fatalf("du -x fixture invariant broken: native itself included the cross-fs file\n%s", native.Stdout)
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
		assertDuSizesMatch(t, "DU-008", gobox.Stdout, native.Stdout)
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
		assertDuSizesMatch(t, "DU-sh", gobox.Stdout, native.Stdout)
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
		assertDuSizesMatch(t, "DU-multi-exclude", gobox.Stdout, native.Stdout)
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

	// DU-permission-denied: an unreadable subdirectory should be reported,
	// not silently skipped. As with FIND-permission-denied, this can only be
	// exercised meaningfully as a non-root user (root bypasses DAC checks),
	// so it is skipped with that specific reason when running as root.
	t.Run("DU-permission-denied", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("DU-permission-denied: unix permission bits not applicable on windows")
		}
		if os.Geteuid() == 0 {
			t.Skip("DU-permission-denied: running as root, which bypasses Unix permission checks; cannot construct a real permission-denied directory in this environment")
		}
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "tree", "visible.txt"), "x")
		writeFile(t, filepath.Join(env, "tree", "locked", "hidden.txt"), "x")
		if err := os.Chmod(filepath.Join(env, "tree", "locked"), 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(env, "tree", "locked"), 0o755) })
		gobox := runGoboxCLI(t, env, "", "du", "-a", "tree")
		native := runNativeCLI(t, env, "", "du", "-a", "tree")
		if gobox.ExitCode == 0 {
			t.Fatalf("DU-permission-denied: gobox should report a non-zero exit when a subdirectory is unreadable")
		}
		if native.ExitCode == 0 {
			t.Fatalf("DU-permission-denied: native du should also report a non-zero exit for the unreadable directory")
		}
		if !strings.Contains(gobox.Stdout, "visible.txt") {
			t.Fatalf("DU-permission-denied: gobox should still report readable siblings\n%s", gobox.Stdout)
		}
		if gobox.Stderr == "" {
			t.Fatalf("DU-permission-denied: gobox should emit a diagnostic for the unreadable directory")
		}
	})
}

func TestParity_DfCases(t *testing.T) {
	t.Run("DF-001", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("DF-001: default df row identification requires linux /proc/self/mountinfo")
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
		goboxFields := strings.Fields(nonEmptyLines(res.Stdout)[1])
		nativeFields := strings.Fields(nonEmptyLines(native.Stdout)[1])
		if goboxFields[0] != nativeFields[0] {
			t.Fatalf("DF-001 filesystem name mismatch gobox=%q native=%q", goboxFields[0], nativeFields[0])
		}
		// Cross-compare the Size/Used/Available numeric columns for that same
		// filesystem, not merely the filesystem name.
		assertDfColumnsMatch(t, "DF-001", goboxFields, nativeFields,
			[]dfCol{{1, "Size"}, {2, "Used"}, {3, "Available"}}, dfIntFieldToFloat, 0.02)
	})

	t.Run("DF-002", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("DF-002: -h human-readable size formatting comparison requires linux /proc/self/mountinfo")
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
		// Cross-compare: parse the actual converted numeric value (bytes)
		// behind each human-readable field for the same filesystem row,
		// rather than merely checking that some K/M/G/T-style suffix is
		// present on both sides.
		goboxFields := strings.Fields(nonEmptyLines(res.Stdout)[1])
		nativeFields := strings.Fields(nonEmptyLines(native.Stdout)[1])
		assertDfColumnsMatch(t, "DF-002", goboxFields, nativeFields,
			[]dfCol{{1, "Size"}, {2, "Used"}, {3, "Avail"}}, dfHumanFieldToBytes, 0.10)
	})

	t.Run("DF-003", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("DF-003: -T filesystem-type column comparison requires linux /proc/self/mountinfo")
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
		// Cross-compare: gobox's reported filesystem type must match native's
		// for the same mount, not merely be present.
		gType := strings.Fields(nonEmptyLines(res.Stdout)[1])[1]
		nType := strings.Fields(nonEmptyLines(native.Stdout)[1])[1]
		if gType != nType {
			t.Fatalf("DF-003 filesystem type mismatch: gobox=%q native=%q", gType, nType)
		}
	})

	t.Run("DF-004", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("DF-004: -i inode column comparison requires linux /proc/self/mountinfo")
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
		// Cross-compare: the total inode count for the same filesystem
		// should be nearly identical between gobox and native, not merely
		// independently non-zero in each. Exact equality is too strict: the
		// two invocations query live filesystem state sequentially (not
		// atomically together), and concurrent activity on the host (e.g.
		// other tests creating/removing files on the same filesystem) can
		// shift the free-inode count by a handful between the two calls.
		// Allow a small relative tolerance rather than requiring the two
		// live syscalls to observe an identical instant.
		gInodesStr := strings.Fields(nonEmptyLines(res.Stdout)[1])[1]
		nInodesStr := strings.Fields(nonEmptyLines(native.Stdout)[1])[1]
		gInodes, gErr := strconv.ParseInt(gInodesStr, 10, 64)
		nInodes, nErr := strconv.ParseInt(nInodesStr, 10, 64)
		if gErr != nil || nErr != nil {
			t.Fatalf("DF-004 inode totals not numeric: gobox=%q native=%q", gInodesStr, nInodesStr)
		}
		diff := gInodes - nInodes
		if diff < 0 {
			diff = -diff
		}
		max := gInodes
		if nInodes > max {
			max = nInodes
		}
		if max == 0 || float64(diff)/float64(max) > 0.01 {
			t.Fatalf("DF-004 inode total mismatch beyond tolerance: gobox=%d native=%d", gInodes, nInodes)
		}
	})

	t.Run("DF-005", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("DF-005: single-path df row rendering requires linux /proc/self/mountinfo")
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
		id   string
		args []string
	}{
		{"DF-006", []string{"df", "-H", "."}},
		{"DF-007", []string{"df", "-a"}},
		{"DF-008", []string{"df", "-l"}},
		{"DF-009", []string{"df", "-t", "tmpfs"}},
		{"DF-010", []string{"df", "-x", "tmpfs"}},
		{"DF-011", []string{"df", "--total"}},
		{"DF-012", []string{"df", "-P", "."}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			if runtime.GOOS != "linux" {
				t.Skip(fmt.Sprintf("%s: df flag comparison requires linux /proc/self/mountinfo", tc.id))
			}
			// Share a single fixture directory between gobox and native so
			// path-sensitive cases (df -H ., df -P .) observe the same
			// filesystem, instead of two unrelated t.TempDir() calls that
			// only happen to agree because they're both on the same host.
			env := t.TempDir()
			res := runGoboxCLI(t, env, "", tc.args...)
			native := runNativeCLI(t, env, "", tc.args[0], tc.args[1:]...)
			if res.ExitCode != native.ExitCode {
				t.Fatalf("%s exit mismatch gobox=%d native=%d\n--- gobox ---\n%s\n--- native ---\n%s", tc.id, res.ExitCode, native.ExitCode, res.Stdout, native.Stdout)
			}
			switch tc.id {
			case "DF-006":
				base := runGoboxCLI(t, env, "", "df", ".")
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
				base := runGoboxCLI(t, env, "", "df")
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

				// The checks above only prove ambient host mounts happen to
				// be local; they never exercise a genuinely different
				// filesystem, so an implementation that completely ignores
				// -l would still pass. Construct a controlled fixture that
				// is verifiably its own filesystem (a tmpfs mount) and prove
				// -l keeps it.
				//
				// Exercising the exclusion side properly would require a
				// real "remote" filesystem (nfs/cifs/9p/...). Standing up a
				// loopback NFS/CIFS server from within this test is blocked
				// by the harness's network/service sandboxing policy (it
				// classifies starting rpcbind/rpc.nfsd/rpc.mountd and
				// exporting a share as exposing a local service), so per
				// TEST-DESIGN.md §13/§11 rule 6 we document that limitation
				// here and fall back to an opportunistic check against any
				// ambient remote mount instead of failing outright.
				mountDir := filepath.Join(t.TempDir(), "df008mnt")
				if !mountTmpfsAt(t, mountDir) {
					t.Skip("DF-008: cannot mount tmpfs to construct a controlled local-filesystem fixture (requires CAP_SYS_ADMIN)")
				}
				resLocal := runGoboxCLI(t, mountDir, "", "df", "-l", ".")
				nativeLocal := runNativeCLI(t, mountDir, "", "df", "-l", ".")
				if resLocal.ExitCode != nativeLocal.ExitCode {
					t.Fatalf("DF-008 -l exit mismatch on controlled tmpfs fixture gobox=%d native=%d", resLocal.ExitCode, nativeLocal.ExitCode)
				}
				if len(nonEmptyLines(resLocal.Stdout)) < 2 {
					t.Fatalf("DF-008: -l should still list the controlled local tmpfs fixture mount\n%s", resLocal.Stdout)
				}
				if len(nonEmptyLines(nativeLocal.Stdout)) < 2 {
					t.Fatalf("DF-008: native -l should still list the controlled local tmpfs fixture mount\n%s", nativeLocal.Stdout)
				}

				remoteType, remoteTarget := findRemoteMount(t)
				if remoteType == "" {
					t.Skip("DF-008: no remote-type filesystem (nfs/cifs/9p/sshfs/...) mounted in this environment; cannot verify -l's exclusion side beyond the controlled local fixture proven above")
				}
				for _, out := range []struct {
					name string
					text string
				}{
					{"gobox", res.Stdout},
					{"native", native.Stdout},
				} {
					if strings.Contains(out.text, remoteTarget) {
						t.Fatalf("DF-008 %s -l should exclude remote mount %q (%s)\n%s", out.name, remoteTarget, remoteType, out.text)
					}
				}
			case "DF-009":
				// Capture an unfiltered baseline first: without it, an
				// environment with zero ambient tmpfs mounts would make
				// "df -t tmpfs" trivially produce an empty (header-only)
				// result, and the per-row loop below would vacuously pass
				// without ever exercising the inclusion filter.
				base := runGoboxCLI(t, env, "", "df")
				if base.ExitCode != 0 {
					t.Fatalf("DF-009 baseline failed: %+v", base)
				}
				baseSawTmpfs := false
				for _, line := range nonEmptyLines(base.Stdout)[1:] {
					fields := strings.Fields(line)
					if len(fields) > 0 && fields[0] == "tmpfs" {
						baseSawTmpfs = true
						break
					}
				}
				if !baseSawTmpfs {
					t.Skip("DF-009: no tmpfs mount present in unfiltered df baseline; -t tmpfs filtering cannot be meaningfully exercised in this environment")
				}
				resRows := nonEmptyLines(res.Stdout)[1:]
				if len(resRows) == 0 {
					t.Fatalf("DF-009 baseline has tmpfs mounts but -t tmpfs returned none\n--- base ---\n%s\n--- filtered ---\n%s", base.Stdout, res.Stdout)
				}
				for _, line := range resRows {
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
				// Capture an unfiltered baseline first: without it, an
				// environment with only tmpfs mounts (nothing "other than
				// tmpfs" to keep), or with no tmpfs mounts at all (nothing
				// for -x to exclude, so a no-op implementation would still
				// pass the per-row loop below), would let a broken -x
				// implementation pass vacuously.
				base := runGoboxCLI(t, env, "", "df")
				if base.ExitCode != 0 {
					t.Fatalf("DF-010 baseline failed: %+v", base)
				}
				baseSawTmpfs := false
				baseNonTmpfsCount := 0
				for _, line := range nonEmptyLines(base.Stdout)[1:] {
					fields := strings.Fields(line)
					if len(fields) == 0 {
						continue
					}
					if fields[0] == "tmpfs" {
						baseSawTmpfs = true
					} else {
						baseNonTmpfsCount++
					}
				}
				if !baseSawTmpfs {
					t.Skip("DF-010: no tmpfs mount present in unfiltered df baseline; -x tmpfs exclusion cannot be meaningfully exercised in this environment")
				}
				if baseNonTmpfsCount == 0 {
					t.Skip("DF-010: unfiltered df baseline contains only tmpfs mounts; -x tmpfs would leave nothing, so exclusion cannot be meaningfully exercised in this environment")
				}
				resRows := nonEmptyLines(res.Stdout)[1:]
				if len(resRows) == 0 {
					t.Fatalf("DF-010 baseline has %d non-tmpfs mount(s) but -x tmpfs returned none\n--- base ---\n%s\n--- filtered ---\n%s", baseNonTmpfsCount, base.Stdout, res.Stdout)
				}
				for _, line := range resRows {
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
				// Verify the total row's numeric columns are actually the
				// sum of the per-filesystem rows above it, not just
				// correctly shaped. Columns 1/2/3 are the blocks/used/
				// available columns for the default (non -i, non -T) header.
				dataRows := rows[:len(rows)-1]
				for _, col := range []int{1, 2, 3} {
					var sum int64
					for _, row := range dataRows {
						v, err := strconv.ParseInt(row[col], 10, 64)
						if err != nil {
							t.Fatalf("DF-011 non-numeric column %d in row %v", col, row)
						}
						sum += v
					}
					got, err := strconv.ParseInt(total[col], 10, 64)
					if err != nil {
						t.Fatalf("DF-011 non-numeric total column %d: %v", col, total)
					}
					if got != sum {
						t.Fatalf("DF-011 total column %d (%s) is not the sum of per-filesystem rows: total=%d sum-of-rows=%d\n%s", col, header[col], got, sum, res.Stdout)
					}
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
		})
	}

	// DF-hT: combined -h and -T flags.
	t.Run("DF-hT", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("DF-hT: combined -h -T rendering comparison requires linux /proc/self/mountinfo")
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
			t.Skip("DF-error: missing-path exit-code comparison requires linux /proc/self/mountinfo")
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
				// gobox's default output now mirrors GNU stat's full multi-line
				// layout (File/Size/Device/Access/Modify/Change), so compare
				// line-for-line. gobox intentionally omits the optional
				// " Birth:" line (filesystem-birth-time support varies and
				// isn't universally available), so strip it from native
				// before comparing.
				var nativeLines []string
				for _, line := range strings.Split(strings.TrimRight(native.Stdout, "\n"), "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), "Birth:") {
						continue
					}
					nativeLines = append(nativeLines, line)
				}
				goboxLines := strings.Split(strings.TrimRight(gobox.Stdout, "\n"), "\n")
				if strings.Join(goboxLines, "\n") != strings.Join(nativeLines, "\n") {
					t.Fatalf("stat default output mismatch\n--- gobox ---\n%s\n--- native (Birth line stripped) ---\n%s",
						strings.Join(goboxLines, "\n"), strings.Join(nativeLines, "\n"))
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
				// Mode field parity (gobox's default output now includes an
				// "Access: (mode/perm)" line like native).
				goboxMode := statModeFromAccess(gobox.Stdout)
				nativeMode := statModeFromAccess(native.Stdout)
				if goboxMode == "" || nativeMode == "" {
					t.Fatalf("stat -L missing Access mode field gobox=%q native=%q", goboxMode, nativeMode)
				}
				if goboxMode != nativeMode {
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
				goboxBlockLine := statLineWithPrefix(gobox.Stdout, "Block size:")
				nativeBlockLine := statLineWithPrefix(native.Stdout, "Block size:")
				if goboxBlockLine == "" || nativeBlockLine == "" {
					t.Fatalf("stat -f missing block size line\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
				}
				if !strings.Contains(gobox.Stdout, "Fundamental block size:") || !strings.Contains(native.Stdout, "Fundamental block size:") {
					t.Fatalf("stat -f missing Fundamental block size field\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
				}
				// Cross-compare: block size is fixed filesystem metadata, so
				// gobox's reported value should match native's exactly, not
				// merely be independently parseable.
				goboxBlockSize := statFieldValue(goboxBlockLine, "Block size:")
				nativeBlockSize := statFieldValue(nativeBlockLine, "Block size:")
				if _, err := strconv.ParseUint(goboxBlockSize, 10, 64); goboxBlockSize == "" || err != nil {
					t.Fatalf("stat -f block size should be numeric, got %q: %v", goboxBlockSize, err)
				}
				if goboxBlockSize != nativeBlockSize {
					t.Fatalf("stat -f block size mismatch gobox=%q native=%q", goboxBlockSize, nativeBlockSize)
				}
				goboxInodes := statLineWithPrefix(gobox.Stdout, "Inodes:")
				nativeInodes := statLineWithPrefix(native.Stdout, "Inodes:")
				if goboxInodes == "" || nativeInodes == "" {
					t.Fatalf("stat -f missing Inodes line\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
				}
				goboxTotalStr := statFieldValue(goboxInodes, "Total:")
				nativeTotalStr := statFieldValue(nativeInodes, "Total:")
				goboxTotal, gErr := strconv.ParseUint(goboxTotalStr, 10, 64)
				if goboxTotalStr == "" || gErr != nil {
					t.Fatalf("stat -f Inodes Total should be a real numeric value, got %q: %v", goboxTotalStr, gErr)
				}
				nativeTotal, nErr := strconv.ParseUint(nativeTotalStr, 10, 64)
				if nativeTotalStr == "" || nErr != nil {
					t.Fatalf("stat -f native Inodes Total should be a real numeric value, got %q: %v", nativeTotalStr, nErr)
				}
				// Cross-compare: gobox's inode total for the filesystem must
				// match native's actual reported value, not merely be
				// independently numeric. A small relative tolerance (mirrors
				// DF-004's inode cross-check) absorbs the rare case where the
				// two live syscalls race a concurrent mount/remount.
				diff := int64(goboxTotal) - int64(nativeTotal)
				if diff < 0 {
					diff = -diff
				}
				max := goboxTotal
				if nativeTotal > max {
					max = nativeTotal
				}
				if max == 0 || float64(diff)/float64(max) > 0.01 {
					t.Fatalf("stat -f Inodes Total mismatch beyond tolerance: gobox=%d native=%d", goboxTotal, nativeTotal)
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
				// gobox's terse format now mirrors GNU coreutils' full field
				// layout: name size blocks rawmode(hex) uid gid device(hex)
				// inode links major(hex) minor(hex) atime mtime ctime
				// birthtime blksize (CMD-SPECS.md "stat -t"). Every field
				// should match native exactly except birthtime (index 14),
				// which gobox always reports as 0 (birth time isn't tracked)
				// while native's value depends on filesystem support.
				gFields := strings.Fields(normalizeText(gobox.Stdout))
				nFields := strings.Fields(normalizeText(native.Stdout))
				if len(gFields) != len(nFields) {
					t.Fatalf("stat -t field count mismatch gobox=%d native=%d\ngobox:  %q\nnative: %q", len(gFields), len(nFields), gobox.Stdout, native.Stdout)
				}
				const birthtimeIdx = 14
				for i := range gFields {
					if i == birthtimeIdx {
						continue
					}
					if gFields[i] != nFields[i] {
						t.Fatalf("stat -t field %d mismatch gobox=%q native=%q\ngobox:  %q\nnative: %q", i, gFields[i], nFields[i], gobox.Stdout, native.Stdout)
					}
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

	// STAT-long-flags: --dereference/--file-system/--format/--terse must
	// produce output identical to their short-flag equivalents (-L/-f/-c/-t).
	t.Run("STAT-long-flags", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "target"), "hello")
		if err := os.Symlink("target", filepath.Join(env, "link")); err != nil {
			t.Fatal(err)
		}

		short := runGoboxCLI(t, env, "", "stat", "-L", "link")
		long := runGoboxCLI(t, env, "", "stat", "--dereference", "link")
		if short.ExitCode != long.ExitCode || short.Stdout != long.Stdout {
			t.Fatalf("stat -L vs --dereference mismatch\n-L=%+v\n--dereference=%+v", short, long)
		}

		short = runGoboxCLI(t, env, "", "stat", "-f", ".")
		long = runGoboxCLI(t, env, "", "stat", "--file-system", ".")
		if short.ExitCode != long.ExitCode || short.Stdout != long.Stdout {
			t.Fatalf("stat -f vs --file-system mismatch\n-f=%+v\n--file-system=%+v", short, long)
		}

		short = runGoboxCLI(t, env, "", "stat", "-c", "%n:%s", "target")
		long = runGoboxCLI(t, env, "", "stat", "--format", "%n:%s", "target")
		if short.ExitCode != long.ExitCode || short.Stdout != long.Stdout {
			t.Fatalf("stat -c vs --format mismatch\n-c=%+v\n--format=%+v", short, long)
		}

		short = runGoboxCLI(t, env, "", "stat", "-t", "target")
		long = runGoboxCLI(t, env, "", "stat", "--terse", "target")
		if short.ExitCode != long.ExitCode || short.Stdout != long.Stdout {
			t.Fatalf("stat -t vs --terse mismatch\n-t=%+v\n--terse=%+v", short, long)
		}
	})

	// STAT-symlink-broken: statting a dangling symlink without -L should
	// succeed and report the link itself (native parity); with -L it must
	// fail because the target doesn't exist.
	t.Run("STAT-symlink-broken", func(t *testing.T) {
		env := t.TempDir()
		if err := os.Symlink("does-not-exist", filepath.Join(env, "dangling")); err != nil {
			t.Fatal(err)
		}
		gobox := runGoboxCLI(t, env, "", "stat", "dangling")
		native := runNativeCLI(t, env, "", "stat", "dangling")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("STAT-symlink-broken (no -L) exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if gobox.ExitCode != 0 {
			t.Fatalf("STAT-symlink-broken (no -L) should succeed on the symlink itself, gobox=%+v", gobox)
		}
		goboxL := runGoboxCLI(t, env, "", "stat", "-L", "dangling")
		nativeL := runNativeCLI(t, env, "", "stat", "-L", "dangling")
		if (goboxL.ExitCode == 0) != (nativeL.ExitCode == 0) {
			t.Fatalf("STAT-symlink-broken -L exit-zero mismatch gobox=%d native=%d", goboxL.ExitCode, nativeL.ExitCode)
		}
		if goboxL.ExitCode == 0 {
			t.Fatalf("STAT-symlink-broken -L should fail to follow a dangling symlink, got exit 0: %+v", goboxL)
		}
	})

	// STAT-permission-denied: statting a path through an unreadable parent
	// directory should fail. As with FIND/DU, this can only be exercised
	// meaningfully as a non-root user.
	t.Run("STAT-permission-denied", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("STAT-permission-denied: unix permission bits not applicable on windows")
		}
		if os.Geteuid() == 0 {
			t.Skip("STAT-permission-denied: running as root, which bypasses Unix permission checks; cannot construct a real permission-denied directory in this environment")
		}
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "locked", "hidden.txt"), "x")
		if err := os.Chmod(filepath.Join(env, "locked"), 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(env, "locked"), 0o755) })
		gobox := runGoboxCLI(t, env, "", "stat", "locked/hidden.txt")
		native := runNativeCLI(t, env, "", "stat", "locked/hidden.txt")
		if gobox.ExitCode == 0 {
			t.Fatalf("STAT-permission-denied: gobox should fail to stat a path behind an unreadable directory")
		}
		if native.ExitCode == 0 {
			t.Fatalf("STAT-permission-denied: native stat should also fail")
		}
		if gobox.Stderr == "" {
			t.Fatalf("STAT-permission-denied: gobox should emit an error message on stderr")
		}
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

	// TRUNCATE-no-create-long: --no-create must behave identically to -c.
	t.Run("TRUNCATE-no-create-long", func(t *testing.T) {
		env := t.TempDir()
		short := runGoboxCLI(t, env, "", "truncate", "-c", "-s", "2", "missing-short")
		long := runGoboxCLI(t, env, "", "truncate", "--no-create", "-s", "2", "missing-long")
		if short.ExitCode != long.ExitCode {
			t.Fatalf("truncate -c vs --no-create exit mismatch -c=%d --no-create=%d", short.ExitCode, long.ExitCode)
		}
		if short.Stdout != long.Stdout || short.Stderr != long.Stderr {
			t.Fatalf("truncate -c vs --no-create output mismatch\n-c: stdout=%q stderr=%q\n--no-create: stdout=%q stderr=%q",
				short.Stdout, short.Stderr, long.Stdout, long.Stderr)
		}
		if _, err := os.Stat(filepath.Join(env, "missing-short")); !os.IsNotExist(err) {
			t.Fatalf("truncate -c unexpectedly created missing-short")
		}
		if _, err := os.Stat(filepath.Join(env, "missing-long")); !os.IsNotExist(err) {
			t.Fatalf("truncate --no-create unexpectedly created missing-long")
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

// TestParity_AliasCases covers the gobox-only `alias` command (ALIAS-001/
// 002/003 in docs/TEST-CASES.md). `alias` is a 🆕 gobox扩展 with no native
// baseline, so these are contract tests: they verify gobox's own documented
// script-generation behavior (docs/TEST-CASES.md "alias" table), driven off
// the live command registry (cmds/base.Commands()) rather than a hardcoded
// command list, so the case stays correct as commands are added/removed.
func TestParity_AliasCases(t *testing.T) {
	// ALIAS-001: default script exports gobox_alias_type=bash, aliases every
	// registered subcommand except "alias" itself (no recursive alias).
	t.Run("ALIAS-001", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "alias")
		if res.ExitCode != 0 {
			t.Fatalf("alias exit %d: %+v", res.ExitCode, res)
		}
		if !strings.Contains(res.Stdout, "gobox_alias_type=bash") {
			t.Fatalf("alias script missing gobox_alias_type=bash\n%s", res.Stdout)
		}
		if strings.Contains(res.Stdout, "alias alias=") {
			t.Fatalf("alias script must not create a recursive alias for 'alias' itself\n%s", res.Stdout)
		}
		cmds := base.Commands()
		if len(cmds) == 0 {
			t.Fatal("base.Commands() returned no registered commands; cannot verify alias coverage")
		}
		for _, cmd := range cmds {
			if cmd.Name() == "alias" {
				continue
			}
			want := fmt.Sprintf("alias %s='gobox %s'", cmd.Name(), cmd.Name())
			if !strings.Contains(res.Stdout, want) {
				t.Fatalf("alias script missing alias line for registered command %q (want %q)\n%s", cmd.Name(), want, res.Stdout)
			}
		}
	})

	// ALIAS-002: -u prints the mirrored unalias script for the same command
	// set, and cleans up gobox_alias_type at the end.
	t.Run("ALIAS-002", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "alias", "-u")
		if res.ExitCode != 0 {
			t.Fatalf("alias -u exit %d: %+v", res.ExitCode, res)
		}
		if !strings.Contains(res.Stdout, "unset gobox_alias_type") {
			t.Fatalf("alias -u script should unset gobox_alias_type\n%s", res.Stdout)
		}
		for _, cmd := range base.Commands() {
			if cmd.Name() == "alias" {
				if strings.Contains(res.Stdout, "unalias alias ") {
					t.Fatalf("alias -u script must not unalias 'alias' itself\n%s", res.Stdout)
				}
				continue
			}
			want := fmt.Sprintf("unalias %s 2>/dev/null || true", cmd.Name())
			if !strings.Contains(res.Stdout, want) {
				t.Fatalf("alias -u script missing unalias line for registered command %q (want %q)\n%s", cmd.Name(), want, res.Stdout)
			}
		}
	})

	// ALIAS-003: -h prints usage/help text including the documented
	// `gobox alias [-u]` usage line.
	t.Run("ALIAS-003", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "alias", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("alias -h exit %d: %+v", res.ExitCode, res)
		}
		if !strings.Contains(res.Stdout, "gobox alias [-u]") {
			t.Fatalf("alias -h help text missing usage line 'gobox alias [-u]'\n%s", res.Stdout)
		}
		if !strings.Contains(strings.ToLower(res.Stdout), "alias") {
			t.Fatalf("alias -h help text should describe the command's purpose\n%s", res.Stdout)
		}
	})
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
