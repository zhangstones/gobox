package net

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ifstatGOOS = runtime.GOOS

func IfstatCmd(args []string) error {
	fsFlags := flag.NewFlagSet("ifstat", flag.ContinueOnError)
	interval := fsFlags.Int("p", 1, "sample interval in seconds")
	count := fsFlags.Int("n", 0, "number of samples to take (0 = continuous)")
	iface := fsFlags.String("i", "", "network interface(s), comma-separated (default all physical NICs)")
	absolute := fsFlags.Bool("a", false, "show absolute values (cumulative, no averaging)")
	showAll := fsFlags.Bool("A", false, "show all interfaces including virtual (veth/tun/tap)")
	showErrors := fsFlags.Bool("e", false, "show error packet counts (rx_errors, tx_errors)")
	showDrops := fsFlags.Bool("d", false, "show drop packet counts (rx_dropped, tx_dropped)")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox ifstat [-p sec] [-n count] [-a] [-A] [-e] [-d] [-i iface]")
		fmt.Fprintln(os.Stderr, "Print network interface statistics (packets/s, bytes/s).")
		fmt.Fprintln(os.Stderr, "Options:")
		fsFlags.PrintDefaults()
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if ifstatGOOS != "linux" {
		return errors.New("ifstat: supported only on Linux")
	}

	// Parse comma-separated interfaces
	var wantedIfaces map[string]bool
	if *iface != "" {
		wantedIfaces = make(map[string]bool)
		for _, i := range strings.Split(*iface, ",") {
			wantedIfaces[strings.TrimSpace(i)] = true
		}
	}

	// Read network interface type from /sys/class/net/<iface>/type
	isPhysical := func(iface string) bool {
		data, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/type", iface))
		if err != nil {
			return false
		}
		t, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return false
		}
		// ARPHRD_ETHER = 1 means physical NIC
		return t == 1
	}

	// Get list of all network interfaces
	listIfaces := func() ([]string, error) {
		entries, err := os.ReadDir("/sys/class/net")
		if err != nil {
			return nil, err
		}
		var ifaces []string
		for _, e := range entries {
			name := e.Name()
			// Filter by wanted interfaces if specified
			if wantedIfaces != nil && !wantedIfaces[name] {
				continue
			}
			// Filter virtual interfaces unless -A is specified
			if !*showAll && !isPhysical(name) {
				continue
			}
			ifaces = append(ifaces, name)
		}
		sort.Strings(ifaces)
		return ifaces, nil
	}

	// Read stats for a single interface
	type IfStats struct {
		RxPackets uint64
		TxPackets uint64
		RxBytes   uint64
		TxBytes   uint64
		RxErrors  uint64
		TxErrors  uint64
		RxDropped uint64
		TxDropped uint64
	}

	readStats := func(iface string) (IfStats, error) {
		var s IfStats
		readU64 := func(path string) (uint64, error) {
			data, err := os.ReadFile(path)
			if err != nil {
				return 0, err
			}
			return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		}
		base := fmt.Sprintf("/sys/class/net/%s/statistics", iface)

		if v, err := readU64(base + "/rx_packets"); err == nil {
			s.RxPackets = v
		}
		if v, err := readU64(base + "/tx_packets"); err == nil {
			s.TxPackets = v
		}
		if v, err := readU64(base + "/rx_bytes"); err == nil {
			s.RxBytes = v
		}
		if v, err := readU64(base + "/tx_bytes"); err == nil {
			s.TxBytes = v
		}
		if v, err := readU64(base + "/rx_errors"); err == nil {
			s.RxErrors = v
		}
		if v, err := readU64(base + "/tx_errors"); err == nil {
			s.TxErrors = v
		}
		if v, err := readU64(base + "/rx_dropped"); err == nil {
			s.RxDropped = v
		}
		if v, err := readU64(base + "/tx_dropped"); err == nil {
			s.TxDropped = v
		}
		return s, nil
	}

	// Continuous mode: count == 0 means run until Ctrl+C
	continuous := *count == 0

	// Get initial interfaces list
	ifaces, err := listIfaces()
	if err != nil {
		return fmt.Errorf("failed to list interfaces: %w", err)
	}

	// If specific interfaces requested but not found, warn (before checking if list is empty)
	if wantedIfaces != nil {
		for wanted := range wantedIfaces {
			found := false
			for _, i := range ifaces {
				if i == wanted {
					found = true
					break
				}
			}
			if !found {
				fmt.Fprintf(os.Stderr, "ifstat: warning: interface %s not found or not a physical NIC\n", wanted)
			}
		}
	}

	if len(ifaces) == 0 {
		if wantedIfaces != nil {
			// User specified interfaces but none were found - warn was already printed above
			// Print header (to stdout) so the command produces some output
			fmt.Printf("%-12s  %9s  %9s  %9s  %9s\n",
				"Interface", "rxpps/s", "txpps/s", "rxKB/s", "txKB/s")
			return nil
		}
		return errors.New("ifstat: no network interfaces found")
	}

	// Prepare output
	showErrorsCol := *showErrors
	showDropsCol := *showDrops

	// Header
	printHeader := func() {
		if showErrorsCol && showDropsCol {
			fmt.Printf("%-12s  %9s  %9s  %9s  %9s  %7s  %7s  %7s  %7s\n",
				"Interface", "rxpps/s", "txpps/s", "rxKB/s", "txKB/s", "rxerrs", "txerrs", "rxdrop", "txdrop")
		} else if showErrorsCol {
			fmt.Printf("%-12s  %9s  %9s  %9s  %9s  %7s  %7s\n",
				"Interface", "rxpps/s", "txpps/s", "rxKB/s", "txKB/s", "rxerrs", "txerrs")
		} else if showDropsCol {
			fmt.Printf("%-12s  %9s  %9s  %9s  %9s  %7s  %7s\n",
				"Interface", "rxpps/s", "txpps/s", "rxKB/s", "txKB/s", "rxdrop", "txdrop")
		} else {
			fmt.Printf("%-12s  %9s  %9s  %9s  %9s\n",
				"Interface", "rxpps/s", "txpps/s", "rxKB/s", "txKB/s")
		}
	}

	// Collect initial stats for all interfaces
	prevStats := make(map[string]IfStats)
	for _, ifaceName := range ifaces {
		prevStats[ifaceName], _ = readStats(ifaceName)
	}

	iter := 0
	for continuous || iter < *count {
		iter++
		if *interval <= 0 {
			*interval = 1
		}
		if iter > 1 {
			// Sleep between samples; emit the first sample immediately so finite runs don't stall.
			time.Sleep(time.Duration(*interval) * time.Second)
		}

		// Re-list interfaces in case they changed
		currentIfaces, err := listIfaces()
		if err != nil {
			return fmt.Errorf("failed to list interfaces: %w", err)
		}

		// Ensure prevStats has entry for all current ifaces
		for _, ifaceName := range currentIfaces {
			if _, ok := prevStats[ifaceName]; !ok {
				prevStats[ifaceName], _ = readStats(ifaceName)
			}
		}

		// Read current stats
		currStats := make(map[string]IfStats)
		for _, ifaceName := range currentIfaces {
			currStats[ifaceName], _ = readStats(ifaceName)
		}

		// Print header once at start
		if iter == 1 {
			printHeader()
		}

		// Calculate and print rates
		var dur float64 = 1.0
		if *interval > 0 {
			dur = float64(*interval)
		}

		for _, ifaceName := range currentIfaces {
			prev := prevStats[ifaceName]
			curr := currStats[ifaceName]

			var rxPps, txPps, rxKBps, txKBps float64

			if *absolute {
				// Absolute mode: show cumulative values
				rxPps = float64(curr.RxPackets)
				txPps = float64(curr.TxPackets)
				rxKBps = float64(curr.RxBytes) / 1024.0
				txKBps = float64(curr.TxBytes) / 1024.0
			} else {
				// Rate mode: calculate per-second values
				if curr.RxPackets >= prev.RxPackets {
					rxPps = float64(curr.RxPackets-prev.RxPackets) / dur
				}
				if curr.TxPackets >= prev.TxPackets {
					txPps = float64(curr.TxPackets-prev.TxPackets) / dur
				}
				if curr.RxBytes >= prev.RxBytes {
					rxKBps = float64(curr.RxBytes-prev.RxBytes) / dur / 1024.0
				}
				if curr.TxBytes >= prev.TxBytes {
					txKBps = float64(curr.TxBytes-prev.TxBytes) / dur / 1024.0
				}
			}

			if showErrorsCol && showDropsCol {
				fmt.Printf("%-12s  %9.2f  %9.2f  %9.2f  %9.2f  %7d  %7d  %7d  %7d\n",
					ifaceName, rxPps, txPps, rxKBps, txKBps,
					curr.RxErrors, curr.TxErrors, curr.RxDropped, curr.TxDropped)
			} else if showErrorsCol {
				fmt.Printf("%-12s  %9.2f  %9.2f  %9.2f  %9.2f  %7d  %7d\n",
					ifaceName, rxPps, txPps, rxKBps, txKBps,
					curr.RxErrors, curr.TxErrors)
			} else if showDropsCol {
				fmt.Printf("%-12s  %9.2f  %9.2f  %9.2f  %9.2f  %7d  %7d\n",
					ifaceName, rxPps, txPps, rxKBps, txKBps,
					curr.RxDropped, curr.TxDropped)
			} else {
				fmt.Printf("%-12s  %9.2f  %9.2f  %9.2f  %9.2f\n",
					ifaceName, rxPps, txPps, rxKBps, txKBps)
			}
		}

		// Store current as previous for next iteration
		prevStats = currStats

		// For absolute mode with count > 1, we show cumulative values each time
		// so we need to re-read stats for the next iteration
		if *absolute {
			for _, ifaceName := range currentIfaces {
				prevStats[ifaceName], _ = readStats(ifaceName)
			}
		}

		// Exit if not continuous and we've shown enough samples
		if !continuous && iter >= *count {
			break
		}

		// For continuous mode, we just keep going until Ctrl+C
	}

	return nil
}
