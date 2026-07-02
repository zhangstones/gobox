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
		fmt.Fprintln(os.Stdout, "Usage: gobox ip [-o] addr | [-s] link | route | neigh")
		return nil
	default:
		return fmt.Errorf("unsupported ip object %s", object)
	}
}

func ipAddr(oneLine bool) error {
	ifaces, err := ipInterfaces()
	if err != nil {
		return err
	}
	for _, iface := range ifaces {
		addrs, _ := ipInterfaceAddrs(iface)
		state := "DOWN"
		if iface.Flags&net.FlagUp != 0 {
			state = "UP"
		}
		if oneLine {
			for _, addr := range addrs {
				fam := "inet"
				if strings.Contains(addr.String(), ":") {
					fam = "inet6"
				}
				fmt.Printf("%d: %s    %s %s scope global %s\n", iface.Index, iface.Name, fam, addr.String(), iface.Name)
			}
			continue
		}
		fmt.Printf("%d: %s: <%s> mtu %d state %s\n", iface.Index, iface.Name, iface.Flags.String(), iface.MTU, state)
		for _, addr := range addrs {
			fam := "inet"
			if strings.Contains(addr.String(), ":") {
				fam = "inet6"
			}
			fmt.Printf("    %s %s\n", fam, addr.String())
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
		state := "DOWN"
		if iface.Flags&net.FlagUp != 0 {
			state = "UP"
		}
		fmt.Printf("%d: %s: <%s> mtu %d state %s\n", iface.Index, iface.Name, iface.Flags.String(), iface.MTU, state)
		fmt.Printf("    link/ether %s\n", iface.HardwareAddr.String())
		if stats {
			s := readIfaceStats(iface.Name)
			fmt.Printf("    RX: bytes %d packets %d errors %d dropped %d\n", s["rx_bytes"], s["rx_packets"], s["rx_errors"], s["rx_dropped"])
			fmt.Printf("    TX: bytes %d packets %d errors %d dropped %d\n", s["tx_bytes"], s["tx_packets"], s["tx_errors"], s["tx_dropped"])
		}
	}
	return nil
}

func readIfaceStats(iface string) map[string]uint64 {
	keys := []string{"rx_bytes", "rx_packets", "rx_errors", "rx_dropped", "tx_bytes", "tx_packets", "tx_errors", "tx_dropped"}
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
		mask := parseRouteHex(fields[7])
		if fields[1] == "00000000" {
			fmt.Printf("default via %s dev %s\n", gw, iface)
		} else {
			fmt.Printf("%s/%s dev %s\n", dest, mask, iface)
		}
	}
	return scanner.Err()
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
			lines = append(lines, fmt.Sprintf("%s dev %s lladdr %s REACHABLE", fields[0], fields[5], fields[3]))
		}
	}
	sort.Strings(lines)
	for _, line := range lines {
		fmt.Println(line)
	}
	return scanner.Err()
}
