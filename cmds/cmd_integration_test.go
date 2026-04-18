package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gobox/cmds/disk"
	"gobox/cmds/fs"
	"gobox/cmds/net"
	"gobox/cmds/proc"
)

func TestFindCmdHandlesFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := fs.FindCmd([]string{"-name", "*.txt", "-maxdepth", "1", dir}); err != nil {
		t.Fatalf("FindCmd returned error: %v", err)
	}
}

func TestDuCmdSummary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := fs.DuCmd([]string{"-s", dir}); err != nil {
		t.Fatalf("DuCmd returned error: %v", err)
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

	if err := proc.XargsCmd([]string{"-r"}); err != nil {
		t.Fatalf("XargsCmd returned error: %v", err)
	}
}

func TestPsCmdMinimal(t *testing.T) {
	if err := proc.PsCmd([]string{"-n", "1", "-i", "0"}); err != nil {
		t.Fatalf("PsCmd returned error: %v", err)
	}
}

func TestTopCmdSingleIteration(t *testing.T) {
	if err := proc.TopCmd([]string{"-n", "1", "-d", "0"}); err != nil {
		t.Fatalf("TopCmd returned error: %v", err)
	}
}

func TestIostatCmdZeroSamples(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("iostat supported only on Linux")
	}
	if err := disk.IostatCmd([]string{"-n", "0"}); err != nil {
		t.Fatalf("IostatCmd returned error: %v", err)
	}
}

func TestNetstatCmdRuns(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat supported only on Linux")
	}
	if err := net.NetstatCmd([]string{}); err != nil {
		t.Fatalf("NetstatCmd returned error: %v", err)
	}
}
