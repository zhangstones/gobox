package net

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// NpCmd implements network ping/connectivity troubleshooting tool
func NpCmd(args []string) error {
	fsFlags := flag.NewFlagSet("np", flag.ContinueOnError)
	fsFlags.SetOutput(os.Stderr)
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox np [OPTIONS] [HOST]")
		fmt.Fprintln(os.Stderr, "Network connectivity troubleshooting tool (TCP/UDP/ICMP/ARP ping, port scanning)")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Modes:")
		fmt.Fprintln(os.Stderr, "  --tcp              TCP mode (default)")
		fmt.Fprintln(os.Stderr, "  --udp              UDP mode")
		fmt.Fprintln(os.Stderr, "  --icmp             ICMP mode")
		fmt.Fprintln(os.Stderr, "  --arp              ARP mode")
		fmt.Fprintln(os.Stderr, "  --scan             port scanning mode")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -c COUNT           packet count to send")
		fmt.Fprintln(os.Stderr, "  -i USEC            interval between packets (microseconds)")
		fmt.Fprintln(os.Stderr, "  -p PORT            target port")
		fmt.Fprintln(os.Stderr, "  -s PORT            source port")
		fmt.Fprintln(os.Stderr, "  -I IFACE           network interface to use")
		fmt.Fprintln(os.Stderr, "  -W SEC             timeout in seconds")
		fmt.Fprintln(os.Stderr, "  --flood            flood mode (max speed)")
		fmt.Fprintln(os.Stderr, "  -w N               concurrent workers (for TCP/UDP ping)")
		fmt.Fprintln(os.Stderr, "  -l N               long connection mode")
		fmt.Fprintln(os.Stderr, "  -q                 quiet mode, only show final statistics")
		fmt.Fprintln(os.Stderr, "  -v                 verbose output")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gobox np --tcp -p 80 -c 1 127.0.0.1")
		fmt.Fprintln(os.Stderr, "  gobox np --udp -p 53 -c 2 127.0.0.1")
		fmt.Fprintln(os.Stderr, "  gobox np --scan 80,443 127.0.0.1")
	}

	// Mode selection
	tcpMode := fsFlags.Bool("tcp", false, "TCP mode (default)")
	udpMode := fsFlags.Bool("udp", false, "UDP mode")
	icmpMode := fsFlags.Bool("icmp", false, "ICMP mode")
	arpMode := fsFlags.Bool("arp", false, "ARP mode")
	scanMode := fsFlags.Bool("scan", false, "Port scanning mode")

	// General parameters
	count := fsFlags.Int("c", 4, "Packet count to send")
	interval := fsFlags.Int("i", 1000000, "Interval between packets (microseconds)")
	port := fsFlags.Int("p", 0, "Target port")
	sourcePort := fsFlags.Int("s", 0, "Source port")
	iface := fsFlags.String("I", "", "Network interface to use")
	waitSec := fsFlags.Int("W", 5, "Timeout in seconds")
	flood := fsFlags.Bool("flood", false, "Flood mode (max speed)")
	workers := fsFlags.Int("w", 1, "Concurrent workers (for TCP/UDP ping)")

	// Long connection mode
	longMode := fsFlags.Int("l", 0, "Long connection mode (ping once, wait for close, repeat)")

	// Output parameters
	quiet := fsFlags.Bool("q", false, "Quiet mode, only show final statistics")
	verbose := fsFlags.Bool("v", false, "Verbose output")

	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	countExplicit := false
	fsFlags.Visit(func(f *flag.Flag) {
		if f.Name == "c" {
			countExplicit = true
		}
	})

	remaining := fsFlags.Args()

	// Determine mode
	mode := "tcp"
	if *tcpMode {
		mode = "tcp"
	} else if *udpMode {
		mode = "udp"
	} else if *icmpMode {
		mode = "icmp"
	} else if *arpMode {
		mode = "arp"
	} else if *scanMode {
		mode = "scan"
	}

	// Validate arguments
	if len(remaining) < 1 && mode != "scan" {
		return fmt.Errorf("missing host argument")
	}

	var host string
	if len(remaining) > 0 {
		host = remaining[0]
	}

	// Parse port range/string for scan mode
	var portRange []int
	if mode == "scan" {
		if len(remaining) < 2 {
			return fmt.Errorf("scan mode requires ports and target: gobox np --scan PORTS TARGET")
		}
		portRange = parsePortRange(remaining[0])
		host = remaining[1]
		if len(portRange) == 0 {
			return fmt.Errorf("no valid ports specified")
		}
	}

	// Validate port
	if *count < 0 {
		return fmt.Errorf("count must be >= 0, got %d", *count)
	}
	if *port == 0 && mode != "scan" && mode != "icmp" && mode != "arp" {
		return fmt.Errorf("port is required for TCP/UDP mode (use -p)")
	}
	if *port < 0 || *port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535, got %d", *port)
	}

	// Configure flood mode
	if *flood {
		if !countExplicit {
			*count = 0 // unlimited unless -c provides a bounded run.
		}
		*interval = 0
	}

	// Build options
	opts := &npOptions{
		mode:       mode,
		host:       host,
		port:       *port,
		sourcePort: *sourcePort,
		iface:      *iface,
		count:      *count,
		interval:   time.Duration(*interval) * time.Microsecond,
		wait:       time.Duration(*waitSec) * time.Second,
		flood:      *flood,
		workers:    *workers,
		longMode:   *longMode,
		quiet:      *quiet,
		verbose:    *verbose,
		portRange:  portRange,
	}

	return runNp(opts)
}

type npOptions struct {
	mode       string
	host       string
	port       int
	sourcePort int
	iface      string
	count      int
	interval   time.Duration
	wait       time.Duration
	flood      bool
	workers    int
	longMode   int
	quiet      bool
	verbose    bool
	portRange  []int
}

func runNp(opts *npOptions) error {
	switch opts.mode {
	case "tcp":
		return npTCP(opts)
	case "udp":
		return npUDP(opts)
	case "icmp":
		return npICMP(opts)
	case "arp":
		return npARP(opts)
	case "scan":
		return npScan(opts)
	default:
		return fmt.Errorf("unknown mode: %s", opts.mode)
	}
}

// TCP ping - connect and measure latency
func npTCP(opts *npOptions) error {
	var wg sync.WaitGroup
	var sent, received, errors int64
	var minLatency, maxLatency, totalLatency int64
	var mu sync.Mutex
	latencies := []int64{}

	stopChan := make(chan struct{})
	var progressWG sync.WaitGroup

	// Worker pool
	for w := 0; w < opts.workers; w++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			npTCPWorker(workerId, opts, &sent, &received, &errors, &mu, &latencies, stopChan)
		}(w)
	}

	// Progress reporter
	if !opts.quiet {
		progressWG.Add(1)
		go func() {
			defer progressWG.Done()
			npProgressReporter(&sent, &received, &errors, opts, stopChan)
		}()
	}

	wg.Wait()
	close(stopChan)
	progressWG.Wait()

	// Calculate statistics
	if len(latencies) > 0 {
		minLatency = latencies[0]
		maxLatency = latencies[0]
		for _, l := range latencies {
			if l < minLatency {
				minLatency = l
			}
			if l > maxLatency {
				maxLatency = l
			}
			totalLatency += l
		}
	}

	if opts.quiet || opts.verbose {
		printNpStats(sent, received, errors, minLatency, maxLatency, totalLatency)
	}

	return nil
}

func npTCPWorker(workerId int, opts *npOptions, sent, received, errors *int64, mu *sync.Mutex, latencies *[]int64, stopChan chan struct{}) {
	addr := net.JoinHostPort(opts.host, strconv.Itoa(opts.port))
	dialer := net.Dialer{Timeout: opts.wait}
	configureNpDialer(&dialer, "tcp", opts)

	for i := 0; ; i++ {
		select {
		case <-stopChan:
			return
		default:
		}

		if opts.count > 0 && atomic.LoadInt64(sent) >= int64(opts.count) {
			return
		}

		start := time.Now()

		conn, err := dialer.Dial("tcp", addr)
		latency := time.Since(start).Microseconds()

		atomic.AddInt64(sent, 1)

		if err != nil {
			atomic.AddInt64(errors, 1)
			if opts.verbose {
				fmt.Printf("From %s: seq=%d Connection failed: %v\n", opts.host, i, err)
			}
			if opts.interval > 0 {
				time.Sleep(opts.interval)
			}
			continue
		}

		atomic.AddInt64(received, 1)

		mu.Lock()
		*latencies = append(*latencies, latency)
		mu.Unlock()

		if !opts.quiet {
			fmt.Printf("%d bytes from %s: seq=%d ttl=64 time=%.3f ms\n",
				64, opts.host, i, float64(latency)/1000.0)
		}

		// Long connection mode: wait for server to close, then continue
		if opts.longMode > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(time.Duration(opts.longMode) * time.Second))
			_, _ = io.Copy(io.Discard, conn)
		}
		conn.Close()

		if opts.interval > 0 && !opts.flood {
			time.Sleep(opts.interval)
		}
	}
}

// UDP ping
func npUDP(opts *npOptions) error {
	addr := net.JoinHostPort(opts.host, strconv.Itoa(opts.port))
	dialer := net.Dialer{Timeout: opts.wait}
	configureNpDialer(&dialer, "udp", opts)

	var sent, received, errors int64
	var minLatency, maxLatency, totalLatency int64
	latencies := []int64{}
	mu := sync.Mutex{}

	stopChan := make(chan struct{})

	for i := 0; i < opts.count || opts.flood; i++ {
		select {
		case <-stopChan:
		default:
		}

		start := time.Now()

		conn, err := dialer.Dial("udp", addr)
		latency := time.Since(start).Microseconds()

		atomic.AddInt64(&sent, 1)

		if err != nil {
			atomic.AddInt64(&errors, 1)
			if opts.verbose {
				fmt.Printf("From %s: seq=%d Connection failed: %v\n", opts.host, i, err)
			}
		} else {
			atomic.AddInt64(&received, 1)
			mu.Lock()
			latencies = append(latencies, latency)
			mu.Unlock()

			if !opts.quiet {
				fmt.Printf("%d bytes from %s: seq=%d ttl=64 time=%.3f ms\n",
					64, opts.host, i, float64(latency)/1000.0)
			}
			conn.Close()
		}

		if opts.interval > 0 && !opts.flood {
			time.Sleep(opts.interval)
		}

		if opts.count > 0 && i >= opts.count-1 {
			break
		}
	}

	if len(latencies) > 0 {
		minLatency = latencies[0]
		maxLatency = latencies[0]
		for _, l := range latencies {
			if l < minLatency {
				minLatency = l
			}
			if l > maxLatency {
				maxLatency = l
			}
			totalLatency += l
		}
	}

	if opts.quiet || opts.verbose {
		printNpStats(sent, received, errors, minLatency, maxLatency, totalLatency)
	}

	return nil
}

// ICMP ping using raw socket
func npICMP(opts *npOptions) error {
	// Prefer the system ping binary when present; it usually carries the
	// capabilities needed for ICMP echo without requiring a privileged gobox process.
	if path, err := exec.LookPath("ping"); err == nil {
		args := []string{}
		if opts.flood {
			args = append(args, "-f")
		}
		if opts.quiet {
			args = append(args, "-q")
		}
		if opts.iface != "" {
			args = append(args, "-I", opts.iface)
		}
		if opts.interval > 0 {
			args = append(args, "-i", strconv.FormatFloat(opts.interval.Seconds(), 'f', -1, 64))
		}
		if opts.count > 0 {
			args = append(args, "-c", strconv.Itoa(opts.count))
		}
		args = append(args, "-W", strconv.Itoa(npWaitSeconds(opts)), opts.host)
		cmd := exec.Command(path, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		return nil
	}

	// Try to resolve hostname first
	ipAddr, err := net.ResolveIPAddr("ip", opts.host)
	if err != nil {
		return fmt.Errorf("cannot resolve %s: %w", opts.host, err)
	}

	fmt.Printf("PING %s (%s): 56 data bytes\n", opts.host, ipAddr.String())

	var sent, received, errors int64
	var minLatency, maxLatency, totalLatency int64
	latencies := []int64{}
	mu := sync.Mutex{}

	for i := 0; i < opts.count || opts.flood; i++ {
		latencyDuration, err := sendICMPEcho(opts.host, i, opts.wait)
		latency := latencyDuration.Microseconds()

		atomic.AddInt64(&sent, 1)

		if err != nil {
			atomic.AddInt64(&errors, 1)
			if opts.verbose || !opts.quiet {
				fmt.Printf("From %s: seq=%d Connection failed: %v\n", opts.host, i, err)
			}
		} else {
			atomic.AddInt64(&received, 1)
			mu.Lock()
			latencies = append(latencies, latency)
			mu.Unlock()

			if !opts.quiet {
				fmt.Printf("64 bytes from %s: seq=%d ttl=64 time=%.3f ms\n",
					opts.host, i, float64(latency)/1000.0)
			}
		}

		if opts.interval > 0 && !opts.flood {
			time.Sleep(opts.interval)
		}

		if opts.count > 0 && i >= opts.count-1 {
			break
		}
	}

	if len(latencies) > 0 {
		minLatency = latencies[0]
		maxLatency = latencies[0]
		for _, l := range latencies {
			if l < minLatency {
				minLatency = l
			}
			if l > maxLatency {
				maxLatency = l
			}
			totalLatency += l
		}
	}

	if opts.quiet || opts.verbose {
		printNpStats(sent, received, errors, minLatency, maxLatency, totalLatency)
	}

	return nil
}

func sendICMPEcho(host string, seq int, timeout time.Duration) (time.Duration, error) {
	ipAddr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return 0, err
	}
	conn, err := net.DialTimeout("ip4:icmp", ipAddr.String(), timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	id := os.Getpid() & 0xffff
	packet := make([]byte, 8)
	packet[0] = 8 // echo request
	packet[1] = 0
	binary.BigEndian.PutUint16(packet[4:6], uint16(id))
	binary.BigEndian.PutUint16(packet[6:8], uint16(seq))
	binary.BigEndian.PutUint16(packet[2:4], icmpChecksum(packet))

	start := time.Now()
	if _, err := conn.Write(packet); err != nil {
		return 0, err
	}
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return 0, err
	}

	buf := make([]byte, 1500)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return 0, err
		}
		msg := icmpPayload(buf[:n])
		if len(msg) < 8 {
			continue
		}
		if msg[0] == 0 &&
			binary.BigEndian.Uint16(msg[4:6]) == uint16(id) &&
			binary.BigEndian.Uint16(msg[6:8]) == uint16(seq) {
			return time.Since(start), nil
		}
	}
}

func icmpPayload(packet []byte) []byte {
	if len(packet) < 1 {
		return packet
	}
	if packet[0]>>4 == 4 && len(packet) >= 20 {
		headerLen := int(packet[0]&0x0f) * 4
		if headerLen >= 20 && len(packet) >= headerLen {
			return packet[headerLen:]
		}
	}
	return packet
}

func icmpChecksum(packet []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(packet); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(packet[i : i+2]))
	}
	if len(packet)%2 == 1 {
		sum += uint32(packet[len(packet)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

// ARP ping (ARP discovery on local network)
func npARP(opts *npOptions) error {
	if path, err := exec.LookPath("arping"); err == nil {
		args := []string{"-c", strconv.Itoa(npPacketCount(opts)), "-w", strconv.Itoa(npWaitSeconds(opts))}
		if opts.iface != "" {
			args = append(args, "-I", opts.iface)
		}
		args = append(args, opts.host)
		cmd := exec.Command(path, args...)
		if opts.quiet {
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
		} else {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		return cmd.Run()
	}

	if !opts.quiet {
		fmt.Printf("ARPING %s from unspecified\n", opts.host)
	}

	// Try to get MAC address via ARP
	// On Linux, we can check /proc/net/arp
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		// Fallback: try to ping and see if we can get ARP info
		conn, err := net.DialTimeout("ip4:icmp", opts.host, opts.wait)
		if err != nil {
			return fmt.Errorf("cannot reach %s: %w", opts.host, err)
		}
		conn.Close()

		if !opts.quiet {
			fmt.Printf("%s is alive (ARP request sent)\n", opts.host)
		}
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip header
		if strings.HasPrefix(line, "IP address") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		ip := fields[0]
		mac := fields[3]
		if ip == opts.host && mac != "" && mac != "00:00:00:00:00:00" {
			fmt.Printf("%s is at %s\n", opts.host, mac)
			return nil
		}
	}

	fmt.Printf("%s not found in ARP cache (request sent)\n", opts.host)
	return nil
}

func npPacketCount(opts *npOptions) int {
	if opts.count > 0 {
		return opts.count
	}
	return 1
}

func npWaitSeconds(opts *npOptions) int {
	if opts.wait <= 0 {
		return 1
	}
	seconds := int((opts.wait + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

// Port scanning
func npScan(opts *npOptions) error {
	if !opts.quiet {
		fmt.Printf("Starting scan of %d ports on %s\n", len(opts.portRange), opts.host)
	}

	var mu sync.Mutex
	var openPorts, closedPorts, errors int
	var wg sync.WaitGroup
	var portIndex int
	var portLock sync.Mutex

	// Worker pool for parallel scanning
	for w := 0; w < opts.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				portLock.Lock()
				if portIndex >= len(opts.portRange) {
					portLock.Unlock()
					return
				}
				port := opts.portRange[portIndex]
				portIndex++
				portLock.Unlock()

				addr := net.JoinHostPort(opts.host, strconv.Itoa(port))
				err := npScanPort(addr, opts.wait)

				mu.Lock()
				if err != nil {
					// Port is closed or filtered
					closedPorts++
					if opts.verbose {
						fmt.Printf("Port %d: closed\n", port)
					}
				} else {
					openPorts++
					fmt.Printf("Port %d: open\n", port)
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if !opts.quiet {
		fmt.Printf("\nScan complete: %d open, %d closed, %d errors\n",
			openPorts, closedPorts, errors)
	}

	return nil
}

func npScanPort(addr string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func configureNpDialer(dialer *net.Dialer, network string, opts *npOptions) {
	ip := net.IP(nil)
	if opts.iface != "" {
		ifi, err := net.InterfaceByName(opts.iface)
		if err == nil {
			if addrs, addrErr := ifi.Addrs(); addrErr == nil {
				for _, addr := range addrs {
					switch v := addr.(type) {
					case *net.IPNet:
						if v.IP.IsGlobalUnicast() || v.IP.IsPrivate() {
							ip = v.IP
							break
						}
					case *net.IPAddr:
						if v.IP.IsGlobalUnicast() || v.IP.IsPrivate() {
							ip = v.IP
							break
						}
					}
				}
			}
		}
	}
	if opts.sourcePort > 0 || ip != nil {
		switch network {
		case "udp":
			dialer.LocalAddr = &net.UDPAddr{IP: ip, Port: opts.sourcePort}
		default:
			dialer.LocalAddr = &net.TCPAddr{IP: ip, Port: opts.sourcePort}
		}
	}
}

func npProgressReporter(sent, received, errors *int64, opts *npOptions, stopChan chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s := atomic.LoadInt64(sent)
			r := atomic.LoadInt64(received)
			e := atomic.LoadInt64(errors)
			if !opts.quiet {
				fmt.Printf("\rSent=%d Received=%d Errors=%d", s, r, e)
			}
		case <-stopChan:
			return
		}
	}
}

func printNpStats(sent, received, errors, minLatency, maxLatency, totalLatency int64) {
	fmt.Println()
	fmt.Printf("--- %s ping statistics ---\n", "netping")
	fmt.Printf("%d packets transmitted, %d packets received, %d errors\n",
		sent, received, errors)

	if received > 0 {
		packetLoss := float64(sent-received) / float64(sent) * 100
		fmt.Printf("round-trip min/avg/max = %.3f/%.3f/%.3f ms\n",
			float64(minLatency)/1000.0,
			float64(totalLatency)/1000.0/float64(received),
			float64(maxLatency)/1000.0)
		fmt.Printf("%.1f%% packet loss\n", packetLoss)
	}
}

func parsePortRange(input string) []int {
	var ports []int

	// Handle comma-separated ports and ranges
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// Range
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start, err1 := strconv.Atoi(rangeParts[0])
				end, err2 := strconv.Atoi(rangeParts[1])
				if err1 == nil && err2 == nil && start > 0 && end > 0 && start <= end {
					for p := start; p <= end && p <= 65535; p++ {
						ports = append(ports, p)
					}
				}
			}
		} else {
			// Single port
			p, err := strconv.Atoi(part)
			if err == nil && p > 0 && p <= 65535 {
				ports = append(ports, p)
			}
		}
	}

	return ports
}

// Helper to convert bytes to human readable
func npHumanSize(n int64) string {
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
