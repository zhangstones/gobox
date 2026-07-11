package net

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var (
	ipInterfaces     = net.Interfaces
	ipInterfaceAddrs = func(iface net.Interface) ([]net.Addr, error) { return iface.Addrs() }
	ipOpenFile       = os.Open
	ipStatsRoot      = "/sys/class/net"
)

func IpCmd(args []string) error {
	oneLine := false
	stats := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "-o":
			oneLine = true
		case "-s":
			stats = true
		default:
			filtered = append(filtered, arg)
		}
	}
	if len(filtered) == 0 {
		filtered = []string{"addr"}
	}
	object := filtered[0]
	if oneLine && object != "addr" && object != "a" {
		return fmt.Errorf("-o is supported only with addr")
	}
	if stats && object != "link" && object != "l" {
		return fmt.Errorf("-s is supported only with link")
	}
	switch object {
	case "addr", "a":
		return ipAddr(oneLine)
	case "link", "l":
		return ipLink(stats)
	case "route", "r":
		return ipRoute()
	case "neigh", "n":
		return ipNeigh()
	case "-h", "--help", "help":
		printIpUsage()
		return nil
	default:
		return fmt.Errorf("unsupported ip object %s", object)
	}
}

func printIpUsage() {
	fmt.Fprintln(os.Stdout, "Usage: gobox ip [-o] addr | [-s] link | route | neigh")
	fmt.Fprintln(os.Stdout, "Show network interfaces, routes, and neighbours (read-only subset).")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Objects:")
	fmt.Fprintln(os.Stdout, "  addr, a       show interface addresses")
	fmt.Fprintln(os.Stdout, "  link, l       show interface link state")
	fmt.Fprintln(os.Stdout, "  route, r      show the routing table")
	fmt.Fprintln(os.Stdout, "  neigh, n      show the ARP/neighbour table")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Options:")
	fmt.Fprintln(os.Stdout, "  -o            single-line output (addr only)")
	fmt.Fprintln(os.Stdout, "  -s            show extra statistics (link only)")
	fmt.Fprintln(os.Stdout, "  -h, --help    show this help")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Examples:")
	fmt.Fprintln(os.Stdout, "  gobox ip addr")
	fmt.Fprintln(os.Stdout, "  gobox ip -o addr")
	fmt.Fprintln(os.Stdout, "  gobox ip -s link")
	fmt.Fprintln(os.Stdout, "  gobox ip route")
}

func ipAddr(oneLine bool) error {
	ifaces, err := ipInterfaces()
	if err != nil {
		return err
	}
	for _, iface := range ifaces {
		addrs, _ := ipInterfaceAddrs(iface)
		if !oneLine {
			fmt.Printf("%d: %s: <%s> mtu %d state %s\n", iface.Index, iface.Name, ipFlagsString(iface), iface.MTU, ipOperState(iface))
			printIpLinkLine(iface)
		}
		for _, addr := range addrs {
			fam := "inet"
			if strings.Contains(addr.String(), ":") {
				fam = "inet6"
			}
			scope := ipAddrScope(iface.Name, addr)
			// Native ip shows the owning interface name after "scope" only
			// for inet (IPv4) address lines, never for inet6.
			ifaceSuffix := ""
			if fam == "inet" {
				ifaceSuffix = " " + iface.Name
			}
			addrText := addr.String()
			// Native ip omits "brd" for host-scope (loopback) addresses;
			// broadcasting to 127.255.255.255 is meaningless.
			if scope != "host" {
				if brd := ipBroadcastAddr(addr); brd != "" {
					addrText += " brd " + brd
				}
			}
			if oneLine {
				// -o packs the whole record onto a single physical line,
				// using a literal "\" continuation marker (not an actual
				// newline) before the valid_lft/preferred_lft fields.
				fmt.Printf("%d: %s    %s %s scope %s%s\\       valid_lft forever preferred_lft forever\n", iface.Index, iface.Name, fam, addrText, scope, ifaceSuffix)
				continue
			}
			fmt.Printf("    %s %s scope %s%s\n", fam, addrText, scope, ifaceSuffix)
			fmt.Println("       valid_lft forever preferred_lft forever")
		}
	}
	return nil
}

func ipLink(stats bool) error {
	ifaces, err := ipInterfaces()
	if err != nil {
		return err
	}
	for _, iface := range ifaces {
		fmt.Printf("%d: %s: <%s> mtu %d state %s\n", iface.Index, iface.Name, ipFlagsString(iface), iface.MTU, ipOperState(iface))
		printIpLinkLine(iface)
		if stats {
			s := readIfaceStats(iface.Name)
			fmt.Printf("    RX: %10s %8s %6s %7s %7s %7s\n", "bytes", "packets", "errors", "dropped", "missed", "mcast")
			fmt.Printf("    %10d %8d %6d %7d %7d %7d\n", s["rx_bytes"], s["rx_packets"], s["rx_errors"], s["rx_dropped"], s["rx_missed_errors"], s["multicast"])
			fmt.Printf("    TX: %10s %8s %6s %7s %7s %7s\n", "bytes", "packets", "errors", "dropped", "carrier", "collsns")
			fmt.Printf("    %10d %8d %6d %7d %7d %7d\n", s["tx_bytes"], s["tx_packets"], s["tx_errors"], s["tx_dropped"], s["tx_carrier_errors"], s["collisions"])
		}
	}
	return nil
}

// zeroHardwareAddrOr returns addr's string form, or fallback if addr is empty
// (loopback interfaces typically report an empty/all-zero hardware address).
func zeroHardwareAddrOr(addr net.HardwareAddr, fallback string) string {
	if len(addr) == 0 {
		return fallback
	}
	return addr.String()
}

// printIpLinkLine prints the "link/ether MAC brd BROADCAST" (or
// "link/loopback ...") line ip addr/link both show beneath the interface
// summary line.
func printIpLinkLine(iface net.Interface) {
	if iface.Flags&net.FlagLoopback != 0 {
		mac := zeroHardwareAddrOr(iface.HardwareAddr, "00:00:00:00:00:00")
		fmt.Printf("    link/loopback %s brd 00:00:00:00:00:00\n", mac)
		return
	}
	fmt.Printf("    link/ether %s brd ff:ff:ff:ff:ff:ff\n", iface.HardwareAddr.String())
}

// ipOperState reads the interface's kernel-reported operational state from
// /sys/class/net/IFACE/operstate (e.g. "up"/"down"/"unknown", loopback is
// conventionally "unknown"), matching native ip's "state X" field. This is
// the true link-carrier state, distinct from the administrative FlagUp bit:
// e.g. a bridge interface can be administratively up but operationally down
// when it has no carrier. Falls back to a FlagUp-based guess if the sysfs
// file can't be read (e.g. non-Linux, or an injected test interface).
func ipOperState(iface net.Interface) string {
	data, err := os.ReadFile(filepath.Join(ipStatsRoot, iface.Name, "operstate"))
	if err != nil {
		if iface.Flags&net.FlagUp != 0 {
			return "UP"
		}
		return "DOWN"
	}
	return strings.ToUpper(strings.TrimSpace(string(data)))
}

// ipFlagsString renders the interface flag list the way iproute2 does, e.g.
// "BROADCAST,MULTICAST,UP,LOWER_UP", translating Go's lowercase net.Flags
// bits into the uppercase names/order ip addr|link use. Go's net.Flags only
// exposes a subset of kernel IFF_* bits (no netlink), so flags iproute2
// derives from other sources (qdisc, group, NO-CARRIER via a separate
// carrier check) beyond this subset are intentionally not reproduced here.
func ipFlagsString(iface net.Interface) string {
	var parts []string
	if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagRunning == 0 {
		parts = append(parts, "NO-CARRIER")
	}
	if iface.Flags&net.FlagBroadcast != 0 {
		parts = append(parts, "BROADCAST")
	}
	if iface.Flags&net.FlagLoopback != 0 {
		parts = append(parts, "LOOPBACK")
	}
	if iface.Flags&net.FlagPointToPoint != 0 {
		parts = append(parts, "POINTOPOINT")
	}
	if iface.Flags&net.FlagMulticast != 0 {
		parts = append(parts, "MULTICAST")
	}
	if iface.Flags&net.FlagUp != 0 {
		parts = append(parts, "UP")
	}
	if iface.Flags&net.FlagRunning != 0 {
		parts = append(parts, "LOWER_UP")
	}
	return strings.Join(parts, ",")
}

// ipBroadcastAddr computes the IPv4 broadcast address for addr (IP OR'd with
// the inverted subnet mask), matching the "brd X.X.X.X" native ip shows on
// inet address lines. Returns "" for IPv6 (no broadcast concept) or when the
// mask isn't a 4-byte IPv4 mask.
func ipBroadcastAddr(addr net.Addr) string {
	ipnet, ok := addr.(*net.IPNet)
	if !ok {
		return ""
	}
	ip4 := ipnet.IP.To4()
	if ip4 == nil {
		return ""
	}
	mask := ipnet.Mask
	if len(mask) != net.IPv4len {
		return ""
	}
	bcast := make(net.IP, net.IPv4len)
	for i := 0; i < net.IPv4len; i++ {
		bcast[i] = ip4[i] | ^mask[i]
	}
	return bcast.String()
}

func readIfaceStats(iface string) map[string]uint64 {
	keys := []string{
		"rx_bytes", "rx_packets", "rx_errors", "rx_dropped", "rx_missed_errors", "multicast",
		"tx_bytes", "tx_packets", "tx_errors", "tx_dropped", "tx_carrier_errors", "collisions",
	}
	out := map[string]uint64{}
	for _, key := range keys {
		data, err := os.ReadFile(filepath.Join(ipStatsRoot, iface, "statistics", key))
		if err == nil {
			out[key], _ = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		}
	}
	return out
}

func ipRoute() error {
	f, err := ipOpenFile("/proc/net/route")
	if err != nil {
		return err
	}
	defer f.Close()
	return ipRouteFromReader(f)
}

func ipRouteFromReader(r io.Reader) error {
	scanner := bufio.NewScanner(r)
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
		iface := fields[0]
		dest := parseRouteHex(fields[1])
		gw := parseRouteHex(fields[2])
		mask := routeMaskPrefixLen(fields[7])
		metric := ""
		if len(fields) > 6 {
			if m, err := strconv.ParseUint(fields[6], 10, 64); err == nil && m != 0 {
				metric = fmt.Sprintf(" metric %d", m)
			}
		}
		if fields[1] == "00000000" {
			// A route with a real gateway is (heuristically, since
			// /proc/net/route carries no explicit "proto" field) always a
			// manually/DHCP-configured route, matching native ip route's
			// "proto static" for the default gateway.
			fmt.Printf("default via %s dev %s proto static%s\n", gw, iface, metric)
			continue
		}
		// A route with no gateway is a directly-connected subnet route,
		// which native ip route reports as "proto kernel scope link" with
		// the interface's own address as "src".
		line := fmt.Sprintf("%s/%s dev %s proto kernel scope link", dest, mask, iface)
		if src := ipRouteSrcFor(iface); src != "" {
			line += " src " + src
		}
		line += metric
		if !ipInterfaceIsRunning(iface) {
			line += " linkdown"
		}
		fmt.Println(line)
	}
	return scanner.Err()
}

// ipRouteSrcFor returns the interface's own IPv4 address, used as the "src"
// field on a directly-connected route line.
func ipRouteSrcFor(name string) string {
	ifaces, err := ipInterfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Name != name {
			continue
		}
		addrs, _ := ipInterfaceAddrs(iface)
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if ok && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// ipInterfaceIsRunning reports whether the named interface has carrier
// (kernel IFF_RUNNING), used for the "linkdown" route flag.
func ipInterfaceIsRunning(name string) bool {
	ifaces, err := ipInterfaces()
	if err != nil {
		return true
	}
	for _, iface := range ifaces {
		if iface.Name == name {
			return iface.Flags&net.FlagRunning != 0
		}
	}
	return true
}

func parseRouteHex(s string) string {
	if len(s) != 8 {
		return "0.0.0.0"
	}
	var b [4]byte
	for i := 0; i < 4; i++ {
		v, _ := strconv.ParseUint(s[i*2:i*2+2], 16, 8)
		b[3-i] = byte(v)
	}
	return net.IPv4(b[0], b[1], b[2], b[3]).String()
}

// routeMaskPrefixLen converts /proc/net/route's little-endian hex netmask
// into a CIDR prefix length string (e.g. "16"), matching native ip route's
// "DEST/PREFIXLEN" notation instead of a dotted-decimal mask.
func routeMaskPrefixLen(s string) string {
	if len(s) != 8 {
		return "32"
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return "32"
	}
	ones := 0
	for b := uint32(0); b < 32; b++ {
		if v&(1<<b) != 0 {
			ones++
		}
	}
	return strconv.Itoa(ones)
}

func ipNeigh() error {
	f, err := ipOpenFile("/proc/net/arp")
	if err != nil {
		return err
	}
	defer f.Close()
	return ipNeighFromReader(f)
}

func ipNeighFromReader(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	first := true
	var lines []string
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 6 {
			lines = append(lines, fmt.Sprintf("%s dev %s lladdr %s %s", fields[0], fields[5], fields[3], arpFlagsToState(fields[2])))
		}
	}
	sort.Strings(lines)
	for _, line := range lines {
		fmt.Println(line)
	}
	return scanner.Err()
}

// arpFlagsToState maps /proc/net/arp's legacy HW-flags column to the
// closest native ip neigh state name. The kernel's real neighbour-cache
// states (STALE/DELAY/PROBE/...) are only exposed via rtnetlink, not
// /proc/net/arp, so entries that are actually STALE (the most common
// non-REACHABLE state in practice) can't be distinguished here; this at
// least stops permanent/static and incomplete entries from being
// mislabeled as REACHABLE.
func arpFlagsToState(hexFlags string) string {
	v, err := strconv.ParseUint(strings.TrimPrefix(hexFlags, "0x"), 16, 32)
	if err != nil {
		return "REACHABLE"
	}
	const (
		atfComplete = 0x2
		atfPerm     = 0x4
	)
	switch {
	case v&atfPerm != 0:
		return "PERMANENT"
	case v&atfComplete != 0:
		return "REACHABLE"
	default:
		return "INCOMPLETE"
	}
}

// ipAddrScope returns the scope name to print for an address on the given
// interface. Loopback addresses (127.0.0.0/8, ::1) get scope host; link-local
// addresses (169.254.0.0/16, fe80::/10) get scope link; everything else is
// scope global.
func ipAddrScope(ifName string, addr net.Addr) string {
	s := addr.String()
	if strings.HasPrefix(s, "127.") || s == "::1" || strings.HasPrefix(s, "::1/") {
		return "host"
	}
	if strings.HasPrefix(s, "169.254.") || strings.HasPrefix(s, "fe80:") || strings.HasPrefix(s, "fe80::") {
		return "link"
	}
	return "global"
}
