package net

import (
	"bufio"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

func NetstatCmd(args []string) error {
	fsFlags := flag.NewFlagSet("netstat", flag.ContinueOnError)
	stateFilter := fsFlags.String("state", "", "filter by connection state (comma-separated, e.g., LISTEN,ESTABLISHED)")
	portFilter := fsFlags.Int("port", 0, "filter by local or remote port")
	sortBy := fsFlags.String("sort", "", "sort by recvq|sendq|local|remote|pid")
	allSockets := fsFlags.Bool("a", false, "show all sockets")
	allSocketsLong := fsFlags.Bool("all", false, "show all sockets")
	tcpOnly := fsFlags.Bool("t", false, "show TCP sockets")
	tcpOnlyLong := fsFlags.Bool("tcp", false, "show TCP sockets")
	udpOnly := fsFlags.Bool("u", false, "show UDP sockets")
	udpOnlyLong := fsFlags.Bool("udp", false, "show UDP sockets")
	unixOnly := fsFlags.Bool("x", false, "show Unix domain sockets")
	unixOnlyLong := fsFlags.Bool("unix", false, "show Unix domain sockets")
	listeningOnly := fsFlags.Bool("l", false, "show listening sockets only")
	listeningOnlyLong := fsFlags.Bool("listening", false, "show listening sockets only")
	numericOnly := fsFlags.Bool("n", false, "show numeric addresses and ports")
	numericOnlyLong := fsFlags.Bool("numeric", false, "show numeric addresses and ports")
	programs := fsFlags.Bool("p", false, "show PID/Program name for sockets")
	programsLong := fsFlags.Bool("programs", false, "show PID/Program name for sockets")
	ipv4Only := fsFlags.Bool("4", false, "show IPv4 sockets")
	ipv6Only := fsFlags.Bool("6", false, "show IPv6 sockets")
	extended := fsFlags.Bool("e", false, "show extended socket information")
	extendedLong := fsFlags.Bool("extend", false, "show extended socket information")
	timers := fsFlags.Bool("o", false, "show TCP timer information")
	timersLong := fsFlags.Bool("timers", false, "show TCP timer information")
	wide := fsFlags.Bool("W", false, "wide output (accepted; gobox does not truncate addresses)")
	wideLong := fsFlags.Bool("wide", false, "wide output (accepted; gobox does not truncate addresses)")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox netstat")
		fmt.Fprintln(os.Stderr, "Print network connection statistics (Linux /proc/net/tcp*, /proc/net/udp*, /proc/net/unix).")
		fmt.Fprintln(os.Stderr, "Flags:")
		fsFlags.PrintDefaults()
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	_ = *allSockets || *allSocketsLong
	_ = *numericOnly || *numericOnlyLong
	_ = *programs || *programsLong
	_ = *wide || *wideLong
	if *listeningOnlyLong {
		*listeningOnly = true
	}
	if *tcpOnlyLong {
		*tcpOnly = true
	}
	if *udpOnlyLong {
		*udpOnly = true
	}
	if *unixOnlyLong {
		*unixOnly = true
	}
	if *extendedLong {
		*extended = true
	}
	if *timersLong {
		*timers = true
	}
	if *ipv4Only && *ipv6Only {
		return fmt.Errorf("netstat: -4 and -6 cannot be used together")
	}

	if runtime.GOOS != "linux" {
		return errors.New("netstat: supported only on Linux in this implementation")
	}

	// Parse tcp/udp tables
	conns := make([]tcpConn, 0)
	if !*unixOnly && !*ipv6Only {
		if cs, err := parseProcNetTCP("/proc/net/tcp", "TCP"); err == nil {
			conns = append(conns, cs...)
		}
	}
	if !*unixOnly && !*ipv4Only {
		if cs, err := parseProcNetTCP("/proc/net/tcp6", "TCP6"); err == nil {
			conns = append(conns, cs...)
		}
	}
	if !*unixOnly && !*ipv6Only {
		if cs, err := parseProcNetUDP("/proc/net/udp", "UDP"); err == nil {
			conns = append(conns, cs...)
		}
	}
	if !*unixOnly && !*ipv4Only {
		if cs, err := parseProcNetUDP("/proc/net/udp6", "UDP6"); err == nil {
			conns = append(conns, cs...)
		}
	}
	if *unixOnly || (!*tcpOnly && !*udpOnly && !*ipv4Only && !*ipv6Only) {
		if cs, err := parseProcNetUnix("/proc/net/unix"); err == nil {
			conns = append(conns, cs...)
		}
	}

	if *tcpOnly || *udpOnly || *unixOnly {
		filtered := conns[:0]
		for _, c := range conns {
			if *tcpOnly && strings.HasPrefix(c.Proto, "TCP") {
				filtered = append(filtered, c)
			}
			if *udpOnly && strings.HasPrefix(c.Proto, "UDP") {
				filtered = append(filtered, c)
			}
			if *unixOnly && c.Proto == "UNIX" {
				filtered = append(filtered, c)
			}
		}
		conns = filtered
	}

	inodeToPid, pidName := buildInodePidMap()

	// Apply filtering by state and port
	if *stateFilter != "" {
		wanted := make(map[string]bool)
		for _, s := range strings.Split(*stateFilter, ",") {
			wanted[strings.ToUpper(strings.TrimSpace(s))] = true
		}
		filtered := conns[:0]
		for _, c := range conns {
			if wanted[strings.ToUpper(c.State)] {
				filtered = append(filtered, c)
			}
		}
		conns = filtered
	}
	if *portFilter != 0 {
		pf := *portFilter
		filtered := conns[:0]
		for _, c := range conns {
			if c.LocalPort == pf || c.RemotePort == pf {
				filtered = append(filtered, c)
			}
		}
		conns = filtered
	}
	if *listeningOnly {
		filtered := conns[:0]
		for _, c := range conns {
			if isListeningConn(c) {
				filtered = append(filtered, c)
			}
		}
		conns = filtered
	}

	// Sorting
	switch strings.ToLower(*sortBy) {
	case "recvq":
		sort.Slice(conns, func(i, j int) bool { return conns[i].RxQueue > conns[j].RxQueue })
	case "sendq":
		sort.Slice(conns, func(i, j int) bool { return conns[i].TxQueue > conns[j].TxQueue })
	case "local":
		sort.Slice(conns, func(i, j int) bool { return conns[i].LocalPort < conns[j].LocalPort })
	case "remote":
		sort.Slice(conns, func(i, j int) bool { return conns[i].RemotePort < conns[j].RemotePort })
	case "pid":
		sort.Slice(conns, func(i, j int) bool {
			pi := inodeToPid[conns[i].Inode]
			pj := inodeToPid[conns[j].Inode]
			if pi == pj {
				return conns[i].Inode < conns[j].Inode
			}
			return pi < pj
		})
	}

	printNetstatHeader(*extended, *timers)
	for _, c := range conns {
		pid := "-"
		pname := "-"
		if p, ok := inodeToPid[c.Inode]; ok {
			pid = strconv.Itoa(p)
			if n, ok2 := pidName[pid]; ok2 {
				pname = n
			}
		}
		local := formatNetstatAddress(c.LocalIP, c.LocalPort)
		remote := formatNetstatAddress(c.RemoteIP, c.RemotePort)
		proto := c.Proto
		if proto == "" {
			proto = "TCP"
		}
		printNetstatRow(c, proto, local, remote, pid+"/"+pname, *extended, *timers)
	}
	return nil
}

func printNetstatHeader(extended, timers bool) {
	fmt.Printf("%-7s %-7s %-6s %-25s %-25s %-12s %s", "Recv-Q", "Send-Q", "Proto", "LocalAddress", "RemoteAddress", "State", "PID/Program")
	if extended {
		fmt.Printf(" %-8s %s", "User", "Inode")
	}
	if timers {
		fmt.Printf(" %s", "Timer")
	}
	fmt.Println()
}

func printNetstatRow(c tcpConn, proto, local, remote, pidProgram string, extended, timers bool) {
	fmt.Printf("%-7d %-7d %-6s %-25s %-25s %-12s %s", c.RxQueue, c.TxQueue, proto, local, remote, c.State, pidProgram)
	if extended {
		fmt.Printf(" %-8s %s", c.UID, c.Inode)
	}
	if timers {
		fmt.Printf(" %s", c.Timer)
	}
	fmt.Println()
}

func formatNetstatAddress(addr string, port int) string {
	if port == 0 {
		if addr == "" {
			return "-"
		}
		return addr
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

func isListeningConn(c tcpConn) bool {
	if strings.EqualFold(c.State, "LISTEN") {
		return true
	}
	if c.Proto == "UNIX" && strings.EqualFold(c.State, "LISTENING") {
		return true
	}
	return strings.HasPrefix(c.Proto, "UDP") && c.RemotePort == 0
}

type tcpConn struct {
	LocalPort  int
	RemotePort int
	TxQueue    int
	RxQueue    int
	Inode      string
	UID        string
	LocalIP    string
	RemoteIP   string
	State      string
	Proto      string
	Timer      string
}

func parseProcNetTCP(path string, proto string) ([]tcpConn, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var res []tcpConn
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		// fields[1] = local_address, fields[2] = rem_address, fields[3] = st, fields[4] = tx_queue:rx_queue, fields[9] = inode
		local := fields[1]
		remote := fields[2]
		stateHex := fields[3]
		txrx := fields[4]
		timer := fields[5]
		uid := fields[7]
		inode := fields[9]

		lp := parsePortFromAddr(local)
		rp := parsePortFromAddr(remote)
		lip := parseIPFromAddr(local)
		rip := parseIPFromAddr(remote)

		tx, rx := 0, 0
		if parts := strings.Split(txrx, ":"); len(parts) == 2 {
			if v, err := strconv.ParseUint(parts[0], 16, 64); err == nil {
				tx = int(v)
			}
			if v, err := strconv.ParseUint(parts[1], 16, 64); err == nil {
				rx = int(v)
			}
		}

		res = append(res, tcpConn{
			LocalPort:  lp,
			RemotePort: rp,
			TxQueue:    tx,
			RxQueue:    rx,
			Inode:      inode,
			UID:        uid,
			LocalIP:    lip,
			RemoteIP:   rip,
			State:      tcpStateName(stateHex),
			Proto:      proto,
			Timer:      timer,
		})
	}
	if err := scanner.Err(); err != nil {
		return res, err
	}
	return res, nil
}

func parsePortFromAddr(addr string) int {
	// addr is like "0100007F:0035" or for IPv6 a larger hex; we only need port after ':'
	parts := strings.Split(addr, ":")
	if len(parts) < 2 {
		return 0
	}
	ph := parts[len(parts)-1]
	if v, err := strconv.ParseUint(ph, 16, 16); err == nil {
		return int(v)
	}
	return 0
}

func parseIPFromAddr(addr string) string {
	// addr like "0100007F:0035" for IPv4 (8 hex chars) or 32 hex chars for IPv6
	parts := strings.Split(addr, ":")
	if len(parts) < 2 {
		return ""
	}
	ih := parts[0]
	// IPv4 (8 hex chars) appears in little-endian in /proc/net/tcp
	if len(ih) == 8 {
		// read bytes in pairs and reverse
		var bytes [4]byte
		for i := 0; i < 4; i++ {
			b, err := strconv.ParseUint(ih[i*2:i*2+2], 16, 8)
			if err != nil {
				return ""
			}
			bytes[3-i] = byte(b)
		}
		return fmt.Sprintf("%d.%d.%d.%d", bytes[0], bytes[1], bytes[2], bytes[3])
	}
	// IPv6: 32 hex chars -> 16 bytes
	if len(ih) == 32 {
		b, err := hex.DecodeString(ih)
		if err != nil || len(b) != 16 {
			return ""
		}
		ip := net.IP(b)
		return ip.String()
	}
	// fallback: return the hex string
	return ih
}

func parseProcNetUDP(path string, proto string) ([]tcpConn, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var res []tcpConn
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		// fields[1] = local_address, fields[2] = rem_address, fields[3] = st, fields[4] = tx_queue:rx_queue, fields[9] = inode
		local := fields[1]
		remote := fields[2]
		stateHex := fields[3]
		txrx := fields[4]
		timer := fields[5]
		uid := fields[7]
		inode := fields[9]

		lp := parsePortFromAddr(local)
		rp := parsePortFromAddr(remote)
		lip := parseIPFromAddr(local)
		rip := parseIPFromAddr(remote)

		tx, rx := 0, 0
		if parts := strings.Split(txrx, ":"); len(parts) == 2 {
			if v, err := strconv.ParseUint(parts[0], 16, 64); err == nil {
				tx = int(v)
			}
			if v, err := strconv.ParseUint(parts[1], 16, 64); err == nil {
				rx = int(v)
			}
		}

		res = append(res, tcpConn{
			LocalPort:  lp,
			RemotePort: rp,
			TxQueue:    tx,
			RxQueue:    rx,
			Inode:      inode,
			UID:        uid,
			LocalIP:    lip,
			RemoteIP:   rip,
			State:      tcpStateName(stateHex),
			Proto:      proto,
			Timer:      timer,
		})
	}
	if err := scanner.Err(); err != nil {
		return res, err
	}
	return res, nil
}

func parseProcNetUnix(path string) ([]tcpConn, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var res []tcpConn
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		path := "-"
		if len(fields) >= 8 {
			path = strings.Join(fields[7:], " ")
		}
		res = append(res, tcpConn{
			Inode:    fields[6],
			LocalIP:  path,
			RemoteIP: "-",
			State:    unixStateName(fields[5]),
			Proto:    "UNIX",
		})
	}
	if err := scanner.Err(); err != nil {
		return res, err
	}
	return res, nil
}

func unixStateName(h string) string {
	switch strings.ToUpper(strings.TrimPrefix(h, "0x")) {
	case "01", "1":
		return "LISTENING"
	case "02", "2":
		return "CONNECTED"
	case "03", "3":
		return "CONNECTING"
	case "04", "4":
		return "DISCONNECTING"
	default:
		return h
	}
}

func tcpStateName(h string) string {
	switch strings.ToUpper(h) {
	case "01":
		return "ESTABLISHED"
	case "02":
		return "SYN_SENT"
	case "03":
		return "SYN_RECV"
	case "04":
		return "FIN_WAIT1"
	case "05":
		return "FIN_WAIT2"
	case "06":
		return "TIME_WAIT"
	case "07":
		return "CLOSE"
	case "08":
		return "CLOSE_WAIT"
	case "09":
		return "LAST_ACK"
	case "0A", "0a":
		return "LISTEN"
	case "0B", "0b":
		return "CLOSING"
	default:
		return h
	}
}

// buildInodePidMap walks /proc and finds which pid owns a given socket inode
func buildInodePidMap() (map[string]int, map[string]string) {
	inodeToPid := make(map[string]int)
	pidName := make(map[string]string)

	procEntries, err := os.ReadDir("/proc")
	if err != nil {
		return inodeToPid, pidName
	}
	for _, e := range procEntries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// pid directories are numeric
		if _, err := strconv.Atoi(name); err != nil {
			continue
		}
		pid := name
		// read process name
		commPath := filepath.Join("/proc", pid, "comm")
		pname := ""
		if b, err := os.ReadFile(commPath); err == nil {
			pname = strings.TrimSpace(string(b))
		}
		pidName[pid] = pname

		fdDir := filepath.Join("/proc", pid, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link := filepath.Join(fdDir, fd.Name())
			target, err := os.Readlink(link)
			if err != nil {
				continue
			}
			// socket:[12345]
			if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") {
				inode := target[len("socket:[") : len(target)-1]
				if inode != "" {
					if _, exists := inodeToPid[inode]; !exists {
						if pidInt, err := strconv.Atoi(pid); err == nil {
							inodeToPid[inode] = pidInt
						}
					}
				}
			}
		}
	}
	return inodeToPid, pidName
}
