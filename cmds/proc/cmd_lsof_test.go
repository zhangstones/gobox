package proc

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLsofCurrentProcess(t *testing.T) {
	out, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{"-p", strconv.Itoa(os.Getpid())})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, strconv.Itoa(os.Getpid())) {
		t.Fatalf("expected current pid in lsof output, got %q", out)
	}
}

func TestLsofCmdHelpUsesGroupedSections(t *testing.T) {
	out, err := captureProcOutput(t, func() error {
		return LsofCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("lsof --help failed: %v", err)
	}
	for _, want := range []string{"Usage: gobox lsof [OPTION]... [FILE]...", "Filters:", "Output:", "-p PID", "-t"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
}

func TestLsofCmdOptionsPidsOnly(t *testing.T) {

	out, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{"-t", "-p", strconv.Itoa(os.Getpid())})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("unexpected lsof -t output %q", out)
	}

}

func TestLsofCmdOptionsFileFilter(t *testing.T) {

	file, err := os.CreateTemp(t.TempDir(), "open")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	out, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{file.Name()})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, file.Name()) {
		t.Fatalf("expected open file in lsof output %q", out)
	}

}

func TestLsofCmdOptionsTcpPortFilter(t *testing.T) {

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	out, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{"-iTCP", "-i:" + port})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "TCP") || !strings.Contains(out, ":"+port) {
		t.Fatalf("expected TCP port in lsof output %q", out)
	}

}

func TestLsofCmdOptionsSeparatedTcpPortFilter(t *testing.T) {

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	out, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{"-i", ":" + port})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "TCP") || !strings.Contains(out, ":"+port) {
		t.Fatalf("expected separated -i :PORT output to include TCP port, got %q", out)
	}

}

func TestLsofCmdOptionsUdpFilter(t *testing.T) {

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	port := strconv.Itoa(conn.LocalAddr().(*net.UDPAddr).Port)
	out, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{"-iUDP", "-i:" + port})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "UDP") || !strings.Contains(out, ":"+port) {
		t.Fatalf("expected UDP port in lsof output %q", out)
	}

}

func TestLsofCmdOptionsCommandPrefixAndNoResolveFlags(t *testing.T) {

	root := setupFakeLsofProc(t, []fakeLsofProcess{
		{pid: "1234", comm: "demo", fdTargets: map[string]string{"3": filepath.Join(t.TempDir(), "demo-file")}},
		{pid: "2345", comm: "other", fdTargets: map[string]string{"3": filepath.Join(t.TempDir(), "other-file")}},
	}, nil)
	_ = root
	out, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{"-c", "dem", "-n", "-P"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "demo") || strings.Contains(out, "other") {
		t.Fatalf("command prefix filter did not isolate demo process: %q", out)
	}

}

func TestLsofCmdOptionsNonexistentPidReturnsNoDataRows(t *testing.T) {

	out, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{"-p", "99999999"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "99999999") {
		t.Fatalf("unexpected nonexistent pid output %q", out)
	}

}

func TestLsofCmdOptionsInvalidPidFlag(t *testing.T) {

	if _, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{"-p", "bad"})
	}); err == nil {
		t.Fatal("expected invalid pid flag error")
	}

}

func TestLsofCmdOptionsClosedPortDoesNotMatch(t *testing.T) {

	setupFakeLsofProc(t, []fakeLsofProcess{
		{pid: "1234", comm: "demo", fdTargets: map[string]string{"3": "socket:[100]"}},
	}, map[string]string{"100": "TCP 0100007F:2222->00000000:0"})
	out, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{"-iTCP", "-i:1"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, ":1->") {
		t.Fatalf("unexpected closed port output %q", out)
	}

}

func TestLsofCmdOptionsInvalidPortTokenDoesNotBecomeFileOperand(t *testing.T) {

	out, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{"-i", ":bad"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, ":bad") {
		t.Fatalf("unexpected invalid port output %q", out)
	}

}

func TestLsofCmdOptionsUnopenedFileDoesNotMatch(t *testing.T) {

	path := filepath.Join(t.TempDir(), "closed")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureProcCmd(t, func() error {
		return LsofCmd([]string{path})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, path) {
		t.Fatalf("unexpected unopened file output %q", out)
	}

}

func TestLsofCmdInjectedProcRootMissingProcRootReturnsError(t *testing.T) {
	oldRoot, oldSockets := lsofProcRoot, collectSocketsLsof
	restore := func(t *testing.T) {
		t.Helper()
		lsofProcRoot, collectSocketsLsof = oldRoot, oldSockets
		t.Cleanup(func() { lsofProcRoot, collectSocketsLsof = oldRoot, oldSockets })
	}

	restore(t)
	lsofProcRoot = filepath.Join(t.TempDir(), "missing-proc")
	if _, err := captureProcCmd(t, func() error { return LsofCmd(nil) }); err == nil {
		t.Fatal("expected missing proc root error")
	}

}

func TestLsofCmdInjectedProcRootFileFilterAndPidsOnlyUseInjectedProcTree(t *testing.T) {
	target := filepath.Join(t.TempDir(), "open-file")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	setupFakeLsofProc(t, []fakeLsofProcess{{pid: "1234", comm: "demo", fdTargets: map[string]string{"3": target, "4": target}}}, map[string]string{})
	out, err := captureProcCmd(t, func() error { return LsofCmd([]string{"-t", target}) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "1234" {
		t.Fatalf("expected de-duplicated pid-only output, got %q", out)
	}

}

func TestLsofLongCommandAndFDAreAligned(t *testing.T) {
	setupFakeLsofProc(t, []fakeLsofProcess{
		{
			pid:  "1234",
			comm: "very-long-command-name",
			fdTargets: map[string]string{
				"123u": filepath.Join(t.TempDir(), "demo-file"),
			},
		},
	}, nil)
	out, err := captureProcCmd(t, func() error { return LsofCmd(nil) })
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header and row, got %q", out)
	}
	headerName := strings.Index(lines[0], "NAME")
	rowName := strings.Index(lines[1], "/tmp/")
	if headerName == -1 || rowName != headerName {
		t.Fatalf("expected lsof columns to align, got %q", out)
	}
}

// TestLsofCmdIncludesUserTypeDeviceSizeNodeColumns is a regression test for
// gobox lsof previously only showing COMMAND/PID/FD/NAME; native lsof also
// shows USER/TYPE/DEVICE/SIZE-OFF/NODE.
func TestLsofCmdIncludesUserTypeDeviceSizeNodeColumns(t *testing.T) {
	target := filepath.Join(t.TempDir(), "open-file")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	setupFakeLsofProc(t, []fakeLsofProcess{
		{pid: "1234", comm: "demo", fdTargets: map[string]string{"3": target}},
	}, nil)
	out, err := captureProcCmd(t, func() error { return LsofCmd(nil) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"USER", "TYPE", "DEVICE", "SIZE/OFF", "NODE"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected header column %q in %q", want, out)
		}
	}
	if !strings.Contains(out, "REG") {
		t.Fatalf("expected REG file type in %q", out)
	}
	if !strings.Contains(out, "5") { // file size in bytes ("hello" = 5 bytes)
		t.Fatalf("expected file size 5 in %q", out)
	}
}

// TestLsofCmdCharDeviceUsesRdevForDevice is a regression test for a bug
// where a character device's DEVICE column used the filesystem device
// (st_dev, e.g. the root filesystem's major:minor) instead of the device's
// own major:minor (st_rdev), which for /dev/null is always "1,3".
func TestLsofCmdCharDeviceUsesRdevForDevice(t *testing.T) {
	if _, err := os.Stat("/dev/null"); err != nil {
		t.Skip("/dev/null not available in this sandbox")
	}
	setupFakeLsofProc(t, []fakeLsofProcess{
		{pid: "1234", comm: "demo", fdTargets: map[string]string{"0": "/dev/null"}},
	}, nil)
	out, err := captureProcCmd(t, func() error { return LsofCmd(nil) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CHR") {
		t.Fatalf("expected CHR type for /dev/null, got %q", out)
	}
	if !strings.Contains(out, "1,3") {
		t.Fatalf("expected device 1,3 (st_rdev) for /dev/null, got %q", out)
	}
}

// TestLsofCmdNetworkFilterExcludesUnresolvedSockets is a regression test for
// a bug where -i included every socket fd (matching only the "socket:"
// readlink prefix), leaking unix domain sockets into the output. Native
// lsof -i only lists sockets it can resolve to a real internet (TCP/UDP)
// address.
func TestLsofCmdNetworkFilterExcludesUnresolvedSockets(t *testing.T) {
	setupFakeLsofProc(t, []fakeLsofProcess{
		{pid: "1234", comm: "demo", fdTargets: map[string]string{
			"5": "socket:[999001]", // unresolved (e.g. unix domain socket)
			"6": "socket:[999002]", // resolved TCP socket
		}},
	}, map[string]string{"999002": "TCP 127.0.0.1:80->0.0.0.0:0"})
	out, err := captureProcCmd(t, func() error { return LsofCmd([]string{"-i"}) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "999001") {
		t.Fatalf("expected unresolved socket excluded from -i output, got %q", out)
	}
	if !strings.Contains(out, "999002") {
		t.Fatalf("expected resolved TCP socket included in -i output, got %q", out)
	}
}

// TestLsofCmdProtoFilterMatchesNodeColumn is a regression test: the proto
// name (TCP/UDP) moved from the NAME column into the new NODE column, so
// the -iTCP/-iUDP proto filter (which previously matched against NAME) had
// to be updated to match NODE instead.
func TestLsofCmdProtoFilterMatchesNodeColumn(t *testing.T) {
	setupFakeLsofProc(t, []fakeLsofProcess{
		{pid: "1234", comm: "demo", fdTargets: map[string]string{
			"5": "socket:[999003]", // TCP
			"6": "socket:[999004]", // UDP
		}},
	}, map[string]string{
		"999003": "TCP 127.0.0.1:80->0.0.0.0:0",
		"999004": "UDP 127.0.0.1:53->0.0.0.0:0",
	})
	out, err := captureProcCmd(t, func() error { return LsofCmd([]string{"-iTCP"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "999003") {
		t.Fatalf("expected TCP socket in -iTCP output, got %q", out)
	}
	if strings.Contains(out, "999004") {
		t.Fatalf("expected UDP socket excluded from -iTCP output, got %q", out)
	}
}

// TestLsofCmdResolvesUnixDomainSocketPath is a regression test for unix
// domain socket fds always falling back to a bare "socket:[inode]"
// placeholder with TYPE "sock". Native lsof resolves them to TYPE "unix"
// with the bound filesystem path as NAME (from /proc/net/unix), which
// collectProcNetSockets now provides via a "UNIX <path>" detail string.
func TestLsofCmdResolvesUnixDomainSocketPath(t *testing.T) {
	setupFakeLsofProc(t, []fakeLsofProcess{
		{pid: "1234", comm: "demo", fdTargets: map[string]string{
			"5": "socket:[999005]",
		}},
	}, map[string]string{"999005": "UNIX /run/demo.sock"})
	out, err := captureProcCmd(t, func() error { return LsofCmd(nil) })
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header and one data row, got %q", out)
	}
	fields := strings.Fields(lines[1])
	// COMMAND PID USER FD TYPE SIZE/OFF NODE NAME (DEVICE is blank for unix
	// sockets, gobox doesn't have the kernel socket address; a blank field
	// doesn't produce its own token under strings.Fields).
	if len(fields) < 8 {
		t.Fatalf("unexpected row shape %v", fields)
	}
	if fields[4] != "unix" {
		t.Fatalf("expected TYPE column \"unix\", got %q in %v", fields[4], fields)
	}
	if fields[6] != "999005" {
		t.Fatalf("expected NODE column to be the socket inode, got %q in %v", fields[6], fields)
	}
	if fields[7] != "/run/demo.sock" {
		t.Fatalf("expected NAME column to be the resolved unix socket path, got %q in %v", fields[7], fields)
	}
}

// TestLsofCmdNetworkFilterExcludesUnixSockets is a regression test for -i
// treating any resolved (non-empty detail) socket as an internet socket.
// Unix domain sockets now resolve to a non-empty "UNIX ..." detail too, so
// -i must keep excluding them specifically, matching native lsof -i.
func TestLsofCmdNetworkFilterExcludesUnixSockets(t *testing.T) {
	setupFakeLsofProc(t, []fakeLsofProcess{
		{pid: "1234", comm: "demo", fdTargets: map[string]string{
			"5": "socket:[999006]", // unix domain socket, now resolved
			"6": "socket:[999007]", // resolved TCP socket
		}},
	}, map[string]string{
		"999006": "UNIX /run/demo.sock",
		"999007": "TCP 127.0.0.1:80->0.0.0.0:0",
	})
	out, err := captureProcCmd(t, func() error { return LsofCmd([]string{"-i"}) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "999006") {
		t.Fatalf("expected unix domain socket excluded from -i output, got %q", out)
	}
	if !strings.Contains(out, "999007") {
		t.Fatalf("expected resolved TCP socket included in -i output, got %q", out)
	}
}

// TestReadProcNetUnix is a unit test for the /proc/net/unix line parser
// backing unix domain socket resolution: it must extract the inode (column
// 7) and the bound path (column 8, only present for filesystem-bound
// sockets) into a "UNIX <path>" detail string, and tolerate paths with
// spaces or a missing path column (anonymous/unbound sockets).
func TestReadProcNetUnix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unix")
	content := "Num       RefCount Protocol Flags    Type St Inode Path\n" +
		"0000000000000000: 00000002 00000000 00010000 0001 01 14173 /run/systemd/userdb/io.systemd.DynamicUser\n" +
		"0000000000000000: 00000003 00000000 00000000 0001 03 22663\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	readProcNetUnix(out, path)
	if got := out["14173"]; got != "UNIX /run/systemd/userdb/io.systemd.DynamicUser" {
		t.Fatalf("unexpected detail for bound socket: %q", got)
	}
	if got := out["22663"]; got != "UNIX" {
		t.Fatalf("unexpected detail for anonymous socket: %q", got)
	}
}

type fakeLsofProcess struct {
	pid       string
	comm      string
	fdTargets map[string]string
}

func setupFakeLsofProc(t *testing.T, procs []fakeLsofProcess, sockets map[string]string) string {
	t.Helper()
	oldRoot, oldSockets := lsofProcRoot, collectSocketsLsof
	root := t.TempDir()
	lsofProcRoot = root
	collectSocketsLsof = func() map[string]string { return sockets }
	t.Cleanup(func() {
		lsofProcRoot, collectSocketsLsof = oldRoot, oldSockets
	})
	for _, proc := range procs {
		pidDir := filepath.Join(root, proc.pid)
		fdDir := filepath.Join(pidDir, "fd")
		if err := os.MkdirAll(fdDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte(proc.comm+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for fd, target := range proc.fdTargets {
			if !strings.HasPrefix(target, "socket:") && !filepath.IsAbs(target) {
				if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(target, filepath.Join(fdDir, fd)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}
