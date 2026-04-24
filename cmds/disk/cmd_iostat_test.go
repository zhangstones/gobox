package disk

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIostatCmdUsesDiskstatsByDefault(t *testing.T) {
	oldReadFile := readFileIostat
	oldSleep := sleepIostat
	t.Cleanup(func() {
		readFileIostat = oldReadFile
		sleepIostat = oldSleep
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
	if !strings.Contains(output, "10.00/s") || !strings.Contains(output, "20.00/s") {
		t.Fatalf("missing derived IOPS values: %q", output)
	}
}

func TestIostatCmdSupportsPositionals(t *testing.T) {
	oldReadFile := readFileIostat
	oldSleep := sleepIostat
	t.Cleanup(func() {
		readFileIostat = oldReadFile
		sleepIostat = oldSleep
	})

	snapshots := []string{
		"8 0 sda 1 0 2 0 3 0 4 0 0 5 6\n",
		"8 0 sda 2 0 4 0 5 0 8 0 0 9 10\n",
	}
	readFileIostat = func(path string) ([]byte, error) {
		if path != "/proc/diskstats" {
			return nil, os.ErrNotExist
		}
		out := snapshots[0]
		snapshots = snapshots[1:]
		return []byte(out), nil
	}
	sleepIostat = func(time.Duration) {}

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
	oldSleep := sleepIostat
	oldStat := statIostat
	t.Cleanup(func() {
		readFileIostat = oldReadFile
		sleepIostat = oldSleep
		statIostat = oldStat
	})

	snapshots := []string{
		"sda rbytes=1024 wbytes=2048 rios=10 wios=20\n",
		"sda rbytes=3072 wbytes=6144 rios=14 wios=26\n",
	}
	readFileIostat = func(path string) ([]byte, error) {
		if path != "/sys/fs/cgroup/io.stat" {
			return nil, os.ErrNotExist
		}
		out := snapshots[0]
		snapshots = snapshots[1:]
		return []byte(out), nil
	}
	statIostat = func(path string) (os.FileInfo, error) {
		if path == "/sys/fs/cgroup/io.stat" {
			return fakeFileInfo{name: "io.stat"}, nil
		}
		return nil, os.ErrNotExist
	}
	sleepIostat = func(time.Duration) {}

	var out bytes.Buffer
	if err := iostatCmd([]string{"--cgroup", "-i", "1", "-n", "1"}, &out); err != nil {
		t.Fatalf("iostatCmd failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "sda") {
		t.Fatalf("missing cgroup device row: %q", output)
	}
	if !strings.Contains(output, "4.00/s") || !strings.Contains(output, "6.00/s") {
		t.Fatalf("missing cgroup iops values: %q", output)
	}
}

func TestIostatCmdZeroFilterHidesInactiveRows(t *testing.T) {
	oldReadFile := readFileIostat
	oldSleep := sleepIostat
	t.Cleanup(func() {
		readFileIostat = oldReadFile
		sleepIostat = oldSleep
	})

	snapshots := []string{
		"8 0 sda 1 0 2 0 3 0 4 0 0 5 6\n8 1 sdb 7 0 8 0 9 0 10 0 0 11 12\n",
		"8 0 sda 1 0 2 0 3 0 4 0 0 5 6\n8 1 sdb 8 0 10 0 11 0 14 0 0 15 18\n",
	}
	readFileIostat = func(path string) ([]byte, error) {
		if path != "/proc/diskstats" {
			return nil, os.ErrNotExist
		}
		out := snapshots[0]
		snapshots = snapshots[1:]
		return []byte(out), nil
	}
	sleepIostat = func(time.Duration) {}

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

func TestIostatCmdRejectsInvalidPositionals(t *testing.T) {
	var out bytes.Buffer
	err := iostatCmd([]string{"abc", "1"}, &out)
	if err == nil || !strings.Contains(err.Error(), `invalid interval "abc"`) {
		t.Fatalf("expected invalid interval error, got %v", err)
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
