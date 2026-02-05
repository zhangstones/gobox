package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindCmdHandlesFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := findCmd([]string{"-name", "*.txt", "-maxdepth", "1", dir}); err != nil {
		t.Fatalf("findCmd returned error: %v", err)
	}
}

func TestDuCmdSummary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := duCmd([]string{"-s", dir}); err != nil {
		t.Fatalf("duCmd returned error: %v", err)
	}
}

func TestXargsCmdNoRunWithNoInput(t *testing.T) {
	orig := os.Stdin
	t.Cleanup(func() { os.Stdin = orig })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_ = w.Close()
	os.Stdin = r

	if err := xargsCmd([]string{"-r"}); err != nil {
		t.Fatalf("xargsCmd returned error: %v", err)
	}
}

func TestPsCmdMinimal(t *testing.T) {
	if err := psCmd([]string{"-n", "1", "-i", "0"}); err != nil {
		t.Fatalf("psCmd returned error: %v", err)
	}
}

func TestTopCmdSingleIteration(t *testing.T) {
	if err := topCmd([]string{"-n", "1", "-d", "0"}); err != nil {
		t.Fatalf("topCmd returned error: %v", err)
	}
}

func TestIostatCmdZeroSamples(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("iostat supported only on Linux")
	}
	if err := iostatCmd([]string{"-n", "0"}); err != nil {
		t.Fatalf("iostatCmd returned error: %v", err)
	}
}

func TestNetstatCmdRuns(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat supported only on Linux")
	}
	if err := netstatCmd([]string{}); err != nil {
		t.Fatalf("netstatCmd returned error: %v", err)
	}
}
