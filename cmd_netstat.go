package main

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

func netstatCmd(args []string) error {
	fsFlags := flag.NewFlagSet("netstat", flag.ContinueOnError)
	stateFilter := fsFlags.String("state", "", "filter by connection state (comma-separated, e.g., LISTEN,ESTABLISHED)")
	portFilter := fsFlags.Int("port", 0, "filter by local or remote port")
	sortBy := fsFlags.String("sort", "", "sort by recvq|sendq|local|remote|pid")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox netstat")
		fmt.Fprintln(os.Stderr, "Print simple network device statistics (Linux /proc/net/dev).")
		fmt.Fprintln(os.Stderr, "Flags:")
		fsFlags.PrintDefaults()
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if runtime.GOOS != "linux" {
		return errors.New("netstat: supported only on Linux in this implementation")
	}

	// Parse tcp/udp tables
	conns := make([]tcpConn, 0)
	if cs, err := parseProcNetTCP("/proc/net/tcp"); err == nil {
		conns = append(conns, cs...)
	}
	if cs, err := parseProcNetTCP("/proc/net/tcp6"); err == nil {
		conns = append(conns, cs...)
	}
	if cs, err := parseProcNetUDP("/proc/net/udp", "UDP"); err == nil {
		conns = append(conns, cs...)
	}
	if cs, err := parseProcNetUDP("/proc/net/udp6", "UDP6"); err == nil {
		conns = append(conns, cs...)
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
		sort.Slice(conns, func(i, j int) bool { return conns[i].Inode < conns[j].Inode })
	}

	// Print header: Recv-Q Send-Q Proto LocalAddress RemoteAddress State PID/Program
	fmt.Printf("%-7s %-7s %-6s %-25s %-25s %-12s %s\n", "Recv-Q", "Send-Q", "Proto", "LocalAddress", "RemoteAddress", "State", "PID/Program")
	for _, c := range conns {
		pid := "-"
		pname := "-"
		if p, ok := inodeToPid[c.Inode]; ok {
			pid = strconv.Itoa(p)
			if n, ok2 := pidName[pid]; ok2 {
				pname = n
			}
		}
		local := fmt.Sprintf("%s:%d", c.LocalIP, c.LocalPort)
		remote := fmt.Sprintf("%s:%d", c.RemoteIP, c.RemotePort)
		proto := c.Proto
		if proto == "" {
			proto = "TCP"
		}
		fmt.Printf("%-7d %-7d %-6s %-25s %-25s %-12s %s\n", c.RxQueue, c.TxQueue, proto, local, remote, c.State, pid+"/"+pname)
	}
	return nil
}

type tcpConn struct {
	LocalPort  int
	RemotePort int
	TxQueue    int
	RxQueue    int
	Inode      string
	LocalIP    string
	RemoteIP   string
	State      string
	Proto      string
}

func parseProcNetTCP(path string) ([]tcpConn, error) {
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
			LocalIP:    lip,
			RemoteIP:   rip,
			State:      tcpStateName(stateHex),
			Proto:      "TCP",
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
			LocalIP:    lip,
			RemoteIP:   rip,
			State:      tcpStateName(stateHex),
			Proto:      proto,
		})
	}
	if err := scanner.Err(); err != nil {
		return res, err
	}
	return res, nil
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
