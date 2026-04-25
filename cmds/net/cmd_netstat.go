package net

import (
	"bufio"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var parseProcNetDevNetstat = parseProcNetDev

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
	numericOnly := fsFlags.Bool("n", false, "show numeric addresses and ports (current output is already numeric)")
	numericOnlyLong := fsFlags.Bool("numeric", false, "show numeric addresses and ports (current output is already numeric)")
	programs := fsFlags.Bool("p", false, "show PID/Program name for sockets")
	programsLong := fsFlags.Bool("programs", false, "show PID/Program name for sockets")
	routeTable := fsFlags.Bool("r", false, "show routing table")
	routeTableLong := fsFlags.Bool("route", false, "show routing table")
	interfaces := fsFlags.Bool("i", false, "show network interfaces")
	interfacesLong := fsFlags.Bool("interfaces", false, "show network interfaces")
	statistics := fsFlags.Bool("s", false, "show protocol statistics")
	statisticsLong := fsFlags.Bool("statistics", false, "show protocol statistics")
	continuous := fsFlags.Bool("c", false, "show output continuously")
	continuousLong := fsFlags.Bool("continuous", false, "show output continuously")
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
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Filters:")
		fmt.Fprintln(os.Stderr, "  -t, --tcp           show TCP sockets only")
		fmt.Fprintln(os.Stderr, "  -u, --udp           show UDP sockets only")
		fmt.Fprintln(os.Stderr, "  -x, --unix          show Unix domain sockets only")
		fmt.Fprintln(os.Stderr, "  -l, --listening     show listening sockets only")
		fmt.Fprintln(os.Stderr, "  -4                  show IPv4 sockets only")
		fmt.Fprintln(os.Stderr, "  -6                  show IPv6 sockets only")
		fmt.Fprintln(os.Stderr, "      --state STATES  filter by connection state list")
		fmt.Fprintln(os.Stderr, "      --port PORT     filter by local or remote port")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Output:")
		fmt.Fprintln(os.Stderr, "  -p, --programs      show PID/Program column")
		fmt.Fprintln(os.Stderr, "  -e, --extend        show extended socket information")
		fmt.Fprintln(os.Stderr, "  -o, --timers        show TCP timer information")
		fmt.Fprintln(os.Stderr, "  -n, --numeric       numeric output; current gobox output is already numeric")
		fmt.Fprintln(os.Stderr, "  -W, --wide          accepted for compatibility; gobox does not truncate addresses")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Views:")
		fmt.Fprintln(os.Stderr, "  -r, --route         show routing table")
		fmt.Fprintln(os.Stderr, "  -i, --interfaces    show network interfaces")
		fmt.Fprintln(os.Stderr, "  -s, --statistics    show protocol statistics")
		fmt.Fprintln(os.Stderr, "  -c, --continuous    refresh output continuously")
		fmt.Fprintln(os.Stderr, "  -a, --all           accepted; currently does not change default socket selection")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Sorting:")
		fmt.Fprintln(os.Stderr, "      --sort FIELD    sort by recvq|sendq|local|remote|pid")
	}
	if err := fsFlags.Parse(normalizeNetstatArgs(args)); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *allSocketsLong {
		*allSockets = true
	}
	if *numericOnlyLong {
		*numericOnly = true
	}
	if *wideLong {
		*wide = true
	}
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
	if *programsLong {
		*programs = true
	}
	if *routeTableLong {
		*routeTable = true
	}
	if *interfacesLong {
		*interfaces = true
	}
	if *statisticsLong {
		*statistics = true
	}
	if *continuousLong {
		*continuous = true
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

	render := func() error {
		if *routeTable || *interfaces || *statistics {
			first := true
			if *routeTable {
				if err := printNetstatRoutes(); err != nil {
					return err
				}
				first = false
			}
			if *interfaces {
				if !first {
					fmt.Println()
				}
				if err := printNetstatInterfaces(*extended); err != nil {
					return err
				}
				first = false
			}
			if *statistics {
				if !first {
					fmt.Println()
				}
				if err := printNetstatStats(*tcpOnly, *udpOnly, *unixOnly, *ipv4Only, *ipv6Only); err != nil {
					return err
				}
			}
			return nil
		}
		return printNetstatSockets(*allSockets, *tcpOnly, *udpOnly, *unixOnly, *listeningOnly, *numericOnly, *ipv4Only, *ipv6Only, *extended, *timers, *programs, *wide, *stateFilter, *portFilter, *sortBy)
	}

	if *continuous {
		return runNetstatContinuous(render)
	}
	return render()
}

func printNetstatSockets(allSockets, tcpOnly, udpOnly, unixOnly, listeningOnly, numericOnly, ipv4Only, ipv6Only, extended, timers, programs, wide bool, stateFilter string, portFilter int, sortBy string) error {
	_ = allSockets
	_ = numericOnly
	_ = wide
	// Parse tcp/udp tables
	conns := make([]tcpConn, 0)
	if !unixOnly && !ipv6Only {
		if cs, err := parseProcNetTCP("/proc/net/tcp", "TCP"); err == nil {
			conns = append(conns, cs...)
		}
	}
	if !unixOnly && !ipv4Only {
		if cs, err := parseProcNetTCP("/proc/net/tcp6", "TCP6"); err == nil {
			conns = append(conns, cs...)
		}
	}
	if !unixOnly && !ipv6Only {
		if cs, err := parseProcNetUDP("/proc/net/udp", "UDP"); err == nil {
			conns = append(conns, cs...)
		}
	}
	if !unixOnly && !ipv4Only {
		if cs, err := parseProcNetUDP("/proc/net/udp6", "UDP6"); err == nil {
			conns = append(conns, cs...)
		}
	}
	if unixOnly || (!tcpOnly && !udpOnly && !ipv4Only && !ipv6Only) {
		if cs, err := parseProcNetUnix("/proc/net/unix"); err == nil {
			conns = append(conns, cs...)
		}
	}

	if tcpOnly || udpOnly || unixOnly {
		filtered := conns[:0]
		for _, c := range conns {
			if tcpOnly && strings.HasPrefix(c.Proto, "TCP") {
				filtered = append(filtered, c)
			}
			if udpOnly && strings.HasPrefix(c.Proto, "UDP") {
				filtered = append(filtered, c)
			}
			if unixOnly && c.Proto == "UNIX" {
				filtered = append(filtered, c)
			}
		}
		conns = filtered
	}

	inodeToPid, pidName := buildInodePidMap()

	// Apply filtering by state and port
	if stateFilter != "" {
		wanted := make(map[string]bool)
		for _, s := range strings.Split(stateFilter, ",") {
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
	if portFilter != 0 {
		pf := portFilter
		filtered := conns[:0]
		for _, c := range conns {
			if c.LocalPort == pf || c.RemotePort == pf {
				filtered = append(filtered, c)
			}
		}
		conns = filtered
	}
	if listeningOnly {
		filtered := conns[:0]
		for _, c := range conns {
			if isListeningConn(c) {
				filtered = append(filtered, c)
			}
		}
		conns = filtered
	}

	// Sorting
	switch strings.ToLower(sortBy) {
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

	rows := make([]netstatSocketRow, 0, len(conns))
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
		rows = append(rows, netstatSocketRow{
			conn:       c,
			proto:      proto,
			local:      local,
			remote:     remote,
			pidProgram: pid + "/" + pname,
		})
	}
	printNetstatTable(rows, extended, timers, programs)
	return nil
}

func normalizeNetstatArgs(args []string) []string {
	knownWordFlags := map[string]bool{
		"state": true, "port": true, "sort": true,
		"all": true, "tcp": true, "udp": true, "unix": true, "listening": true,
		"numeric": true, "programs": true, "route": true, "interfaces": true,
		"statistics": true, "continuous": true, "extend": true, "timers": true,
		"wide": true,
	}
	boolShort := "atulnpriscexoW46"
	out := make([]string, 0, len(args))
	for _, arg := range args {
		name := strings.TrimPrefix(arg, "-")
		if strings.HasPrefix(arg, "--") || !strings.HasPrefix(arg, "-") || len(arg) <= 2 || strings.Contains(arg, "=") || knownWordFlags[name] {
			out = append(out, arg)
			continue
		}
		combined := true
		for _, r := range name {
			if !strings.ContainsRune(boolShort, r) {
				combined = false
				break
			}
		}
		if !combined {
			out = append(out, arg)
			continue
		}
		for _, r := range name {
			out = append(out, "-"+string(r))
		}
	}
	return out
}

func runNetstatContinuous(render func() error) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	for {
		if err := render(); err != nil {
			return err
		}
		select {
		case <-sigCh:
			return nil
		case <-time.After(time.Second):
			fmt.Println()
		}
	}
}

type netstatSocketRow struct {
	conn       tcpConn
	proto      string
	local      string
	remote     string
	pidProgram string
}

func printNetstatTable(rows []netstatSocketRow, extended, timers, programs bool) {
	recvWidth := len("Recv-Q")
	sendWidth := len("Send-Q")
	protoWidth := len("Proto")
	localWidth := len("LocalAddress")
	remoteWidth := len("RemoteAddress")
	stateWidth := len("State")
	userWidth := len("User")
	inodeWidth := len("Inode")
	pidProgramWidth := len("PID/Program")
	timerWidth := len("Timer")

	for _, row := range rows {
		if l := len(strconv.Itoa(row.conn.RxQueue)); l > recvWidth {
			recvWidth = l
		}
		if l := len(strconv.Itoa(row.conn.TxQueue)); l > sendWidth {
			sendWidth = l
		}
		if len(row.proto) > protoWidth {
			protoWidth = len(row.proto)
		}
		if len(row.local) > localWidth {
			localWidth = len(row.local)
		}
		if len(row.remote) > remoteWidth {
			remoteWidth = len(row.remote)
		}
		if len(row.conn.State) > stateWidth {
			stateWidth = len(row.conn.State)
		}
		if len(row.conn.UID) > userWidth {
			userWidth = len(row.conn.UID)
		}
		if len(row.conn.Inode) > inodeWidth {
			inodeWidth = len(row.conn.Inode)
		}
		if len(row.pidProgram) > pidProgramWidth {
			pidProgramWidth = len(row.pidProgram)
		}
		if len(row.conn.Timer) > timerWidth {
			timerWidth = len(row.conn.Timer)
		}
	}

	fmt.Printf("%*s %*s %-*s %-*s %-*s %-*s", recvWidth, "Recv-Q", sendWidth, "Send-Q", protoWidth, "Proto", localWidth, "LocalAddress", remoteWidth, "RemoteAddress", stateWidth, "State")
	if programs {
		fmt.Printf(" %-*s", pidProgramWidth, "PID/Program")
	}
	if extended {
		fmt.Printf(" %-*s %-*s", userWidth, "User", inodeWidth, "Inode")
	}
	if timers {
		fmt.Printf(" %-*s", timerWidth, "Timer")
	}
	fmt.Println()

	for _, row := range rows {
		fmt.Printf("%*d %*d %-*s %-*s %-*s %-*s", recvWidth, row.conn.RxQueue, sendWidth, row.conn.TxQueue, protoWidth, row.proto, localWidth, row.local, remoteWidth, row.remote, stateWidth, row.conn.State)
		if programs {
			fmt.Printf(" %-*s", pidProgramWidth, row.pidProgram)
		}
		if extended {
			fmt.Printf(" %-*s %-*s", userWidth, row.conn.UID, inodeWidth, row.conn.Inode)
		}
		if timers {
			fmt.Printf(" %-*s", timerWidth, row.conn.Timer)
		}
		fmt.Println()
	}
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

type netstatIPv4Route struct {
	Iface       string
	Destination string
	Gateway     string
	Genmask     string
	Flags       string
	Metric      string
}

type netstatIPv6Route struct {
	Iface       string
	Destination string
	Gateway     string
	Flags       string
	Metric      string
}

type netstatInterface struct {
	Name    string
	MTU     int
	Flags   string
	HWAddr  string
	RXBytes uint64
	RXOK    uint64
	RXErr   uint64
	RXDrop  uint64
	TXBytes uint64
	TXOK    uint64
	TXErr   uint64
	TXDrop  uint64
}

type netstatStatSection struct {
	Name   string
	Stats  map[string]string
	Fields []string
}

func printNetstatRoutes() error {
	ipv4, err4 := parseProcNetRoute("/proc/net/route")
	ipv6, err6 := parseProcNetIPv6Route("/proc/net/ipv6_route")
	if err4 != nil && err6 != nil {
		return err4
	}
	fmt.Println("Kernel IP routing table")
	fmt.Printf("%-15s %-15s %-15s %-6s %-6s %s\n", "Destination", "Gateway", "Genmask", "Flags", "Metric", "Iface")
	for _, r := range ipv4 {
		fmt.Printf("%-15s %-15s %-15s %-6s %-6s %s\n", r.Destination, r.Gateway, r.Genmask, r.Flags, r.Metric, r.Iface)
	}
	if len(ipv6) > 0 {
		fmt.Println()
		fmt.Println("Kernel IPv6 routing table")
		fmt.Printf("%-39s %-39s %-6s %-6s %s\n", "Destination", "Gateway", "Flags", "Metric", "Iface")
		for _, r := range ipv6 {
			fmt.Printf("%-39s %-39s %-6s %-6s %s\n", r.Destination, r.Gateway, r.Flags, r.Metric, r.Iface)
		}
	}
	return nil
}

func parseProcNetRoute(path string) ([]netstatIPv4Route, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var routes []netstatIPv4Route
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 {
			continue
		}
		routes = append(routes, netstatIPv4Route{
			Iface:       fields[0],
			Destination: parseIPv4RouteHex(fields[1]),
			Gateway:     parseIPv4RouteHex(fields[2]),
			Flags:       routeFlagsName(fields[3]),
			Metric:      fields[6],
			Genmask:     parseIPv4RouteHex(fields[7]),
		})
	}
	return routes, scanner.Err()
}

func parseProcNetIPv6Route(path string) ([]netstatIPv6Route, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var routes []netstatIPv6Route
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		destPrefix := parseHexInt(fields[1], 64)
		destination := parseIPv6RouteHex(fields[0])
		if destPrefix >= 0 {
			destination = fmt.Sprintf("%s/%d", destination, destPrefix)
		}
		routes = append(routes, netstatIPv6Route{
			Destination: destination,
			Gateway:     parseIPv6RouteHex(fields[4]),
			Metric:      fmt.Sprintf("%d", parseHexInt(fields[5], 64)),
			Flags:       routeFlagsName(fields[8]),
			Iface:       fields[9],
		})
	}
	return routes, scanner.Err()
}

func parseIPv4RouteHex(s string) string {
	if len(s) != 8 {
		return s
	}
	var bytes [4]byte
	for i := 0; i < 4; i++ {
		v, err := strconv.ParseUint(s[i*2:i*2+2], 16, 8)
		if err != nil {
			return s
		}
		bytes[3-i] = byte(v)
	}
	return fmt.Sprintf("%d.%d.%d.%d", bytes[0], bytes[1], bytes[2], bytes[3])
}

func parseIPv6RouteHex(s string) string {
	if len(s) != 32 {
		return s
	}
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 16 {
		return s
	}
	return net.IP(b).String()
}

func routeFlagsName(s string) string {
	flags := parseHexInt(s, 64)
	if flags < 0 {
		return s
	}
	var out strings.Builder
	if flags&0x1 != 0 {
		out.WriteByte('U')
	}
	if flags&0x2 != 0 {
		out.WriteByte('G')
	}
	if flags&0x4 != 0 {
		out.WriteByte('H')
	}
	if flags&0x10 != 0 {
		out.WriteByte('D')
	}
	if flags&0x20 != 0 {
		out.WriteByte('M')
	}
	if out.Len() == 0 {
		return "-"
	}
	return out.String()
}

func printNetstatInterfaces(extended bool) error {
	ifaces, err := parseProcNetDevNetstat("/proc/net/dev")
	if err != nil {
		return err
	}
	nameWidth := len("Iface")
	hwAddrWidth := len("HWaddr")
	for _, iface := range ifaces {
		if len(iface.Name) > nameWidth {
			nameWidth = len(iface.Name)
		}
		if len(iface.HWAddr) > hwAddrWidth {
			hwAddrWidth = len(iface.HWAddr)
		}
	}
	if extended {
		fmt.Printf("%-*s %6s %12s %10s %7s %7s %12s %10s %7s %7s %-*s %s\n", nameWidth, "Iface", "MTU", "RX-Bytes", "RX-OK", "RX-ERR", "RX-DRP", "TX-Bytes", "TX-OK", "TX-ERR", "TX-DRP", hwAddrWidth, "HWaddr", "Flg")
		for _, iface := range ifaces {
			fmt.Printf("%-*s %6d %12d %10d %7d %7d %12d %10d %7d %7d %-*s %s\n", nameWidth, iface.Name, iface.MTU, iface.RXBytes, iface.RXOK, iface.RXErr, iface.RXDrop, iface.TXBytes, iface.TXOK, iface.TXErr, iface.TXDrop, hwAddrWidth, iface.HWAddr, iface.Flags)
		}
		return nil
	}
	fmt.Printf("%-*s %6s %10s %7s %7s %10s %7s %7s %s\n", nameWidth, "Iface", "MTU", "RX-OK", "RX-ERR", "RX-DRP", "TX-OK", "TX-ERR", "TX-DRP", "Flg")
	for _, iface := range ifaces {
		fmt.Printf("%-*s %6d %10d %7d %7d %10d %7d %7d %s\n", nameWidth, iface.Name, iface.MTU, iface.RXOK, iface.RXErr, iface.RXDrop, iface.TXOK, iface.TXErr, iface.TXDrop, iface.Flags)
	}
	return nil
}

func parseProcNetDev(path string) ([]netstatInterface, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ifaces []netstatInterface
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if lineNo <= 2 {
			continue
		}
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}
		iface := netstatInterface{
			Name:    name,
			RXBytes: parseUintField(fields[0]),
			RXOK:    parseUintField(fields[1]),
			RXErr:   parseUintField(fields[2]),
			RXDrop:  parseUintField(fields[3]),
			TXBytes: parseUintField(fields[8]),
			TXOK:    parseUintField(fields[9]),
			TXErr:   parseUintField(fields[10]),
			TXDrop:  parseUintField(fields[11]),
			MTU:     -1,
			Flags:   "-",
			HWAddr:  "-",
		}
		if ni, err := net.InterfaceByName(name); err == nil {
			iface.MTU = ni.MTU
			if ni.Flags.String() != "" {
				iface.Flags = ni.Flags.String()
			}
			if ni.HardwareAddr.String() != "" {
				iface.HWAddr = ni.HardwareAddr.String()
			}
		}
		ifaces = append(ifaces, iface)
	}
	return ifaces, scanner.Err()
}

func printNetstatStats(tcpOnly, udpOnly, unixOnly, ipv4Only, ipv6Only bool) error {
	sections, err := parseNetstatStatsFiles([]string{"/proc/net/snmp", "/proc/net/netstat", "/proc/net/snmp6"})
	if err != nil {
		return err
	}
	sections = filterNetstatStatSections(sections, tcpOnly, udpOnly, unixOnly, ipv4Only, ipv6Only)
	for i, section := range sections {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s:\n", section.Name)
		for _, field := range section.Fields {
			fmt.Printf("    %-32s %s\n", field, section.Stats[field])
		}
	}
	return nil
}

func filterNetstatStatSections(sections []netstatStatSection, tcpOnly, udpOnly, unixOnly, ipv4Only, ipv6Only bool) []netstatStatSection {
	if unixOnly {
		return nil
	}
	allowed := make(map[string]bool)
	addTCP := func() {
		allowed["Tcp"] = true
		allowed["TcpExt"] = true
		if !ipv4Only {
			allowed["Tcp6"] = true
		}
	}
	addUDP := func() {
		allowed["Udp"] = true
		allowed["UdpLite"] = true
		if !ipv4Only {
			allowed["Udp6"] = true
			allowed["UdpLite6"] = true
		}
	}
	if tcpOnly || udpOnly {
		if tcpOnly {
			addTCP()
		}
		if udpOnly {
			addUDP()
		}
		filtered := sections[:0]
		for _, section := range sections {
			if allowed[section.Name] {
				filtered = append(filtered, section)
			}
		}
		return filtered
	}
	return sections
}

func parseNetstatStatsFiles(paths []string) ([]netstatStatSection, error) {
	sectionsByName := make(map[string]*netstatStatSection)
	var order []string
	var firstErr error
	for _, path := range paths {
		if strings.HasSuffix(path, "snmp6") {
			if err := parseProcNetSingleStats(path, sectionsByName, &order); err != nil && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := parseProcNetPairedStats(path, sectionsByName, &order); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if len(order) == 0 && firstErr != nil {
		return nil, firstErr
	}
	sections := make([]netstatStatSection, 0, len(order))
	for _, name := range order {
		sections = append(sections, *sectionsByName[name])
	}
	return sections, nil
}

func parseProcNetPairedStats(path string, sections map[string]*netstatStatSection, order *[]string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var headers []string
	var sectionName string
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSuffix(fields[0], ":")
		if len(headers) == 0 || name != sectionName {
			sectionName = name
			headers = fields[1:]
			continue
		}
		section := ensureNetstatStatSection(name, sections, order)
		for i, header := range headers {
			if i+1 >= len(fields) {
				break
			}
			if _, exists := section.Stats[header]; !exists {
				section.Fields = append(section.Fields, header)
			}
			section.Stats[header] = fields[i+1]
		}
		headers = nil
		sectionName = ""
	}
	return scanner.Err()
}

func parseProcNetSingleStats(path string, sections map[string]*netstatStatSection, order *[]string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		name, field := splitSNMP6Field(fields[0])
		section := ensureNetstatStatSection(name, sections, order)
		if _, exists := section.Stats[field]; !exists {
			section.Fields = append(section.Fields, field)
		}
		section.Stats[field] = fields[1]
	}
	return scanner.Err()
}

func ensureNetstatStatSection(name string, sections map[string]*netstatStatSection, order *[]string) *netstatStatSection {
	if section, ok := sections[name]; ok {
		return section
	}
	section := &netstatStatSection{Name: name, Stats: make(map[string]string)}
	sections[name] = section
	*order = append(*order, name)
	return section
}

func splitSNMP6Field(field string) (string, string) {
	for _, prefix := range []string{"Ip6", "Icmp6", "Udp6", "Tcp6", "UdpLite6"} {
		if strings.HasPrefix(field, prefix) {
			return prefix, strings.TrimPrefix(field, prefix)
		}
	}
	return "Snmp6", field
}

func parseUintField(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

func parseHexInt(s string, bitSize int) int64 {
	v, err := strconv.ParseInt(s, 16, bitSize)
	if err != nil {
		return -1
	}
	return v
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
