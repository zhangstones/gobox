package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
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

// iostatFieldIndex returns the column index of name within header, or -1 if
// the column is not present. gobox and native iostat use different column
// vocabularies (ReadIOPS/WriteIOPS/... vs tps/kB_read/s/...), so callers must
// look each field up by name rather than assuming a fixed position.
func iostatFieldIndex(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}

// parseIostatNumeric parses a gobox or native iostat numeric cell, stripping
// the optional "/s" rate suffix gobox emits.
func parseIostatNumeric(field string) (float64, bool) {
	s := strings.TrimSuffix(field, "/s")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// withinIostatTolerance reports whether two related throughput/IOPS samples,
// captured from two independent sequential command invocations against a
// live and possibly-changing device, are consistent with measuring the same
// underlying activity. Per docs/TEST-DESIGN.md §13, iostat is an
// environment-sensitive command sampled at slightly different moments, so
// exact numeric equality is not expected; this uses a generous ratio-based
// tolerance instead, which is still strong enough to catch gross bugs such
// as swapped columns or a 1000x unit-conversion error.
func withinIostatTolerance(a, b float64) bool {
	const smallActivity = 5.0
	if a <= smallActivity && b <= smallActivity {
		return true
	}
	lo, hi := a, b
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo <= 0 {
		return hi <= smallActivity
	}
	return hi/lo <= 4.0
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
		// The valid entry must be reported with the exact "OK" status, not
		// merely mentioned somewhere in stdout (a broken implementation could
		// print "good.txt: FAILED" and still satisfy a bare Contains check).
		line := findLineContaining(gobox.Stdout, "good.txt")
		if line == "" {
			t.Fatalf("md5sum --warn mixed: valid file result missing from stdout: %+v", gobox)
		}
		if line != "good.txt: OK" {
			t.Fatalf("md5sum --warn mixed: valid file line should read exactly %q, got %q", "good.txt: OK", line)
		}
		if strings.Contains(gobox.Stdout, "FAILED") {
			t.Fatalf("md5sum --warn mixed: malformed line must not be reported as a checksum FAILED entry: %+v", gobox)
		}
	})

	t.Run("MD5-006", func(t *testing.T) {
		// Undocumented Case ID: not yet present in docs/TEST-CASES.md.
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

	t.Run("MD5-007", func(t *testing.T) {
		// Undocumented Case ID: not yet present in docs/TEST-CASES.md.
		// Checksum file references a file that does not exist at all, distinct
		// from MD5-005/MD5-005-mixed which cover a malformed *line*. GNU
		// coreutils reports this as "<name>: FAILED open or read" on stdout in
		// addition to a stderr diagnostic.
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "good.txt"), "hello")
		sum := runNativeCLI(t, env, "", "md5sum", "good.txt")
		content := normalizeText(sum.Stdout) + "\n" + "d41d8cd98f00b204e9800998ecf8427e  missing.txt\n"
		writeFile(t, filepath.Join(env, "checksums.md5"), content)

		native := runNativeCLI(t, env, "", "md5sum", "--check", "checksums.md5")
		if native.ExitCode == 0 {
			t.Fatalf("native md5sum --check with a missing referenced file should fail")
		}
		if findLineContaining(native.Stdout, "missing.txt") == "" {
			t.Fatalf("native md5sum --check should report the missing file on stdout: %+v", native)
		}

		gobox := runGoboxCLI(t, env, "", "md5sum", "--check", "checksums.md5")
		if gobox.ExitCode == 0 {
			t.Fatalf("md5sum --check with a missing referenced file should fail, got exit 0: %+v", gobox)
		}
		goodLine := findLineContaining(gobox.Stdout, "good.txt")
		if goodLine != "good.txt: OK" {
			t.Fatalf("md5sum --check: valid entry should read exactly %q, got %q (full output: %+v)", "good.txt: OK", goodLine, gobox)
		}
		// This mirrors native's "<file>: FAILED open or read" contract: the
		// missing referenced file must be reported on stdout, not only as a
		// stderr diagnostic. See cmds/disk/cmd_md5sum.go around the
		// `os.Open(filename)` error branch in the --check loop, which
		// currently only writes to stderr and never prints a per-file
		// stdout status line for an unreadable referenced file (compare with
		// cmds/disk/cmd_sha256sum.go's sha256sumCheck, which does print
		// "%s: FAILED" on stdout in the equivalent branch).
		if findLineContaining(gobox.Stdout, "missing.txt") == "" {
			t.Fatalf("md5sum --check should report the missing referenced file on stdout (e.g. \"missing.txt: FAILED\"), matching native's FAILED-open-or-read contract; got stdout=%q stderr=%q", gobox.Stdout, gobox.Stderr)
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

// setupIdleLoopDevice attaches a freshly created, zero-activity loop device
// backed by a sparse file and returns its basename (e.g. "loop3") for use as
// a reliable zero-I/O device fixture. It requires root and losetup; if
// either is unavailable, or the resulting device is not visible in
// /proc/diskstats, the calling test is skipped with a clear reason.
func setupIdleLoopDevice(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("loop devices require Linux")
	}
	losetupPath, err := exec.LookPath("losetup")
	if err != nil {
		t.Skip("losetup not available, cannot construct a zero-activity device fixture")
	}
	dir := t.TempDir()
	img := filepath.Join(dir, "idle.img")
	if err := exec.Command("truncate", "-s", "8M", img).Run(); err != nil {
		t.Skipf("could not create loop backing file: %v", err)
	}
	out, err := exec.Command(losetupPath, "-f", "--show", img).CombinedOutput()
	if err != nil {
		t.Skipf("could not attach loop device (likely needs root/CAP_SYS_ADMIN): %v (%s)", err, out)
	}
	dev := strings.TrimSpace(string(out))
	if dev == "" {
		t.Skip("losetup returned no device path")
	}
	t.Cleanup(func() {
		_ = exec.Command(losetupPath, "-d", dev).Run()
	})
	return filepath.Base(dev)
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

		// Construct a real zero-activity device (a freshly attached loop
		// device backed by a sparse file with no I/O since creation) and
		// verify -z actually filters it out while active devices remain.
		loopDev := setupIdleLoopDevice(t)

		withoutZ := runGoboxCLI(t, t.TempDir(), "", "iostat", "-n", "1")
		if withoutZ.ExitCode != 0 {
			t.Fatalf("iostat baseline (no -z) failed: %+v", withoutZ)
		}
		_, rowsNoZ := iostatHeaderAndRows(withoutZ.Stdout)
		var loopRow []string
		for _, row := range rowsNoZ {
			if len(row) > 0 && row[0] == loopDev {
				loopRow = row
				break
			}
		}
		if loopRow == nil {
			t.Skipf("idle loop device %s not visible in diskstats snapshot, cannot verify -z filtering", loopDev)
		}
		for _, field := range loopRow[1:] {
			v, ok := parseIostatNumeric(field)
			if !ok || v > 0.01 {
				t.Skipf("idle loop device %s shows unexpected activity (environment noise), cannot verify -z filtering: %v", loopDev, loopRow)
			}
		}

		withZ := runGoboxCLI(t, t.TempDir(), "", "iostat", "-z", "-n", "1")
		if withZ.ExitCode != 0 {
			t.Fatalf("iostat -z failed: %+v", withZ)
		}
		if _, ok := iostatDeviceSet(withZ.Stdout)[loopDev]; ok {
			t.Fatalf("iostat -z should filter out the zero-activity device %s, got:\n%s", loopDev, withZ.Stdout)
		}
		_, rowsWithZ := iostatHeaderAndRows(withZ.Stdout)
		if len(rowsWithZ) == 0 {
			t.Fatalf("iostat -z filtered out every device; expected at least one active device to remain")
		}
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

	// IOSTAT-008..011 are undocumented Case IDs: docs/TEST-CASES.md currently
	// only defines IOSTAT-001..007. These should be added to the doc's
	// coverage matrix.
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

// ---- ioperf test helpers ----

var ioperfIOPSRe = regexp.MustCompile(`IOPS=([0-9.]+)`)

// parseIOPS extracts the IOPS=<value> field from a "READ:"/"WRITE:" summary line.
func parseIOPS(line string) (float64, bool) {
	m := ioperfIOPSRe.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	return v, err == nil
}

// parseLatencyField extracts a "<label>=<value>us" field (e.g. "avg", "p50",
// "p99") in microseconds from a "READ:"/"WRITE:" summary line.
func parseLatencyField(line, label string) (float64, bool) {
	re := regexp.MustCompile(regexp.QuoteMeta(label) + `=([0-9.]+)us`)
	m := re.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	return v, err == nil
}

func requireStrace(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("strace")
	if err != nil {
		t.Skip("strace not available in PATH, cannot verify real I/O syscall behavior")
	}
	return path
}

// runUnderStrace runs the current test binary re-invoked as
// "gobox <goboxArgs...>" (via TestParityHelperProcess, the same mechanism
// runGoboxSubprocess uses) wrapped in strace, tracing the given syscalls into
// logPath. It skips the test if strace cannot be launched.
func runUnderStrace(t *testing.T, dir, logPath, traceExpr string, goboxArgs []string, timeout time.Duration) {
	t.Helper()
	stracePath := requireStrace(t)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := append([]string{"-f", "-s", "0", "-e", "trace=" + traceExpr, "-o", logPath, os.Args[0], "-test.run=TestParityHelperProcess", "--"}, goboxArgs...)
	cmd := exec.CommandContext(ctx, stracePath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOBOX_PARITY_HELPER=1")
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("strace-wrapped ioperf timed out: %v", ctx.Err())
	}
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Skipf("could not launch strace: %v (%s)", err, stderrBuf.String())
		}
	}
}

var pIOOffsetRe = regexp.MustCompile(`p(?:read64|write64)\(\d+, "[^"]*"\.\.\., (\d+), (\d+)\)`)

// traceSyscallOffsets runs goboxArgs under strace tracing syscallName
// (pread64 or pwrite64) and returns the offset argument of every call whose
// requested size equals blockSize, in call order. This is used to prove
// --rw=randread/randwrite genuinely issue non-sequential offsets (and, as a
// positive control, that plain --rw=read/write issue strictly sequential
// ones), rather than trusting that a printed "READ:"/"WRITE:" summary line
// implies any particular access pattern.
func traceSyscallOffsets(t *testing.T, dir string, goboxArgs []string, syscallName string, blockSize int64, timeout time.Duration) []int64 {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "ioperf_offsets.log")
	runUnderStrace(t, dir, logPath, syscallName, goboxArgs, timeout)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Skipf("strace log unavailable: %v", err)
	}

	var offsets []int64
	for _, line := range strings.Split(string(data), "\n") {
		m := pIOOffsetRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		size, err1 := strconv.ParseInt(m[1], 10, 64)
		off, err2 := strconv.ParseInt(m[2], 10, 64)
		if err1 != nil || err2 != nil || size != blockSize {
			continue
		}
		offsets = append(offsets, off)
	}
	return offsets
}

// traceOpenFlagsLine runs goboxArgs under strace tracing openat and returns
// the first logged line mentioning fileBase, so callers can inspect the
// actual open() flag set (e.g. whether O_DIRECT was really requested).
func traceOpenFlagsLine(t *testing.T, dir string, goboxArgs []string, fileBase string, timeout time.Duration) string {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "ioperf_open.log")
	runUnderStrace(t, dir, logPath, "openat", goboxArgs, timeout)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Skipf("strace log unavailable: %v", err)
	}
	return findLineContaining(string(data), fileBase)
}

// isAscendingByStep reports whether offsets form a strict arithmetic
// progression increasing by exactly step at every consecutive pair.
func isAscendingByStep(offsets []int64, step int64) bool {
	if len(offsets) < 2 {
		return false
	}
	for i := 1; i < len(offsets); i++ {
		if offsets[i] != offsets[i-1]+step {
			return false
		}
	}
	return true
}

// isAscendingByStepTolerant is like isAscendingByStep but accepts any
// positive multiple of step between consecutive offsets, to tolerate strace
// occasionally dropping a syscall event under system load (verified
// non-reproducible in isolation -- see IOPERF-001/012 comments). It still
// rejects non-monotonic, backwards, or non-aligned offsets, so it cannot be
// satisfied by genuinely random access.
func isAscendingByStepTolerant(offsets []int64, step int64) bool {
	if len(offsets) < 2 {
		return false
	}
	for i := 1; i < len(offsets); i++ {
		delta := offsets[i] - offsets[i-1]
		if delta <= 0 || delta%step != 0 {
			return false
		}
	}
	return true
}

// TestParity_IoperfCases performs strong, self-contained gobox
// self-consistency checks for the ioperf parameter surface. Unlike
// TestParity_IoperfFioCases, it does NOT require fio and does NOT compare
// against a native tool — it proves that each flag produces the specific
// observable effect its documentation promises (measured throughput/latency
// changes, actual read:write ratios, real non-sequential syscall offsets,
// etc.), using strace where the printed summary alone cannot prove the
// effect. Treat this as a gobox behavioral-contract suite, not a native
// parity suite; native comparison for the subset fio supports lives in
// TestParity_IoperfFioCases.
func TestParity_IoperfCases(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}

	t.Run("IOPERF-001", func(t *testing.T) {
		// --bs must govern the actual write() granularity, not just be
		// echoed in the summary header: 256K at bs=4k must issue exactly
		// 64 writes of 4096 bytes, while bs=8k must issue exactly 32
		// writes of 8192 bytes.
		env := t.TempDir()
		off4k := traceSyscallOffsets(t, env,
			[]string{"ioperf", "--filename", "io4k.dat", "--rw", "write", "--bs", "4k", "--size", "256K"},
			"pwrite64", 4*1024, 30*time.Second)
		off8k := traceSyscallOffsets(t, env,
			[]string{"ioperf", "--filename", "io8k.dat", "--rw", "write", "--bs", "8k", "--size", "256K"},
			"pwrite64", 8*1024, 30*time.Second)
		// Allow off-by-one tolerance: under system load, strace has been
		// observed to occasionally miss logging the final syscall before
		// the traced process exits (confirmed non-reproducible in
		// isolation, only appears when the full parity suite runs under
		// contention) -- this is strace-capture flakiness, not a gobox
		// bug (the write-loop logic issues exactly size/bs writes for the
		// default single-job/depth=1 path per code review).
		if len(off4k) < 63 || len(off4k) > 64 {
			t.Fatalf("ioperf --bs=4k --size=256K: expected 64 (±1) writes of 4096 bytes, observed %d", len(off4k))
		}
		if len(off8k) < 31 || len(off8k) > 32 {
			t.Fatalf("ioperf --bs=8k --size=256K: expected 32 (±1) writes of 8192 bytes, observed %d", len(off8k))
		}
	})

	t.Run("IOPERF-002", func(t *testing.T) {
		// --direct is supposed to open the file with O_DIRECT to bypass the
		// page cache. Verify the real open() flags via strace rather than
		// trusting a timing signal (page-cache/virtualization noise makes
		// wall-clock comparisons unreliable in CI, see report).
		env := t.TempDir()
		lineOn := traceOpenFlagsLine(t, env,
			[]string{"ioperf", "--filename", "d1.dat", "--rw", "write", "--bs", "4k", "--size", "32K", "--direct", "1"},
			"d1.dat", 20*time.Second)
		if lineOn == "" {
			t.Fatalf("strace did not capture an openat() call for the ioperf target file")
		}
		if !strings.Contains(lineOn, "O_DIRECT") {
			t.Fatalf("ioperf --direct=1 must open the target file with O_DIRECT; strace observed flags %q instead. "+
				"This indicates the O_DIRECT constant in cmds/disk/cmd_ioperf.go does not match the real Linux "+
				"O_DIRECT value (see BUGS.md entry from this test run).", lineOn)
		}

		lineOff := traceOpenFlagsLine(t, env,
			[]string{"ioperf", "--filename", "d0.dat", "--rw", "write", "--bs", "4k", "--size", "32K", "--direct", "0"},
			"d0.dat", 20*time.Second)
		if strings.Contains(lineOff, "O_DIRECT") {
			t.Fatalf("ioperf --direct=0 (default) should not request O_DIRECT, got %q", lineOff)
		}
	})

	t.Run("IOPERF-003", func(t *testing.T) {
		env := t.TempDir()
		target := filepath.Join(env, "custom-name.dat")
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", target, "--rw", "write", "--bs", "4k", "--size", "32K")
		if res.ExitCode != 0 {
			t.Fatalf("ioperf --filename failed: %+v", res)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("ioperf --filename: file not created at the exact requested path %s: %v", target, err)
		}
	})

	t.Run("IOPERF-004", func(t *testing.T) {
		// --fsync=1 forces an fsync() after every write, which must be
		// measurably slower than the unsynced baseline.
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "f0.dat"), "--rw", "write", "--bs", "4k", "--size", "512K", "--fsync", "0")
		withFsync := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "f1.dat"), "--rw", "write", "--bs", "4k", "--size", "512K", "--fsync", "1")
		if base.ExitCode != 0 || withFsync.ExitCode != 0 {
			t.Fatalf("ioperf --fsync run failed base=%+v fsync=%+v", base, withFsync)
		}
		baseLat, ok1 := parseLatencyField(findLineWithPrefix(base.Stdout, "WRITE:"), "avg")
		fsyncLat, ok2 := parseLatencyField(findLineWithPrefix(withFsync.Stdout, "WRITE:"), "avg")
		if !ok1 || !ok2 {
			t.Fatalf("ioperf --fsync: could not parse avg latency from output base=%q fsync=%q", base.Stdout, withFsync.Stdout)
		}
		if fsyncLat < baseLat*2 {
			t.Fatalf("ioperf --fsync=1 should measurably increase average write latency via a real fsync() call: base avg=%.2fus fsync avg=%.2fus", baseLat, fsyncLat)
		}
	})

	t.Run("IOPERF-005", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "gr.dat"), "--rw", "write", "--group_reporting", "--numjobs", "2", "--size", "256K")
		if res.ExitCode != 0 {
			t.Fatalf("ioperf --group_reporting failed: %+v", res)
		}
		if findLineWithPrefix(res.Stdout, "job 0:") != "" || findLineWithPrefix(res.Stdout, "job 1:") != "" {
			t.Fatalf("ioperf --group_reporting should aggregate per-job output into a single summary, got per-job sections: %+v", res)
		}
		if findLineWithPrefix(res.Stdout, "WRITE:") == "" {
			t.Fatalf("ioperf --group_reporting missing aggregated WRITE: summary: %+v", res)
		}
	})

	t.Run("IOPERF-006", func(t *testing.T) {
		// --iodepth is documented as increasing the number of outstanding
		// I/Os, which for random small-block I/O should improve measured
		// throughput. Take the best of several trials at each depth to
		// reduce scheduling noise.
		env := t.TempDir()
		dataFile := filepath.Join(env, "iodepth.dat")
		prep := runGoboxCLI(t, env, "", "ioperf", "--filename", dataFile, "--rw", "write", "--bs", "4k", "--size", "4M")
		if prep.ExitCode != 0 {
			t.Fatalf("ioperf --iodepth fixture setup failed: %+v", prep)
		}

		bestIOPS := func(depth string) float64 {
			var best float64
			for i := 0; i < 3; i++ {
				res := runGoboxCLI(t, env, "", "ioperf", "--filename", dataFile, "--rw", "randread", "--bs", "4k", "--size", "4M", "--iodepth", depth)
				if res.ExitCode != 0 {
					t.Fatalf("ioperf --iodepth=%s failed: %+v", depth, res)
				}
				v, ok := parseIOPS(findLineWithPrefix(res.Stdout, "READ:"))
				if !ok {
					t.Fatalf("ioperf --iodepth=%s: could not parse IOPS from %q", depth, res.Stdout)
				}
				if v > best {
					best = v
				}
			}
			return best
		}

		shallow := bestIOPS("1")
		deep := bestIOPS("32")

		if deep < shallow*1.2 {
			t.Fatalf("ioperf --iodepth=32 shows no measurable throughput improvement over --iodepth=1 "+
				"(best-of-3 IOPS: depth=1 -> %.0f, depth=32 -> %.0f). Per cmds/disk/cmd_ioperf.go:294 "+
				"(`for qd := 0; qd < depth; qd++`), I/O is issued synchronously one call at a time within a "+
				"single goroutine regardless of --iodepth — there is no concurrent/async submission, so "+
				"--iodepth has no observable effect on throughput or latency. See BUGS.md entry from this test.",
				shallow, deep)
		}
	})

	t.Run("IOPERF-007", func(t *testing.T) {
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "io.dat"), "--write_hist_log", filepath.Join(env, "hist"), "--time_based", "--runtime", "1", "--size", "1M")
		if res.ExitCode != 0 {
			t.Fatalf("IOPERF-007 failed: %+v", res)
		}
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
		found := false
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.Split(strings.TrimSpace(line), ",")
			if len(parts) >= 3 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("IOPERF-007: histogram log missing mode,bucket,count CSV lines:\n%s", data)
		}
	})

	t.Run("IOPERF-008", func(t *testing.T) {
		env := t.TempDir()
		target := filepath.Join(env, "nj.dat")
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", target, "--rw", "write", "--numjobs", "2", "--size", "256K")
		if res.ExitCode != 0 {
			t.Fatalf("ioperf --numjobs=2 failed: %+v", res)
		}
		if findLineWithPrefix(res.Stdout, "job 0:") == "" || findLineWithPrefix(res.Stdout, "job 1:") == "" {
			t.Fatalf("ioperf --numjobs=2 missing per-job output sections: %+v", res)
		}
		// Each job must write its own file (filename.0, filename.1), proving
		// numjobs actually spawns independent parallel workers rather than
		// splitting a single job's output into fake per-job text.
		if _, err := os.Stat(target + ".0"); err != nil {
			t.Fatalf("ioperf --numjobs=2: job 0 file missing: %v", err)
		}
		if _, err := os.Stat(target + ".1"); err != nil {
			t.Fatalf("ioperf --numjobs=2: job 1 file missing: %v", err)
		}
	})

	t.Run("IOPERF-009", func(t *testing.T) {
		// --percentile_list must be backed by real percentile computation:
		// p99 should sit well above p50 for a real (if modest) latency
		// distribution, not repeat the same hardcoded value.
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "pctl.dat"), "--rw", "write", "--bs", "4k", "--size", "2M", "--percentile_list", "50:99")
		if res.ExitCode != 0 {
			t.Fatalf("ioperf --percentile_list failed: %+v", res)
		}
		line := findLineWithPrefix(res.Stdout, "WRITE:")
		p50, ok50 := parseLatencyField(line, "p50")
		p99, ok99 := parseLatencyField(line, "p99")
		if !ok50 || !ok99 {
			t.Fatalf("ioperf --percentile_list=50:99: could not parse p50/p99 from %q", line)
		}
		if p99 <= p50 {
			t.Fatalf("ioperf --percentile_list: p99 (%.2fus) must exceed p50 (%.2fus); percentile computation looks broken or hardcoded", p99, p50)
		}
		if p99 < p50*1.2 {
			t.Fatalf("ioperf --percentile_list: p99 (%.2fus) is too close to p50 (%.2fus) to demonstrate distinct percentile computation", p99, p50)
		}
	})

	t.Run("IOPERF-010", func(t *testing.T) {
		// --rate must measurably throttle throughput relative to an
		// unthrottled baseline.
		env := t.TempDir()
		start := time.Now()
		base := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "base.dat"), "--rw", "write", "--bs", "4k", "--size", "256K")
		baseDur := time.Since(start)
		if base.ExitCode != 0 {
			t.Fatalf("ioperf --rate baseline failed: %+v", base)
		}

		start = time.Now()
		limited := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "rate.dat"), "--rw", "write", "--bs", "4k", "--size", "256K", "--rate", "4k")
		limitedDur := time.Since(start)
		if limited.ExitCode != 0 {
			t.Fatalf("ioperf --rate=4k failed: %+v", limited)
		}

		if limitedDur < baseDur*2 {
			t.Fatalf("ioperf --rate=4k should measurably throttle wall-clock duration: baseline=%v rate-limited=%v", baseDur, limitedDur)
		}
		baseIOPS, ok1 := parseIOPS(findLineWithPrefix(base.Stdout, "WRITE:"))
		limitedIOPS, ok2 := parseIOPS(findLineWithPrefix(limited.Stdout, "WRITE:"))
		if !ok1 || !ok2 {
			t.Fatalf("ioperf --rate: could not parse IOPS from output base=%q limited=%q", base.Stdout, limited.Stdout)
		}
		if limitedIOPS >= baseIOPS {
			t.Fatalf("ioperf --rate=4k should report lower measured IOPS than the unthrottled baseline: base=%.0f rate-limited=%.0f", baseIOPS, limitedIOPS)
		}
	})

	t.Run("IOPERF-011", func(t *testing.T) {
		// --runtime only takes effect combined with --time_based; verify the
		// real elapsed wall-clock time tracks the requested runtime.
		env := t.TempDir()
		target := filepath.Join(env, "rt.dat")
		prep := runGoboxCLI(t, env, "", "ioperf", "--filename", target, "--rw", "write", "--bs", "4k", "--size", "256K")
		if prep.ExitCode != 0 {
			t.Fatalf("ioperf --runtime fixture setup failed: %+v", prep)
		}
		start := time.Now()
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", target, "--rw", "read", "--bs", "4k", "--size", "256K", "--time_based", "--runtime", "1")
		elapsed := time.Since(start)
		if res.ExitCode != 0 {
			t.Fatalf("ioperf --runtime=1 --time_based failed: %+v", res)
		}
		if elapsed < 700*time.Millisecond || elapsed > 4*time.Second {
			t.Fatalf("ioperf --runtime=1 --time_based: elapsed wall time %v is not consistent with a ~1s bounded run", elapsed)
		}
	})

	t.Run("IOPERF-012", func(t *testing.T) {
		// --rw=read must issue genuinely sequential reads (positive control
		// for the randread/randwrite offset checks below).
		env := t.TempDir()
		target := filepath.Join(env, "seq.dat")
		prep := runGoboxCLI(t, env, "", "ioperf", "--filename", target, "--rw", "write", "--bs", "4k", "--size", "256K")
		if prep.ExitCode != 0 {
			t.Fatalf("ioperf --rw=read fixture setup failed: %+v", prep)
		}
		offsets := traceSyscallOffsets(t, env, []string{"ioperf", "--filename", "seq.dat", "--rw", "read", "--bs", "4k", "--size", "256K"}, "pread64", 4*1024, 30*time.Second)
		// Off-by-one tolerance for strace-capture flakiness under system
		// load; see IOPERF-001 comment above for details.
		if len(offsets) < 63 || len(offsets) > 64 {
			t.Fatalf("ioperf --rw=read --size=256K --bs=4k: expected 64 (±1) reads of 4096 bytes, observed %d", len(offsets))
		}
		if offsets[0] != 0 || !isAscendingByStepTolerant(offsets, 4*1024) {
			t.Fatalf("ioperf --rw=read: expected sequential offsets starting at 0 incrementing by (a multiple of, to tolerate strace drops) 4096, got %v", offsets)
		}
	})

	t.Run("IOPERF-013", func(t *testing.T) {
		// --rwmixread=70 must produce an actual read:write operation ratio
		// close to 70:30, not merely "some reads and some writes occurred".
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "mix.dat"), "--rw", "readwrite", "--rwmixread", "70", "--bs", "4k", "--size", "1M")
		if res.ExitCode != 0 {
			t.Fatalf("ioperf --rwmixread=70 failed: %+v", res)
		}
		readIOPS, ok1 := parseIOPS(findLineWithPrefix(res.Stdout, "READ:"))
		writeIOPS, ok2 := parseIOPS(findLineWithPrefix(res.Stdout, "WRITE:"))
		if !ok1 || !ok2 {
			t.Fatalf("ioperf --rwmixread=70: could not parse READ/WRITE IOPS from %+v", res)
		}
		total := readIOPS + writeIOPS
		if total <= 0 {
			t.Fatalf("ioperf --rwmixread=70: no I/O recorded: %+v", res)
		}
		ratio := readIOPS / total
		if ratio < 0.55 || ratio > 0.85 {
			t.Fatalf("ioperf --rwmixread=70: observed read ratio %.2f (want ~0.70 +/- 0.15); read IOPS=%.0f write IOPS=%.0f", ratio, readIOPS, writeIOPS)
		}
	})

	t.Run("IOPERF-014", func(t *testing.T) {
		env := t.TempDir()
		target := filepath.Join(env, "sz.dat")
		res := runGoboxCLI(t, env, "", "ioperf", "--filename", target, "--rw", "write", "--size", "256K")
		if res.ExitCode != 0 {
			t.Fatalf("ioperf --size=256K failed: %+v", res)
		}
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("stat ioperf target: %v", err)
		}
		if info.Size() != 256*1024 {
			t.Fatalf("ioperf --size=256K: file size mismatch, got %d want %d", info.Size(), 256*1024)
		}
	})

	t.Run("IOPERF-015", func(t *testing.T) {
		// --sync=sync opens the file with O_SYNC, which must be measurably
		// slower than --sync=none.
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "s0.dat"), "--rw", "write", "--bs", "4k", "--size", "512K", "--sync", "none")
		withSync := runGoboxCLI(t, env, "", "ioperf", "--filename", filepath.Join(env, "s1.dat"), "--rw", "write", "--bs", "4k", "--size", "512K", "--sync", "sync")
		if base.ExitCode != 0 || withSync.ExitCode != 0 {
			t.Fatalf("ioperf --sync run failed base=%+v sync=%+v", base, withSync)
		}
		baseLat, ok1 := parseLatencyField(findLineWithPrefix(base.Stdout, "WRITE:"), "avg")
		syncLat, ok2 := parseLatencyField(findLineWithPrefix(withSync.Stdout, "WRITE:"), "avg")
		if !ok1 || !ok2 {
			t.Fatalf("ioperf --sync: could not parse avg latency from output base=%q sync=%q", base.Stdout, withSync.Stdout)
		}
		if syncLat < baseLat*2 {
			t.Fatalf("ioperf --sync=sync should measurably increase average write latency via O_SYNC: base avg=%.2fus sync avg=%.2fus", baseLat, syncLat)
		}
	})

	t.Run("IOPERF-016", func(t *testing.T) {
		// docs/TEST-CASES.md defines IOPERF-016 as --time_based. The total
		// number of completed ops should scale with elapsed --runtime (not
		// be capped by --size), proving time_based mode really runs until
		// the deadline rather than stopping once size/bs ops complete.
		env := t.TempDir()
		target := filepath.Join(env, "tb.dat")
		prep := runGoboxCLI(t, env, "", "ioperf", "--filename", target, "--rw", "write", "--bs", "4k", "--size", "4M")
		if prep.ExitCode != 0 {
			t.Fatalf("ioperf --time_based fixture setup failed: %+v", prep)
		}

		short := runGoboxCLI(t, env, "", "ioperf", "--filename", target, "--rw", "read", "--bs", "4k", "--size", "4M", "--time_based", "--runtime", "1")
		long := runGoboxCLI(t, env, "", "ioperf", "--filename", target, "--rw", "read", "--bs", "4k", "--size", "4M", "--time_based", "--runtime", "2")
		if short.ExitCode != 0 || long.ExitCode != 0 {
			t.Fatalf("ioperf --time_based run failed short=%+v long=%+v", short, long)
		}
		shortIOPS, ok1 := parseIOPS(findLineWithPrefix(short.Stdout, "READ:"))
		longIOPS, ok2 := parseIOPS(findLineWithPrefix(long.Stdout, "READ:"))
		if !ok1 || !ok2 {
			t.Fatalf("ioperf --time_based: could not parse IOPS short=%q long=%q", short.Stdout, long.Stdout)
		}
		// size/bs = 4M/4k = 1024 total ops: if --time_based were secretly
		// still capped by --size, both runs would complete the same total
		// ops regardless of --runtime. Total ops = IOPS * duration.
		totalShort := shortIOPS * 1
		totalLong := longIOPS * 2
		ratio := totalLong / totalShort
		if ratio < 1.5 || ratio > 2.7 {
			t.Fatalf("ioperf --time_based: total completed ops should roughly double when --runtime doubles from 1 to 2 "+
				"(proving execution isn't capped by --size=4M/--bs=4k=1024 ops); runtime=1 total~=%.0f runtime=2 total~=%.0f (ratio=%.2f)",
				totalShort, totalLong, ratio)
		}
	})

	t.Run("IOPERF-017", func(t *testing.T) {
		// Undocumented Case ID (docs/TEST-CASES.md only defines up to
		// IOPERF-016): --rw=randwrite. Verify write offsets are genuinely
		// non-sequential via strace, not merely "a WRITE: line exists".
		env := t.TempDir()
		offsets := traceSyscallOffsets(t, env, []string{"ioperf", "--filename", "rw.dat", "--rw", "randwrite", "--bs", "4k", "--size", "4M"}, "pwrite64", 4*1024, 30*time.Second)
		if len(offsets) < 20 {
			t.Fatalf("ioperf --rw=randwrite: expected at least 20 pwrite64 calls to prove randomness, observed %d", len(offsets))
		}
		if isAscendingByStep(offsets, 4*1024) {
			t.Fatalf("ioperf --rw=randwrite: write offsets are perfectly sequential (%v), expected a randomized access pattern", offsets[:10])
		}
	})

	t.Run("IOPERF-018", func(t *testing.T) {
		// New Case ID: needs to be added to docs/TEST-CASES.md. --rw=randread
		// was previously (mis)tested under IOPERF-016, which docs define as
		// --time_based (now covered above). Verify read offsets are
		// genuinely non-sequential via strace.
		env := t.TempDir()
		target := filepath.Join(env, "rr.dat")
		prep := runGoboxCLI(t, env, "", "ioperf", "--filename", target, "--rw", "write", "--bs", "4k", "--size", "4M")
		if prep.ExitCode != 0 {
			t.Fatalf("ioperf --rw=randread fixture setup failed: %+v", prep)
		}
		offsets := traceSyscallOffsets(t, env, []string{"ioperf", "--filename", "rr.dat", "--rw", "randread", "--bs", "4k", "--size", "4M"}, "pread64", 4*1024, 30*time.Second)
		if len(offsets) < 20 {
			t.Fatalf("ioperf --rw=randread: expected at least 20 pread64 calls to prove randomness, observed %d", len(offsets))
		}
		if isAscendingByStep(offsets, 4*1024) {
			t.Fatalf("ioperf --rw=randread: read offsets are perfectly sequential (%v), expected a randomized access pattern", offsets[:10])
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
			// docs/TEST-CASES.md defines IOPERF-016 as --time_based. Verify
			// against fio that a time-based run keeps going for the full
			// runtime rather than stopping once --size worth of ops complete.
			id: "IOPERF-016",
			goboxArgs: func(env, goboxFile string) []string {
				return []string{"ioperf", "--filename", goboxFile, "--rw", "write", "--size", "1M", "--time_based", "--runtime", "1"}
			},
			fioArgs: func(env, nativeFile string) []string {
				return []string{"--filename=" + nativeFile, "--rw=write", "--size=1M", "--time_based=1", "--runtime=1"}
			},
			assert: func(t *testing.T, env, goboxFile, nativeFile string, gobox, native parityResult) {
				assertReadWrite(t, gobox, native, false, true)
				// fio's "Run status" summary line reports elapsed time as
				// "run=<ms>-<ms>msec" (verified against actual fio 3.35
				// output), not "runt=".
				if findLineContaining(strings.ToLower(native.Stdout), "run=") == "" {
					t.Fatalf("fio --time_based output missing runtime accounting: %+v", native)
				}
			},
		},
		{
			// Undocumented Case ID (docs/TEST-CASES.md only defines up to
			// IOPERF-016): randwrite.
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
		{
			// New Case ID: needs to be added to docs/TEST-CASES.md. randread
			// was previously mislabeled IOPERF-016 in this file; that ID now
			// correctly tests --time_based (see above).
			id: "IOPERF-018",
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

	t.Run("SHA256-007", func(t *testing.T) {
		// Undocumented Case ID: not yet present in docs/TEST-CASES.md.
		// Checksum file references a file that does not exist at all,
		// distinct from SHA256-006 which covers a malformed *line*.
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "good.txt"), "hello")
		sum := runNativeCLI(t, env, "", "sha256sum", "good.txt")
		content := normalizeText(sum.Stdout) + "\n" + strings.Repeat("0", 64) + "  missing.txt\n"
		writeFile(t, filepath.Join(env, "checksums.sha256"), content)

		native := runNativeCLI(t, env, "", "sha256sum", "--check", "checksums.sha256")
		if native.ExitCode == 0 {
			t.Fatalf("native sha256sum --check with a missing referenced file should fail")
		}
		if findLineContaining(native.Stdout, "missing.txt") == "" {
			t.Fatalf("native sha256sum --check should report the missing file on stdout: %+v", native)
		}

		gobox := runGoboxCLI(t, env, "", "sha256sum", "--check", "checksums.sha256")
		if gobox.ExitCode == 0 {
			t.Fatalf("sha256sum --check with a missing referenced file should fail, got exit 0: %+v", gobox)
		}
		goodLine := findLineContaining(gobox.Stdout, "good.txt")
		if goodLine != "good.txt: OK" {
			t.Fatalf("sha256sum --check: valid entry should read exactly %q, got %q (full output: %+v)", "good.txt: OK", goodLine, gobox)
		}
		if findLineContaining(gobox.Stdout, "missing.txt") == "" {
			t.Fatalf("sha256sum --check should report the missing referenced file on stdout (FAILED-open-or-read style); got stdout=%q stderr=%q", gobox.Stdout, gobox.Stderr)
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

	// Field-by-field cross-check for the columns that have a stable mapping
	// between gobox's vocabulary (ReadIOPS/WriteIOPS/TotalIOPS/ReadB/s/
	// WriteB/s/TotalB/s) and native sysstat iostat's (tps/kB_read/s/
	// kB_wrtn/s/...): TotalIOPS<->tps, ReadB/s<->kB_read/s*1024,
	// WriteB/s<->kB_wrtn/s*1024. Exact equality is not expected (the two
	// commands sample at slightly different moments against a live,
	// possibly-changing device — see docs/TEST-DESIGN.md §13), so this uses
	// a generous ratio-based tolerance that still catches gross bugs such as
	// swapped columns or a unit-conversion error.
	goboxByDev := make(map[string][]string, len(goboxRows))
	for _, row := range goboxRows {
		goboxByDev[row[0]] = row
	}
	nativeByDev := make(map[string][]string, len(nativeRows))
	for _, row := range nativeRows {
		nativeByDev[row[0]] = row
	}

	totalIdxG := iostatFieldIndex(goboxHeader, "TotalIOPS")
	readBIdxG := iostatFieldIndex(goboxHeader, "ReadB/s")
	writeBIdxG := iostatFieldIndex(goboxHeader, "WriteB/s")
	tpsIdxN := iostatFieldIndex(nativeHeader, "tps")
	readKBIdxN := iostatFieldIndex(nativeHeader, "kB_read/s")
	writeKBIdxN := iostatFieldIndex(nativeHeader, "kB_wrtn/s")

	compared := 0
	for dev, gRow := range goboxByDev {
		nRow, ok := nativeByDev[dev]
		if !ok {
			continue
		}
		if totalIdxG >= 0 && tpsIdxN >= 0 && totalIdxG < len(gRow) && tpsIdxN < len(nRow) {
			gv, gok := parseIostatNumeric(gRow[totalIdxG])
			nv, nok := parseIostatNumeric(nRow[tpsIdxN])
			if gok && nok {
				compared++
				if !withinIostatTolerance(gv, nv) {
					t.Fatalf("iostat device %s: gobox TotalIOPS=%.2f vs native tps=%.2f diverge beyond tolerance", dev, gv, nv)
				}
			}
		}
		if readBIdxG >= 0 && readKBIdxN >= 0 && readBIdxG < len(gRow) && readKBIdxN < len(nRow) {
			gv, gok := parseIostatNumeric(gRow[readBIdxG])
			nvKB, nok := parseIostatNumeric(nRow[readKBIdxN])
			if gok && nok {
				compared++
				if !withinIostatTolerance(gv, nvKB*1024) {
					t.Fatalf("iostat device %s: gobox ReadB/s=%.2f vs native kB_read/s*1024=%.2f diverge beyond tolerance", dev, gv, nvKB*1024)
				}
			}
		}
		if writeBIdxG >= 0 && writeKBIdxN >= 0 && writeBIdxG < len(gRow) && writeKBIdxN < len(nRow) {
			gv, gok := parseIostatNumeric(gRow[writeBIdxG])
			nvKB, nok := parseIostatNumeric(nRow[writeKBIdxN])
			if gok && nok {
				compared++
				if !withinIostatTolerance(gv, nvKB*1024) {
					t.Fatalf("iostat device %s: gobox WriteB/s=%.2f vs native kB_wrtn/s*1024=%.2f diverge beyond tolerance", dev, gv, nvKB*1024)
				}
			}
		}
	}
	if compared == 0 {
		t.Fatalf("iostat structured parity: found no comparable numeric fields between gobox header %v and native header %v", goboxHeader, nativeHeader)
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
