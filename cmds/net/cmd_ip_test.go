package net

import (
	"errors"
	stdnet "net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIpAddrAndLink(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"addr"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ut0") || !strings.Contains(out, "inet 192.0.2.10/24") {
		t.Fatalf("unexpected ip addr output %q", out)
	}
	out, err = captureNetOutput(t, func() error { return IpCmd([]string{"-s", "link"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "RX:") || !strings.Contains(out, "TX:") {
		t.Fatalf("unexpected ip -s link output %q", out)
	}
}

func TestIpDefaultAddr(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd(nil) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ut0", "inet 192.0.2.10/24", "inet6 2001:db8::1/64"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in ip output %q", want, out)
		}
	}
}

func TestIpAddrAlias(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"a"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ut0") {
		t.Fatalf("unexpected ip a output %q", out)
	}
}

func TestIpOneLineAddr(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"-o", "addr"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ut0", "inet 192.0.2.10/24", "inet6 2001:db8::1/64"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in ip output %q", want, out)
		}
	}
}

func TestIpOneLineAlias(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"-o", "a"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ut0") || !strings.Contains(out, "inet6") {
		t.Fatalf("unexpected ip -o a output %q", out)
	}
}

func TestIpLinkAlias(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"l"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ut0") || !strings.Contains(out, "mtu") {
		t.Fatalf("unexpected ip l output %q", out)
	}
}

func TestIpLinkStats(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"-s", "link"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "RX:") || !strings.Contains(out, "TX:") {
		t.Fatalf("unexpected ip -s link output %q", out)
	}
}

func TestIpRoute(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"route"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "default via 127.0.0.1 dev eth0") {
		t.Fatalf("unexpected ip route output %q", out)
	}
}

func TestIpRouteAlias(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"r"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "default via 127.0.0.1 dev eth0") {
		t.Fatalf("unexpected ip r output %q", out)
	}
}

func TestIpNeigh(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"neigh"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "10.0.0.1 dev eth0 lladdr aa:bb:cc:dd:ee:01 REACHABLE") {
		t.Fatalf("unexpected ip neigh output %q", out)
	}
}

func TestIpNeighAlias(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"n"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "10.0.0.1 dev eth0") {
		t.Fatalf("unexpected ip n output %q", out)
	}
}

func TestIpHelp(t *testing.T) {
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"help"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Usage: gobox ip") {
		t.Fatalf("unexpected ip help output %q", out)
	}
}

func TestIpShortHelp(t *testing.T) {
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"-h"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Usage: gobox ip") {
		t.Fatalf("unexpected ip -h output %q", out)
	}
}

func TestIpUnsupportedObject(t *testing.T) {
	if _, err := captureNetOutput(t, func() error { return IpCmd([]string{"set"}) }); err == nil {
		t.Fatal("expected unsupported object error")
	}
}

func TestIpUnsupportedObjectWithOptions(t *testing.T) {
	if _, err := captureNetOutput(t, func() error { return IpCmd([]string{"-s", "set"}) }); err == nil {
		t.Fatal("expected unsupported object error")
	}
}

func TestIpInvalidOptionObjectCombinations(t *testing.T) {
	if _, err := captureNetOutput(t, func() error { return IpCmd([]string{"-o", "link"}) }); err == nil {
		t.Fatal("expected -o link error")
	}
	if _, err := captureNetOutput(t, func() error { return IpCmd([]string{"-s", "route"}) }); err == nil {
		t.Fatal("expected -s route error")
	}
}

func TestIpInternalBoundaries(t *testing.T) {
	if got := parseRouteHex("0100007F"); got != "127.0.0.1" {
		t.Fatalf("unexpected route hex parse %q", got)
	}
	if got := parseRouteHex("bad"); got != "0.0.0.0" {
		t.Fatalf("unexpected invalid route hex parse %q", got)
	}
	stats := readIfaceStats("definitely-missing-gobox-ut-iface")
	for key, val := range stats {
		if val != 0 {
			t.Fatalf("expected missing iface stat %s to default zero, got %d", key, val)
		}
	}
}

func TestIpInterfacesFailureReturnsError(t *testing.T) {
	oldInterfaces, oldAddrs, oldOpenFile, oldStatsRoot := ipInterfaces, ipInterfaceAddrs, ipOpenFile, ipStatsRoot
	ipInterfaces = func() ([]stdnet.Interface, error) { return nil, errors.New("interfaces unavailable") }
	t.Cleanup(func() {
		ipInterfaces, ipInterfaceAddrs, ipOpenFile, ipStatsRoot = oldInterfaces, oldAddrs, oldOpenFile, oldStatsRoot
	})
	if _, err := captureNetOutput(t, func() error { return IpCmd([]string{"addr"}) }); err == nil {
		t.Fatal("expected addr interface error")
	}
	if _, err := captureNetOutput(t, func() error { return IpCmd([]string{"link"}) }); err == nil {
		t.Fatal("expected link interface error")
	}
}

func TestIpStatsRootMissingDefaultsToZero(t *testing.T) {
	oldInterfaces, oldAddrs, oldOpenFile, oldStatsRoot := ipInterfaces, ipInterfaceAddrs, ipOpenFile, ipStatsRoot
	ipStatsRoot = filepath.Join(t.TempDir(), "missing-sys")
	t.Cleanup(func() {
		ipInterfaces, ipInterfaceAddrs, ipOpenFile, ipStatsRoot = oldInterfaces, oldAddrs, oldOpenFile, oldStatsRoot
	})
	stats := readIfaceStats("lo")
	for key, val := range stats {
		if val != 0 {
			t.Fatalf("expected missing stat %s to default to zero, got %d", key, val)
		}
	}
}

func TestIpRouteOpenFailureReturnsError(t *testing.T) {
	oldInterfaces, oldAddrs, oldOpenFile, oldStatsRoot := ipInterfaces, ipInterfaceAddrs, ipOpenFile, ipStatsRoot
	ipOpenFile = func(string) (*os.File, error) { return nil, os.ErrPermission }
	t.Cleanup(func() {
		ipInterfaces, ipInterfaceAddrs, ipOpenFile, ipStatsRoot = oldInterfaces, oldAddrs, oldOpenFile, oldStatsRoot
	})
	if _, err := captureNetOutput(t, func() error { return IpCmd([]string{"route"}) }); err == nil {
		t.Fatal("expected route open error")
	}
}

func TestIpNeighOpenFailureReturnsError(t *testing.T) {
	oldInterfaces, oldAddrs, oldOpenFile, oldStatsRoot := ipInterfaces, ipInterfaceAddrs, ipOpenFile, ipStatsRoot
	ipOpenFile = func(string) (*os.File, error) { return nil, os.ErrPermission }
	t.Cleanup(func() {
		ipInterfaces, ipInterfaceAddrs, ipOpenFile, ipStatsRoot = oldInterfaces, oldAddrs, oldOpenFile, oldStatsRoot
	})
	if _, err := captureNetOutput(t, func() error { return IpCmd([]string{"neigh"}) }); err == nil {
		t.Fatal("expected neigh open error")
	}
}

func setupInjectedIP(t *testing.T) {
	t.Helper()
	oldInterfaces, oldAddrs, oldOpenFile, oldStatsRoot := ipInterfaces, ipInterfaceAddrs, ipOpenFile, ipStatsRoot
	dir := t.TempDir()
	statsDir := filepath.Join(dir, "ut0", "statistics")
	if err := os.MkdirAll(statsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rx_bytes", "rx_packets", "rx_errors", "rx_dropped", "tx_bytes", "tx_packets", "tx_errors", "tx_dropped"} {
		if err := os.WriteFile(filepath.Join(statsDir, name), []byte("0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	routeFile := filepath.Join(dir, "route")
	arpFile := filepath.Join(dir, "arp")
	if err := os.WriteFile(routeFile, []byte("Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\neth0 00000000 0100007F 0003 0 0 0 00000000 0 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(arpFile, []byte("IP address HW type Flags HW address Mask Device\n10.0.0.1 0x1 0x2 aa:bb:cc:dd:ee:01 * eth0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v4 := &stdnet.IPNet{IP: stdnet.ParseIP("192.0.2.10"), Mask: stdnet.CIDRMask(24, 32)}
	v6 := &stdnet.IPNet{IP: stdnet.ParseIP("2001:db8::1"), Mask: stdnet.CIDRMask(64, 128)}
	ipInterfaces = func() ([]stdnet.Interface, error) {
		return []stdnet.Interface{{Index: 7, Name: "ut0", MTU: 1500, Flags: stdnet.FlagUp}}, nil
	}
	ipInterfaceAddrs = func(stdnet.Interface) ([]stdnet.Addr, error) {
		return []stdnet.Addr{v4, v6}, nil
	}
	ipStatsRoot = dir
	ipOpenFile = func(path string) (*os.File, error) {
		if strings.HasSuffix(path, "/route") {
			return os.Open(routeFile)
		}
		if strings.HasSuffix(path, "/arp") {
			return os.Open(arpFile)
		}
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() {
		ipInterfaces, ipInterfaceAddrs, ipOpenFile, ipStatsRoot = oldInterfaces, oldAddrs, oldOpenFile, oldStatsRoot
	})
}

func TestIpRouteReaderParsesDefaultRoute(t *testing.T) {
	out, err := captureNetOutput(t, func() error {
		return ipRouteFromReader(strings.NewReader("Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\neth0 00000000 0100007F 0003 0 0 0 00000000 0 0 0\n"))
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "default via 127.0.0.1 dev eth0") {
		t.Fatalf("unexpected route output %q", out)
	}
}

func TestIpNeighReaderSortsRows(t *testing.T) {
	input := "IP address HW type Flags HW address Mask Device\n10.0.0.2 0x1 0x2 aa:bb:cc:dd:ee:02 * eth0\n10.0.0.1 0x1 0x2 aa:bb:cc:dd:ee:01 * eth0\n"
	out, err := captureNetOutput(t, func() error { return ipNeighFromReader(strings.NewReader(input)) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "10.0.0.1 ") {
		t.Fatalf("expected sorted neigh rows, got %q", out)
	}
}
