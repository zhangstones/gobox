package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	ps "github.com/mitchellh/go-ps"
)

type procInfo struct {
	pid     int
	ppid    int
	exe     string
	cmdline string
	vsize   int64 // bytes
	rss     int64 // bytes
	utime   int64
	stime   int64
	cpu     float64 // percent
}

func psCmd(args []string) error {
	fsFlags := flag.NewFlagSet("ps", flag.ContinueOnError)
	all := fsFlags.Bool("e", false, "show all processes (best-effort)")
	full := fsFlags.Bool("f", false, "full format (show PPID and executable)")
	// always show human-readable memory sizes
	sortBy := fsFlags.String("sort", "pid", "sort by: pid|cpu|rss|vms|cmd")
	rev := fsFlags.Bool("r", false, "reverse sort order")
	nameFilter := fsFlags.String("name", "", "filter by substring in command name/cmdline")
	limit := fsFlags.Int("n", 0, "show only N entries (0 = all)")
	sampleMs := fsFlags.Int("i", 500, "CPU sample interval in milliseconds")
	maxCmd := fsFlags.Int("l", 40, "max command length (0 = unlimited)")

	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox ps [OPTIONS]")
		fmt.Fprintln(os.Stderr, "List processes. On Linux this shows CPU% and memory (RSS/VMS) by sampling /proc.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fsFlags.PrintDefaults()
	}

	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if runtime.GOOS == "linux" {
		infos, err := gatherLinuxProcInfos(time.Duration(*sampleMs) * time.Millisecond)
		if err != nil {
			// fallback to go-ps listing if gathering detailed info fails
			return psFallback(fsFlags, all, full)
		}
		// filtering by name
		if *nameFilter != "" {
			filtered := infos[:0]
			for _, pi := range infos {
				if strings.Contains(pi.cmdline, *nameFilter) || strings.Contains(pi.exe, *nameFilter) {
					filtered = append(filtered, pi)
				}
			}
			infos = filtered
		}

		// sorting
		switch *sortBy {
		case "cpu":
			sort.Slice(infos, func(i, j int) bool { return infos[i].cpu < infos[j].cpu })
		case "rss":
			sort.Slice(infos, func(i, j int) bool { return infos[i].rss < infos[j].rss })
		case "vms", "vsize":
			sort.Slice(infos, func(i, j int) bool { return infos[i].vsize < infos[j].vsize })
		case "cmd":
			sort.Slice(infos, func(i, j int) bool { return infos[i].cmdline < infos[j].cmdline })
		default:
			sort.Slice(infos, func(i, j int) bool { return infos[i].pid < infos[j].pid })
		}
		if *rev {
			for i, j := 0, len(infos)-1; i < j; i, j = i+1, j-1 {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}

		// limit
		if *limit > 0 && *limit < len(infos) {
			infos = infos[:*limit]
		}

		// print
		// check if stdout is a terminal; only truncate when output is a terminal
		isatty := isStdoutTerminal()
		if *full {
			fmt.Printf("%6s %6s %6s %8s %8s %s\n", "PID", "PPID", "%CPU", "RSS", "VMS", "CMD")
			for _, pi := range infos {
				rss := humanSize(pi.rss)
				vms := humanSize(pi.vsize)
				cmd := pi.cmdline
				if *maxCmd > 0 && isatty {
					cmd = truncateString(cmd, *maxCmd)
				}
				fmt.Printf("%6d %6d %6.1f %8s %8s %s\n", pi.pid, pi.ppid, pi.cpu, rss, vms, cmd)
			}
			return nil
		}

		fmt.Printf("%6s %6s %8s %8s %s\n", "PID", "%CPU", "RSS", "VMS", "CMD")
		for _, pi := range infos {
			rss := humanSize(pi.rss)
			vms := humanSize(pi.vsize)
			cmd := pi.cmdline
			if *maxCmd > 0 && isatty {
				cmd = truncateString(cmd, *maxCmd)
			}
			fmt.Printf("%6d %6.1f %8s %8s %s\n", pi.pid, pi.cpu, rss, vms, cmd)
		}
		return nil
	}

	// Non-Linux fallback using go-ps (limited info)
	return psFallback(fsFlags, all, full)
}

func psFallback(fsFlags *flag.FlagSet, all, full *bool) error {
	procs, err := ps.Processes()
	if err != nil {
		return err
	}
	sort.Slice(procs, func(i, j int) bool { return procs[i].Pid() < procs[j].Pid() })

	if *full {
		fmt.Printf("PID\tPPID\tEXE\n")
		for _, p := range procs {
			fmt.Printf("%d\t%d\t%s\n", p.Pid(), p.PPid(), p.Executable())
		}
		return nil
	}

	fmt.Printf("PID\tEXE\n")
	for _, p := range procs {
		fmt.Printf("%d\t%s\n", p.Pid(), p.Executable())
	}
	return nil
}

// gatherLinuxProcInfos samples process and system jiffies to compute CPU% and reads memory info.
// interval is the sampling duration (e.g. 500ms). CPU% is normalized by CPU count to better match top.
func gatherLinuxProcInfos(interval time.Duration) ([]procInfo, error) {
	pids, err := listPIDsProc()
	if err != nil {
		return nil, err
	}
	pageSize := int64(os.Getpagesize())

	infos := make([]procInfo, 0, len(pids))
	// initial sample
	total1, _ := readTotalJiffies()
	for _, pid := range pids {
		pi, err := readProcStat(pid, pageSize)
		if err != nil {
			continue
		}
		infos = append(infos, pi)
	}

	// sleep interval
	time.Sleep(interval)

	total2, _ := readTotalJiffies()
	// second sample and compute cpu%
	pidToIndex := make(map[int]int)
	for i, pi := range infos {
		pidToIndex[pi.pid] = i
	}

	numCPU := float64(runtime.NumCPU())
	for _, pid := range pids {
		pi2, err := readProcStat(pid, pageSize)
		if err != nil {
			continue
		}
		if idx, ok := pidToIndex[pi2.pid]; ok {
			prev := infos[idx]
			deltaProc := (pi2.utime + pi2.stime) - (prev.utime + prev.stime)
			deltaTotal := total2 - total1
			cpu := 0.0
			if deltaTotal > 0 {
				// normalize by CPU count so per-process % aligns with top-style %CPU
				cpu = (float64(deltaProc) / float64(deltaTotal)) * 100.0 * numCPU
			}
			prev.utime = pi2.utime
			prev.stime = pi2.stime
			prev.vsize = pi2.vsize
			prev.rss = pi2.rss
			prev.cmdline = pi2.cmdline
			prev.ppid = pi2.ppid
			prev.cpu = cpu
			infos[idx] = prev
		}
	}
	return infos, nil
}

func listPIDsProc() ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if pid, err := strconv.Atoi(name); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return s
	}
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	if max <= 3 {
		return string(rs[:max])
	}
	return string(rs[:max-3]) + "..."
}

func readTotalJiffies() (int64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, scanner.Err()
	}
	line := scanner.Text()
	fields := strings.Fields(line)
	var total int64
	for _, v := range fields[1:] {
		n, _ := strconv.ParseInt(v, 10, 64)
		total += n
	}
	return total, nil
}

// readProcStat reads /proc/<pid>/stat and /proc/<pid>/cmdline to populate procInfo (best-effort)
func readProcStat(pid int, pageSize int64) (procInfo, error) {
	var pi procInfo
	pi.pid = pid
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	data, err := os.ReadFile(statPath)
	if err != nil {
		return pi, err
	}
	s := string(data)
	// extract comm which is between first '(' and last ')'
	li := strings.Index(s, "(")
	ri := strings.LastIndex(s, ")")
	if li < 0 || ri < 0 || ri <= li {
		return pi, fmt.Errorf("unexpected stat format")
	}
	// rest fields after )
	rest := strings.Fields(s[ri+1:])
	// indexes relative to rest (field 3 onwards)
	// utime = field 14 -> rest[11], stime = field 15 -> rest[12]
	if len(rest) > 21 {
		// ppid is overall field 4 -> rest[1]
		if n, err := strconv.Atoi(rest[1]); err == nil {
			pi.ppid = n
		}
		if ut, err := strconv.ParseInt(rest[11], 10, 64); err == nil {
			pi.utime = ut
		}
		if st, err := strconv.ParseInt(rest[12], 10, 64); err == nil {
			pi.stime = st
		}
		if v, err := strconv.ParseInt(rest[20], 10, 64); err == nil {
			pi.vsize = v
		}
		if r, err := strconv.ParseInt(rest[21], 10, 64); err == nil {
			pi.rss = r * pageSize
		}
	}

	// cmdline
	cmdPath := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")
	if data, err := os.ReadFile(cmdPath); err == nil {
		cmdline := strings.ReplaceAll(string(data), "\x00", " ")
		cmdline = strings.TrimSpace(cmdline)
		if cmdline == "" {
			// fallback to exe name
			if p, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe")); err == nil {
				pi.cmdline = p
			}
		} else {
			pi.cmdline = cmdline
		}
	}

	// exe field from /proc/<pid>/comm or exe link
	commPath := filepath.Join("/proc", strconv.Itoa(pid), "comm")
	if data, err := os.ReadFile(commPath); err == nil {
		pi.exe = strings.TrimSpace(string(data))
	} else {
		// try exe link
		if p, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe")); err == nil {
			pi.exe = filepath.Base(p)
		}
	}

	return pi, nil
}
