package disk

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// findGoboxBinary finds the gobox binary, trying multiple locations
func findGoboxBinary() string {
	// Try current directory first
	if _, err := os.Stat("./gobox"); err == nil {
		return "./gobox"
	}
	// Try project root (two levels up from cmds/disk)
	if _, err := os.Stat("../../gobox"); err == nil {
		return "../../gobox"
	}
	// Try absolute path from test file location
	testDir, _ := os.Getwd()
	// Navigate up to project root
	for i := 0; i < 4; i++ {
		if _, err := os.Stat(filepath.Join(testDir, "gobox")); err == nil {
			return filepath.Join(testDir, "gobox")
		}
		testDir = filepath.Dir(testDir)
	}
	// Fallback to default
	return "./gobox"
}

// runIoperfCmd runs the ioperf command via exec.Command and returns output
func runIoperfCmd(args []string) (string, error) {
	goboxPath := findGoboxBinary()
	cmd := exec.Command(goboxPath, append([]string{"ioperf"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Return combined output even on error for debugging
		return string(output), err
	}
	return string(output), nil
}

// ============== NORMAL CASES ==============

func TestIoperfCmdReadMode(t *testing.T) {
	// Create a temp file to read from
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile")

	// First create the file with some data
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	// Write 1MB of data
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	tmpFile.Write(data)
	tmpFile.Close()

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=1M",
		"--bs=4k",
		"--numjobs=1",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf read mode failed: %v\nOutput: %s", err, output)
	}

	// Verify output contains expected elements
	result := string(output)
	if !strings.Contains(result, "READ:") {
		t.Errorf("Expected READ: in output, got: %s", result)
	}
	if !strings.Contains(result, "IOPS=") {
		t.Errorf("Expected IOPS= in output, got: %s", result)
	}
	if !strings.Contains(result, "BW=") {
		t.Errorf("Expected BW= in output, got: %s", result)
	}
	if !strings.Contains(result, "lat=") {
		t.Errorf("Expected lat= in output, got: %s", result)
	}
}

func TestIoperfCmdWriteMode(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_write")

	args := []string{
		"--rw=write",
		"--filename=" + filename,
		"--size=64k",
		"--bs=4k",
		"--numjobs=1",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf write mode failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "WRITE:") {
		t.Errorf("Expected WRITE: in output, got: %s", result)
	}

	// Verify file was created (ioperf appends .0 for job 0)
	if _, err := os.Stat(filename + ".0"); os.IsNotExist(err) {
		t.Errorf("Expected output file to be created at %s", filename+".0")
	}
}

func TestIoperfCmdRandreadMode(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_randread")

	// Pre-create the file
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	data := make([]byte, 256*1024) // 256KB
	tmpFile.Write(data)
	tmpFile.Close()

	args := []string{
		"--rw=randread",
		"--filename=" + filename,
		"--size=128k",
		"--bs=4k",
		"--numjobs=1",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf randread mode failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "READ:") {
		t.Errorf("Expected READ: in output, got: %s", result)
	}
}

func TestIoperfCmdRandwriteMode(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_randwrite")

	args := []string{
		"--rw=randwrite",
		"--filename=" + filename,
		"--size=64k",
		"--bs=4k",
		"--numjobs=1",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf randwrite mode failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "WRITE:") {
		t.Errorf("Expected WRITE: in output, got: %s", result)
	}

	// Verify file was created (ioperf appends .0 for job 0)
	if _, err := os.Stat(filename + ".0"); os.IsNotExist(err) {
		t.Errorf("Expected output file to be created at %s", filename+".0")
	}
}

func TestIoperfCmdReadwriteMode(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_readwrite")

	// Pre-create the file with some data
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	data := make([]byte, 128*1024) // 128KB
	tmpFile.Write(data)
	tmpFile.Close()

	args := []string{
		"--rw=readwrite",
		"--rwmixread=70",
		"--filename=" + filename,
		"--size=64k",
		"--bs=4k",
		"--numjobs=1",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf readwrite mode failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	// readwrite mode should show both READ and WRITE
	if !strings.Contains(result, "READ:") {
		t.Errorf("Expected READ: in output, got: %s", result)
	}
	if !strings.Contains(result, "WRITE:") {
		t.Errorf("Expected WRITE: in output, got: %s", result)
	}
}

// ============== EDGE CASES ==============

func TestIoperfCmdSmallFileSize(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_small")

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=1k",
		"--bs=1k",
		"--numjobs=1",
	}

	// First create a 1k file to read
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	data := make([]byte, 1024)
	tmpFile.Write(data)
	tmpFile.Close()

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf small file size failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "READ:") {
		t.Errorf("Expected READ: in output, got: %s", result)
	}
}

func TestIoperfCmdSingleBlock(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_singleblock")

	// Create a 1k file
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	data := make([]byte, 1024)
	tmpFile.Write(data)
	tmpFile.Close()

	args := []string{
		"--rw=write",
		"--filename=" + filename,
		"--size=1k",
		"--bs=1k",
		"--numjobs=1",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf single block failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "WRITE:") {
		t.Errorf("Expected WRITE: in output, got: %s", result)
	}
}

func TestIoperfCmdZeroIoDepth(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_iodepth0")

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=4k",
		"--bs=4k",
		"--iodepth=0", // Zero iodepth - should use default behavior
		"--numjobs=1",
	}

	// Create the file first
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 4096))
	tmpFile.Close()

	output, err := runIoperfCmd(args)
	// iodepth=0 might cause issues or might be treated as 1
	// Just verify command runs
	if err != nil {
		t.Logf("Note: iodepth=0 returned error (may be expected): %v\nOutput: %s", err, output)
	} else {
		result := string(output)
		if !strings.Contains(result, "READ:") && !strings.Contains(result, "WRITE:") {
			t.Errorf("Expected READ: or WRITE: in output, got: %s", result)
		}
	}
}

func TestIoperfCmdMultipleJobs(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_multi")

	// Pre-create files for each job (job 0 and job 1)
	for i := 0; i < 2; i++ {
		tmpFile, err := os.Create(filename)
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpFile.Write(make([]byte, 256*1024)) // 256KB
		tmpFile.Close()
	}

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=128k",
		"--bs=4k",
		"--numjobs=2",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf multiple jobs failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	// With multiple jobs, we should still see results
	if !strings.Contains(result, "READ:") {
		t.Errorf("Expected READ: in output, got: %s", result)
	}
}

// ============== ERROR CASES ==============

func TestIoperfCmdDeviceFileRejectionNull(t *testing.T) {
	// /dev/null should be rejected as a device file
	args := []string{
		"--rw=write",
		"--filename=/dev/null",
		"--size=1k",
		"--bs=1k",
	}

	output, err := runIoperfCmd(args)
	if err == nil {
		t.Fatalf("Expected error when writing to /dev/null, got none")
	}

	result := string(output)
	if !strings.Contains(result, "device") && !strings.Contains(result, "/dev/") {
		t.Logf("Note: error message: %s", result)
	}
}

func TestIoperfCmdDeviceFileRejectionZero(t *testing.T) {
	// /dev/zero should be rejected as a device file
	args := []string{
		"--rw=read",
		"--filename=/dev/zero",
		"--size=1k",
		"--bs=1k",
	}

	output, err := runIoperfCmd(args)
	if err == nil {
		t.Fatalf("Expected error when using /dev/zero, got none")
	}

	result := string(output)
	if !strings.Contains(result, "device") && !strings.Contains(result, "/dev/") {
		t.Logf("Note: error message: %s", result)
	}
}

func TestIoperfCmdInvalidRwMode(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_invalidrw")

	args := []string{
		"--rw=invalid",
		"--filename=" + filename,
		"--size=1k",
		"--bs=1k",
	}

	output, err := runIoperfCmd(args)
	if err == nil {
		t.Fatalf("Expected error for invalid rw mode, got none")
	}

	result := string(output)
	if !strings.Contains(result, "invalid rw mode") && !strings.Contains(result, "valid:") {
		t.Logf("Note: error message: %s", result)
	}
}

func TestIoperfCmdMissingFilename(t *testing.T) {
	// Default filename is /tmp/ioperf_test which should be rejected as a device file
	// Actually the default is a regular file path that may not exist - let's just test without filename
	// The default filename will be used, so we test with a path that doesn't exist
	args := []string{
		"--rw=read",
		"--size=1k",
		"--bs=1k",
	}

	output, err := runIoperfCmd(args)
	// This will likely fail because the default file doesn't exist for read mode
	// or will work if /tmp/ioperf_test doesn't exist as a read target
	t.Logf("Output: %s, Error: %v", output, err)
}

func TestIoperfCmdNonExistentFile(t *testing.T) {
	// Note: ioperf opens files with O_CREATE, so it will create
	// non-existent files instead of erroring when reading.
	// This test verifies the file gets created.
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "nonexistent_file")

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=1k",
		"--bs=1k",
	}

	output, err := runIoperfCmd(args)
	// The file doesn't exist but will be created due to O_CREATE
	// So no error is expected, but the file should be created
	t.Logf("Output: %s, Error: %v", output, err)

	// Verify the file was created (ioperf creates it)
	if _, err := os.Stat(filename + ".0"); os.IsNotExist(err) {
		t.Errorf("Expected file to be created at %s", filename+".0")
	}
}

func TestIoperfCmdInvalidBlockSize(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile")

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=1k",
		"--bs=invalid",
	}

	output, err := runIoperfCmd(args)
	if err == nil {
		t.Fatalf("Expected error for invalid block size, got none")
	}

	result := string(output)
	if !strings.Contains(result, "invalid block size") {
		t.Errorf("Expected 'invalid block size' in error output, got: %s", result)
	}
}

func TestIoperfCmdInvalidSize(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile")

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=invalid",
		"--bs=1k",
	}

	output, err := runIoperfCmd(args)
	if err == nil {
		t.Fatalf("Expected error for invalid size, got none")
	}

	result := string(output)
	if !strings.Contains(result, "invalid size") && !strings.Contains(result, "size") {
		t.Logf("Note: error message: %s", result)
	}
}

func TestIoperfCmdRwmixreadInvalidRange(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile")

	// rwmixread outside 0-100 should fail
	args := []string{
		"--rw=readwrite",
		"--rwmixread=150",
		"--filename=" + filename,
		"--size=1k",
		"--bs=1k",
	}

	output, err := runIoperfCmd(args)
	if err == nil {
		t.Fatalf("Expected error for rwmixread > 100, got none")
	}

	result := string(output)
	if !strings.Contains(result, "rwmixread") {
		t.Logf("Note: error message: %s", result)
	}
}

func TestIoperfCmdRwmixreadOnlyForReadwrite(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile")

	// rwmixread should only be valid in readwrite mode
	// Using rwmixread=30 (not 50, which is the default that bypasses the check)
	args := []string{
		"--rw=read",
		"--rwmixread=30",
		"--filename=" + filename,
		"--size=1k",
		"--bs=1k",
	}

	output, err := runIoperfCmd(args)
	if err == nil {
		t.Fatalf("Expected error when using rwmixread with read mode, got none")
	}

	result := string(output)
	if !strings.Contains(result, "rwmixread") {
		t.Logf("Note: error message: %s", result)
	}
}

func TestIoperfCmdTimeBasedWithoutRuntime(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile")

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=1k",
		"--bs=1k",
		"--time_based",
		// Missing --runtime
	}

	output, err := runIoperfCmd(args)
	if err == nil {
		t.Fatalf("Expected error when using time_based without runtime, got none")
	}

	result := string(output)
	if !strings.Contains(result, "runtime") {
		t.Logf("Note: error message: %s", result)
	}
}

func TestIoperfCmdTimeBasedMode(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_timebased")

	// Create the file first
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 1024*1024)) // 1MB
	tmpFile.Close()

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--time_based",
		"--runtime=1", // 1 second
		"--bs=4k",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf time_based mode failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "READ:") {
		t.Errorf("Expected READ: in output, got: %s", result)
	}
}

// ============== LATENCY HISTOGRAM TESTS ==============

func TestIoperfCmdLatencyHistogramOutput(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_latency")

	// Create the file first
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 256*1024)) // 256KB
	tmpFile.Close()

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=128k",
		"--bs=4k",
		"--latency",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf latency histogram failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	// Check for latency histogram output
	if !strings.Contains(result, "Latency histogram") {
		t.Errorf("Expected 'Latency histogram' in output, got: %s", result)
	}
	if !strings.Contains(result, "READ latency distribution") {
		t.Errorf("Expected 'READ latency distribution' in output, got: %s", result)
	}
	if !strings.Contains(result, "Bucket") && !strings.Contains(result, "Count") {
		t.Logf("Note: histogram format may vary, output: %s", result)
	}
}

func TestIoperfCmdLatencyHistogramWrite(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_latency_write")

	args := []string{
		"--rw=write",
		"--filename=" + filename,
		"--size=64k",
		"--bs=4k",
		"--latency",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf latency histogram for write failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	// Check for latency histogram output
	if !strings.Contains(result, "Latency histogram") {
		t.Errorf("Expected 'Latency histogram' in output, got: %s", result)
	}
	if !strings.Contains(result, "WRITE latency distribution") {
		t.Errorf("Expected 'WRITE latency distribution' in output, got: %s", result)
	}
}

func TestIoperfCmdLatencyWithPercentile(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_latency_pct")

	// Create the file first
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 256*1024)) // 256KB
	tmpFile.Close()

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=128k",
		"--bs=4k",
		"--percentile=95",
		"--latency",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf latency with percentile failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	// Should show p95 in the output
	if !strings.Contains(result, "p95=") && !strings.Contains(result, "95") {
		t.Logf("Note: percentile format may vary, output: %s", result)
	}
}

// ============== HELP FLAG TEST ==============

func TestIoperfCmdHelp(t *testing.T) {
	args := []string{"--help"}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf --help failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information, got: %s", result)
	}
	if !strings.Contains(result, "ioperf") {
		t.Errorf("Expected 'ioperf' in help output, got: %s", result)
	}
}

// ============== DIRECT I/O TESTS ==============

func TestIoperfCmdDirectIO(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_direct")

	// Create the file first
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 256*1024)) // 256KB
	tmpFile.Close()

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=128k",
		"--bs=4k",
		"--direct=1",
	}

	output, err := runIoperfCmd(args)
	// O_DIRECT may fail on certain filesystems or with certain alignments
	if err != nil {
		t.Logf("Note: direct I/O may fail on some filesystems: %v\nOutput: %s", err, output)
	} else {
		result := string(output)
		if !strings.Contains(result, "READ:") {
			t.Errorf("Expected READ: in output, got: %s", result)
		}
	}
}

// ============== BLOCK SIZE PARSING TESTS ==============

func TestIoperfCmdVariousBlockSizes(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_bs")

	// Create the file first
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 512*1024)) // 512KB
	tmpFile.Close()

	// Test different block size formats
	bsSizes := []string{"1k", "4k", "128k", "1M"}

	for _, bs := range bsSizes {
		args := []string{
			"--rw=read",
			"--filename=" + filename,
			"--size=64k",
			"--bs=" + bs,
		}

		output, err := runIoperfCmd(args)
		if err != nil {
			t.Fatalf("ioperf with bs=%s failed: %v\nOutput: %s", bs, err, output)
		}

		result := string(output)
		if !strings.Contains(result, "READ:") {
			t.Errorf("Expected READ: with bs=%s, got: %s", bs, result)
		}
	}
}

// ============== SIZE PARSING TESTS ==============

func TestIoperfCmdVariousSizes(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_size")

	// Create the file with enough data
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 4*1024*1024)) // 4MB
	tmpFile.Close()

	// Test different size formats
	sizes := []string{"1k", "64k", "1M", "4M"}

	for _, size := range sizes {
		args := []string{
			"--rw=read",
			"--filename=" + filename,
			"--size=" + size,
			"--bs=4k",
		}

		output, err := runIoperfCmd(args)
		if err != nil {
			t.Fatalf("ioperf with size=%s failed: %v\nOutput: %s", size, err, output)
		}

		result := string(output)
		if !strings.Contains(result, "READ:") && !strings.Contains(result, "WRITE:") {
			t.Errorf("Expected results with size=%s, got: %s", size, result)
		}
	}
}

// ============== RATE LIMIT TESTS ==============

func TestIoperfCmdRateLimit(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_rate")

	// Create the file first
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 256*1024)) // 256KB
	tmpFile.Close()

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=128k",
		"--bs=4k",
		"--rate=1M",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf rate limit failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "READ:") {
		t.Errorf("Expected READ: in output, got: %s", result)
	}
}

// ============== OUTPUT FORMAT TESTS ==============

func TestIoperfCmdOutputFormat(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_format")

	// Create the file first
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 256*1024)) // 256KB
	tmpFile.Close()

	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=128k",
		"--bs=4k",
		"--numjobs=1",
		"--iodepth=1",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf output format failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	// Check header contains job and iodepth info
	if !strings.Contains(result, "bs=") {
		t.Errorf("Expected bs= in header output, got: %s", result)
	}
	if !strings.Contains(result, "jobs=") {
		t.Errorf("Expected jobs= in header output, got: %s", result)
	}
	if !strings.Contains(result, "iodepth=") {
		t.Errorf("Expected iodepth= in header output, got: %s", result)
	}
}

// ============== FSYNC TESTS ==============

func TestIoperfCmdFsync(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_fsync")

	args := []string{
		"--rw=write",
		"--filename=" + filename,
		"--size=64k",
		"--bs=4k",
		"--fsync=1",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf fsync failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "WRITE:") {
		t.Errorf("Expected WRITE: in output, got: %s", result)
	}
}

// ============== SYNC FLAG TESTS ==============

func TestIoperfCmdSyncFlag(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_syncflag")

	args := []string{
		"--rw=write",
		"--filename=" + filename,
		"--size=64k",
		"--bs=4k",
		"--sync=1",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf sync flag failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "WRITE:") {
		t.Errorf("Expected WRITE: in output, got: %s", result)
	}
}

// ============== NON-LINUX ERROR TEST ==============

func TestIoperfCmdLinuxOnly(t *testing.T) {
	// This test verifies that the command checks for Linux
	// On non-Linux systems, this would return an error
	// Since we're running on Linux, the command should work
	// This test is more of a placeholder to document the Linux-only behavior

	if runtime.GOOS != "linux" {
		t.Skip("ioperf is Linux-only")
	}
}

