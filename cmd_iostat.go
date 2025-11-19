package main

import (
	"bufio"
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

func iostatCmd(args []string) error {
	fsFlags := flag.NewFlagSet("iostat", flag.ContinueOnError)
	interval := fsFlags.Int("i", 1, "sample interval in seconds")
	count := fsFlags.Int("n", 1, "number of samples to take")
	human := fsFlags.Bool("H", true, "humanize IOPS and throughput (e.g. 1.2K, 3.4M)")
	showNonZero := fsFlags.Bool("z", false, "show only devices with non-zero I/O rates")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox iostat [-i sec] [-n count] [-H] [-z]")
		fmt.Fprintln(os.Stderr, "Print block device IOPS and throughput based on cgroup blkio (io.stat or blkio.* files).")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if runtime.GOOS != "linux" {
		return errors.New("iostat: supported only on Linux")
	}

	// helper types
	type DevStats struct {
		RBytes uint64
		WBytes uint64
		RIOs   uint64
		WIOs   uint64
	}

	// resolve major:minor (e.g. "8:0") to device name via /sys/dev/block/<maj>:<min>/uevent
	devNameFromID := func(id string) string {
		// if already contains letters (e.g. "sda"), return as-is
		if strings.IndexFunc(id, func(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }) >= 0 {
			return id
		}
		if !strings.Contains(id, ":") {
			return id
		}
		ueventPath := "/sys/dev/block/" + id + "/uevent"
		if data, err := os.ReadFile(ueventPath); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "DEVNAME=") {
					return strings.TrimPrefix(line, "DEVNAME=")
				}
			}
		}
		// fallback: try to read symlink name under /sys/dev/block/<id>
		if fi, err := os.ReadDir("/sys/dev/block/" + id); err == nil {
			for _, e := range fi {
				// look for a directory starting with letters (block device name)
				if e.IsDir() {
					name := e.Name()
					if len(name) > 0 && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) {
						return name
					}
				}
			}
		}
		return id
	}

	// read cgroup v2 io.stat if available
	readCgroupV2 := func(path string) (map[string]DevStats, error) {
		out := make(map[string]DevStats)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			dev := fields[0]
			var s DevStats
			for _, tok := range fields[1:] {
				kv := strings.SplitN(tok, "=", 2)
				if len(kv) != 2 {
					continue
				}
				v, err := strconv.ParseUint(kv[1], 10, 64)
				if err != nil {
					continue
				}
				switch kv[0] {
				case "rbytes":
					s.RBytes = v
				case "wbytes":
					s.WBytes = v
				case "rios":
					s.RIOs = v
				case "wios":
					s.WIOs = v
				}
			}
			out[dev] = s
		}
		return out, nil
	}

	// read cgroup v1 blkio files (bytes and serviced)
	readCgroupV1 := func(pathBytes, pathServiced string) (map[string]DevStats, error) {
		out := make(map[string]DevStats)
		// parse bytes file
		if bdata, err := os.ReadFile(pathBytes); err == nil {
			sc := bufio.NewScanner(strings.NewReader(string(bdata)))
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" {
					continue
				}
				fields := strings.Fields(line)
				dev := fields[0]
				var s DevStats
				// attempt multiple formats
				if len(fields) >= 3 {
					if fields[1] == "Read" {
						if v, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
							s.RBytes = v
						}
						// check for Write token later
						for i := 3; i < len(fields)-1; i++ {
							if fields[i] == "Write" {
								if v, err := strconv.ParseUint(fields[i+1], 10, 64); err == nil {
									s.WBytes = v
								}
							}
						}
					} else {
						// try numeric pairs: <maj:min> <rbytes> <wbytes>
						if v, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
							s.RBytes = v
						}
						if len(fields) >= 3 {
							if v, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
								s.WBytes = v
							}
						}
					}
				}
				out[dev] = s
			}
		}
		// parse serviced file for IO counts
		if sdata, err := os.ReadFile(pathServiced); err == nil {
			sc := bufio.NewScanner(strings.NewReader(string(sdata)))
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" {
					continue
				}
				fields := strings.Fields(line)
				dev := fields[0]
				s := out[dev]
				if len(fields) >= 3 {
					if fields[1] == "Read" {
						if v, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
							s.RIOs = v
						}
						for i := 3; i < len(fields)-1; i++ {
							if fields[i] == "Write" {
								if v, err := strconv.ParseUint(fields[i+1], 10, 64); err == nil {
									s.WIOs = v
								}
							}
						}
					} else {
						if v, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
							s.RIOs = v
						}
						if len(fields) >= 3 {
							if v, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
								s.WIOs = v
							}
						}
					}
				}
				out[dev] = s
			}
		}
		return out, nil
	}

	// pick available source
	var reader func() (map[string]DevStats, error)
	// try cgroup v2 root io.stat
	if _, err := os.Stat("/sys/fs/cgroup/io.stat"); err == nil {
		reader = func() (map[string]DevStats, error) { return readCgroupV2("/sys/fs/cgroup/io.stat") }
	} else if _, err := os.Stat("/sys/fs/cgroup/blkio/blkio.throttle.io_service_bytes"); err == nil {
		reader = func() (map[string]DevStats, error) {
			return readCgroupV1("/sys/fs/cgroup/blkio/blkio.throttle.io_service_bytes", "/sys/fs/cgroup/blkio/blkio.throttle.io_serviced")
		}
	} else if _, err := os.Stat("/sys/fs/cgroup/blkio/blkio.io_service_bytes"); err == nil {
		reader = func() (map[string]DevStats, error) {
			return readCgroupV1("/sys/fs/cgroup/blkio/blkio.io_service_bytes", "/sys/fs/cgroup/blkio/blkio.io_serviced")
		}
	} else {
		return errors.New("iostat: no supported cgroup blkio/io.stat files found (expecting /sys/fs/cgroup/io.stat or /sys/fs/cgroup/blkio/...)")
	}

	// sampling loop
	for iter := 0; iter < *count; iter++ {
		s1, err := reader()
		if err != nil {
			return err
		}
		if *interval <= 0 {
			*interval = 1
		}
		time.Sleep(time.Duration(*interval) * time.Second)
		s2, err := reader()
		if err != nil {
			return err
		}
		// compute results and formatted strings for alignment
		type Row struct {
			Dev        string
			RIOPS      float64
			WIOPS      float64
			TotalIOPS  float64
			RBps       float64
			WBps       float64
			TotalBps   float64
			FDev       string
			FRIOPS     string
			FWIOPS     string
			FTotalIOPS string
			FRBps      string
			FWBps      string
			FTotalBps  string
		}
		var rows []Row
		// union of devices
		seen := make(map[string]struct{})
		for dev := range s1 {
			seen[dev] = struct{}{}
		}
		for dev := range s2 {
			seen[dev] = struct{}{}
		}
		dur := float64(*interval)
		for dev := range seen {
			a := s1[dev]
			b := s2[dev]
			var rIOPS, wIOPS, rBps, wBps float64
			if b.RIOs >= a.RIOs {
				rIOPS = float64(b.RIOs-a.RIOs) / dur
			}
			if b.WIOs >= a.WIOs {
				wIOPS = float64(b.WIOs-a.WIOs) / dur
			}
			if b.RBytes >= a.RBytes {
				rBps = float64(b.RBytes-a.RBytes) / dur
			}
			if b.WBytes >= a.WBytes {
				wBps = float64(b.WBytes-a.WBytes) / dur
			}
			totalIOPS := rIOPS + wIOPS
			totalBps := rBps + wBps

			// helpers for humanizing
			humanBytes := func(v float64) string {
				if !*human {
					return fmt.Sprintf("%.2f", v)
				}
				abs := v
				units := []string{"B/s", "K/s", "M/s", "G/s", "T/s"}
				i := 0
				for abs >= 1024.0 && i < len(units)-1 {
					abs /= 1024.0
					i++
				}
				return fmt.Sprintf("%.2f%s", abs, units[i])
			}
			humanCount := func(v float64) string {
				// always append "/s" to indicate per-second for IOPS
				if !*human {
					return fmt.Sprintf("%.2f/s", v)
				}
				abs := v
				units := []string{"", "K", "M", "G", "T"}
				i := 0
				for abs >= 1000.0 && i < len(units)-1 {
					abs /= 1000.0
					i++
				}
				if units[i] == "" {
					return fmt.Sprintf("%.0f/s", abs)
				}
				return fmt.Sprintf("%.2f%s/s", abs, units[i])
			}

			rBpsStr := humanBytes(rBps)
			wBpsStr := humanBytes(wBps)
			totBpsStr := humanBytes(totalBps)
			rIOPSStr := humanCount(rIOPS)
			wIOPSStr := humanCount(wIOPS)
			totIOPSStr := humanCount(totalIOPS)

			// filter zero rows if requested
			if *showNonZero && rBps == 0 && wBps == 0 && rIOPS == 0 && wIOPS == 0 {
				continue
			}

			devName := devNameFromID(dev)
			rows = append(rows, Row{
				Dev:        dev,
				RIOPS:      rIOPS,
				WIOPS:      wIOPS,
				TotalIOPS:  totalIOPS,
				RBps:       rBps,
				WBps:       wBps,
				TotalBps:   totalBps,
				FDev:       devName,
				FRIOPS:     rIOPSStr,
				FWIOPS:     wIOPSStr,
				FTotalIOPS: totIOPSStr,
				FRBps:      rBpsStr,
				FWBps:      wBpsStr,
				FTotalBps:  totBpsStr,
			})
		}

		// sort rows by formatted device name (default behavior)
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].FDev == rows[j].FDev {
				return rows[i].Dev < rows[j].Dev
			}
			return rows[i].FDev < rows[j].FDev
		})

		// compute column widths
		nameW := len("Device")
		rIOPSW := len("ReadIOPS")
		wIOPSW := len("WriteIOPS")
		totIOPSW := len("TotalIOPS")
		rBpsW := len("ReadB/s")
		wBpsW := len("WriteB/s")
		totBpsW := len("TotalB/s")
		for _, r := range rows {
			if lw := len(r.FDev); lw > nameW {
				nameW = lw
			}
			if lw := len(r.FRIOPS); lw > rIOPSW {
				rIOPSW = lw
			}
			if lw := len(r.FWIOPS); lw > wIOPSW {
				wIOPSW = lw
			}
			if lw := len(r.FTotalIOPS); lw > totIOPSW {
				totIOPSW = lw
			}
			if lw := len(r.FRBps); lw > rBpsW {
				rBpsW = lw
			}
			if lw := len(r.FWBps); lw > wBpsW {
				wBpsW = lw
			}
			if lw := len(r.FTotalBps); lw > totBpsW {
				totBpsW = lw
			}
		}

		// print header
		fmtStr := fmt.Sprintf("%%-%ds  %%%ds  %%%ds  %%%ds  %%%ds  %%%ds  %%%ds\n",
			nameW, rIOPSW, wIOPSW, totIOPSW, rBpsW, wBpsW, totBpsW)
		fmt.Printf(fmtStr, "Device", "ReadIOPS", "WriteIOPS", "TotalIOPS", "ReadB/s", "WriteB/s", "TotalB/s")
		// print rows
		for _, r := range rows {
			fmt.Printf(fmtStr, r.FDev, r.FRIOPS, r.FWIOPS, r.FTotalIOPS, r.FRBps, r.FWBps, r.FTotalBps)
		}
		// separation between samples if multiple
		if iter != *count-1 {
			fmt.Println("")
		}
	}
	return nil
}
