package disk

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// runIoperfCmd runs IoperfCmd and captures stdout and stderr
func runIoperfCmd(args []string) (string, error) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	err := IoperfCmd(args)

	wOut.Close()
	wErr.Close()
	io.Copy(&buf, rOut)
	io.Copy(&buf, rErr)

	// If there was an error, also include it in the output for debugging
	if err != nil {
		buf.WriteString(err.Error())
	}

	os.Stdout = oldStdout
	os.Stderr = oldStderr
	return buf.String(), err
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

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Errorf("Expected output file to be created at %s", filename)
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

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Errorf("Expected output file to be created at %s", filename)
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

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Errorf("Expected file to be created at %s", filename)
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
		"--write_hist_log=" + filepath.Join(tmpDir, "latency_read"),
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
	if _, err := os.Stat(filepath.Join(tmpDir, "latency_read_read_hist.1.log")); err != nil {
		t.Errorf("Expected histogram log to be created: %v", err)
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
		"--write_hist_log=" + filepath.Join(tmpDir, "latency_write"),
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
	if _, err := os.Stat(filepath.Join(tmpDir, "latency_write_write_hist.1.log")); err != nil {
		t.Errorf("Expected histogram log to be created: %v", err)
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
		"--percentile_list=95",
		"--write_hist_log=" + filepath.Join(tmpDir, "latency_pct"),
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
	if !strings.Contains(result, "Usage: gobox ioperf [OPTION]...") {
		t.Errorf("Expected canonical usage line, got: %s", result)
	}
	if !strings.Contains(result, "ioperf") {
		t.Errorf("Expected 'ioperf' in help output, got: %s", result)
	}
	for _, want := range []string{"Workload:", "Parallelism and rate:", "I/O behavior:", "Latency and histograms:", "--percentile_list LIST"} {
		if !strings.Contains(result, want) {
			t.Fatalf("expected help output to contain %q, got: %s", want, result)
		}
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

// Bug regression test: --rate must actually throttle throughput. Previously
// the parsed rate limit was converted into an "ops" threshold and only
// triggered a fixed 1ms sleep every N ops, which barely slowed anything
// down. Verify measured bandwidth stays within a generous multiple of the
// configured rate instead of running unbounded.
func TestIoperfCmdRateLimitThrottlesThroughput(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_rate_throttle")

	const rateBytesPerSec = 64 * 1024 // 64KB/s
	args := []string{
		"--rw=write",
		"--filename=" + filename,
		"--size=96k",
		"--bs=4k",
		"--rate=64k",
		"--numjobs=1",
	}

	start := time.Now()
	output, err := runIoperfCmd(args)
	elapsed := time.Since(start).Seconds()
	if err != nil {
		t.Fatalf("ioperf rate throttle failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "WRITE:") {
		t.Fatalf("Expected WRITE: in output, got: %s", result)
	}

	bwMB := parseBWFromOutput(t, result, "WRITE:")
	measuredBps := bwMB * 1024 * 1024

	// Generous tolerance: measured bandwidth should stay under 3x the
	// configured rate limit. Before the fix this ran at full disk speed,
	// tens of MB/s vs. the 64KB/s target - orders of magnitude over.
	maxAllowed := float64(rateBytesPerSec) * 3
	if measuredBps > maxAllowed {
		t.Fatalf("measured bandwidth %.0f B/s exceeds tolerance %.0f B/s for --rate=64k (elapsed=%.2fs)", measuredBps, maxAllowed, elapsed)
	}
}

// parseBWFromOutput extracts the BW=<value>MB/s figure following the given
// prefix (e.g. "WRITE:") from ioperf's textual output.
func parseBWFromOutput(t *testing.T, output, prefix string) float64 {
	t.Helper()
	idx := strings.Index(output, prefix)
	if idx < 0 {
		t.Fatalf("prefix %q not found in output: %s", prefix, output)
	}
	line := output[idx:]
	if nl := strings.IndexByte(line, '\n'); nl >= 0 {
		line = line[:nl]
	}
	bwIdx := strings.Index(line, "BW=")
	if bwIdx < 0 {
		t.Fatalf("BW= not found in line: %s", line)
	}
	rest := line[bwIdx+len("BW="):]
	mbIdx := strings.Index(rest, "MB/s")
	if mbIdx < 0 {
		t.Fatalf("MB/s not found in line: %s", line)
	}
	valueStr := rest[:mbIdx]
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		t.Fatalf("failed to parse BW value %q: %v", valueStr, err)
	}
	return value
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
		"--sync=sync",
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
