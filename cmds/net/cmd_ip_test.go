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

// TestIpNeighReaderMapsArpFlagsToState is a regression test for a bug where
// ip neigh always printed "REACHABLE" regardless of the actual ARP flags,
// mislabeling permanent/static and incomplete entries.
func TestIpNeighReaderMapsArpFlagsToState(t *testing.T) {
	input := "IP address HW type Flags HW address Mask Device\n" +
		"10.0.0.1 0x1 0x2 aa:bb:cc:dd:ee:01 * eth0\n" + // complete -> REACHABLE
		"10.0.0.2 0x1 0x4 aa:bb:cc:dd:ee:02 * eth0\n" + // permanent -> PERMANENT
		"10.0.0.3 0x1 0x0 aa:bb:cc:dd:ee:03 * eth0\n" // no flags -> INCOMPLETE
	out, err := captureNetOutput(t, func() error { return ipNeighFromReader(strings.NewReader(input)) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"10.0.0.1 dev eth0 lladdr aa:bb:cc:dd:ee:01 REACHABLE",
		"10.0.0.2 dev eth0 lladdr aa:bb:cc:dd:ee:02 PERMANENT",
		"10.0.0.3 dev eth0 lladdr aa:bb:cc:dd:ee:03 INCOMPLETE",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in neigh output %q", want, out)
		}
	}
}

// TestIpRouteReaderIncludesProtoScopeSrcMetric is a regression test: route
// lines previously omitted proto/scope/src/metric entirely. Directly
// connected (no-gateway) routes should show "proto kernel scope link src
// IFACE-ADDR", and the default route should show "proto static" plus its
// metric (parsed from /proc/net/route's Metric column, previously ignored).
func TestIpRouteReaderIncludesProtoScopeSrcMetric(t *testing.T) {
	oldInterfaces, oldAddrs := ipInterfaces, ipInterfaceAddrs
	t.Cleanup(func() { ipInterfaces, ipInterfaceAddrs = oldInterfaces, oldAddrs })
	ipInterfaces = func() ([]stdnet.Interface, error) {
		return []stdnet.Interface{{Name: "eth0", Flags: stdnet.FlagUp | stdnet.FlagRunning}}, nil
	}
	ipInterfaceAddrs = func(stdnet.Interface) ([]stdnet.Addr, error) {
		return []stdnet.Addr{&stdnet.IPNet{IP: stdnet.ParseIP("192.168.1.5"), Mask: stdnet.CIDRMask(24, 32)}}, nil
	}
	// eth0 00000000 0100007F 0003 0 0 100 00000000 -> default via 127.0.0.1 metric 100
	// eth0 0001A8C0 00000000 0001 0 0 0   00FFFFFF -> 192.168.1.0/24 dev eth0 (no gateway)
	input := "Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\n" +
		"eth0 00000000 0100007F 0003 0 0 100 00000000 0 0 0\n" +
		"eth0 0001A8C0 00000000 0001 0 0 0 00FFFFFF 0 0 0\n"
	out, err := captureNetOutput(t, func() error { return ipRouteFromReader(strings.NewReader(input)) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "default via 127.0.0.1 dev eth0 proto static metric 100") {
		t.Fatalf("expected default route with proto static + metric, got %q", out)
	}
	if !strings.Contains(out, "192.168.1.0/24 dev eth0 proto kernel scope link src 192.168.1.5") {
		t.Fatalf("expected connected route with proto/scope/src, got %q", out)
	}
	// The connected route's metric is 0, which native ip route omits entirely.
	if strings.Contains(out, "192.168.1.0/24 dev eth0 proto kernel scope link src 192.168.1.5 metric") {
		t.Fatalf("expected zero metric to be omitted, got %q", out)
	}
}

// TestIpRouteReaderMarksLinkdownWhenNotRunning is a regression test for
// native ip route's "linkdown" flag on a connected route whose interface has
// no carrier.
func TestIpRouteReaderMarksLinkdownWhenNotRunning(t *testing.T) {
	oldInterfaces, oldAddrs := ipInterfaces, ipInterfaceAddrs
	t.Cleanup(func() { ipInterfaces, ipInterfaceAddrs = oldInterfaces, oldAddrs })
	ipInterfaces = func() ([]stdnet.Interface, error) {
		return []stdnet.Interface{{Name: "docker0", Flags: stdnet.FlagUp}}, nil // no FlagRunning
	}
	ipInterfaceAddrs = func(stdnet.Interface) ([]stdnet.Addr, error) {
		return []stdnet.Addr{&stdnet.IPNet{IP: stdnet.ParseIP("172.17.0.1"), Mask: stdnet.CIDRMask(16, 32)}}, nil
	}
	input := "Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\n" +
		"docker0 000011AC 00000000 0001 0 0 0 0000FFFF 0 0 0\n"
	out, err := captureNetOutput(t, func() error { return ipRouteFromReader(strings.NewReader(input)) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "linkdown") {
		t.Fatalf("expected linkdown flag for non-running interface, got %q", out)
	}
}

// TestIpRouteMaskUsesCIDRPrefixLength is a regression test for a bug where
// route destinations printed a dotted-decimal mask (e.g. "/255.255.0.0")
// instead of native ip route's CIDR prefix length ("/16").
func TestIpRouteMaskUsesCIDRPrefixLength(t *testing.T) {
	input := "Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\n" +
		"docker0 000011AC 00000000 0001 0 0 0 0000FFFF 0 0 0\n"
	out, err := captureNetOutput(t, func() error { return ipRouteFromReader(strings.NewReader(input)) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "172.17.0.0/16 ") {
		t.Fatalf("expected CIDR prefix length /16, got %q", out)
	}
	if strings.Contains(out, "255.255") {
		t.Fatalf("did not expect dotted-decimal mask, got %q", out)
	}
}

// TestIpFlagsStringOrderAndNames is a regression test for the interface
// flag list: it previously used Go's raw lowercase net.Flags.String() (e.g.
// "up|broadcast|multicast|running") instead of native ip's uppercase,
// comma-separated, differently-ordered names (e.g.
// "BROADCAST,MULTICAST,UP,LOWER_UP").
func TestIpFlagsStringOrderAndNames(t *testing.T) {
	up := stdnet.FlagUp | stdnet.FlagBroadcast | stdnet.FlagMulticast | stdnet.FlagRunning
	got := ipFlagsString(stdnet.Interface{Flags: up})
	if want := "BROADCAST,MULTICAST,UP,LOWER_UP"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	loopback := stdnet.FlagUp | stdnet.FlagLoopback | stdnet.FlagRunning
	got = ipFlagsString(stdnet.Interface{Flags: loopback})
	if want := "LOOPBACK,UP,LOWER_UP"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	// Administratively up but no carrier (e.g. an unplugged docker bridge)
	// gets NO-CARRIER prepended and omits LOWER_UP.
	noCarrier := stdnet.FlagUp | stdnet.FlagBroadcast | stdnet.FlagMulticast
	got = ipFlagsString(stdnet.Interface{Flags: noCarrier})
	if want := "NO-CARRIER,BROADCAST,MULTICAST,UP"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// TestIpBroadcastAddr verifies the computed IPv4 broadcast address, used to
// add the "brd X" field to inet address lines (previously always omitted).
func TestIpBroadcastAddr(t *testing.T) {
	v4 := &stdnet.IPNet{IP: stdnet.ParseIP("192.168.1.5"), Mask: stdnet.CIDRMask(24, 32)}
	if got, want := ipBroadcastAddr(v4), "192.168.1.255"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	v6 := &stdnet.IPNet{IP: stdnet.ParseIP("2001:db8::1"), Mask: stdnet.CIDRMask(64, 128)}
	if got := ipBroadcastAddr(v6); got != "" {
		t.Fatalf("expected no broadcast for IPv6, got %q", got)
	}
}

// TestIpAddrShowsBrdScopeAndLifetime is a regression test for ip addr's
// default (non -o) output previously omitting "brd", never computing scope
// for non-oneline output, and omitting the valid_lft/preferred_lft line.
func TestIpAddrShowsBrdScopeAndLifetime(t *testing.T) {
	setupInjectedIP(t)
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"addr"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "inet 192.0.2.10/24 brd 192.0.2.255 scope global ut0") {
		t.Fatalf("expected brd + scope + iface suffix on inet line, got %q", out)
	}
	if !strings.Contains(out, "valid_lft forever preferred_lft forever") {
		t.Fatalf("expected valid_lft/preferred_lft line, got %q", out)
	}
	if !strings.Contains(out, "inet6 2001:db8::1/64 scope global") {
		t.Fatalf("expected scope on inet6 line, got %q", out)
	}
	if strings.Contains(out, "inet6 2001:db8::1/64 scope global ut0") {
		t.Fatalf("inet6 lines should not get an interface-name suffix, got %q", out)
	}
}

// TestIpOperStateUsesSysfs is a regression test for ip addr/link's "state"
// field: it previously derived state purely from the administrative FlagUp
// bit, so an interface that's administratively up but has no carrier (e.g.
// a bridge with no attached ports) incorrectly showed "state UP" instead of
// the kernel-reported operational state from /sys/class/net/IFACE/operstate.
func TestIpOperStateUsesSysfs(t *testing.T) {
	oldInterfaces, oldStatsRoot := ipInterfaces, ipStatsRoot
	t.Cleanup(func() { ipInterfaces, ipStatsRoot = oldInterfaces, oldStatsRoot })
	dir := t.TempDir()
	ifDir := filepath.Join(dir, "br0")
	if err := os.MkdirAll(ifDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ifDir, "operstate"), []byte("down\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ipStatsRoot = dir
	ipInterfaces = func() ([]stdnet.Interface, error) {
		return []stdnet.Interface{{Name: "br0", Flags: stdnet.FlagUp}}, nil // administratively up
	}
	out, err := captureNetOutput(t, func() error { return IpCmd([]string{"link"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "state DOWN") {
		t.Fatalf("expected operstate-derived \"state DOWN\" despite FlagUp, got %q", out)
	}
}
