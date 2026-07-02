package disk

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// O_DIRECT flag value for Linux
const O_DIRECT = 0x40000

// O_DSYNC flag value for Linux
const O_DSYNC = 0x1000

// IoperfCmd implements an I/O performance benchmark tool, simplified fio-like.
func IoperfCmd(args []string) error {
	fsFlags := flag.NewFlagSet("ioperf", flag.ContinueOnError)
	fsFlags.SetOutput(os.Stderr)
	rwMode := fsFlags.String("rw", "read", "I/O mode: read, write, randread, randwrite, readwrite")
	rwMixRead := fsFlags.Int("rwmixread", 50, "read ratio (0-100) for readwrite mode")
	filename := fsFlags.String("filename", "/tmp/ioperf_test", "test file path (jobs create filename.0, filename.1, ...)")
	blockSize := fsFlags.String("bs", "4k", "block size (e.g., 4k, 8k, 128k)")
	totalSize := fsFlags.String("size", "1G", "total I/O data size (e.g., 1G, 10G)")
	numJobs := fsFlags.Int("numjobs", 1, "parallel job count")
	ioDepth := fsFlags.Int("iodepth", 1, "queue depth")
	direct := fsFlags.Int("direct", 0, "use O_DIRECT to bypass cache (0 or 1)")
	fsync := fsFlags.Int("fsync", 0, "execute fsync after each write (0 or 1)")
	syncMode := fsFlags.String("sync", "none", "use synchronous write IO: none|sync|dsync|0|1")
	rate := fsFlags.String("rate", "", "rate limit (e.g., 100M)")
	timeBased := fsFlags.Bool("time_based", false, "run based on time")
	runtimeSec := fsFlags.Int("runtime", 0, "runtime in seconds (for time_based mode)")
	groupReporting := fsFlags.Bool("group_reporting", false, "aggregate multi-job reports")
	percentile := fsFlags.Int("percentile", 0, "legacy alias for percentile_list with a single percentile (e.g., 99)")
	percentileList := fsFlags.String("percentile_list", "", "fio-compatible latency percentile list (e.g., 95 or 95:99)")
	latency := fsFlags.Bool("latency", false, "output latency distribution histogram")
	writeHistLog := fsFlags.String("write_hist_log", "", "fio-compatible histogram log prefix")

	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox ioperf [OPTION]...")
		fmt.Fprintln(os.Stderr, "I/O performance benchmark tool, simplified fio-like")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Workload:")
		fmt.Fprintln(os.Stderr, "  --rw MODE                 I/O mode: read, write, randread, randwrite, readwrite")
		fmt.Fprintln(os.Stderr, "  --rwmixread N             read ratio for readwrite mode")
		fmt.Fprintln(os.Stderr, "  --filename PATH           test file path")
		fmt.Fprintln(os.Stderr, "  --bs SIZE                 block size")
		fmt.Fprintln(os.Stderr, "  --size SIZE               total I/O data size")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Parallelism and rate:")
		fmt.Fprintln(os.Stderr, "  --numjobs N               parallel job count")
		fmt.Fprintln(os.Stderr, "  --iodepth N               queue depth")
		fmt.Fprintln(os.Stderr, "  --rate RATE               rate limit")
		fmt.Fprintln(os.Stderr, "  --time_based              run based on time")
		fmt.Fprintln(os.Stderr, "  --runtime SEC             runtime in seconds for time_based mode")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "I/O behavior:")
		fmt.Fprintln(os.Stderr, "  --direct 0|1              use O_DIRECT to bypass cache")
		fmt.Fprintln(os.Stderr, "  --fsync 0|1               execute fsync after each write")
		fmt.Fprintln(os.Stderr, "  --sync MODE               synchronous write mode: none|sync|dsync|0|1")
		fmt.Fprintln(os.Stderr, "  --group_reporting         aggregate multi-job reports")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Latency and histograms:")
		fmt.Fprintln(os.Stderr, "  --percentile N            single percentile alias")
		fmt.Fprintln(os.Stderr, "  --percentile_list LIST    fio-compatible latency percentile list")
		fmt.Fprintln(os.Stderr, "  --latency                 output latency distribution histogram")
		fmt.Fprintln(os.Stderr, "  --write_hist_log PREFIX   fio-compatible histogram log prefix")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gobox ioperf --rw=write --filename=/tmp/testfile --size=1G --bs=4k")
		fmt.Fprintln(os.Stderr, "  gobox ioperf --rw=randread --filename=/tmp/testfile --size=1G --numjobs=4 --direct=1")
		fmt.Fprintln(os.Stderr, "  gobox ioperf --rw=readwrite --rwmixread=70 --filename=/tmp/testfile --size=1G --numjobs=4 --iodepth=4")
	}

	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if runtime.GOOS != "linux" {
		return fmt.Errorf("ioperf: supported only on Linux")
	}

	// Validate rw mode
	validModes := map[string]bool{"read": true, "write": true, "randread": true, "randwrite": true, "readwrite": true}
	if !validModes[*rwMode] {
		return fmt.Errorf("ioperf: invalid rw mode %q (valid: read, write, randread, randwrite, readwrite)", *rwMode)
	}

	// Parse block size
	bsBytes, err := parseSize(*blockSize)
	if err != nil {
		return fmt.Errorf("ioperf: invalid block size: %v", err)
	}
	if bsBytes <= 0 {
		return fmt.Errorf("ioperf: block size must be positive")
	}

	// Parse total size
	sizeBytes, err := parseSize(*totalSize)
	if err != nil {
		return fmt.Errorf("ioperf: invalid size: %v", err)
	}
	if sizeBytes <= 0 {
		return fmt.Errorf("ioperf: total size must be positive")
	}

	// Parse rate limit
	var rateLimitBytes int64
	if *rate != "" {
		rateLimitBytes, err = parseSize(*rate)
		if err != nil {
			return fmt.Errorf("ioperf: invalid rate limit: %v", err)
		}
	}

	// Validate rwmixread
	if *rwMixRead < 0 || *rwMixRead > 100 {
		return fmt.Errorf("ioperf: rwmixread must be between 0 and 100")
	}
	if *rwMode != "readwrite" && *rwMixRead != 50 {
		return fmt.Errorf("ioperf: rwmixread is only valid in readwrite mode")
	}

	var syncFileFlag int
	switch strings.ToLower(strings.TrimSpace(*syncMode)) {
	case "", "none", "0":
		syncFileFlag = 0
	case "sync", "1":
		syncFileFlag = os.O_SYNC
	case "dsync":
		syncFileFlag = O_DSYNC
	default:
		return fmt.Errorf("ioperf: invalid sync mode %q (valid: none, 0, sync, 1, dsync)", *syncMode)
	}

	latencyPercentiles, err := parsePercentileList(*percentileList, *percentile)
	if err != nil {
		return fmt.Errorf("ioperf: invalid percentile_list: %v", err)
	}
	if len(latencyPercentiles) == 0 {
		latencyPercentiles = []float64{99}
	}
	histogramEnabled := *latency || *writeHistLog != ""

	// Validate time-based parameters
	if *timeBased && *runtimeSec <= 0 {
		return fmt.Errorf("ioperf: runtime must be specified with time_based mode")
	}
	if !*timeBased && sizeBytes <= 0 {
		return fmt.Errorf("ioperf: size must be specified")
	}

	// Ensure filename is not a device file
	if isDevicePath(*filename) {
		return fmt.Errorf("ioperf: refusing to write to device file %q", *filename)
	}

	// Calculate total I/O operations
	// groupReporting is always enabled - we aggregate all job results by default
	_ = groupReporting

	var totalOps int64
	if *timeBased {
		totalOps = math.MaxInt64 // Run until runtime expires
	} else {
		totalOps = int64(sizeBytes / bsBytes)
	}

	// Create job results channel
	type jobResult struct {
		jobID          int
		readOps        int64
		writeOps       int64
		readBytes      int64
		writeBytes     int64
		readLatencies  []int64 // nanoseconds
		writeLatencies []int64
	}

	resultChan := make(chan jobResult, *numJobs)
	var wg sync.WaitGroup

	// Determine effective runtime
	runDuration := time.Duration(*runtimeSec) * time.Second

	benchStart := time.Now()

	// Launch jobs
	for jobID := 0; jobID < *numJobs; jobID++ {
		wg.Add(1)
		go func(jid int) {
			defer wg.Done()
			result := jobResult{jobID: jid}

			// Create job-specific filename
			jobFilename := *filename
			if *numJobs > 1 {
				jobFilename = fmt.Sprintf("%s.%d", *filename, jid)
			}

			// Open file for this job
			fileFlags := os.O_RDWR | os.O_CREATE
			if *direct == 1 {
				fileFlags |= O_DIRECT
			}
			if syncFileFlag != 0 {
				fileFlags |= syncFileFlag
			}
			file, err := os.OpenFile(jobFilename, fileFlags, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ioperf: job %d: failed to open %s: %v\n", jid, jobFilename, err)
				resultChan <- result
				return
			}
			defer file.Close()

			// Pre-allocate file if writing
			if *rwMode == "write" || *rwMode == "randwrite" || *rwMode == "readwrite" {
				if err := file.Truncate(sizeBytes); err != nil {
					fmt.Fprintf(os.Stderr, "ioperf: job %d: truncate %s: %v\n", jid, jobFilename, err)
					resultChan <- result
					return
				}
			}

			// Allocate buffer for I/O
			buf := make([]byte, bsBytes)
			for i := range buf {
				buf[i] = byte(i % 256)
			}

			// Track position for sequential modes
			var pos int64 = 0
			maxPos := sizeBytes - bsBytes
			if maxPos < 0 {
				maxPos = 0
			}

			// Track ops for time-based mode
			var opsCompleted int64
			startTime := time.Now()
			deadline := startTime.Add(runDuration)

			// Rate limiting
			var rateLimitOps int64
			if rateLimitBytes > 0 {
				rateLimitOps = rateLimitBytes / bsBytes
				if rateLimitOps < 1 {
					rateLimitOps = 1
				}
			}

			// Random for randread/randwrite
			rng := NewRand(int64(jid)*1234567 + time.Now().UnixNano())

			depth := *ioDepth
			if depth < 1 {
				depth = 1
			}

			// Main I/O loop
			for {
				// Check exit conditions
				if *timeBased {
					if time.Now().After(deadline) {
						break
					}
				} else {
					if atomic.LoadInt64(&opsCompleted) >= totalOps {
						break
					}
				}

				// Rate limiting check
				if rateLimitOps > 0 {
					currentOps := atomic.LoadInt64(&opsCompleted)
					if currentOps > 0 && currentOps%rateLimitOps == 0 {
						time.Sleep(1 * time.Millisecond)
					}
				}

				for qd := 0; qd < depth; qd++ {
					if *timeBased {
						if time.Now().After(deadline) {
							break
						}
					} else if atomic.LoadInt64(&opsCompleted) >= totalOps {
						break
					}

					// Determine operation type for readwrite mode
					var isRead bool
					switch *rwMode {
					case "read", "randread":
						isRead = true
					case "write", "randwrite":
						isRead = false
					case "readwrite":
						isRead = (rng.Int63n(100) < int64(*rwMixRead))
					}

					// Calculate offset
					var offset int64
					if *rwMode == "randread" || *rwMode == "randwrite" {
						offset = (rng.Int63n(maxPos+1) / bsBytes) * bsBytes
					} else {
						offset = pos
						pos += bsBytes
						if pos >= sizeBytes {
							pos = 0
						}
					}

					// Perform I/O
					ioStart := time.Now()
					var n int
					var ioErr error

					if isRead {
						n, ioErr = file.ReadAt(buf, offset)
						if ioErr == io.EOF {
							ioErr = nil
						}
					} else {
						n, ioErr = file.WriteAt(buf, offset)
						if *fsync == 1 && ioErr == nil {
							file.Sync()
						}
					}
					ioDuration := time.Since(ioStart).Nanoseconds()

					atomic.AddInt64(&opsCompleted, 1)

					if ioErr != nil {
						if isRead && ioErr != io.EOF {
							continue
						}
						continue
					}

					// Record stats (result is goroutine-local, no atomic needed)
					if isRead {
						result.readOps++
						result.readBytes += int64(n)
						result.readLatencies = append(result.readLatencies, ioDuration)
					} else {
						result.writeOps++
						result.writeBytes += int64(n)
						result.writeLatencies = append(result.writeLatencies, ioDuration)
					}
				}
			}

			resultChan <- result
		}(jobID)
	}

	// Wait for all jobs and collect results
	wg.Wait()
	close(resultChan)

	// Aggregate results
	var results []jobResult
	var totalReadOps, totalWriteOps int64
	var totalReadBytes, totalWriteBytes int64
	var allReadLatencies, allWriteLatencies []int64

	for result := range resultChan {
		results = append(results, result)
		totalReadOps += result.readOps
		totalWriteOps += result.writeOps
		totalReadBytes += result.readBytes
		totalWriteBytes += result.writeBytes
		allReadLatencies = append(allReadLatencies, result.readLatencies...)
		allWriteLatencies = append(allWriteLatencies, result.writeLatencies...)
	}

	// Calculate duration
	elapsed := time.Since(benchStart).Seconds()
	duration := elapsed
	if *timeBased && *runtimeSec > 0 {
		duration = float64(*runtimeSec)
	} else if elapsed < 0.001 {
		duration = 1.0
	}

	// Calculate latency stats
	calcLatencyStats := func(latencies []int64, percentiles []float64) (avg float64, pVals map[float64]float64) {
		if len(latencies) == 0 {
			return 0, map[float64]float64{}
		}
		var sum int64
		for _, l := range latencies {
			sum += l
		}
		avg = float64(sum) / float64(len(latencies))

		sorted := make([]int64, len(latencies))
		copy(sorted, latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		pVals = make(map[float64]float64, len(percentiles))
		for _, percentile := range percentiles {
			if percentile <= 0 {
				continue
			}
			idx := int(math.Ceil(float64(len(sorted))*percentile/100.0)) - 1
			if idx < 0 {
				idx = 0
			}
			if idx >= len(sorted) {
				idx = len(sorted) - 1
			}
			pVals[percentile] = float64(sorted[idx])
		}
		return
	}

	// Format latency for output
	formatLat := func(avg float64, pVals map[float64]float64, percentiles []float64) string {
		if len(pVals) == 0 {
			return fmt.Sprintf("avg=%.2fus", avg/1000)
		}
		parts := []string{fmt.Sprintf("avg=%.2fus", avg/1000)}
		for _, percentile := range percentiles {
			if pVal, ok := pVals[percentile]; ok {
				parts = append(parts, fmt.Sprintf("p%s=%.2fus", formatPercentile(percentile), pVal/1000))
			}
		}
		return strings.Join(parts, ", ")
	}

	// Print header
	fmt.Printf("ioperf: bs=%s, jobs=%d, iodepth=%d\n", *blockSize, *numJobs, *ioDepth)

	printResult := func(prefix string, readOps, writeOps, readBytes, writeBytes int64, readLatencies, writeLatencies []int64) {
		localReadBW := float64(readBytes) / (1024 * 1024) / duration
		localWriteBW := float64(writeBytes) / (1024 * 1024) / duration
		localReadIOPS := float64(readOps) / duration
		localWriteIOPS := float64(writeOps) / duration

		if readOps > 0 || *rwMode == "read" || *rwMode == "randread" || *rwMode == "readwrite" {
			avgLat, pLat := calcLatencyStats(readLatencies, latencyPercentiles)
			fmt.Printf("%sREAD:  IOPS=%.0f, BW=%.2fMB/s, lat=%s\n", prefix, localReadIOPS, localReadBW, formatLat(avgLat, pLat, latencyPercentiles))
		}
		if writeOps > 0 || *rwMode == "write" || *rwMode == "randwrite" || *rwMode == "readwrite" {
			avgLat, pLat := calcLatencyStats(writeLatencies, latencyPercentiles)
			fmt.Printf("%sWRITE: IOPS=%.0f, BW=%.2fMB/s, lat=%s\n", prefix, localWriteIOPS, localWriteBW, formatLat(avgLat, pLat, latencyPercentiles))
		}
	}

	if *groupReporting {
		printResult("", totalReadOps, totalWriteOps, totalReadBytes, totalWriteBytes, allReadLatencies, allWriteLatencies)
	} else {
		for _, result := range results {
			fmt.Printf("job %d:\n", result.jobID)
			printResult("  ", result.readOps, result.writeOps, result.readBytes, result.writeBytes, result.readLatencies, result.writeLatencies)
		}
	}

	// Print latency histogram if requested
	if histogramEnabled {
		fmt.Println("\nLatency histogram (us):")
		if len(allReadLatencies) > 0 {
			printLatencyHistogram("READ", allReadLatencies)
		}
		if len(allWriteLatencies) > 0 {
			printLatencyHistogram("WRITE", allWriteLatencies)
		}
	}
	if *writeHistLog != "" {
		if len(allReadLatencies) > 0 {
			if err := writeLatencyHistogramLog(*writeHistLog, "read", allReadLatencies); err != nil {
				return err
			}
		}
		if len(allWriteLatencies) > 0 {
			if err := writeLatencyHistogramLog(*writeHistLog, "write", allWriteLatencies); err != nil {
				return err
			}
		}
	}

	return nil
}

// Simple pseudo-random number generator for reproducible results
type Rand struct {
	seed int64
}

func NewRand(seed int64) *Rand {
	return &Rand{seed: seed}
}

func (r *Rand) Int63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	// Linear congruential generator
	r.seed = (r.seed*6364136223846793005 + 1) & 0x7fffffffffffffff
	return r.seed % n
}

// parseSize parses size strings like "4k", "128M", "1G" into bytes
func parseSize(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}

	var multiplier int64 = 1
	s = strings.ToUpper(strings.TrimSpace(s))

	switch {
	case len(s) > 2 && s[len(s)-2:] == "KB":
		multiplier = 1024
		s = s[:len(s)-2]
	case len(s) > 2 && s[len(s)-2:] == "MB":
		multiplier = 1024 * 1024
		s = s[:len(s)-2]
	case len(s) > 2 && s[len(s)-2:] == "GB":
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-2]
	case len(s) > 2 && s[len(s)-2:] == "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-2]
	case len(s) > 1 && s[len(s)-1:] == "K":
		multiplier = 1024
		s = s[:len(s)-1]
	case len(s) > 1 && s[len(s)-1:] == "M":
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case len(s) > 1 && s[len(s)-1:] == "G":
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	case len(s) > 1 && s[len(s)-1:] == "T":
		multiplier = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}

	val := strings.TrimSpace(s)
	if val == "" {
		return 0, fmt.Errorf("empty size value")
	}

	var result int64
	for _, c := range val {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid character in size: %c", c)
		}
		result = result*10 + int64(c-'0')
	}

	return result * multiplier, nil
}

// isDevicePath returns true if the path appears to be a device file
func isDevicePath(path string) bool {
	// Normalize the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Check if it starts with /dev/
	if strings.HasPrefix(absPath, "/dev/") {
		return true
	}

	// Check if it's a device file using stat
	if fi, err := os.Stat(absPath); err == nil {
		// Check if it's a character device or block device
		mode := fi.Mode()
		if mode&os.ModeDevice != 0 {
			return true
		}
	}

	// Also check by resolving symlinks
	if fi, err := os.Lstat(absPath); err == nil {
		mode := fi.Mode()
		if mode&os.ModeDevice != 0 {
			return true
		}
	}

	return false
}

// printLatencyHistogram prints a latency distribution histogram
func printLatencyHistogram(label string, latencies []int64) {
	if len(latencies) == 0 {
		return
	}

	// Define buckets in microseconds
	buckets := []int64{100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000}
	bucketCounts := make([]int, len(buckets)+1)

	for _, l := range latencies {
		l_us := l / 1000 // Convert ns to us
		idx := len(buckets)
		for i, b := range buckets {
			if l_us < b {
				idx = i
				break
			}
		}
		bucketCounts[idx]++
	}

	total := len(latencies)
	fmt.Printf("%s latency distribution (%d samples):\n", label, total)

	// Print header
	fmt.Printf("%-15s %10s %10s\n", "Bucket (us)", "Count", "Percent")

	// Print each bucket
	for i, b := range buckets {
		count := bucketCounts[i]
		percent := float64(count) * 100 / float64(total)
		if i == 0 {
			fmt.Printf("%-15s %10d %9.2f%%\n", fmt.Sprintf("< %d", b), count, percent)
		} else {
			fmt.Printf("%-15s %10d %9.2f%%\n", fmt.Sprintf("%d-%d", buckets[i-1], b), count, percent)
		}
	}
	// Last bucket (above max)
	count := bucketCounts[len(buckets)]
	percent := float64(count) * 100 / float64(total)
	fmt.Printf("%-15s %10d %9.2f%%\n", fmt.Sprintf("> %d", buckets[len(buckets)-1]), count, percent)
}

func parsePercentileList(list string, single int) ([]float64, error) {
	if strings.TrimSpace(list) == "" {
		if single <= 0 {
			return nil, nil
		}
		return []float64{float64(single)}, nil
	}

	parts := strings.Split(list, ":")
	result := make([]float64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty percentile value")
		}
		value, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return nil, err
		}
		if value <= 0 || value > 100 {
			return nil, fmt.Errorf("percentile %.2f out of range", value)
		}
		result = append(result, value)
	}
	return result, nil
}

func formatPercentile(v float64) string {
	if math.Mod(v, 1) == 0 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(v, 'f', 2, 64), "0"), ".")
}

func writeLatencyHistogramLog(prefix, mode string, latencies []int64) error {
	if prefix == "" || len(latencies) == 0 {
		return nil
	}

	path := fmt.Sprintf("%s_%s_hist.1.log", prefix, mode)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("ioperf: create histogram log %s: %w", path, err)
	}
	defer file.Close()

	buckets := []int64{100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000}
	bucketCounts := make([]int, len(buckets)+1)
	for _, latency := range latencies {
		latencyUs := latency / 1000
		idx := len(buckets)
		for i, bucket := range buckets {
			if latencyUs < bucket {
				idx = i
				break
			}
		}
		bucketCounts[idx]++
	}

	for i, bucket := range buckets {
		if _, err := fmt.Fprintf(file, "%s,%d,%d\n", mode, bucket, bucketCounts[i]); err != nil {
			return fmt.Errorf("ioperf: write histogram log %s: %w", path, err)
		}
	}
	if _, err := fmt.Fprintf(file, "%s,%d+,%d\n", mode, buckets[len(buckets)-1], bucketCounts[len(buckets)]); err != nil {
		return fmt.Errorf("ioperf: write histogram log %s: %w", path, err)
	}
	return nil
}
