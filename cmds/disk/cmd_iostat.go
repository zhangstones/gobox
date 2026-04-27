package disk

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const diskSectorSize = 512

type ioCounters struct {
	Name         string
	ReadBytes    uint64
	WriteBytes   uint64
	ReadIOs      uint64
	WriteIOs     uint64
	IoMillis     uint64
	WeightedIOms uint64
}

type iostatRow struct {
	Dev            string
	ReadIOPS       float64
	WriteIOPS      float64
	TotalIOPS      float64
	ReadBps        float64
	WriteBps       float64
	TotalBps       float64
	Await          float64
	AvgQueueSize   float64
	UtilPercent    float64
	FormattedDev   string
	FormattedRead  string
	FormattedWrite string
	FormattedTotal string
	FormattedRBps  string
	FormattedWBps  string
	FormattedTBps  string
}

var (
	readFileIostat = os.ReadFile
	statIostat     = os.Stat
	sleepIostat    = time.Sleep
	uptimeIostat   = readUptimeIostat
)

func IostatCmd(args []string) error {
	return iostatCmd(args, os.Stdout)
}

func iostatCmd(args []string, stdout io.Writer) error {
	fsFlags := flag.NewFlagSet("iostat", flag.ContinueOnError)
	fsFlags.SetOutput(os.Stderr)

	interval := fsFlags.Int("i", 1, "sample interval in seconds")
	count := fsFlags.Int("n", 1, "number of samples to take")
	human := fsFlags.Bool("H", false, "humanize IOPS and throughput")
	showNonZero := fsFlags.Bool("z", false, "show only devices with non-zero I/O rates")
	useCgroup := fsFlags.Bool("cgroup", false, "use cgroup io.stat/blkio based output")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox iostat [OPTION]... [interval [count]]")
		fmt.Fprintln(os.Stderr, "Report block device I/O activity sampled over time.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "By default gobox reads /proc/diskstats and prints per-device rates.")
		fmt.Fprintln(os.Stderr, "With --cgroup it reads cgroup io.stat / blkio counters instead.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -i SEC      sample interval in seconds")
		fmt.Fprintln(os.Stderr, "  -n COUNT    number of samples to take")
		fmt.Fprintln(os.Stderr, "  -H          humanize IOPS and throughput")
		fmt.Fprintln(os.Stderr, "  -z          show only devices with non-zero I/O rates")
		fmt.Fprintln(os.Stderr, "  --cgroup    use cgroup io.stat/blkio based output")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Positionals:")
		fmt.Fprintln(os.Stderr, "  interval   sample interval in seconds (same as -i)")
		fmt.Fprintln(os.Stderr, "  count      number of reports to print (same as -n)")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Columns:")
		fmt.Fprintln(os.Stderr, "  Device     device name from diskstats or cgroup entry")
		fmt.Fprintln(os.Stderr, "  ReadIOPS   read operations per second")
		fmt.Fprintln(os.Stderr, "  WriteIOPS  write operations per second")
		fmt.Fprintln(os.Stderr, "  TotalIOPS  combined read + write IOPS")
		fmt.Fprintln(os.Stderr, "  ReadB/s    read throughput in bytes per second")
		fmt.Fprintln(os.Stderr, "  WriteB/s   write throughput in bytes per second")
		fmt.Fprintln(os.Stderr, "  TotalB/s   combined throughput in bytes per second")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gobox iostat")
		fmt.Fprintln(os.Stderr, "  gobox iostat 1 5")
		fmt.Fprintln(os.Stderr, "  gobox iostat -i 2 -n 3 -H -z")
		fmt.Fprintln(os.Stderr, "  gobox iostat --cgroup 1 1")
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

	if err := applyIostatPositionals(fsFlags.Args(), interval, count); err != nil {
		return err
	}
	if *interval <= 0 {
		*interval = 1
	}
	if *count <= 0 {
		return errors.New("iostat: count must be >= 1")
	}

	reader, err := buildIostatReader(*useCgroup)
	if err != nil {
		return err
	}

	// Match native iostat semantics: the first report is since-boot average,
	// then subsequent reports are interval deltas.
	current, err := reader()
	if err != nil {
		return err
	}

	for iter := 0; iter < *count; iter++ {
		var start, end map[string]ioCounters
		var dur float64
		if iter == 0 {
			end = current
			dur, err = uptimeIostat()
			if err != nil {
				return err
			}
		} else {
			start = current
			sleepIostat(time.Duration(*interval) * time.Second)
			current, err = reader()
			if err != nil {
				return err
			}
			end = current
			dur = float64(*interval)
		}

		rows := buildIostatRows(start, end, dur, *human, *showNonZero, *useCgroup)
		writeIostatTable(stdout, rows)
		if iter != *count-1 {
			fmt.Fprintln(stdout)
		}
	}

	return nil
}

func applyIostatPositionals(args []string, interval, count *int) error {
	if len(args) > 2 {
		return fmt.Errorf("iostat: unexpected arguments: %v", args)
	}
	if len(args) >= 1 {
		v, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("iostat: invalid interval %q", args[0])
		}
		*interval = v
	}
	if len(args) == 2 {
		v, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("iostat: invalid count %q", args[1])
		}
		*count = v
	}
	return nil
}

func buildIostatReader(useCgroup bool) (func() (map[string]ioCounters, error), error) {
	if useCgroup {
		return buildCgroupReader()
	}
	return func() (map[string]ioCounters, error) {
		return readDiskstats("/proc/diskstats")
	}, nil
}

func readUptimeIostat() (float64, error) {
	data, err := readFileIostat("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, errors.New("iostat: malformed /proc/uptime")
	}
	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("iostat: parse /proc/uptime: %w", err)
	}
	if uptime <= 0 {
		return 1, nil
	}
	return uptime, nil
}

func readDiskstats(path string) (map[string]ioCounters, error) {
	data, err := readFileIostat(path)
	if err != nil {
		return nil, err
	}

	out := make(map[string]ioCounters)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		readIOs, err1 := strconv.ParseUint(fields[3], 10, 64)
		readSectors, err2 := strconv.ParseUint(fields[5], 10, 64)
		writeIOs, err3 := strconv.ParseUint(fields[7], 10, 64)
		writeSectors, err4 := strconv.ParseUint(fields[9], 10, 64)
		ioMillis, err5 := strconv.ParseUint(fields[12], 10, 64)
		weightedIOms, err6 := strconv.ParseUint(fields[13], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil || err6 != nil {
			continue
		}

		name := fields[2]
		out[name] = ioCounters{
			Name:         name,
			ReadBytes:    readSectors * diskSectorSize,
			WriteBytes:   writeSectors * diskSectorSize,
			ReadIOs:      readIOs,
			WriteIOs:     writeIOs,
			IoMillis:     ioMillis,
			WeightedIOms: weightedIOms,
		}
	}
	return out, scanner.Err()
}

func buildCgroupReader() (func() (map[string]ioCounters, error), error) {
	if _, err := statIostat("/sys/fs/cgroup/io.stat"); err == nil {
		return func() (map[string]ioCounters, error) {
			return readCgroupV2("/sys/fs/cgroup/io.stat")
		}, nil
	}
	if _, err := statIostat("/sys/fs/cgroup/blkio/blkio.throttle.io_service_bytes"); err == nil {
		return func() (map[string]ioCounters, error) {
			return readCgroupV1("/sys/fs/cgroup/blkio/blkio.throttle.io_service_bytes", "/sys/fs/cgroup/blkio/blkio.throttle.io_serviced")
		}, nil
	}
	if _, err := statIostat("/sys/fs/cgroup/blkio/blkio.io_service_bytes"); err == nil {
		return func() (map[string]ioCounters, error) {
			return readCgroupV1("/sys/fs/cgroup/blkio/blkio.io_service_bytes", "/sys/fs/cgroup/blkio/blkio.io_serviced")
		}, nil
	}
	return nil, errors.New("iostat: no supported cgroup blkio/io.stat files found")
}

func readCgroupV2(path string) (map[string]ioCounters, error) {
	data, err := readFileIostat(path)
	if err != nil {
		return nil, err
	}

	out := make(map[string]ioCounters)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		dev := fields[0]
		stats := ioCounters{Name: cgroupDeviceName(dev)}
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
				stats.ReadBytes = v
			case "wbytes":
				stats.WriteBytes = v
			case "rios":
				stats.ReadIOs = v
			case "wios":
				stats.WriteIOs = v
			}
		}
		out[dev] = stats
	}
	return out, scanner.Err()
}

func readCgroupV1(pathBytes, pathServiced string) (map[string]ioCounters, error) {
	out := make(map[string]ioCounters)

	if data, err := readFileIostat(pathBytes); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 3 {
				continue
			}
			dev := fields[0]
			stats := out[dev]
			stats.Name = cgroupDeviceName(dev)
			if fields[1] == "Read" {
				if v, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
					stats.ReadBytes = v
				}
				for i := 3; i < len(fields)-1; i++ {
					if fields[i] == "Write" {
						if v, err := strconv.ParseUint(fields[i+1], 10, 64); err == nil {
							stats.WriteBytes = v
						}
					}
				}
			} else {
				if v, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
					stats.ReadBytes = v
				}
				if v, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
					stats.WriteBytes = v
				}
			}
			out[dev] = stats
		}
	}

	if data, err := readFileIostat(pathServiced); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 3 {
				continue
			}
			dev := fields[0]
			stats := out[dev]
			stats.Name = cgroupDeviceName(dev)
			if fields[1] == "Read" {
				if v, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
					stats.ReadIOs = v
				}
				for i := 3; i < len(fields)-1; i++ {
					if fields[i] == "Write" {
						if v, err := strconv.ParseUint(fields[i+1], 10, 64); err == nil {
							stats.WriteIOs = v
						}
					}
				}
			} else {
				if v, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
					stats.ReadIOs = v
				}
				if v, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
					stats.WriteIOs = v
				}
			}
			out[dev] = stats
		}
	}

	return out, nil
}

func cgroupDeviceName(id string) string {
	if strings.IndexFunc(id, func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}) >= 0 {
		return id
	}
	return id
}

func buildIostatRows(start, end map[string]ioCounters, dur float64, human, showNonZero, cgroupMode bool) []iostatRow {
	seen := make(map[string]struct{})
	for dev := range start {
		seen[dev] = struct{}{}
	}
	for dev := range end {
		seen[dev] = struct{}{}
	}

	rows := make([]iostatRow, 0, len(seen))
	for dev := range seen {
		a := start[dev]
		b := end[dev]
		name := b.Name
		if name == "" {
			name = a.Name
		}
		if name == "" {
			name = dev
		}

		readIOPS := deltaPerSecond(a.ReadIOs, b.ReadIOs, dur)
		writeIOPS := deltaPerSecond(a.WriteIOs, b.WriteIOs, dur)
		readBps := deltaPerSecond(a.ReadBytes, b.ReadBytes, dur)
		writeBps := deltaPerSecond(a.WriteBytes, b.WriteBytes, dur)
		totalIOPS := readIOPS + writeIOPS
		totalBps := readBps + writeBps

		if showNonZero && readIOPS == 0 && writeIOPS == 0 && readBps == 0 && writeBps == 0 {
			continue
		}

		row := iostatRow{
			Dev:            dev,
			ReadIOPS:       readIOPS,
			WriteIOPS:      writeIOPS,
			TotalIOPS:      totalIOPS,
			ReadBps:        readBps,
			WriteBps:       writeBps,
			TotalBps:       totalBps,
			FormattedDev:   name,
			FormattedRead:  formatCountPerSecond(readIOPS, human),
			FormattedWrite: formatCountPerSecond(writeIOPS, human),
			FormattedTotal: formatCountPerSecond(totalIOPS, human),
			FormattedRBps:  formatBytesPerSecond(readBps, human),
			FormattedWBps:  formatBytesPerSecond(writeBps, human),
			FormattedTBps:  formatBytesPerSecond(totalBps, human),
		}
		if !cgroupMode {
			deltaIOs := deltaUint64(a.ReadIOs+a.WriteIOs, b.ReadIOs+b.WriteIOs)
			if deltaIOs > 0 {
				row.Await = float64(deltaUint64(a.WeightedIOms, b.WeightedIOms)) / float64(deltaIOs)
			}
			row.AvgQueueSize = float64(deltaUint64(a.WeightedIOms, b.WeightedIOms)) / 1000.0 / dur
			row.UtilPercent = float64(deltaUint64(a.IoMillis, b.IoMillis)) / (dur * 10.0)
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].FormattedDev == rows[j].FormattedDev {
			return rows[i].Dev < rows[j].Dev
		}
		return rows[i].FormattedDev < rows[j].FormattedDev
	})
	return rows
}

func writeIostatTable(w io.Writer, rows []iostatRow) {
	nameW := len("Device")
	readW := len("ReadIOPS")
	writeW := len("WriteIOPS")
	totalW := len("TotalIOPS")
	rbpsW := len("ReadB/s")
	wbpsW := len("WriteB/s")
	tbpsW := len("TotalB/s")

	for _, row := range rows {
		nameW = max(nameW, len(row.FormattedDev))
		readW = max(readW, len(row.FormattedRead))
		writeW = max(writeW, len(row.FormattedWrite))
		totalW = max(totalW, len(row.FormattedTotal))
		rbpsW = max(rbpsW, len(row.FormattedRBps))
		wbpsW = max(wbpsW, len(row.FormattedWBps))
		tbpsW = max(tbpsW, len(row.FormattedTBps))
	}

	format := fmt.Sprintf("%%-%ds  %%%ds  %%%ds  %%%ds  %%%ds  %%%ds  %%%ds\n", nameW, readW, writeW, totalW, rbpsW, wbpsW, tbpsW)
	fmt.Fprintf(w, format, "Device", "ReadIOPS", "WriteIOPS", "TotalIOPS", "ReadB/s", "WriteB/s", "TotalB/s")
	for _, row := range rows {
		fmt.Fprintf(w, format, row.FormattedDev, row.FormattedRead, row.FormattedWrite, row.FormattedTotal, row.FormattedRBps, row.FormattedWBps, row.FormattedTBps)
	}
}

func deltaPerSecond(a, b uint64, dur float64) float64 {
	if b < a || dur <= 0 {
		return 0
	}
	return float64(b-a) / dur
}

func deltaUint64(a, b uint64) uint64 {
	if b < a {
		return 0
	}
	return b - a
}

func formatBytesPerSecond(v float64, human bool) string {
	if !human {
		return fmt.Sprintf("%.2f", v)
	}
	units := []string{"B/s", "K/s", "M/s", "G/s", "T/s"}
	idx := 0
	for v >= 1024 && idx < len(units)-1 {
		v /= 1024
		idx++
	}
	return fmt.Sprintf("%.2f%s", v, units[idx])
}

func formatCountPerSecond(v float64, human bool) string {
	if !human {
		return fmt.Sprintf("%.2f/s", v)
	}
	units := []string{"", "K", "M", "G", "T"}
	idx := 0
	for v >= 1000 && idx < len(units)-1 {
		v /= 1000
		idx++
	}
	if units[idx] == "" {
		return fmt.Sprintf("%.0f/s", v)
	}
	return fmt.Sprintf("%.2f%s/s", v, units[idx])
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
