package net

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ncCmd implements netcat functionality
func NcCmd(args []string) error {
	return NcCmdWithContext(context.Background(), args)
}

// NcCmdWithContext implements netcat functionality and allows tests/callers to cancel listen mode.
func NcCmdWithContext(ctx context.Context, args []string) error {
	var (
		listenMode     bool
		zeroIO         bool
		udpMode        bool
		waitSec        int
		verbose        bool
		numericOnly    bool
		forceIPv4      bool
		forceIPv6      bool
		benchMode      bool
		blockSize      int64 = 64 * 1024 // default 64KB
		concurrent     int   = 1
		totalRequests  int   = 100
		testDuration   int   = 0 // 0 means use totalRequests
		reportInterval int   = 1
		showHelp       bool
	)

	// -n never consumes the next token when -l/--listen is present, so port parsing doesn't depend on flag order.
	hasListenFlag := false
	for _, a := range args {
		if a == "-l" || a == "--listen" {
			hasListenFlag = true
			break
		}
	}

	// Parse flags
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			showHelp = true
		case arg == "-l" || arg == "--listen":
			listenMode = true
		case arg == "-z" || arg == "--zero":
			zeroIO = true
		case arg == "-u" || arg == "--udp":
			udpMode = true
		case arg == "-v" || arg == "--verbose":
			verbose = true
		case arg == "--numeric-only":
			numericOnly = true
		case arg == "-n":
			if !hasListenFlag && i+1 < len(args) {
				if next, err := strconv.Atoi(args[i+1]); err == nil {
					i++
					totalRequests = next
					break
				}
			}
			numericOnly = true
		case arg == "-4":
			forceIPv4 = true
			forceIPv6 = false
		case arg == "-6":
			forceIPv6 = true
			forceIPv4 = false
		case strings.HasPrefix(arg, "-w"):
			// -wSEC or -w SEC
			if len(arg) > 2 {
				waitSec, _ = strconv.Atoi(arg[2:])
			} else if i+1 < len(args) {
				i++
				waitSec, _ = strconv.Atoi(args[i])
			}
		case strings.HasPrefix(arg, "--wait="):
			waitSec, _ = strconv.Atoi(arg[7:])
		case arg == "--bench":
			benchMode = true
		case strings.HasPrefix(arg, "-s"):
			// -sN or -s N
			if len(arg) > 2 {
				blockSize = parseNCSize(arg[2:])
			} else if i+1 < len(args) {
				i++
				blockSize = parseNCSize(args[i])
			}
		case strings.HasPrefix(arg, "--size="):
			blockSize = parseNCSize(arg[7:])
		case strings.HasPrefix(arg, "-c"):
			// -cN or -c N
			if len(arg) > 2 {
				concurrent, _ = strconv.Atoi(arg[2:])
			} else if i+1 < len(args) {
				i++
				concurrent, _ = strconv.Atoi(args[i])
			}
		case strings.HasPrefix(arg, "--concurrent="):
			concurrent, _ = strconv.Atoi(arg[13:])
		case strings.HasPrefix(arg, "-n"):
			// -nN or -n N (requests)
			if len(arg) > 2 {
				totalRequests, _ = strconv.Atoi(arg[2:])
			} else if i+1 < len(args) {
				i++
				totalRequests, _ = strconv.Atoi(args[i])
			}
		case strings.HasPrefix(arg, "--requests="):
			totalRequests, _ = strconv.Atoi(arg[11:])
		case strings.HasPrefix(arg, "-t"):
			// -tSEC or -t SEC
			if len(arg) > 2 {
				testDuration, _ = strconv.Atoi(arg[2:])
			} else if i+1 < len(args) {
				i++
				testDuration, _ = strconv.Atoi(args[i])
			}
		case strings.HasPrefix(arg, "--time="):
			testDuration, _ = strconv.Atoi(arg[7:])
		case strings.HasPrefix(arg, "-i"):
			// -iSEC or -i SEC
			if len(arg) > 2 {
				reportInterval, _ = strconv.Atoi(arg[2:])
			} else if i+1 < len(args) {
				i++
				reportInterval, _ = strconv.Atoi(args[i])
			}
		case strings.HasPrefix(arg, "--interval="):
			reportInterval, _ = strconv.Atoi(arg[11:])
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown option: %s", arg)
			}
			// Not a flag, stop parsing
			goto doneFlags
		}
		i++
	}
doneFlags:

	remaining := args[i:]

	if showHelp {
		printNCHelp(os.Stdout)
		return nil
	}

	if listenMode {
		// Server mode
		if len(remaining) < 1 {
			return fmt.Errorf("listen mode requires port")
		}
		port := remaining[0]
		return ncServer(ctx, port, udpMode, zeroIO, verbose, benchMode, blockSize, numericOnly, forceIPv4, forceIPv6)
	}

	// Client mode
	if len(remaining) < 2 {
		return fmt.Errorf("requires host and port")
	}
	host := remaining[0]
	port := remaining[1]

	if benchMode {
		return ncBenchmarkClient(host, port, udpMode, verbose, numericOnly, forceIPv4, forceIPv6,
			concurrent, totalRequests, testDuration, reportInterval, int(blockSize), waitSec)
	}

	return ncClient(host, port, udpMode, zeroIO, verbose, numericOnly, forceIPv4, forceIPv6, waitSec)
}

func printNCHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox nc [OPTION]... [HOST] PORT")
	fmt.Fprintln(w, "Netcat - arbitrary TCP/UDP connections and listening")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -l, --listen           Listen mode (server)")
	fmt.Fprintln(w, "  -z, --zero             Zero I/O mode (only scan)")
	fmt.Fprintln(w, "  -u, --udp              UDP mode")
	fmt.Fprintln(w, "  -w SEC, --wait=SEC     Connection timeout")
	fmt.Fprintln(w, "  -v, --verbose          Verbose output")
	fmt.Fprintln(w, "  -n, --numeric-only     Skip DNS resolution")
	fmt.Fprintln(w, "  -4                     Force IPv4")
	fmt.Fprintln(w, "  -6                     Force IPv6")
	fmt.Fprintln(w, "  -h, --help             Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Benchmark mode:")
	fmt.Fprintln(w, "  --bench                Enable benchmark mode")
	fmt.Fprintln(w, "  -c N, --concurrent=N   Concurrent connections (default 1)")
	fmt.Fprintln(w, "  -n N, --requests=N     Total requests (default 100)")
	fmt.Fprintln(w, "  -s N, --size=N         Data block size (default 64KB)")
	fmt.Fprintln(w, "  -t SEC, --time=SEC     Test duration (mutually exclusive with -n)")
	fmt.Fprintln(w, "  -i SEC, --interval=SEC Report interval (default 1s)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox nc -l 8080                    Listen on port 8080")
	fmt.Fprintln(w, "  gobox nc -l -u 8080                 Listen on UDP port 8080")
	fmt.Fprintln(w, "  gobox nc -zv localhost 8080         Scan port 8080")
	fmt.Fprintln(w, "  gobox nc host.example.com 80        Connect to host")
	fmt.Fprintln(w, "  gobox nc --bench localhost 8080      Run benchmark")
	fmt.Fprintln(w, "  gobox nc -c 10 -n 1000 --bench localhost 8080  10 concurrent, 1000 requests")
}

// parseNCSize parses size with optional suffix B/K/M/G
func parseNCSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	var multiplier int64 = 1
	if len(s) > 1 {
		switch s[len(s)-1] {
		case 'B', 'b':
			s = s[:len(s)-1]
		case 'K', 'k':
			multiplier = 1024
			s = s[:len(s)-1]
		case 'M', 'm':
			multiplier = 1024 * 1024
			s = s[:len(s)-1]
		case 'G', 'g':
			multiplier = 1024 * 1024 * 1024
			s = s[:len(s)-1]
		}
	}

	val, _ := strconv.ParseInt(s, 10, 64)
	return val * multiplier
}

// Benchmark protocol magic header
var benchMagic = []byte("GOBENCH\x00")

// ncClient implements basic netcat client
func ncClient(host, port string, udp, zeroIO, verbose, numericOnly, forceIPv4, forceIPv6 bool, waitSec int) error {
	protocol := "tcp"
	if udp {
		protocol = "udp"
	}

	if verbose {
		if numericOnly {
			fmt.Printf("(not resolving host) %s %s\n", protocol, net.JoinHostPort(host, port))
		} else {
			fmt.Printf("%s %s\n", protocol, net.JoinHostPort(host, port))
		}
	}

	network := "tcp"
	if udp {
		network = "udp"
	}
	if forceIPv4 {
		network = "tcp4"
		if udp {
			network = "udp4"
		}
	} else if forceIPv6 {
		network = "tcp6"
		if udp {
			network = "udp6"
		}
	}

	addr := net.JoinHostPort(host, port)
	if numericOnly && net.ParseIP(host) == nil {
		return fmt.Errorf("numeric-only mode requires a literal IP address")
	}
	conn, err := net.DialTimeout(network, addr, time.Duration(waitSec)*time.Second)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	if zeroIO {
		if verbose {
			fmt.Println("Connection successful")
		}
		return nil
	}

	// Copy stdin to connection
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(conn, os.Stdin)
		done <- err
	}()

	// Copy connection to stdout
	_, err = io.Copy(os.Stdout, conn)
	if err != nil {
		return err
	}

	// Wait for stdin to finish
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		// Timeout, connection might be closed
	}

	return nil
}

// ncServer implements netcat server/listen mode
func ncServer(ctx context.Context, port string, udp, zeroIO, verbose, benchMode bool, blockSize int64, numericOnly, forceIPv4, forceIPv6 bool) error {
	network := "tcp"
	if udp {
		network = "udp"
	}
	if forceIPv4 {
		network = "tcp4"
		if udp {
			network = "udp4"
		}
	} else if forceIPv6 {
		network = "tcp6"
		if udp {
			network = "udp6"
		}
	}

	addr := net.JoinHostPort("", port)

	if ctx == nil {
		ctx = context.Background()
	}

	if udp {
		return ncUDPServer(ctx, network, addr, port, zeroIO, verbose)
	}

	listener, err := net.Listen(network, addr)
	if err != nil {
		return fmt.Errorf("listen failed: %w", err)
	}
	defer listener.Close()

	if verbose {
		fmt.Printf("Listening on port %s\n", port)
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = listener.Close()
		case <-done:
		}
	}()

	if benchMode {
		return ncBenchServer(listener, udp, blockSize, verbose)
	}

	conn, err := listener.Accept()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	defer conn.Close()

	if verbose {
		fmt.Printf("Connection from %s\n", conn.RemoteAddr().String())
	}

	if zeroIO {
		return nil
	}

	stdinDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(conn, os.Stdin)
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			_ = tcpConn.CloseWrite()
		}
		stdinDone <- err
	}()

	_, err = io.Copy(os.Stdout, conn)
	if err != nil {
		return err
	}

	select {
	case err := <-stdinDone:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second):
		return nil
	}
}

// ncUDPServer implements netcat UDP listen mode. UDP has no "listener"/"accept"
// concept in net.Listen, so it must use net.ListenPacket (or net.ListenUDP)
// instead of net.Listen("udp", ...), which is not supported and returns an
// "unexpected address type" error.
func ncUDPServer(ctx context.Context, network, addr, port string, zeroIO, verbose bool) error {
	pc, err := net.ListenPacket(network, addr)
	if err != nil {
		return fmt.Errorf("listen failed: %w", err)
	}
	defer pc.Close()

	if verbose {
		fmt.Printf("Listening on port %s (udp)\n", port)
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = pc.Close()
		case <-done:
		}
	}()

	buf := make([]byte, 65535)
	n, raddr, err := pc.ReadFrom(buf)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}

	if verbose {
		fmt.Printf("Connection from %s\n", raddr.String())
	}

	if zeroIO {
		return nil
	}

	if n > 0 {
		os.Stdout.Write(buf[:n])
	}

	stdinDone := make(chan error, 1)
	go func() {
		sbuf := make([]byte, 65535)
		for {
			m, rerr := os.Stdin.Read(sbuf)
			if m > 0 {
				if _, werr := pc.WriteTo(sbuf[:m], raddr); werr != nil {
					stdinDone <- werr
					return
				}
			}
			if rerr != nil {
				if rerr == io.EOF {
					stdinDone <- nil
				} else {
					stdinDone <- rerr
				}
				return
			}
		}
	}()

	for {
		_ = pc.SetReadDeadline(time.Now().Add(time.Second))
		n, _, rerr := pc.ReadFrom(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
		}
		if rerr != nil {
			if ne, ok := rerr.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case serr := <-stdinDone:
					return serr
				default:
					continue
				}
			}
			break
		}
	}

	select {
	case serr := <-stdinDone:
		return serr
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// ncBenchServer implements benchmark server mode
func ncBenchServer(listener net.Listener, udp bool, blockSize int64, verbose bool) error {
	if listener.Addr().Network() != "udp" {
		// TCP listener
		for {
			conn, err := listener.Accept()
			if err != nil {
				return err
			}

			go handleBenchConnection(conn, blockSize, verbose)
		}
	}
	return nil
}

func handleBenchConnection(conn net.Conn, blockSize int64, verbose bool) {
	defer conn.Close()

	// Read magic header
	magic := make([]byte, 8)
	conn.Read(magic)

	if string(magic) != string(benchMagic) {
		// Not a benchmark client, treat as regular connection
		io.Copy(conn, conn)
		return
	}

	// Benchmark mode - echo data and measure
	var totalBytes int64
	var startTime = time.Now()
	var lastReport = startTime
	var reportInterval int64 = 1

	buf := make([]byte, blockSize)

	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(buf)
		if n > 0 {
			totalBytes += int64(n)
			// Echo back
			conn.Write(buf[:n])
		}
		if err != nil {
			if verbose {
				duration := time.Since(startTime)
				fmt.Printf("Benchmark complete: %d bytes in %v (%.2f MB/s)\n",
					totalBytes, duration.Round(time.Second),
					float64(totalBytes)/1024/1024/duration.Seconds())
			}
			break
		}

		// Periodic report
		now := time.Now()
		if now.Sub(lastReport).Seconds() >= float64(reportInterval) {
			duration := now.Sub(startTime)
			if verbose {
				fmt.Printf("Transfer: %d bytes in %v (%.2f MB/s)\n",
					totalBytes, duration.Round(time.Second),
					float64(totalBytes)/1024/1024/duration.Seconds())
			}
			lastReport = now
		}
	}
}

// ncBenchmarkClient implements benchmark client mode
func ncBenchmarkClient(host, port string, udp, verbose, numericOnly, forceIPv4, forceIPv6 bool,
	concurrent, totalRequests, testDuration, reportInterval, blockSize int, waitSec int) error {

	network := "tcp"
	if udp {
		network = "udp"
	}
	if forceIPv4 {
		network = "tcp4"
		if udp {
			network = "udp4"
		}
	} else if forceIPv6 {
		network = "tcp6"
		if udp {
			network = "udp6"
		}
	}

	addr := net.JoinHostPort(host, port)
	if numericOnly && net.ParseIP(host) == nil {
		return fmt.Errorf("numeric-only mode requires a literal IP address")
	}

	if verbose {
		fmt.Printf("Connecting to %s:%s\n", host, port)
	}

	// Use time duration if specified
	useDuration := testDuration > 0

	var wg sync.WaitGroup
	var totalBytes int64
	var totalRequestsCompleted int64
	var connectionErrors int32

	var latencyMu sync.Mutex
	var latencyList []float64

	intervalDuration := time.Duration(reportInterval) * time.Second
	var stopTime time.Time
	if useDuration {
		stopTime = time.Now().Add(time.Duration(testDuration) * time.Second)
	}

	// Start concurrent connections
	for c := 0; c < concurrent; c++ {
		wg.Add(1)
		go func(connId int) {
			defer wg.Done()

			var conn net.Conn
			var err error

			// Connect with retry
			for retries := 0; retries < 3; retries++ {
				conn, err = net.DialTimeout(network, addr, time.Duration(waitSec)*time.Second)
				if err == nil {
					break
				}
				time.Sleep(time.Second)
			}
			if err != nil {
				atomic.AddInt32(&connectionErrors, 1)
				if verbose {
					fmt.Printf("[%2d] connection failed: %v\n", connId+1, err)
				}
				return
			}
			defer conn.Close()

			// Send magic header
			conn.Write(benchMagic)

			if verbose {
				fmt.Printf("[%2d] local=%s port=%d connected\n",
					connId+1, conn.LocalAddr().String(), 0)
			}

			requestsPerConn := totalRequests / concurrent
			if totalRequests%concurrent > connId {
				requestsPerConn++
			}

			data := make([]byte, blockSize)
			for i := 0; useDuration || i < requestsPerConn; i++ {
				// Check if we should stop (duration mode)
				if useDuration && time.Now().After(stopTime) {
					break
				}

				// Send data
				reqStart := time.Now()
				_, err := conn.Write(data)
				if err != nil {
					return
				}

				// Read response (echo)
				respBuf := make([]byte, blockSize)
				_, err = conn.Read(respBuf)
				if err != nil {
					return
				}

				latency := time.Since(reqStart).Microseconds() // microseconds

				atomic.AddInt64(&totalBytes, int64(blockSize))
				atomic.AddInt64(&totalRequestsCompleted, 1)

				latencyMu.Lock()
				latencyList = append(latencyList, float64(latency)/1000.0) // convert to ms
				latencyMu.Unlock()
			}
		}(c)
	}

	// Print header
	fmt.Println()
	fmt.Printf("Connecting to %s\n", addr)
	fmt.Println("[ ID] Interval       Transfer     Bandwidth")
	fmt.Println()

	// Report timer — use ticker directly in the select to avoid race with time.After.
	ticker := time.NewTicker(intervalDuration)
	defer ticker.Stop()

	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	startTime := time.Now()
	oldTotalBytes := int64(0)
	oldTotalReqs := int64(0)
	reportNum := 1
	finished := false

	for !finished {
		select {
		case <-ticker.C:
			duration := time.Since(startTime)
			if duration.Seconds() < 0.1 {
				continue
			}

			bytesPerSec := float64(totalBytes-oldTotalBytes) / duration.Seconds()
			_ = float64(totalRequestsCompleted-oldTotalReqs) / duration.Seconds()

			intervalStr := fmt.Sprintf("[%2d] %.1f-%.1fs", reportNum,
				duration.Seconds()-float64(reportInterval), duration.Seconds())

			transferStr := formatBytes(totalBytes)
			bandwidthStr := formatBandwidth(bytesPerSec)

			fmt.Printf("%s  %-12s  %-12s\n", intervalStr, transferStr, bandwidthStr)

			oldTotalBytes = totalBytes
			oldTotalReqs = totalRequestsCompleted
			reportNum++
		case <-doneChan:
			ticker.Stop()
			// If all workers finished at (or after) the first interval boundary
			// but before the ticker itself fired, still emit that interval's
			// report so output is deterministic regardless of scheduling jitter.
			tailDuration := time.Since(startTime)
			if reportNum == 1 && tailDuration.Seconds() >= float64(reportInterval)-0.05 {
				bytesPerSec := float64(totalBytes-oldTotalBytes) / tailDuration.Seconds()
				intervalStr := fmt.Sprintf("[%2d] %.1f-%.1fs", reportNum, 0.0, tailDuration.Seconds())
				transferStr := formatBytes(totalBytes)
				bandwidthStr := formatBandwidth(bytesPerSec)
				fmt.Printf("%s  %-12s  %-12s\n", intervalStr, transferStr, bandwidthStr)
			}
			finished = true
		}
	}

	duration := time.Since(startTime)

	// Print final stats
	fmt.Println()
	if len(latencyList) > 0 {
		minLat := latencyList[0]
		maxLat := latencyList[0]
		var sumLat float64
		for _, lat := range latencyList {
			if lat < minLat {
				minLat = lat
			}
			if lat > maxLat {
				maxLat = lat
			}
			sumLat += lat
		}
		meanLat := sumLat / float64(len(latencyList))

		fmt.Printf("Latency: min=%.1fms, max=%.1fms, mean=%.1fms\n",
			minLat, maxLat, meanLat)
	}

	fmt.Printf("Total: %.1fs, %s, %.2fMbps\n",
		duration.Seconds(),
		formatBytes(totalBytes),
		float64(totalBytes)*8/1024/1024/duration.Seconds())

	if connectionErrors > 0 {
		fmt.Printf("Connection errors: %d\n", connectionErrors)
	}

	return nil
}

func formatBytes(n int64) string {
	const KB = 1024
	const MB = KB * 1024
	const GB = MB * 1024

	if n >= GB {
		return fmt.Sprintf("%.2fGB", float64(n)/GB)
	}
	if n >= MB {
		return fmt.Sprintf("%.2fMB", float64(n)/MB)
	}
	if n >= KB {
		return fmt.Sprintf("%.2fKB", float64(n)/KB)
	}
	return fmt.Sprintf("%dB", n)
}

func formatBandwidth(bytesPerSec float64) string {
	const Kbps = 1024
	const Mbps = Kbps * 1024
	const Gbps = Mbps * 1024

	bps := bytesPerSec * 8
	if bps >= Gbps {
		return fmt.Sprintf("%.2fGbps", bps/Gbps)
	}
	if bps >= Mbps {
		return fmt.Sprintf("%.2fMbps", bps/Mbps)
	}
	if bps >= Kbps {
		return fmt.Sprintf("%.2fKbps", bps/Kbps)
	}
	return fmt.Sprintf("%.0fbps", bps)
}
