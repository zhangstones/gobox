package net

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func captureNetOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	defer rOut.Close()
	defer rErr.Close()

	os.Stdout = wOut
	os.Stderr = wErr
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&outBuf, rOut)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&errBuf, rErr)
	}()

	runErr := fn()
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	wg.Wait()
	return outBuf.String() + errBuf.String(), runErr
}

func netstatDataLines(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) <= 1 {
		return nil
	}
	return lines[1:]
}

func TestParsePortFromAddr(t *testing.T) {
	if got := parsePortFromAddr("0100007F:0035"); got != 53 {
		t.Fatalf("expected port 53, got %d", got)
	}
	if got := parsePortFromAddr("bad"); got != 0 {
		t.Fatalf("expected port 0 for bad input, got %d", got)
	}
}

func TestParseIPFromAddr(t *testing.T) {
	if got := parseIPFromAddr("0100007F:0035"); got != "127.0.0.1" {
		t.Fatalf("expected IPv4 127.0.0.1, got %q", got)
	}
	if got := parseIPFromAddr("00000000000000000000000000000001:0035"); got != "::1" {
		t.Fatalf("expected IPv6 ::1, got %q", got)
	}
	if got := parseIPFromAddr("bad"); got != "" {
		t.Fatalf("expected empty IP for bad input, got %q", got)
	}
}

func TestTCPStateName(t *testing.T) {
	if got := tcpStateName("01"); got != "ESTABLISHED" {
		t.Fatalf("expected ESTABLISHED, got %q", got)
	}
	if got := tcpStateName("0A"); got != "LISTEN" {
		t.Fatalf("expected LISTEN, got %q", got)
	}
	if got := tcpStateName("ZZ"); got != "ZZ" {
		t.Fatalf("expected fallback to input, got %q", got)
	}
}

func TestParseProcNetTCP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tcp")
	content := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:0035 00000000:0000 0A 00000000:00000000 00:00000000 00000000   100        0 12345 1 0000000000000000 100 0 0 10 0\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	conns, err := parseProcNetTCP(path, "TCP")
	if err != nil {
		t.Fatalf("parseProcNetTCP: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 conn, got %d", len(conns))
	}
	c := conns[0]
	if c.LocalPort != 53 || c.LocalIP != "127.0.0.1" || c.State != "LISTEN" {
		t.Fatalf("unexpected conn: %+v", c)
	}
}

func TestParseProcNetUDP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "udp")
	content := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:1F90 00000000:0000 07 00000000:00000000 00:00000000 00000000   100        0 54321 1 0000000000000000 100 0 0 10 0\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	conns, err := parseProcNetUDP(path, "UDP")
	if err != nil {
		t.Fatalf("parseProcNetUDP: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 conn, got %d", len(conns))
	}
	if conns[0].Proto != "UDP" {
		t.Fatalf("expected proto UDP, got %q", conns[0].Proto)
	}
}

func TestParseProcNetUnix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unix")
	content := "Num       RefCount Protocol Flags    Type St Inode Path\n" +
		"00000000: 00000002 00000000 00010000 0001 01 12345 " + filepath.Join(dir, "sock") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	conns, err := parseProcNetUnix(path)
	if err != nil {
		t.Fatalf("parseProcNetUnix: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 conn, got %d", len(conns))
	}
	if conns[0].Proto != "UNIX" || conns[0].State != "LISTENING" || !strings.Contains(conns[0].LocalIP, "sock") {
		t.Fatalf("unexpected unix conn: %+v", conns[0])
	}
}

func TestNetstatCmdPortFilterMatchesExactPort(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	output, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-port", strconv.Itoa(port)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd failed: %v", err)
	}

	target := ":" + strconv.Itoa(port)
	var matched []string
	for _, line := range netstatDataLines(output) {
		if strings.Contains(line, target) {
			matched = append(matched, line)
		}
	}
	if len(matched) == 0 {
		t.Fatalf("expected at least one row for port %d, got %q", port, output)
	}
	for _, line := range matched {
		if !strings.Contains(line, target) {
			t.Fatalf("unexpected matched line for port %d: %q", port, line)
		}
	}
}

func TestNetstatCmdStateFilterSupportsStateList(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	output, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-state", "LISTEN,ESTABLISHED", "-port", strconv.Itoa(port)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd failed: %v", err)
	}

	target := ":" + strconv.Itoa(port)
	rows := netstatDataLines(output)
	if len(rows) == 0 {
		t.Fatalf("expected at least one data row, got %q", output)
	}
	var found bool
	for _, line := range rows {
		if !strings.Contains(line, target) {
			continue
		}
		found = true
		if !strings.Contains(line, "LISTEN") && !strings.Contains(line, "ESTABLISHED") {
			t.Fatalf("expected state-filtered row for port %d to be LISTEN/ESTABLISHED, got %q", port, line)
		}
		if strings.Contains(line, "TIME_WAIT") {
			t.Fatalf("expected state filter to exclude unrelated states, got %q", line)
		}
	}
	if !found {
		t.Fatalf("expected state-filtered row for port %d, got %q", port, output)
	}
}

func TestNetstatCmdSortByPID(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	output, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-sort", "pid", "-p"})
	})
	if err != nil {
		t.Fatalf("NetstatCmd failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and at least one row, got %q", output)
	}

	prev := -1
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		pidProg := fields[len(fields)-1]
		pidStr, _, _ := strings.Cut(pidProg, "/")
		if pidStr == "-" {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		if prev != -1 && pid < prev {
			t.Fatalf("expected pid-sorted output, saw %d before %d in %q", prev, pid, output)
		}
		prev = pid
	}
}

func TestNetstatCmdProgramsFlagControlsPIDColumn(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	withoutPrograms, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-n"})
	})
	if err != nil {
		t.Fatalf("NetstatCmd -n failed: %v", err)
	}
	if strings.Contains(withoutPrograms, "PID/Program") {
		t.Fatalf("expected PID/Program to be hidden without -p, got %q", withoutPrograms)
	}

	withPrograms, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-p"})
	})
	if err != nil {
		t.Fatalf("NetstatCmd -p failed: %v", err)
	}
	if !strings.Contains(withPrograms, "PID/Program") {
		t.Fatalf("expected PID/Program with -p, got %q", withPrograms)
	}
}

func TestNetstatCmdCombinedShortFlags(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	output, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-tnlp", "-port", strconv.Itoa(port)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd combined flags failed: %v", err)
	}
	if !strings.Contains(output, "TCP") || !strings.Contains(output, "LISTEN") || !strings.Contains(output, "PID/Program") {
		t.Fatalf("expected combined -tnlp output for listener, got %q", output)
	}
}

func TestPrintNetstatTableAlignsLongAddresses(t *testing.T) {
	out, err := captureNetOutput(t, func() error {
		printNetstatTable([]netstatSocketRow{
			{
				conn:       tcpConn{RxQueue: 1, TxQueue: 2, State: "ESTABLISHED", UID: "1000", Inode: "123", Timer: "off"},
				proto:      "TCP6",
				local:      "fe80::1ff:fe23:4567:890a%eth0:65535",
				remote:     "/very/long/unix/socket/path/for/testing",
				pidProgram: "1234/really-long-process-name",
			},
		}, true, true, true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header and row, got %q", out)
	}
	headerState := strings.Index(lines[0], "State")
	rowState := strings.Index(lines[1], "ESTABLISHED")
	headerPID := strings.Index(lines[0], "PID/Program")
	rowPID := strings.Index(lines[1], "1234/really-long-process-name")
	if headerState == -1 || rowState != headerState || headerPID == -1 || rowPID != headerPID {
		t.Fatalf("expected aligned netstat output, got %q", out)
	}
}

func TestPrintNetstatInterfacesAlignsLongNames(t *testing.T) {
	oldParse := parseProcNetDevNetstat
	parseProcNetDevNetstat = func(string) ([]netstatInterface, error) {
		return []netstatInterface{{
			Name:   "veth1234567890abcd",
			MTU:    1500,
			RXOK:   1,
			RXErr:  0,
			RXDrop: 0,
			TXOK:   2,
			TXErr:  0,
			TXDrop: 0,
			Flags:  "up",
		}}, nil
	}
	t.Cleanup(func() { parseProcNetDevNetstat = oldParse })

	out, err := captureNetOutput(t, func() error { return printNetstatInterfaces(false) })
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header and row, got %q", out)
	}
	headerFlg := strings.Index(lines[0], "Flg")
	rowFlg := strings.Index(lines[1], "up")
	if headerFlg == -1 || rowFlg != headerFlg {
		t.Fatalf("expected interface column alignment, got %q", out)
	}
}

func TestParseProcNetRoute(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "route")
	content := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t010012AC\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"eth0\t000012AC\t00000000\t0001\t0\t0\t100\t0000FFFF\t0\t0\t0\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write route: %v", err)
	}
	routes, err := parseProcNetRoute(path)
	if err != nil {
		t.Fatalf("parseProcNetRoute: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes[0].Gateway != "172.18.0.1" || routes[0].Flags != "UG" {
		t.Fatalf("unexpected default route: %+v", routes[0])
	}
	if routes[1].Destination != "172.18.0.0" || routes[1].Genmask != "255.255.0.0" {
		t.Fatalf("unexpected network route: %+v", routes[1])
	}
}

func TestParseProcNetDev(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dev")
	content := "Inter-|   Receive                                                |  Transmit\n" +
		" face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed\n" +
		"  test0: 1000 10 1 2 0 0 0 0 2000 20 3 4 0 0 0 0\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write dev: %v", err)
	}
	ifaces, err := parseProcNetDev(path)
	if err != nil {
		t.Fatalf("parseProcNetDev: %v", err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(ifaces))
	}
	if ifaces[0].Name != "test0" || ifaces[0].RXOK != 10 || ifaces[0].TXDrop != 4 {
		t.Fatalf("unexpected interface stats: %+v", ifaces[0])
	}
}

func TestParseNetstatStatsFiles(t *testing.T) {
	dir := t.TempDir()
	snmp := filepath.Join(dir, "snmp")
	netstat := filepath.Join(dir, "netstat")
	snmp6 := filepath.Join(dir, "snmp6")
	if err := os.WriteFile(snmp, []byte("Tcp: RtoAlgorithm ActiveOpens\nTcp: 1 2\n"), 0o644); err != nil {
		t.Fatalf("write snmp: %v", err)
	}
	if err := os.WriteFile(netstat, []byte("TcpExt: SyncookiesSent SyncookiesRecv\nTcpExt: 3 4\n"), 0o644); err != nil {
		t.Fatalf("write netstat: %v", err)
	}
	if err := os.WriteFile(snmp6, []byte("Ip6InReceives 5\nUdp6InDatagrams 6\n"), 0o644); err != nil {
		t.Fatalf("write snmp6: %v", err)
	}
	sections, err := parseNetstatStatsFiles([]string{snmp, netstat, snmp6})
	if err != nil {
		t.Fatalf("parseNetstatStatsFiles: %v", err)
	}
	values := map[string]map[string]string{}
	for _, section := range sections {
		values[section.Name] = section.Stats
	}
	if values["Tcp"]["ActiveOpens"] != "2" || values["TcpExt"]["SyncookiesRecv"] != "4" || values["Ip6"]["InReceives"] != "5" || values["Udp6"]["InDatagrams"] != "6" {
		t.Fatalf("unexpected stats sections: %+v", sections)
	}
}

func TestNetstatCmdRouteInterfaceAndStatsModes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-r"}, "Kernel IP routing table"},
		{[]string{"-i"}, "Iface"},
		{[]string{"-s"}, ":"},
	} {
		output, err := captureNetOutput(t, func() error {
			return NetstatCmd(tc.args)
		})
		if err != nil {
			t.Fatalf("NetstatCmd %v failed: %v", tc.args, err)
		}
		if !strings.Contains(output, tc.want) {
			t.Fatalf("NetstatCmd %v missing %q in %q", tc.args, tc.want, output)
		}
	}
}

func TestRunNetstatContinuousStopsOnInterrupt(t *testing.T) {
	count := 0
	err := runNetstatContinuous(func() error {
		count++
		if count == 1 {
			p, err := os.FindProcess(os.Getpid())
			if err != nil {
				t.Fatalf("find process: %v", err)
			}
			if err := p.Signal(os.Interrupt); err != nil {
				t.Fatalf("signal interrupt: %v", err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("runNetstatContinuous returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one render before interrupt, got %d", count)
	}
}

func TestNetstatCmdPortFilterDoesNotDoRangeMatch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	missPort := port + 1
	time.Sleep(20 * time.Millisecond)

	output, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-port", strconv.Itoa(missPort)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd failed: %v", err)
	}

	if strings.Contains(output, ":"+strconv.Itoa(port)) {
		t.Fatalf("expected exact port filtering, but matched listener on %d for filter %d: %q", port, missPort, output)
	}
}

func TestNetstatCmdListeningOnly(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	output, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-l", "-port", strconv.Itoa(port)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd failed: %v", err)
	}
	if !strings.Contains(output, "LISTEN") {
		t.Fatalf("expected listening-only output to contain LISTEN, got %q", output)
	}
}

func TestNetstatCmdNumericFlagAccepted(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}
	if _, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-n"})
	}); err != nil {
		t.Fatalf("expected -n to be accepted, got %v", err)
	}
}

func TestNetstatCmdTCPUDPAndUnixFilters(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	tcpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer tcpLn.Close()
	tcpPort := tcpLn.Addr().(*net.TCPAddr).Port

	udpConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer udpConn.Close()
	udpPort := udpConn.LocalAddr().(*net.UDPAddr).Port

	unixPath := filepath.Join(t.TempDir(), "netstat.sock")
	unixLn, err := net.Listen("unix", unixPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer unixLn.Close()

	tcpOut, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-t", "-port", strconv.Itoa(tcpPort)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd -t failed: %v", err)
	}
	if !strings.Contains(tcpOut, "TCP") || strings.Contains(tcpOut, "UDP") || strings.Contains(tcpOut, "UNIX") {
		t.Fatalf("expected TCP-only output, got %q", tcpOut)
	}

	udpOut, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-u", "-port", strconv.Itoa(udpPort)})
	})
	if err != nil {
		t.Fatalf("NetstatCmd -u failed: %v", err)
	}
	if !strings.Contains(udpOut, "UDP") || strings.Contains(udpOut, "TCP") || strings.Contains(udpOut, "UNIX") {
		t.Fatalf("expected UDP-only output, got %q", udpOut)
	}

	unixOut, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-x", "-l"})
	})
	if err != nil {
		t.Fatalf("NetstatCmd -x failed: %v", err)
	}
	if !strings.Contains(unixOut, "UNIX") || !strings.Contains(unixOut, unixPath) {
		t.Fatalf("expected unix socket output for %s, got %q", unixPath, unixOut)
	}
}

func TestNetstatCmdCommonFlagsAccepted(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}
	for _, args := range [][]string{
		{"-a"},
		{"-p"},
		{"-4"},
		{"-6"},
		{"-e"},
		{"-o"},
		{"-W"},
		{"--listening"},
		{"--numeric"},
	} {
		if _, err := captureNetOutput(t, func() error {
			return NetstatCmd(args)
		}); err != nil {
			t.Fatalf("expected %v to be accepted, got %v", args, err)
		}
	}
}

func TestNetstatCmdHelpMergesShortAndLongFlags(t *testing.T) {
	out, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("netstat --help failed: %v", err)
	}
	for _, want := range []string{
		"Usage: gobox netstat",
		"-t, --tcp",
		"-u, --udp",
		"-x, --unix",
		"-l, --listening",
		"-p, --programs",
		"-r, --route",
		"-i, --interfaces",
		"-s, --statistics",
		"-c, --continuous",
		"-n, --numeric",
		"-W, --wide",
		"--sort FIELD",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in help output %q", want, out)
		}
	}
	for _, unwanted := range []string{
		"  -t show TCP sockets",
		"  -tcp show TCP sockets",
		"  -listening",
		"  -programs",
	} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("expected merged help output, found duplicate-style entry %q in %q", unwanted, out)
		}
	}
}

func TestNetstatCmdShortAndLongFlagsMatchForTCPListener(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	shortOut, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-t", "-l", "-p", "-e", "-o", "-n", "-W", "-port", port})
	})
	if err != nil {
		t.Fatalf("NetstatCmd short flags failed: %v", err)
	}
	longOut, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"--tcp", "--listening", "--programs", "--extend", "--timers", "--numeric", "--wide", "-port", port})
	})
	if err != nil {
		t.Fatalf("NetstatCmd long flags failed: %v", err)
	}
	if shortOut != longOut {
		t.Fatalf("expected short/long TCP flag output to match\nshort:\n%s\nlong:\n%s", shortOut, longOut)
	}
}

func TestNetstatCmdLongUDPFlagMatchesShort(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer conn.Close()

	port := strconv.Itoa(conn.LocalAddr().(*net.UDPAddr).Port)
	shortOut, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-u", "-port", port})
	})
	if err != nil {
		t.Fatalf("NetstatCmd -u failed: %v", err)
	}
	longOut, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"--udp", "-port", port})
	})
	if err != nil {
		t.Fatalf("NetstatCmd --udp failed: %v", err)
	}
	if shortOut != longOut {
		t.Fatalf("expected short/long UDP flag output to match\nshort:\n%s\nlong:\n%s", shortOut, longOut)
	}
}

func TestNetstatCmdLongUnixFlagMatchesShort(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	unixPath := filepath.Join(t.TempDir(), "netstat-long.sock")
	ln, err := net.Listen("unix", unixPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer ln.Close()

	shortOut, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-x", "-l"})
	})
	if err != nil {
		t.Fatalf("NetstatCmd -x -l failed: %v", err)
	}
	longOut, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"--unix", "--listening"})
	})
	if err != nil {
		t.Fatalf("NetstatCmd --unix --listening failed: %v", err)
	}
	if !strings.Contains(shortOut, unixPath) || !strings.Contains(longOut, unixPath) {
		t.Fatalf("expected unix socket %s in both outputs\nshort:\n%s\nlong:\n%s", unixPath, shortOut, longOut)
	}
}

func TestNetstatCmdLongModeFlagsMatchShort(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	cases := []struct {
		shortArgs []string
		longArgs  []string
		equal     bool
		wants     []string
	}{
		{shortArgs: []string{"-r"}, longArgs: []string{"--route"}, equal: true},
		{shortArgs: []string{"-i"}, longArgs: []string{"--interfaces"}, wants: []string{"Iface", "Flg"}},
		{shortArgs: []string{"-s"}, longArgs: []string{"--statistics"}, wants: []string{"Ip:", "Tcp:", "Udp:"}},
	}
	for _, tc := range cases {
		shortOut, err := captureNetOutput(t, func() error { return NetstatCmd(tc.shortArgs) })
		if err != nil {
			t.Fatalf("NetstatCmd %v failed: %v", tc.shortArgs, err)
		}
		longOut, err := captureNetOutput(t, func() error { return NetstatCmd(tc.longArgs) })
		if err != nil {
			t.Fatalf("NetstatCmd %v failed: %v", tc.longArgs, err)
		}
		if tc.equal && shortOut != longOut {
			t.Fatalf("expected %v and %v outputs to match\nshort:\n%s\nlong:\n%s", tc.shortArgs, tc.longArgs, shortOut, longOut)
		}
		for _, want := range tc.wants {
			if !strings.Contains(shortOut, want) || !strings.Contains(longOut, want) {
				t.Fatalf("expected %v and %v outputs to both contain %q\nshort:\n%s\nlong:\n%s", tc.shortArgs, tc.longArgs, want, shortOut, longOut)
			}
		}
	}
}

func TestNetstatCmdStatsProtocolFilters(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	allOut, err := captureNetOutput(t, func() error { return NetstatCmd([]string{"-s"}) })
	if err != nil {
		t.Fatalf("NetstatCmd -s failed: %v", err)
	}
	tcpOut, err := captureNetOutput(t, func() error { return NetstatCmd([]string{"-s", "-t"}) })
	if err != nil {
		t.Fatalf("NetstatCmd -s -t failed: %v", err)
	}
	udpOut, err := captureNetOutput(t, func() error { return NetstatCmd([]string{"-s", "-u"}) })
	if err != nil {
		t.Fatalf("NetstatCmd -s -u failed: %v", err)
	}
	if allOut == tcpOut {
		t.Fatalf("expected -s -t to differ from bare -s\n-s:\n%s\n-s -t:\n%s", allOut, tcpOut)
	}
	if !strings.Contains(tcpOut, "Tcp:") || strings.Contains(tcpOut, "Ip:") || strings.Contains(tcpOut, "Udp:") {
		t.Fatalf("expected -s -t to only contain TCP-family sections, got %q", tcpOut)
	}
	if !strings.Contains(udpOut, "Udp:") || strings.Contains(udpOut, "Ip:") || strings.Contains(udpOut, "Tcp:") {
		t.Fatalf("expected -s -u to only contain UDP-family sections, got %q", udpOut)
	}
}

func TestNetstatCmdCompatibilityFlagsPreserveFilteredOutput(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	baseOut, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-t", "-l", "-port", port})
	})
	if err != nil {
		t.Fatalf("NetstatCmd base flags failed: %v", err)
	}
	for _, args := range [][]string{
		{"-t", "-l", "-a", "-port", port},
		{"-t", "-l", "--all", "-port", port},
		{"-t", "-l", "-n", "-port", port},
		{"-t", "-l", "--numeric", "-port", port},
		{"-t", "-l", "-W", "-port", port},
		{"-t", "-l", "--wide", "-port", port},
	} {
		out, err := captureNetOutput(t, func() error { return NetstatCmd(args) })
		if err != nil {
			t.Fatalf("NetstatCmd %v failed: %v", args, err)
		}
		if out != baseOut {
			t.Fatalf("expected compatibility flags %v to preserve filtered output\nbase:\n%s\nactual:\n%s", args, baseOut, out)
		}
	}
}

func TestNetstatCmdIPv4AndIPv6Filters(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ipv4Ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	defer ipv4Ln.Close()

	ipv4Port := strconv.Itoa(ipv4Ln.Addr().(*net.TCPAddr).Port)
	ipv4Out, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-4", "-l", "-port", ipv4Port})
	})
	if err != nil {
		t.Fatalf("NetstatCmd -4 failed: %v", err)
	}
	if !strings.Contains(ipv4Out, ":"+ipv4Port) || strings.Contains(ipv4Out, "TCP6") {
		t.Fatalf("expected IPv4-only output for port %s, got %q", ipv4Port, ipv4Out)
	}

	ipv6Ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	defer ipv6Ln.Close()

	ipv6Port := strconv.Itoa(ipv6Ln.Addr().(*net.TCPAddr).Port)
	ipv6Out, err := captureNetOutput(t, func() error {
		return NetstatCmd([]string{"-6", "-l", "-port", ipv6Port})
	})
	if err != nil {
		t.Fatalf("NetstatCmd -6 failed: %v", err)
	}
	if !strings.Contains(ipv6Out, ":"+ipv6Port) || !strings.Contains(ipv6Out, "TCP6") {
		t.Fatalf("expected IPv6-only output for port %s, got %q", ipv6Port, ipv6Out)
	}
}

func TestNetstatCmdLongContinuousFlagRuns(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("netstat is only supported on Linux")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	go func() {
		time.Sleep(50 * time.Millisecond)
		p, err := os.FindProcess(os.Getpid())
		if err == nil {
			_ = p.Signal(os.Interrupt)
		}
	}()

	done := make(chan struct{})
	var runErr error
	go func() {
		defer close(done)
		_, runErr = captureNetOutput(t, func() error {
			return NetstatCmd([]string{"--continuous", "--tcp", "--listening", "-port", port})
		})
	}()

	select {
	case <-done:
		if runErr != nil {
			t.Fatalf("NetstatCmd --continuous failed: %v", runErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("NetstatCmd --continuous did not stop after interrupt")
	}
}
