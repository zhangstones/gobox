package main

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func iostatHeaderAndRows(out string) ([]string, [][]string) {
	lines := nonEmptyLines(out)
	headerIdx := -1
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "Device" {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil, nil
	}
	header := strings.Fields(lines[headerIdx])
	rows := make([][]string, 0, len(lines)-headerIdx-1)
	for _, line := range lines[headerIdx+1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		rows = append(rows, fields)
	}
	return header, rows
}

func iostatCommonDeviceRows(goboxRows, nativeRows [][]string) [][]string {
	nativeByDev := make(map[string][]string, len(nativeRows))
	for _, row := range nativeRows {
		if len(row) > 0 {
			nativeByDev[row[0]] = row
		}
	}
	var common [][]string
	for _, row := range goboxRows {
		if len(row) == 0 {
			continue
		}
		if native, ok := nativeByDev[row[0]]; ok {
			common = append(common, row)
			common = append(common, native)
		}
	}
	return common
}

func isIostatRateField(field string) bool {
	if strings.HasSuffix(field, "/s") {
		return true
	}
	_, err := strconv.ParseFloat(field, 64)
	return err == nil
}

func TestParity_Md5sumCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
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
				if gobox.Stdout != "" {
					t.Fatalf("md5sum --status should produce no stdout, got: %q", gobox.Stdout)
				}
				if gobox.Stderr != "" {
					t.Fatalf("md5sum --status should produce no stderr, got: %q", gobox.Stderr)
				}
			},
		},
	})

	t.Run("MD5-005", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "checksums.md5"), "bad line\n")
		gobox := runGoboxCLI(t, env, "", "md5sum", "--warn", "--check", "checksums.md5")
		native := runNativeCLI(t, env, "", "md5sum", "--warn", "--check", "checksums.md5")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("md5sum --warn exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		// Warning must appear on stderr only — stdout must be empty.
		if gobox.Stdout != "" {
			t.Fatalf("md5sum --warn: warning should be on stderr only, got stdout: %q", gobox.Stdout)
		}
		if findLineContaining(strings.ToLower(gobox.Stderr), "improperly formatted") == "" {
			t.Fatalf("md5sum --warn missing gobox warning on stderr: %+v", gobox)
		}
		if findLineContaining(strings.ToLower(native.Stdout+native.Stderr), "improperly formatted") == "" {
			t.Fatalf("md5sum --warn missing native warning: %+v", native)
		}
	})

	t.Run("MD5-005-mixed", func(t *testing.T) {
		// Mixed file: one valid checksum line and one malformed line.
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "good.txt"), "hello")
		sum := runNativeCLI(t, env, "", "md5sum", "good.txt")
		content := normalizeText(sum.Stdout) + "\n" + "not a valid checksum line\n"
		writeFile(t, filepath.Join(env, "checksums.md5"), content)
		gobox := runGoboxCLI(t, env, "", "md5sum", "--warn", "--check", "checksums.md5")
		if gobox.ExitCode == 0 {
			t.Fatalf("md5sum --warn with malformed line should fail, got exit 0: %+v", gobox)
		}
		if findLineContaining(strings.ToLower(gobox.Stderr), "improperly formatted") == "" {
			t.Fatalf("md5sum --warn mixed: missing per-line warning on stderr: %+v", gobox)
		}
		// Valid file result should appear on stdout.
		if findLineContaining(gobox.Stdout, "good.txt") == "" {
			t.Fatalf("md5sum --warn mixed: valid file result missing from stdout: %+v", gobox)
		}
	})

	t.Run("MD5-006", func(t *testing.T) {
		// --quiet in compute mode (no --check) should behave identically to without --quiet.
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "file.txt"), "hello")
		gobox := runGoboxCLI(t, env, "", "md5sum", "--quiet", "file.txt")
		native := runNativeCLI(t, env, "", "md5sum", "--quiet", "file.txt")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("md5sum --quiet file exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("md5sum --quiet file stdout mismatch\ngobox=%q\nnative=%q", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("MD5-stdin", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "hello\n", "md5sum")
		native := runNativeCLI(t, env, "hello\n", "md5sum")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("md5sum stdin exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("md5sum stdin stdout mismatch\ngobox=%q\nnative=%q", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("MD5-failed", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "data.txt"), "hello")
		sum := runNativeCLI(t, env, "", "md5sum", "data.txt")
		writeFile(t, filepath.Join(env, "checksums.md5"), normalizeText(sum.Stdout)+"\n")
		// Tamper with the file after generating the checksum.
		writeFile(t, filepath.Join(env, "data.txt"), "TAMPERED")
		gobox := runGoboxCLI(t, env, "", "md5sum", "--check", "checksums.md5")
		if gobox.ExitCode == 0 {
			t.Fatalf("md5sum --check with tampered file should fail, got exit 0: %+v", gobox)
		}
		if findLineContaining(gobox.Stdout, "FAILED") == "" {
			t.Fatalf("md5sum --check with tampered file: missing FAILED in stdout: %+v", gobox)
		}
		// Native behaviour for reference.
		native := runNativeCLI(t, env, "", "md5sum", "--check", "checksums.md5")
		if native.ExitCode == 0 {
			t.Fatalf("native md5sum --check with tampered file should fail")
		}
	})
}

func TestParity_IostatCases(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}

	requireNativeCommand(t, "iostat")

	t.Run("IOSTAT-001", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "iostat", "-i", "1", "-n", "1")
		native := runNativeCLI(t, t.TempDir(), "", "iostat", "1", "1")
		assertIostatStructuredParity(t, gobox, native)
	})

	t.Run("IOSTAT-002", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "iostat", "-n", "1")
		native := runNativeCLI(t, t.TempDir(), "", "iostat")
		assertIostatStructuredParity(t, gobox, native)
	})

	t.Run("IOSTAT-003", func(t *testing.T) {
		base := runGoboxCLI(t, t.TempDir(), "", "iostat", "-n", "1")
		gobox := runGoboxCLI(t, t.TempDir(), "", "iostat", "-H", "-n", "1")
		if base.ExitCode != 0 || gobox.ExitCode != 0 {
			t.Fatalf("gobox iostat baseline failed base=%+v human=%+v", base, gobox)
		}
		if base.Stdout == gobox.Stdout {
			t.Fatalf("iostat -H did not change output\n--- base ---\n%s\n--- human ---\n%s", base.Stdout, gobox.Stdout)
		}
		header, rows := iostatHeaderAndRows(gobox.Stdout)
		if len(rows) == 0 || len(header) < 2 || header[0] != "Device" {
			t.Fatalf("iostat -H missing header or rows: %+v", gobox)
		}
		if !strings.Contains(strings.Join(header, " "), "/s") {
			t.Fatalf("iostat -H missing per-second units: %+v", gobox)
		}
		for _, row := range rows {
			if len(row) != len(header) {
				t.Fatalf("iostat -H row width mismatch row=%v header=%v", row, header)
			}
			for _, field := range row[1:] {
				if !isIostatRateField(field) {
					t.Fatalf("iostat -H should emit human-readable rate fields, got %q in row %v", field, row)
				}
			}
		}
		// At least one field across all rows must carry a scaled unit suffix
		// (K/s, M/s or G/s) confirming that -H actually scales the values.
		hasHumanUnit := false
		for _, row := range rows {
			for _, field := range row[1:] {
				if strings.HasSuffix(field, "K/s") || strings.HasSuffix(field, "M/s") || strings.HasSuffix(field, "G/s") {
					hasHumanUnit = true
				}
			}
		}
		if !hasHumanUnit {
			t.Fatalf("iostat -H: no field in any row has a K/s, M/s or G/s unit suffix; output:\n%s", gobox.Stdout)
		}
	})

	t.Run("IOSTAT-004", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "iostat", "-z", "-n", "1")
		native := runNativeCLI(t, t.TempDir(), "", "iostat", "-z", "1", "1")
		assertIostatStructuredParity(t, gobox, native)
		// Verifying that -z actually suppresses zero-I/O devices requires a
		// reliable zero-I/O device that cannot be guaranteed in CI.
		t.Skip("requires reliable zero-I/O device to verify -z suppresses zero-activity devices")
	})

	t.Run("IOSTAT-005", func(t *testing.T) {
		if _, err := os.Stat("/sys/fs/cgroup/io.stat"); err != nil {
			if _, err := os.Stat("/sys/fs/cgroup/blkio/blkio.throttle.io_service_bytes"); err != nil {
				if _, err := os.Stat("/sys/fs/cgroup/blkio/blkio.io_service_bytes"); err != nil {
					t.Skip("no cgroup io stats available")
				}
			}
		}
		base := runGoboxCLI(t, t.TempDir(), "", "iostat", "-n", "1")
		res := runGoboxCLI(t, t.TempDir(), "", "iostat", "--cgroup", "-n", "1")
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("iostat --cgroup failed base=%+v cgroup=%+v", base, res)
		}
		header, rows := iostatHeaderAndRows(res.Stdout)
		if len(rows) == 0 || len(header) < 2 || header[0] != "Device" {
			t.Fatalf("iostat --cgroup missing header: %+v", res)
		}
		if base.Stdout == res.Stdout {
			t.Fatalf("iostat --cgroup did not change output relative to diskstats baseline\n--- base ---\n%s\n--- cgroup ---\n%s", base.Stdout, res.Stdout)
		}
		for _, row := range rows {
			if len(row) != len(header) {
				t.Fatalf("iostat --cgroup expected structured device row width=%d header=%d row=%v", len(row), len(header), row)
			}
		}
		// Validate that all cgroup numeric fields are non-negative numbers.
		for _, row := range rows {
			for _, field := range row[1:] {
				s := strings.TrimSuffix(field, "/s")
				v, err := strconv.ParseFloat(s, 64)
				if err != nil {
					t.Fatalf("iostat --cgroup: non-parseable field %q in row %v", field, row)
				}
				if v < 0 {
					t.Fatalf("iostat --cgroup: negative I/O field %q (value %.4f) in row %v", field, v, row)
				}
			}
		}
	})

	t.Run("IOSTAT-006", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "iostat", "1", "1")
		native := runNativeCLI(t, t.TempDir(), "", "iostat", "1", "1")
		assertIostatStructuredParity(t, gobox, native)
	})

	t.Run("IOSTAT-007", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "iostat", "--help")
		if res.ExitCode != 0 {
			t.Fatalf("iostat --help failed: %+v", res)
		}
		out := res.Stdout + "\n" + res.Stderr
		lastIdx := -1
		for _, want := range []string{"Usage: gobox iostat", "Positionals:", "Columns:", "Examples:"} {
			idx := strings.Index(out, want)
			if idx == -1 {
				t.Fatalf("iostat --help missing %q\nstdout=%q\nstderr=%q", want, res.Stdout, res.Stderr)
			}
			if idx <= lastIdx {
				t.Fatalf("iostat --help section %q out of order\nstdout=%q\nstderr=%q", want, res.Stdout, res.Stderr)
			}
			lastIdx = idx
		}
		if strings.Contains(res.Stdout, "  -H\t") || strings.Contains(res.Stdout, "  --cgroup\t") {
			t.Fatalf("iostat --help should use grouped help text, got %q", res.Stdout)
		}
	})

	t.Run("IOSTAT-008", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "iostat", "-n", "0")
		if res.ExitCode == 0 {
			t.Fatalf("iostat -n 0 should fail, got exit 0: %+v", res)
		}
	})

	t.Run("IOSTAT-009", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "iostat", "abc")
		if res.ExitCode == 0 {
			t.Fatalf("iostat abc should fail, got exit 0: %+v", res)
		}
	})

	t.Run("IOSTAT-010", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "iostat", "1", "2", "3")
		if res.ExitCode == 0 {
			t.Fatalf("iostat 1 2 3 should fail, got exit 0: %+v", res)
		}
	})

	t.Run("IOSTAT-011", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "iostat", "-n", "2")
		if res.ExitCode != 0 {
			t.Fatalf("iostat -n 2 failed: %+v", res)
		}
		// Two samples produce two Device header lines separated by a blank line.
		count := strings.Count(res.Stdout, "Device")
		if count < 2 {
			t.Fatalf("iostat -n 2 should produce 2 output blocks (>=2 Device headers), got %d:\n%s", count, res.Stdout)
		}
	})
}

func TestParity_IoperfCases(t *testing.T) {
	if runtime.GOOS == "linux" {
		for _, tc := range []struct {
			id    string
			args  []string
			check func(t *testing.T, env string, args []string, res parityResult)
		}{
			{"IOPERF-001", []string{"ioperf", "--filename", "io.dat", "--bs", "4k", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-002", []string{"ioperf", "--filename", "io.dat", "--direct", "0", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-003", []string{"ioperf", "--filename", "io.dat", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-004", []string{"ioperf", "--filename", "io.dat", "--fsync", "1", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-005", []string{"ioperf", "--filename", "io.dat", "--group_reporting", "--numjobs", "2", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-007", []string{"ioperf", "--filename", "io.dat", "--write_hist_log", "hist", "--time_based", "--runtime", "1", "--size", "1M"},
				func(t *testing.T, env string, args []string, res parityResult) {
					// Histogram log name: {prefix}_read_hist.1.log (default rw=read).
					logPath := filepath.Join(env, "hist_read_hist.1.log")
					info, err := os.Stat(logPath)
					if err != nil {
						t.Fatalf("IOPERF-007: histogram log missing at %s: %v", logPath, err)
					}
					if info.Size() == 0 {
						t.Fatalf("IOPERF-007: histogram log is empty: %s", logPath)
					}
					data, err := os.ReadFile(logPath)
					if err != nil {
						t.Fatalf("IOPERF-007: read histogram log: %v", err)
					}
					for _, line := range strings.Split(string(data), "\n") {
						parts := strings.Split(strings.TrimSpace(line), ",")
						if len(parts) >= 3 {
							return // found a valid mode,bucket,count CSV line
						}
					}
					t.Fatalf("IOPERF-007: histogram log missing mode,bucket,count CSV lines:\n%s", data)
				}},
			{"IOPERF-008", []string{"ioperf", "--filename", "io.dat", "--numjobs", "2", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-009", []string{"ioperf", "--filename", "io.dat", "--percentile_list", "95", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-010", []string{"ioperf", "--filename", "io.dat", "--rate", "1M", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-011", []string{"ioperf", "--filename", "io.dat", "--runtime", "1", "--size", "32K"}, nil},
			{"IOPERF-012", []string{"ioperf", "--filename", "io.dat", "--rw", "read", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-013", []string{"ioperf", "--filename", "io.dat", "--rw", "readwrite", "--rwmixread", "70", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-014", []string{"ioperf", "--filename", "io.dat", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-015", []string{"ioperf", "--filename", "io.dat", "--sync", "sync", "--size", "32K", "--runtime", "1"}, nil},
			// IOPERF-016 was a duplicate of IOPERF-011; now tests randread (distinct scenario).
			{"IOPERF-016", []string{"ioperf", "--filename", "io.dat", "--rw", "randread", "--size", "32K", "--runtime", "1"}, nil},
			{"IOPERF-017", []string{"ioperf", "--filename", "io.dat", "--rw", "randwrite", "--size", "32K", "--runtime", "1"}, nil},
		} {
			t.Run(tc.id, func(t *testing.T) {
				env := t.TempDir()
				args := append([]string(nil), tc.args...)
				for i := range args {
					if args[i] == "io.dat" {
						args[i] = filepath.Join(env, "io.dat")
					}
					if args[i] == "hist" {
						args[i] = filepath.Join(env, "hist")
					}
				}
				res := runGoboxCLI(t, env, "", args...)
				if res.ExitCode != 0 {
					t.Fatalf("%s failed: %+v", tc.id, res)
				}
				// Content check: verify READ:/WRITE: summary lines based on --rw mode.
				rwMode := "read" // default
				for i, a := range args {
					if a == "--rw" && i+1 < len(args) {
						rwMode = args[i+1]
						break
					}
				}
				switch rwMode {
				case "read", "randread":
					if findLineWithPrefix(res.Stdout, "READ:") == "" {
						t.Fatalf("%s missing READ: line in stdout: %+v", tc.id, res)
					}
				case "write", "randwrite":
					if findLineWithPrefix(res.Stdout, "WRITE:") == "" {
						t.Fatalf("%s missing WRITE: line in stdout: %+v", tc.id, res)
					}
				case "readwrite":
					if findLineWithPrefix(res.Stdout, "READ:") == "" || findLineWithPrefix(res.Stdout, "WRITE:") == "" {
						t.Fatalf("%s missing READ:/WRITE: lines in stdout: %+v", tc.id, res)
					}
				}
				if tc.check != nil {
					tc.check(t, env, args, res)
				}
			})
		}
	}
}

func TestParity_IoperfFioCases(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	requireNativeCommand(t, "fio")

	type ioperfParityCase struct {
		id        string
		setup     func(t *testing.T, goboxFile, nativeFile string)
		goboxArgs func(env, goboxFile string) []string
		fioArgs   func(env, nativeFile string) []string
		assert    func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult)
	}

	assertReadWrite := func(t *testing.T, gobox, native parityResult, wantRead, wantWrite bool) {
		t.Helper()
		if wantRead && (findLineWithPrefix(gobox.Stdout, "READ:") == "" || findLineWithPrefix(strings.ToUpper(native.Stdout), "READ:") == "") {
			t.Fatalf("missing read stats gobox=%+v native=%+v", gobox, native)
		}
		if wantWrite && (findLineWithPrefix(gobox.Stdout, "WRITE:") == "" || findLineWithPrefix(strings.ToUpper(native.Stdout), "WRITE:") == "") {
			t.Fatalf("missing write stats gobox=%+v native=%+v", gobox, native)
		}
	}

	for _, tc := range []ioperfParityCase{
		{
			id: "IOPERF-001",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--bs", "4k", "--size", "32K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--bs=4k", "--size=32K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, false, true)
				if findLineContaining(strings.ToLower(gobox.Stdout), "bs=4k") == "" || findLineContaining(strings.ToLower(native.Stdout), "4096b-4096b") == "" {
					t.Fatalf("block size not reflected gobox=%+v native=%+v", gobox, native)
				}
			},
		},
		{
			id: "IOPERF-002",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--direct", "0", "--size", "32K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--direct=0", "--size=32K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, false, true)
			},
		},
		{
			id: "IOPERF-003",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--size", "32K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--size=32K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, false, true)
				if _, err := os.Stat(goboxFile); err != nil {
					t.Fatalf("gobox filename not created at exact path %s: %v", goboxFile, err)
				}
				if _, err := os.Stat(nativeFile); err != nil {
					t.Fatalf("fio filename not created at exact path %s: %v", nativeFile, err)
				}
			},
		},
		{
			id: "IOPERF-004",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--fsync", "1", "--size", "32K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--fsync=1", "--size=32K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, false, true)
			},
		},
		{
			id: "IOPERF-005",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--group_reporting", "--numjobs", "2", "--size", "32K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--group_reporting=1", "--numjobs=2", "--size=32K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				if findLineWithPrefix(gobox.Stdout, "job 0:") != "" {
					t.Fatalf("gobox group reporting should aggregate output: %+v", gobox)
				}
				if findLineWithPrefix(native.Stdout, "Run status group 0") == "" {
					t.Fatalf("fio group reporting missing group summary: %+v", native)
				}
			},
		},
		{
			id: "IOPERF-006",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--iodepth", "2", "--size", "64K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--iodepth=2", "--size=64K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, false, true)
				if findLineContaining(gobox.Stdout, "iodepth=2") == "" || findLineWithPrefix(strings.TrimSpace(native.Stdout), "IO depths") == "" {
					t.Fatalf("iodepth not reflected gobox=%+v native=%+v", gobox, native)
				}
			},
		},
		{
			id: "IOPERF-007",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--size", "1M", "--time_based", "--runtime", "1", "--write_hist_log", filepath.Join(env, "gobox_hist")}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--size=1M", "--time_based=1", "--runtime=1", "--write_hist_log=" + filepath.Join(env, "native_hist"), "--log_hist_msec=100"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				if _, err := os.Stat(filepath.Join(env, "gobox_hist_write_hist.1.log")); err != nil {
					t.Fatalf("gobox histogram log missing: %v", err)
				}
				if _, err := os.Stat(filepath.Join(env, "native_hist_clat_hist.1.log")); err != nil {
					t.Fatalf("fio histogram log missing: %v", err)
				}
			},
		},
		{
			id: "IOPERF-008",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--numjobs", "2", "--size", "32K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--numjobs=2", "--size=32K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				if findLineWithPrefix(gobox.Stdout, "job 0:") == "" || findLineWithPrefix(gobox.Stdout, "job 1:") == "" {
					t.Fatalf("gobox numjobs output missing per-job sections: %+v", gobox)
				}
				if findLineWithPrefix(native.Stdout, "Starting 2 processes") == "" {
					t.Fatalf("fio numjobs output missing job count: %+v", native)
				}
			},
		},
		{
			id: "IOPERF-009",
			setup: func(t *testing.T, goboxFile, nativeFile string) {
				writeFile(t, goboxFile, strings.Repeat("p", 32*1024))
				writeFile(t, nativeFile, strings.Repeat("p", 32*1024))
			},
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "read", "--size", "32K", "--percentile_list", "95"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=read", "--size=32K", "--percentile_list=95"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				if findLineContaining(strings.ToLower(gobox.Stdout), "p95=") == "" || findLineContaining(strings.ToLower(native.Stdout), "95th=") == "" {
					t.Fatalf("percentile output mismatch gobox=%+v native=%+v", gobox, native)
				}
			},
		},
		{
			id: "IOPERF-010",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--rate", "1M", "--size", "1M", "--time_based", "--runtime", "1"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--rate=1M", "--size=1M", "--time_based=1", "--runtime=1"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, false, true)
			},
		},
		{
			id: "IOPERF-011",
			setup: func(t *testing.T, goboxFile, nativeFile string) {
				writeFile(t, goboxFile, strings.Repeat("r", 32*1024))
				writeFile(t, nativeFile, strings.Repeat("r", 32*1024))
			},
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "read", "--size", "32K", "--time_based", "--runtime", "1"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=read", "--size=32K", "--time_based=1", "--runtime=1"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, true, false)
			},
		},
		{
			id: "IOPERF-012",
			setup: func(t *testing.T, goboxFile, nativeFile string) {
				writeFile(t, goboxFile, strings.Repeat("a", 32*1024))
				writeFile(t, nativeFile, strings.Repeat("a", 32*1024))
			},
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "read", "--size", "32K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=read", "--size=32K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, true, false)
			},
		},
		{
			id: "IOPERF-013",
			setup: func(t *testing.T, goboxFile, nativeFile string) {
				writeFile(t, goboxFile, strings.Repeat("b", 64*1024))
				writeFile(t, nativeFile, strings.Repeat("b", 64*1024))
			},
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "readwrite", "--rwmixread", "70", "--bs", "4k", "--size", "64K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=readwrite", "--rwmixread=70", "--bs=4k", "--size=64K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, true, true)
			},
		},
		{
			id: "IOPERF-014",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--size", "32K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--size=32K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				goboxInfo, err := os.Stat(goboxFile)
				if err != nil {
					t.Fatalf("stat gobox file: %v", err)
				}
				nativeInfo, err := os.Stat(nativeFile)
				if err != nil {
					t.Fatalf("stat fio file: %v", err)
				}
				if goboxInfo.Size() != 32*1024 || nativeInfo.Size() != 32*1024 {
					t.Fatalf("size not exactly 32K: gobox=%d native=%d", goboxInfo.Size(), nativeInfo.Size())
				}
			},
		},
		{
			id: "IOPERF-015",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--sync", "sync", "--size", "32K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--sync=sync", "--size=32K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, false, true)
			},
		},
		{
			// IOPERF-016 was a duplicate of IOPERF-011 (read + time_based).
			// Now tests randread — a genuinely different I/O pattern.
			id: "IOPERF-016",
			setup: func(t *testing.T, goboxFile, nativeFile string) {
				writeFile(t, goboxFile, strings.Repeat("c", 32*1024))
				writeFile(t, nativeFile, strings.Repeat("c", 32*1024))
			},
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "randread", "--size", "32K", "--time_based", "--runtime", "1"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=randread", "--size=32K", "--time_based=1", "--runtime=1"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, true, false)
			},
		},
		{
			id: "IOPERF-017",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "randwrite", "--size", "32K"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=randwrite", "--size=32K"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, false, true)
			},
		},
	} {
		t.Run(tc.id, func(t *testing.T) {
			env := t.TempDir()
			goboxFile := filepath.Join(env, "gobox.dat")
			nativeFile := filepath.Join(env, "native.dat")
			if tc.setup != nil {
				tc.setup(t, goboxFile, nativeFile)
			}

			gobox := runGoboxCLI(t, env, "", tc.goboxArgs(env, goboxFile)...)
			native := runNativeCLI(t, env, "", "fio", append([]string{"--name=job"}, tc.fioArgs(env, nativeFile)...)...)
			if gobox.ExitCode != 0 || native.ExitCode != 0 {
				t.Fatalf("ioperf/fio parity failed gobox=%+v native=%+v", gobox, native)
			}
			tc.assert(t, env, goboxFile, nativeFile, gobox, native)
		})
	}

	t.Run("IOPERF-006", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "io.dat"), "--size", "64K", "--runtime", "1", "--time_based", "--iodepth", "2")
		if res.ExitCode != 0 {
			t.Fatalf("ioperf failed: %+v", res)
		}
	})

	t.Run("IOPERF-latency", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "io.dat"),
			"--rw", "write", "--size", "32K", "--latency")
		if res.ExitCode != 0 {
			t.Fatalf("ioperf --latency failed: %+v", res)
		}
		if findLineContaining(res.Stdout, "Latency histogram") == "" && findLineContaining(res.Stdout, "latency distribution") == "" {
			t.Fatalf("ioperf --latency missing histogram output in stdout: %+v", res)
		}
	})

	t.Run("IOPERF-err-invalid-rw", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "io.dat"),
			"--rw", "badmode", "--size", "32K")
		if res.ExitCode == 0 {
			t.Fatalf("ioperf with invalid --rw should fail, got exit 0: %+v", res)
		}
	})

	t.Run("IOPERF-err-rwmixread-without-readwrite", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "io.dat"),
			"--rw", "read", "--rwmixread", "70", "--size", "32K")
		if res.ExitCode == 0 {
			t.Fatalf("ioperf --rwmixread without readwrite mode should fail, got exit 0: %+v", res)
		}
	})
}

func TestParity_Sha256sumCases(t *testing.T) {
	runExactParityCases(t, []parityCase{
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
			ID:            "SHA256-002",
			Name:          "sha256sum --check",
			GoboxArgs:     []string{"sha256sum", "--check", "checksums.sha256"},
			NativeCommand: "sha256sum",
			NativeArgs:    []string{"--check", "checksums.sha256"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "hello")
				res := runNativeCLI(t, env.Dir, "", "sha256sum", "input.txt")
				writeFile(t, filepath.Join(env.Dir, "checksums.sha256"), normalizeText(res.Stdout)+"\n")
			},
			Normalize: normalizeText,
		},
		{
			ID:            "SHA256-003",
			Name:          "sha256sum --tag",
			GoboxArgs:     []string{"sha256sum", "--tag", "input.txt"},
			NativeCommand: "sha256sum",
			NativeArgs:    []string{"--tag", "input.txt"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "hello")
			},
		},
		{
			ID:            "SHA256-004",
			Name:          "sha256sum --quiet",
			GoboxArgs:     []string{"sha256sum", "--quiet", "--check", "checksums.sha256"},
			NativeCommand: "sha256sum",
			NativeArgs:    []string{"--quiet", "--check", "checksums.sha256"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "hello")
				res := runNativeCLI(t, env.Dir, "", "sha256sum", "input.txt")
				writeFile(t, filepath.Join(env.Dir, "checksums.sha256"), normalizeText(res.Stdout)+"\n")
			},
			Normalize: normalizeText,
		},
		{
			ID:            "SHA256-005",
			Name:          "sha256sum --status",
			GoboxArgs:     []string{"sha256sum", "--status", "--check", "checksums.sha256"},
			NativeCommand: "sha256sum",
			NativeArgs:    []string{"--status", "--check", "checksums.sha256"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "input.txt"), "hello")
				res := runNativeCLI(t, env.Dir, "", "sha256sum", "input.txt")
				writeFile(t, filepath.Join(env.Dir, "checksums.sha256"), normalizeText(res.Stdout)+"\n")
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("sha256sum --status exit mismatch %d != %d", gobox.ExitCode, native.ExitCode)
				}
				if gobox.Stdout != "" {
					t.Fatalf("sha256sum --status should produce no stdout, got: %q", gobox.Stdout)
				}
				if gobox.Stderr != "" {
					t.Fatalf("sha256sum --status should produce no stderr, got: %q", gobox.Stderr)
				}
			},
		},
	})

	t.Run("SHA256-006", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "checksums.sha256"), "bad line\n")
		gobox := runGoboxCLI(t, env, "", "sha256sum", "--warn", "--check", "checksums.sha256")
		native := runNativeCLI(t, env, "", "sha256sum", "--warn", "--check", "checksums.sha256")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("sha256sum --warn exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		// Warning must be on stderr, not stdout.
		if gobox.Stdout != "" {
			t.Fatalf("sha256sum --warn: warning should be on stderr only, got stdout: %q", gobox.Stdout)
		}
		if findLineContaining(strings.ToLower(gobox.Stderr), "improperly formatted") == "" {
			t.Fatalf("sha256sum --warn missing gobox warning on stderr: %+v", gobox)
		}
		if findLineContaining(strings.ToLower(native.Stdout+native.Stderr), "improperly formatted") == "" {
			t.Fatalf("sha256sum --warn missing native warning: %+v", native)
		}
	})

	t.Run("SHA256-failed", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "data.txt"), "hello")
		sum := runNativeCLI(t, env, "", "sha256sum", "data.txt")
		writeFile(t, filepath.Join(env, "checksums.sha256"), normalizeText(sum.Stdout)+"\n")
		// Tamper with the file after generating the checksum.
		writeFile(t, filepath.Join(env, "data.txt"), "TAMPERED")
		gobox := runGoboxCLI(t, env, "", "sha256sum", "--check", "checksums.sha256")
		if gobox.ExitCode == 0 {
			t.Fatalf("sha256sum --check with tampered file should fail, got exit 0: %+v", gobox)
		}
		if findLineContaining(gobox.Stdout, "FAILED") == "" {
			t.Fatalf("sha256sum --check with tampered file: missing FAILED in stdout: %+v", gobox)
		}
		native := runNativeCLI(t, env, "", "sha256sum", "--check", "checksums.sha256")
		if native.ExitCode == 0 {
			t.Fatalf("native sha256sum --check with tampered file should fail")
		}
	})

	t.Run("SHA256-tag-check", func(t *testing.T) {
		// Generate checksum in BSD tag format, then verify --check can read it back.
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "input.txt"), "hello")
		tag := runGoboxCLI(t, env, "", "sha256sum", "--tag", "input.txt")
		if tag.ExitCode != 0 {
			t.Fatalf("sha256sum --tag failed: %+v", tag)
		}
		writeFile(t, filepath.Join(env, "checksums.sha256"), normalizeText(tag.Stdout)+"\n")
		check := runGoboxCLI(t, env, "", "sha256sum", "--check", "checksums.sha256")
		if check.ExitCode != 0 {
			t.Fatalf("sha256sum --check of BSD tag output failed: %+v", check)
		}
		if findLineContaining(check.Stdout, "OK") == "" {
			t.Fatalf("sha256sum --check of BSD tag: missing OK in stdout: %+v", check)
		}
	})
}

func assertIostatStructuredParity(t *testing.T, gobox, native parityResult) {
	t.Helper()
	if gobox.ExitCode != 0 {
		t.Fatalf("gobox iostat failed: %+v", gobox)
	}
	if native.ExitCode != 0 {
		t.Fatalf("native iostat failed: %+v", native)
	}
	goboxHeader, goboxRows := iostatHeaderAndRows(gobox.Stdout)
	nativeHeader, nativeRows := iostatHeaderAndRows(native.Stdout)
	if len(goboxHeader) == 0 || len(nativeHeader) == 0 || goboxHeader[0] != "Device" || nativeHeader[0] != "Device" {
		t.Fatalf("iostat header missing\ngobox=%q\nnative=%q", gobox.Stdout, native.Stdout)
	}
	if len(goboxRows) == 0 {
		t.Fatalf("gobox iostat produced no device rows: %+v", gobox)
	}
	if len(nativeRows) == 0 {
		t.Fatalf("native iostat produced no device rows: %+v", native)
	}
	if len(goboxHeader) < 4 || len(nativeHeader) < 4 {
		t.Fatalf("iostat header too short\ngobox=%v\nnative=%v", goboxHeader, nativeHeader)
	}
	for _, row := range goboxRows {
		if len(row) != len(goboxHeader) {
			t.Fatalf("gobox iostat row width mismatch row=%v header=%v", row, goboxHeader)
		}
	}
	for _, row := range nativeRows {
		if len(row) != len(nativeHeader) {
			t.Fatalf("native iostat row width mismatch row=%v header=%v", row, nativeHeader)
		}
	}
	goboxDevices := iostatDeviceSet(gobox.Stdout)
	nativeDevices := iostatDeviceSet(native.Stdout)
	if !hasSetIntersection(goboxDevices, nativeDevices) {
		t.Fatalf("iostat device sets do not overlap\ngobox=%v\nnative=%v", goboxDevices, nativeDevices)
	}
	if len(iostatCommonDeviceRows(goboxRows, nativeRows)) == 0 {
		t.Fatalf("iostat common-device structured comparison found no shared rows\ngobox=%v\nnative=%v", goboxRows, nativeRows)
	}
	// Validate that all gobox numeric fields are parseable and non-negative.
	for _, row := range goboxRows {
		for _, field := range row[1:] {
			s := strings.TrimSuffix(field, "/s")
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				t.Fatalf("gobox iostat: non-parseable numeric field %q in row %v", field, row)
			}
			if v < 0 {
				t.Fatalf("gobox iostat: negative field %q (value %.4f) in row %v", field, v, row)
			}
		}
	}
}

func iostatDeviceSet(out string) map[string]struct{} {
	devices := make(map[string]struct{})
	for _, line := range strings.Split(normalizeText(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == "Device" {
			continue
		}
		devices[fields[0]] = struct{}{}
	}
	return devices
}

func hasSetIntersection(left, right map[string]struct{}) bool {
	for key := range left {
		if _, ok := right[key]; ok {
			return true
		}
	}
	return false
}

func findLineContaining(out, needle string) string {
	for _, line := range nonEmptyLines(out) {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func TestParity_Md5InternalSanity(t *testing.T) {
	h := md5.Sum([]byte("hello"))
	got := fmt.Sprintf("%x", h[:])
	// Known-correct MD5 of the ASCII string "hello".
	const want = "5d41402abc4b2a76b9719d911017c592"
	if got != want {
		t.Fatalf("md5 sanity: expected %s, got %s", want, got)
	}
}
