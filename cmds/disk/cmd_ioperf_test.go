package disk

import (
	"bytes"
	"fmt"
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

	// Pre-create the file with some data, large enough for ~256 4k ops so
	// the probabilistic read/write split has a statistically stable ratio.
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	data := make([]byte, 1024*1024) // 1MB
	tmpFile.Write(data)
	tmpFile.Close()

	args := []string{
		"--rw=readwrite",
		"--rwmixread=70",
		"--filename=" + filename,
		"--size=1M",
		"--bs=4k",
		"--numjobs=1",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf readwrite mode failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	readIOPS := parseIOPSFromOutput(t, result, "READ:")
	writeIOPS := parseIOPSFromOutput(t, result, "WRITE:")
	if readIOPS <= 0 || writeIOPS <= 0 {
		t.Fatalf("expected both nonzero READ and WRITE IOPS in readwrite mode, got read=%v write=%v: %s", readIOPS, writeIOPS, result)
	}
	// IOPS is ops/duration over the same window for both lines, so the ratio
	// of IOPS equals the ratio of op counts regardless of duration. With
	// ~256 ops split by a 70% Bernoulli draw the sample ratio should land
	// close to 0.70; a hardcoded 50/50 split (ignoring --rwmixread) or a
	// silently-dropped flag would fall well outside this band.
	ratio := readIOPS / (readIOPS + writeIOPS)
	if ratio < 0.55 || ratio > 0.85 {
		t.Fatalf("expected --rwmixread=70 to produce a read ratio near 0.70, got %.2f (readIOPS=%.0f writeIOPS=%.0f): %s", ratio, readIOPS, writeIOPS, result)
	}
}

// parseIOPSFromOutput extracts the IOPS=<value> figure following the given
// prefix (e.g. "WRITE:") from ioperf's textual output.
func parseIOPSFromOutput(t *testing.T, output, prefix string) float64 {
	t.Helper()
	idx := strings.Index(output, prefix)
	if idx < 0 {
		t.Fatalf("prefix %q not found in output: %s", prefix, output)
	}
	line := output[idx:]
	if nl := strings.IndexByte(line, '\n'); nl >= 0 {
		line = line[:nl]
	}
	iopsIdx := strings.Index(line, "IOPS=")
	if iopsIdx < 0 {
		t.Fatalf("IOPS= not found in line: %s", line)
	}
	rest := line[iopsIdx+len("IOPS="):]
	commaIdx := strings.Index(rest, ",")
	if commaIdx < 0 {
		t.Fatalf("comma after IOPS value not found in line: %s", line)
	}
	valueStr := rest[:commaIdx]
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		t.Fatalf("failed to parse IOPS value %q: %v", valueStr, err)
	}
	return value
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
	// With --numjobs=N>1, each job writes its own filename.<jobID> and
	// independently performs the full --size worth of I/O (workload is not
	// divided across jobs). Use write mode so we can deterministically prove
	// two jobs actually ran by checking both per-job files were created at
	// the full requested size, rather than inferring it from timing/IOPS.
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_multi")
	const sizeBytes = 128 * 1024

	args := []string{
		"--rw=write",
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
	if !strings.Contains(result, "WRITE:") {
		t.Fatalf("Expected WRITE: in output, got: %s", result)
	}
	for _, jobID := range []int{0, 1} {
		jobFile := fmt.Sprintf("%s.%d", filename, jobID)
		info, err := os.Stat(jobFile)
		if err != nil {
			t.Fatalf("expected --numjobs=2 to create per-job file %s, got: %v", jobFile, err)
		}
		if info.Size() != sizeBytes {
			t.Errorf("expected %s to contain the full requested %d bytes (job did not silently skip work), got %d", jobFile, sizeBytes, info.Size())
		}
	}
}

func TestIoperfCmdGroupReportingAggregatesJobs(t *testing.T) {
	run := func(t *testing.T, extraArgs ...string) string {
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "testfile_group")
		args := append([]string{
			"--rw=write",
			"--filename=" + filename,
			"--size=64k",
			"--bs=4k",
			"--numjobs=2",
		}, extraArgs...)
		output, err := runIoperfCmd(args)
		if err != nil {
			t.Fatalf("ioperf failed: %v\nOutput: %s", err, output)
		}
		return output
	}

	perJob := run(t)
	if got := strings.Count(perJob, "job "); got != 2 {
		t.Fatalf("expected 2 per-job report headers without --group_reporting, got %d: %s", got, perJob)
	}

	aggregated := run(t, "--group_reporting")
	if strings.Contains(aggregated, "job ") {
		t.Fatalf("expected --group_reporting to aggregate away per-job headers, got: %s", aggregated)
	}
	if got := strings.Count(aggregated, "WRITE:"); got != 1 {
		t.Fatalf("expected exactly one aggregated WRITE: line with --group_reporting, got %d: %s", got, aggregated)
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

	_, err := runIoperfCmd(args)
	if err == nil {
		t.Fatalf("Expected error for invalid rw mode, got none")
	}
	if !strings.Contains(err.Error(), "invalid rw mode") {
		t.Errorf("expected error to mention \"invalid rw mode\", got: %v", err)
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

	_, err := runIoperfCmd(args)
	if err == nil {
		t.Fatalf("Expected error for rwmixread > 100, got none")
	}
	if !strings.Contains(err.Error(), "rwmixread") {
		t.Errorf("expected error to mention \"rwmixread\", got: %v", err)
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

	_, err := runIoperfCmd(args)
	if err == nil {
		t.Fatalf("Expected error when using rwmixread with read mode, got none")
	}
	if !strings.Contains(err.Error(), "rwmixread") {
		t.Errorf("expected error to mention \"rwmixread\", got: %v", err)
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
	if !strings.Contains(result, "p95=") {
		t.Fatalf("expected %q in ioperf output, got: %s", "p95=", result)
	}
}

// TestIoperfCmdLatencyPercentileListMultiValue covers the documented
// "95:99" multi-value syntax, which TestIoperfCmdLatencyWithPercentile does
// not exercise (it only ever passes a single percentile).
func TestIoperfCmdLatencyPercentileListMultiValue(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_latency_multi")
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 256*1024))
	tmpFile.Close()

	output, err := runIoperfCmd([]string{
		"--rw=read",
		"--filename=" + filename,
		"--size=128k",
		"--bs=4k",
		"--percentile_list=95:99",
	})
	if err != nil {
		t.Fatalf("ioperf multi-value percentile_list failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	p95 := extractPercentileLatency(t, result, "p95=")
	p99 := extractPercentileLatency(t, result, "p99=")
	// p99 latency is the tail of the same distribution p95 is drawn from, so
	// it must be >= p95, not just "some other number present somewhere".
	if p99 < p95 {
		t.Fatalf("expected p99 latency (%.2f) >= p95 latency (%.2f), got reversed order: %s", p99, p95, result)
	}
}

// TestIoperfCmdLegacyPercentileAlias verifies --percentile=N (the
// documented legacy alias) is equivalent to --percentile_list=N, rather
// than being silently ignored.
func TestIoperfCmdLegacyPercentileAlias(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_latency_legacy")
	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 256*1024))
	tmpFile.Close()

	output, err := runIoperfCmd([]string{
		"--rw=read",
		"--filename=" + filename,
		"--size=128k",
		"--bs=4k",
		"--percentile=99",
	})
	if err != nil {
		t.Fatalf("ioperf legacy --percentile alias failed: %v\nOutput: %s", err, output)
	}
	if !strings.Contains(string(output), "p99=") {
		t.Fatalf("expected --percentile=99 (legacy alias for --percentile_list=99) to report p99=, got: %s", output)
	}
}

// extractPercentileLatency parses the "pXX=YY.YYus" figure with the given
// label prefix (e.g. "p95=") out of ioperf's latency output.
func extractPercentileLatency(t *testing.T, output, label string) float64 {
	t.Helper()
	idx := strings.Index(output, label)
	if idx < 0 {
		t.Fatalf("expected %q in ioperf output, got: %s", label, output)
	}
	rest := output[idx+len(label):]
	usIdx := strings.Index(rest, "us")
	if usIdx < 0 {
		t.Fatalf("expected %q to be followed by a %q value, got: %s", label, "us", output)
	}
	value, err := strconv.ParseFloat(rest[:usIdx], 64)
	if err != nil {
		t.Fatalf("failed to parse latency value after %q: %v", label, err)
	}
	return value
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
	// Regression coverage for the read-mode side of the rate limiter: the
	// write-mode throttle is proven by TestIoperfCmdRateLimitThrottlesThroughput,
	// but the read path has its own bandwidth accounting and could have its
	// own bug (or never call into the limiter at all) without that test
	// noticing.
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "testfile_rate")

	tmpFile, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Write(make([]byte, 256*1024)) // 256KB
	tmpFile.Close()

	const rateBytesPerSec = 64 * 1024 // 64KB/s
	args := []string{
		"--rw=read",
		"--filename=" + filename,
		"--size=96k",
		"--bs=4k",
		"--rate=64k",
	}

	output, err := runIoperfCmd(args)
	if err != nil {
		t.Fatalf("ioperf rate limit failed: %v\nOutput: %s", err, output)
	}

	result := string(output)
	if !strings.Contains(result, "READ:") {
		t.Fatalf("Expected READ: in output, got: %s", result)
	}
	bwMB := parseBWFromOutput(t, result, "READ:")
	measuredBps := bwMB * 1024 * 1024
	maxAllowed := float64(rateBytesPerSec) * 3
	if measuredBps > maxAllowed {
		t.Fatalf("measured read bandwidth %.0f B/s exceeds tolerance %.0f B/s for --rate=64k", measuredBps, maxAllowed)
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
