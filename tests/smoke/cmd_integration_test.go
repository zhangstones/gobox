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
	"gobox/cmds/text"
)

// ============ Cross-Platform Commands ============

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

func TestGrepCmdBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello world\nfoo bar\nhello again"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.GrepCmd([]string{"hello", file}); err != nil {
		t.Fatalf("GrepCmd returned error: %v", err)
	}
}

func TestGrepCmdInvertMatch(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello\nworld\nfoo"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.GrepCmd([]string{"-v", "hello", file}); err != nil {
		t.Fatalf("GrepCmd -v returned error: %v", err)
	}
}

func TestSedCmdSubstitute(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.SedCmd([]string{"s/hello/HELLO/", file}); err != nil {
		t.Fatalf("SedCmd returned error: %v", err)
	}
}

func TestSedCmdQuietMode(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello\nworld\nhello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.SedCmd([]string{"-n", "s/hello/HELLO/p", file}); err != nil {
		t.Fatalf("SedCmd -n returned error: %v", err)
	}
}

func TestHeadCmdDefault(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.HeadCmd([]string{file}); err != nil {
		t.Fatalf("HeadCmd returned error: %v", err)
	}
}

func TestHeadCmdNLines(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.HeadCmd([]string{"-n", "2", file}); err != nil {
		t.Fatalf("HeadCmd -n returned error: %v", err)
	}
}

func TestTailCmdDefault(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.TailCmd([]string{file}); err != nil {
		t.Fatalf("TailCmd returned error: %v", err)
	}
}

func TestTailCmdNlines(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.TailCmd([]string{"-n", "2", file}); err != nil {
		t.Fatalf("TailCmd -n returned error: %v", err)
	}
}

func TestSortCmdDefault(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("cherry\napple\nbanana"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.SortCmd([]string{file}); err != nil {
		t.Fatalf("SortCmd returned error: %v", err)
	}
}

func TestSortCmdNumeric(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("10\n2\n1\n20"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.SortCmd([]string{"-n", file}); err != nil {
		t.Fatalf("SortCmd -n returned error: %v", err)
	}
}

func TestSortCmdReverse(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("apple\nbanana\ncherry"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.SortCmd([]string{"-r", file}); err != nil {
		t.Fatalf("SortCmd -r returned error: %v", err)
	}
}

func TestUniqCmdBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("apple\napple\nbanana\ncherry\ncherry\ncherry"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.UniqCmd([]string{file}); err != nil {
		t.Fatalf("UniqCmd returned error: %v", err)
	}
}

func TestUniqCmdCount(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("apple\napple\nbanana"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.UniqCmd([]string{"-c", file}); err != nil {
		t.Fatalf("UniqCmd -c returned error: %v", err)
	}
}

func TestWCCmdDefault(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello world\nfoo bar"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.WcCmd([]string{file}); err != nil {
		t.Fatalf("WcCmd returned error: %v", err)
	}
}

func TestWcCmdLines(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("line1\nline2\nline3"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := text.WcCmd([]string{"-l", file}); err != nil {
		t.Fatalf("WcCmd -l returned error: %v", err)
	}
}

// ============ Linux-specific Commands ============

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

// ============ Network Commands ============

func TestDigCmdBasic(t *testing.T) {
	if err := net.DigCmd([]string{"+short", "localhost"}); err != nil {
		t.Fatalf("DigCmd returned error: %v", err)
	}
}

func TestDigCmdWithType(t *testing.T) {
	if err := net.DigCmd([]string{"-t", "A", "+noall", "+answer", "localhost"}); err != nil {
		t.Fatalf("DigCmd with type returned error: %v", err)
	}
}

func TestNcCmdHelp(t *testing.T) {
	if err := net.NcCmd([]string{"-h"}); err != nil {
		t.Fatalf("NcCmd -h returned error: %v", err)
	}
}

func TestCurlCmdHelp(t *testing.T) {
	if err := net.CurlCmd([]string{"-h"}); err != nil {
		t.Fatalf("CurlCmd -h returned error: %v", err)
	}
}

func TestTwCmdHelp(t *testing.T) {
	if err := net.TwCmd([]string{"-h"}); err != nil {
		t.Fatalf("TwCmd -h returned error: %v", err)
	}
}

func TestIfstatCmdSingleSample(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ifstat supported only on Linux")
	}
	if err := net.IfstatCmd([]string{"-n", "1", "-p", "1"}); err != nil {
		t.Fatalf("IfstatCmd returned error: %v", err)
	}
}

func TestNpCmdHelp(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("np supported only on Linux")
	}
	if err := net.NpCmd([]string{"-h"}); err != nil {
		t.Fatalf("NpCmd -h returned error: %v", err)
	}
}

func TestHpingCmdHelp(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("hping supported only on Linux")
	}
	if err := net.HpingCmd([]string{"-h"}); err != nil {
		t.Fatalf("HpingCmd -h returned error: %v", err)
	}
}

// ============ Disk/System Commands ============

func TestMd5sumCmdBasic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := disk.Md5sumCmd([]string{file}); err != nil {
		t.Fatalf("Md5sumCmd returned error: %v", err)
	}
}

func TestMd5sumCmdCheck(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	content := "hello world"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	checksum := "5eb63bbbe01eeed093cb22bb8f5acdc3"
	checkFile := filepath.Join(dir, "check.md5")
	if err := os.WriteFile(checkFile, []byte(checksum+"  "+filepath.Base(file)+"\n"), 0o644); err != nil {
		t.Fatalf("write check file: %v", err)
	}
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := disk.Md5sumCmd([]string{"-c", checkFile}); err != nil {
		t.Fatalf("Md5sumCmd -c returned error: %v", err)
	}
}

func TestIoperfCmdHelp(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ioperf supported only on Linux")
	}
	if err := disk.IoperfCmd([]string{"-h"}); err != nil {
		t.Fatalf("IoperfCmd -h returned error: %v", err)
	}
}

func TestRandCmdBasic(t *testing.T) {
	if err := text.RandCmd([]string{}); err != nil {
		t.Fatalf("RandCmd returned error: %v", err)
	}
}

func TestRandCmdHexOutput(t *testing.T) {
	if err := text.RandCmd([]string{"-n", "16"}); err != nil {
		t.Fatalf("RandCmd -n returned error: %v", err)
	}
}

func TestSeqCmdBasic(t *testing.T) {
	if err := text.SeqCmd([]string{"1", "5"}); err != nil {
		t.Fatalf("SeqCmd returned error: %v", err)
	}
}

func TestSeqCmdWithFormat(t *testing.T) {
	if err := text.SeqCmd([]string{"-f", "%02.0f", "1", "5"}); err != nil {
		t.Fatalf("SeqCmd -f returned error: %v", err)
	}
}

func TestSeqCmdWithSeparator(t *testing.T) {
	if err := text.SeqCmd([]string{"-s", ",", "1", "3"}); err != nil {
		t.Fatalf("SeqCmd -s returned error: %v", err)
	}
}
