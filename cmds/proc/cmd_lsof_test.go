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
