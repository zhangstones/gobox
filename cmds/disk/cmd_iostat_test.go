package disk

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func captureIostatHelp(t *testing.T, args []string) (string, error) {
	t.Helper()

	oldStderr := os.Stderr
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	defer rErr.Close()

	os.Stderr = wErr
	runErr := IostatCmd(args)
	_ = wErr.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, rErr)
	return buf.String(), runErr
}

func TestIostatCmdUsesDiskstatsByDefault(t *testing.T) {
	oldReadFile := readFileIostat
	oldUptime := uptimeIostat
	t.Cleanup(func() {
		readFileIostat = oldReadFile
		uptimeIostat = oldUptime
	})

	readFileIostat = func(path string) ([]byte, error) {
		if path != "/proc/diskstats" {
			return nil, os.ErrNotExist
		}
		return []byte("8 0 sda 100 0 200 0 300 0 400 0 0 500 600\n"), nil
	}
	uptimeIostat = func() (float64, error) { return 10, nil }

	var out bytes.Buffer
	if err := iostatCmd([]string{"-i", "1", "-n", "1"}, &out); err != nil {
		t.Fatalf("iostatCmd failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Device") || !strings.Contains(output, "ReadIOPS") {
		t.Fatalf("missing header: %q", output)
	}
	if !strings.Contains(output, "sda") {
		t.Fatalf("missing device row: %q", output)
	}
	if !strings.Contains(output, "10.00/s") || !strings.Contains(output, "30.00/s") {
		t.Fatalf("missing derived IOPS values: %q", output)
	}
}

func TestIostatCmdSupportsPositionals(t *testing.T) {
	oldReadFile := readFileIostat
	oldUptime := uptimeIostat
	t.Cleanup(func() {
		readFileIostat = oldReadFile
		uptimeIostat = oldUptime
	})

	readFileIostat = func(path string) ([]byte, error) {
		if path != "/proc/diskstats" {
			return nil, os.ErrNotExist
		}
		return []byte("8 0 sda 1 0 2 0 3 0 4 0 0 5 6\n"), nil
	}
	uptimeIostat = func() (float64, error) { return 1, nil }

	var out bytes.Buffer
	if err := iostatCmd([]string{"1", "1"}, &out); err != nil {
		t.Fatalf("iostatCmd failed: %v", err)
	}
	if !strings.Contains(out.String(), "sda") {
		t.Fatalf("expected positional interval/count to execute, got %q", out.String())
	}
}

func TestIostatCmdUsesCgroupWhenRequested(t *testing.T) {
	oldReadFile := readFileIostat
	oldStat := statIostat
	oldUptime := uptimeIostat
	t.Cleanup(func() {
		readFileIostat = oldReadFile
		statIostat = oldStat
		uptimeIostat = oldUptime
	})

	readFileIostat = func(path string) ([]byte, error) {
		if path != "/sys/fs/cgroup/io.stat" {
			return nil, os.ErrNotExist
		}
		return []byte("sda rbytes=3072 wbytes=6144 rios=14 wios=26\n"), nil
	}
	statIostat = func(path string) (os.FileInfo, error) {
		if path == "/sys/fs/cgroup/io.stat" {
			return fakeFileInfo{name: "io.stat"}, nil
		}
		return nil, os.ErrNotExist
	}
	uptimeIostat = func() (float64, error) { return 2, nil }

	var out bytes.Buffer
	if err := iostatCmd([]string{"--cgroup", "-i", "1", "-n", "1"}, &out); err != nil {
		t.Fatalf("iostatCmd failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "sda") {
		t.Fatalf("missing cgroup device row: %q", output)
	}
	if !strings.Contains(output, "7.00/s") || !strings.Contains(output, "13.00/s") {
		t.Fatalf("missing cgroup iops values: %q", output)
	}
}

// Bug regression test: --cgroup previously always read the root cgroup's
// io.stat, which aggregates the whole system and looks identical to plain
// /proc/diskstats output. It should instead resolve the current process's
// own cgroup (via /proc/self/cgroup) and read that cgroup's io.stat when
// available, rather than silently falling back to system-wide data.
func TestIostatCmdUsesOwnCgroupIoStatWhenAvailable(t *testing.T) {
	oldReadFile := readFileIostat
	oldStat := statIostat
	oldUptime := uptimeIostat
	t.Cleanup(func() {
		readFileIostat = oldReadFile
		statIostat = oldStat
		uptimeIostat = oldUptime
	})

	ownPath := "/sys/fs/cgroup/user.slice/session-1.scope/io.stat"
	readFileIostat = func(path string) ([]byte, error) {
		switch path {
		case "/proc/self/cgroup":
			return []byte("0::/user.slice/session-1.scope\n"), nil
		case ownPath:
			return []byte("sda rbytes=1024 wbytes=2048 rios=4 wios=8\n"), nil
		case "/sys/fs/cgroup/io.stat":
			// Root-cgroup (system-wide) data intentionally differs so the
			// test fails if the fallback path is used instead of the fix.
			return []byte("sda rbytes=999999999 wbytes=999999999 rios=99999 wios=99999\n"), nil
		}
		return nil, os.ErrNotExist
	}
	statIostat = func(path string) (os.FileInfo, error) {
		if path == ownPath {
			return fakeFileInfo{name: "io.stat"}, nil
		}
		return nil, os.ErrNotExist
	}
	uptimeIostat = func() (float64, error) { return 2, nil }

	var out bytes.Buffer
	if err := iostatCmd([]string{"--cgroup", "-i", "1", "-n", "1"}, &out); err != nil {
		t.Fatalf("iostatCmd failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "sda") {
		t.Fatalf("missing cgroup device row: %q", output)
	}
	// rios=4/wios=8 over uptime=2s -> 2.00/s read, 4.00/s write.
	if !strings.Contains(output, "2.00/s") || !strings.Contains(output, "4.00/s") {
		t.Fatalf("expected rates derived from own-cgroup io.stat, got %q", output)
	}
	if strings.Contains(output, "99999") {
		t.Fatalf("expected own-cgroup data, but root cgroup (system-wide) data leaked in: %q", output)
	}
}

func TestIostatCmdZeroFilterHidesInactiveRows(t *testing.T) {
	oldReadFile := readFileIostat
	oldUptime := uptimeIostat
	t.Cleanup(func() {
		readFileIostat = oldReadFile
		uptimeIostat = oldUptime
	})

	readFileIostat = func(path string) ([]byte, error) {
		if path != "/proc/diskstats" {
			return nil, os.ErrNotExist
		}
		return []byte("8 0 sda 0 0 0 0 0 0 0 0 0 0 0\n8 1 sdb 8 0 10 0 11 0 14 0 0 15 18\n"), nil
	}
	uptimeIostat = func() (float64, error) { return 1, nil }

	var out bytes.Buffer
	if err := iostatCmd([]string{"-z", "1", "1"}, &out); err != nil {
		t.Fatalf("iostatCmd failed: %v", err)
	}

	output := out.String()
	if strings.Contains(output, "sda") {
		t.Fatalf("expected inactive device to be filtered, got %q", output)
	}
	if !strings.Contains(output, "sdb") {
		t.Fatalf("expected active device to remain, got %q", output)
	}
}

func TestIostatCmdUsesIntervalDeltasAfterFirstReport(t *testing.T) {
	oldReadFile := readFileIostat
	oldSleep := sleepIostat
	oldUptime := uptimeIostat
	t.Cleanup(func() {
		readFileIostat = oldReadFile
		sleepIostat = oldSleep
		uptimeIostat = oldUptime
	})

	snapshots := []string{
		"8 0 sda 100 0 200 0 300 0 400 0 0 500 600\n",
		"8 0 sda 110 0 240 0 320 0 440 0 0 560 720\n",
	}
	readFileIostat = func(path string) ([]byte, error) {
		if path != "/proc/diskstats" {
			return nil, os.ErrNotExist
		}
		if len(snapshots) == 0 {
			return nil, errors.New("missing snapshot")
		}
		out := snapshots[0]
		snapshots = snapshots[1:]
		return []byte(out), nil
	}
	sleepIostat = func(time.Duration) {}
	uptimeIostat = func() (float64, error) { return 10, nil }

	var out bytes.Buffer
	if err := iostatCmd([]string{"-i", "1", "-n", "2"}, &out); err != nil {
		t.Fatalf("iostatCmd failed: %v", err)
	}

	sections := strings.Split(strings.TrimSpace(out.String()), "\n\n")
	if len(sections) != 2 {
		t.Fatalf("expected 2 reports, got %q", out.String())
	}
	if !strings.Contains(sections[0], "10.00/s") || !strings.Contains(sections[0], "30.00/s") {
		t.Fatalf("missing since-boot stats in first report: %q", sections[0])
	}
	if !strings.Contains(sections[1], "10.00/s") || !strings.Contains(sections[1], "20.00/s") {
		t.Fatalf("missing interval delta stats in second report: %q", sections[1])
	}
}

func TestIostatCmdRejectsInvalidPositionals(t *testing.T) {
	var out bytes.Buffer
	err := iostatCmd([]string{"abc", "1"}, &out)
	if err == nil || !strings.Contains(err.Error(), `invalid interval "abc"`) {
		t.Fatalf("expected invalid interval error, got %v", err)
	}
}

func TestIostatCmdHelp(t *testing.T) {
	out, err := captureIostatHelp(t, []string{"--help"})
	if err != nil {
		t.Fatalf("iostat --help failed: %v", err)
	}
	for _, want := range []string{
		"Usage: gobox iostat [OPTION]... [interval [count]]",
		"Options:",
		"Positionals:",
		"Columns:",
		"Examples:",
		"ReadIOPS",
		"--cgroup",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in help output %q", want, out)
		}
	}
}

type fakeFileInfo struct {
	name string
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }
