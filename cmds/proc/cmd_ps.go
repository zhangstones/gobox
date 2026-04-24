package proc

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	ps "github.com/mitchellh/go-ps"

	"gobox/cmds/utils"
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
	uid     int
	user    string
	state   string
	tty     string
	start   time.Time
	elapsed time.Duration
}

func PsCmd(args []string) error {
	args, bsdAux := normalizePSArgs(args)
	fsFlags := flag.NewFlagSet("ps", flag.ContinueOnError)
	all := fsFlags.Bool("e", false, "show all processes (best-effort)")
	allA := fsFlags.Bool("A", false, "show all processes (alias for -e)")
	full := fsFlags.Bool("f", false, "full format (show PPID and executable)")
	extendedFull := fsFlags.Bool("F", false, "extra full format")
	longFormat := fsFlags.Bool("long", false, "long format")
	// always show human-readable memory sizes
	sortBy := fsFlags.String("sort", "pid", "sort by: pid|cpu|rss|vms|cmd")
	rev := fsFlags.Bool("r", false, "reverse sort order")
	fullFilter := fsFlags.String("full", "", "filter by extended regular expression in full command line (pgrep -f style)")
	commFilter := fsFlags.String("comm", "", "filter by exact process name pattern (pgrep -x style)")
	limit := fsFlags.Int("n", 0, "show only N entries (0 = all)")
	sampleMs := fsFlags.Int("i", 500, "CPU sample interval in milliseconds")
	maxCmd := fsFlags.Int("maxcmd", 40, "max command length (0 = unlimited)")
	wideWide := fsFlags.Bool("ww", false, "do not truncate command width")
	outputFormat := fsFlags.String("o", "", "custom output fields (e.g. pid,ppid,cmd,pcpu,pmem)")
	userFilter := fsFlags.String("u", "", "show only processes for comma-separated users or UIDs")
	pidFilter := fsFlags.String("p", "", "show only comma-separated process IDs")
	commandFilter := fsFlags.String("C", "", "show only comma-separated command names")
	hideIdle := fsFlags.Bool("hide-idle", false, "hide processes with zero sampled CPU")

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
	if *allA {
		*all = true
	}
	if *wideWide {
		*maxCmd = 0
	}
	if *fullFilter != "" && *commFilter != "" {
		return fmt.Errorf("-full and -comm cannot be used together")
	}

	var customFields []string
	if *outputFormat != "" {
		customFields = parsePSOutputFields(*outputFormat)
		if len(customFields) == 0 {
			return fmt.Errorf("no valid output fields in %q", *outputFormat)
		}
	}
	if bsdAux {
		*all = true
		*maxCmd = 0
		if len(customFields) == 0 {
			customFields = []string{"user", "pid", "pcpu", "pmem", "vsz", "rss", "tty", "stat", "start", "time", "args"}
		}
	}
	sortField, sortReverse := normalizePSSortField(*sortBy)
	if sortReverse {
		*rev = !*rev
	}

	if runtime.GOOS == "linux" {
		infos, err := gatherLinuxProcInfos(time.Duration(*sampleMs) * time.Millisecond)
		if err != nil {
			// fallback to go-ps listing if gathering detailed info fails
			return psFallback(fsFlags, all, full)
		}
		if *pidFilter != "" {
			pids, err := parsePIDList(*pidFilter)
			if err != nil {
				return err
			}
			infos = filterProcInfos(infos, func(pi procInfo) bool { return pids[pi.pid] })
		}
		if *userFilter != "" {
			users, uids, err := parseUserList(*userFilter)
			if err != nil {
				return err
			}
			infos = filterProcInfos(infos, func(pi procInfo) bool {
				return uids[pi.uid] || users[pi.user] || users[strconv.Itoa(pi.uid)]
			})
		}
		if *commandFilter != "" {
			names := parseStringSet(*commandFilter)
			infos = filterProcInfos(infos, func(pi procInfo) bool { return names[pi.exe] })
		}
		if *hideIdle {
			infos = filterProcInfos(infos, func(pi procInfo) bool { return pi.cpu != 0 })
		}
		if *fullFilter != "" {
			fullRe, err := regexp.Compile(*fullFilter)
			if err != nil {
				return fmt.Errorf("invalid -full pattern: %w", err)
			}
			filtered := infos[:0]
			for _, pi := range infos {
				if fullRe.MatchString(psFullCommand(pi)) {
					filtered = append(filtered, pi)
				}
			}
			infos = filtered
		}

		if *commFilter != "" {
			commRe, err := regexp.Compile("^(?:" + *commFilter + ")$")
			if err != nil {
				return fmt.Errorf("invalid -comm pattern: %w", err)
			}
			filtered := infos[:0]
			for _, pi := range infos {
				if commRe.MatchString(pi.exe) {
					filtered = append(filtered, pi)
				}
			}
			infos = filtered
		}

		// sorting
		switch sortField {
		case "cpu", "pcpu":
			sort.Slice(infos, func(i, j int) bool { return infos[i].cpu < infos[j].cpu })
		case "rss":
			sort.Slice(infos, func(i, j int) bool { return infos[i].rss < infos[j].rss })
		case "vms", "vsize", "vsz":
			sort.Slice(infos, func(i, j int) bool { return infos[i].vsize < infos[j].vsize })
		case "cmd", "args", "command":
			sort.Slice(infos, func(i, j int) bool { return infos[i].cmdline < infos[j].cmdline })
		case "comm":
			sort.Slice(infos, func(i, j int) bool { return infos[i].exe < infos[j].exe })
		case "user", "uid":
			sort.Slice(infos, func(i, j int) bool {
				if infos[i].uid == infos[j].uid {
					return infos[i].pid < infos[j].pid
				}
				return infos[i].uid < infos[j].uid
			})
		case "etime":
			sort.Slice(infos, func(i, j int) bool { return infos[i].elapsed < infos[j].elapsed })
		case "time":
			sort.Slice(infos, func(i, j int) bool { return infos[i].utime+infos[i].stime < infos[j].utime+infos[j].stime })
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

		memTotal := readMemTotalBytes()

		// print
		if len(customFields) > 0 {
			printCustomPS(infos, customFields, *maxCmd, memTotal)
			return nil
		}
		if *extendedFull {
			printCustomPS(infos, []string{"uid", "pid", "ppid", "pcpu", "pmem", "vsz", "rss", "tty", "stat", "start", "time", "args"}, *maxCmd, memTotal)
			return nil
		}
		if *longFormat {
			printCustomPS(infos, []string{"pid", "ppid", "stat", "tty", "time", "args"}, *maxCmd, memTotal)
			return nil
		}
		if *full {
			fmt.Printf("%6s %6s %6s %8s %8s %s\n", "PID", "PPID", "%CPU", "RSS", "VMS", "EXE")
			for _, pi := range infos {
				rss := utils.HumanSize(pi.rss)
				vms := utils.HumanSize(pi.vsize)
				exe := pi.exe
				if exe == "" {
					exe = pi.cmdline
				}
				if *maxCmd > 0 {
					exe = truncateString(exe, *maxCmd)
				}
				fmt.Printf("%6d %6d %6.1f %8s %8s %s\n", pi.pid, pi.ppid, pi.cpu, rss, vms, exe)
			}
			return nil
		}

		fmt.Printf("%6s %6s %8s %8s %s\n", "PID", "%CPU", "RSS", "VMS", "CMD")
		for _, pi := range infos {
			rss := utils.HumanSize(pi.rss)
			vms := utils.HumanSize(pi.vsize)
			cmd := pi.cmdline
			if *maxCmd > 0 {
				cmd = truncateString(cmd, *maxCmd)
			}
			fmt.Printf("%6d %6.1f %8s %8s %s\n", pi.pid, pi.cpu, rss, vms, cmd)
		}
		return nil
	}

	// Non-Linux fallback using go-ps (limited info)
	if len(customFields) > 0 {
		return psFallbackCustom(customFields, *maxCmd)
	}
	return psFallback(fsFlags, all, full)
}

func normalizePSArgs(args []string) ([]string, bool) {
	out := make([]string, 0, len(args))
	bsdAux := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "aux" {
			bsdAux = true
			continue
		}
		if arg == "-l" {
			if i+1 < len(args) && isInteger(args[i+1]) {
				out = append(out, "-maxcmd", args[i+1])
				i++
			} else {
				out = append(out, "-long")
			}
			continue
		}
		if strings.HasPrefix(arg, "-l=") {
			out = append(out, "-maxcmd="+strings.TrimPrefix(arg, "-l="))
			continue
		}
		out = append(out, arg)
	}
	return out, bsdAux
}

func isInteger(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.Atoi(s)
	return err == nil
}

func psFullCommand(pi procInfo) string {
	if pi.cmdline != "" {
		return pi.cmdline
	}
	return pi.exe
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

func psFallbackCustom(fields []string, maxCmd int) error {
	procs, err := ps.Processes()
	if err != nil {
		return err
	}
	sort.Slice(procs, func(i, j int) bool { return procs[i].Pid() < procs[j].Pid() })

	headers := make([]string, 0, len(fields))
	for _, field := range fields {
		headers = append(headers, psFieldHeader(field))
	}
	fmt.Println(strings.Join(headers, " "))
	for _, p := range procs {
		values := make([]string, 0, len(fields))
		for _, field := range fields {
			switch field {
			case "pid":
				values = append(values, strconv.Itoa(p.Pid()))
			case "ppid":
				values = append(values, strconv.Itoa(p.PPid()))
			case "args", "comm":
				cmd := p.Executable()
				if maxCmd > 0 {
					cmd = truncateString(cmd, maxCmd)
				}
				values = append(values, cmd)
			case "pcpu", "pmem", "rss", "vms", "vsz", "uid", "etime", "time":
				values = append(values, "0")
			case "user", "tty", "stat", "start":
				values = append(values, "-")
			default:
				values = append(values, "-")
			}
		}
		fmt.Println(strings.Join(values, " "))
	}
	return nil
}

func normalizePSSortField(sortBy string) (string, bool) {
	sortBy = strings.TrimSpace(sortBy)
	if idx := strings.Index(sortBy, ","); idx >= 0 {
		sortBy = sortBy[:idx]
	}
	reverse := false
	if strings.HasPrefix(sortBy, "-") {
		reverse = true
		sortBy = strings.TrimPrefix(sortBy, "-")
	} else {
		sortBy = strings.TrimPrefix(sortBy, "+")
	}
	field := normalizePSField(sortBy)
	if field == "" {
		field = strings.ToLower(sortBy)
	}
	return field, reverse
}

func parseStringSet(spec string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = true
		}
	}
	return out
}

func parsePIDList(spec string) (map[int]bool, error) {
	out := map[int]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pid, err := strconv.Atoi(part)
		if err != nil || pid <= 0 {
			return nil, fmt.Errorf("invalid pid %q", part)
		}
		out[pid] = true
	}
	return out, nil
}

func parseUserList(spec string) (map[string]bool, map[int]bool, error) {
	users := map[string]bool{}
	uids := map[int]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		users[part] = true
		if uid, err := strconv.Atoi(part); err == nil {
			uids[uid] = true
			continue
		}
		u, err := user.Lookup(part)
		if err != nil {
			continue
		}
		uid, err := strconv.Atoi(u.Uid)
		if err == nil {
			uids[uid] = true
		}
	}
	return users, uids, nil
}

func filterProcInfos(infos []procInfo, keep func(procInfo) bool) []procInfo {
	filtered := infos[:0]
	for _, pi := range infos {
		if keep(pi) {
			filtered = append(filtered, pi)
		}
	}
	return filtered
}

// gatherLinuxProcInfos samples process and system jiffies to compute CPU% and reads memory info.
// interval is the sampling duration (e.g. 500ms). CPU% is normalized by CPU count to better match top.
func gatherLinuxProcInfos(interval time.Duration) ([]procInfo, error) {
	pids, err := listPIDsProc()
	if err != nil {
		return nil, err
	}
	pageSize := int64(os.Getpagesize())
	bootTime := readBootTime()
	now := time.Now()

	infos := make([]procInfo, 0, len(pids))
	// initial sample
	total1, _ := readTotalJiffies()
	for _, pid := range pids {
		pi, err := readProcStat(pid, pageSize, bootTime, now)
		if err != nil {
			continue
		}
		infos = append(infos, pi)
	}

	// sleep interval
	if interval > 0 {
		time.Sleep(interval)
	}

	total2, _ := readTotalJiffies()
	// second sample and compute cpu%
	pidToIndex := make(map[int]int)
	for i, pi := range infos {
		pidToIndex[pi.pid] = i
	}

	numCPU := float64(runtime.NumCPU())
	now = time.Now()
	for _, pid := range pids {
		pi2, err := readProcStat(pid, pageSize, bootTime, now)
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
			prev.uid = pi2.uid
			prev.user = pi2.user
			prev.state = pi2.state
			prev.tty = pi2.tty
			prev.start = pi2.start
			prev.elapsed = pi2.elapsed
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

func normalizePSField(field string) string {
	field = strings.ToLower(strings.TrimSpace(field))
	if idx := strings.Index(field, "="); idx >= 0 {
		field = field[:idx]
	}
	field = strings.TrimPrefix(field, "%")
	switch field {
	case "c", "pcpu", "cpu":
		return "pcpu"
	case "pmem", "mem":
		return "pmem"
	case "args", "command", "cmd":
		return "args"
	case "comm", "ucmd", "ucomm", "exe":
		return "comm"
	case "stat", "state", "s":
		return "stat"
	case "etime", "elapsed":
		return "etime"
	case "time", "cputime":
		return "time"
	case "vsz", "vsize":
		return "vsz"
	case "vms":
		return "vms"
	case "rss", "pid", "ppid", "uid", "user", "tty", "tt", "start", "stime":
		if field == "tt" {
			return "tty"
		}
		if field == "stime" {
			return "start"
		}
		return field
	default:
		return ""
	}
}

func parsePSOutputFields(spec string) []string {
	parts := strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	fields := make([]string, 0, len(parts))
	for _, part := range parts {
		if field := normalizePSField(part); field != "" {
			fields = append(fields, field)
		}
	}
	return fields
}

func psFieldHeader(field string) string {
	switch field {
	case "pid":
		return "PID"
	case "ppid":
		return "PPID"
	case "args":
		return "CMD"
	case "comm":
		return "COMMAND"
	case "pcpu":
		return "%CPU"
	case "pmem":
		return "%MEM"
	case "rss":
		return "RSS"
	case "vsz":
		return "VSZ"
	case "vms":
		return "VMS"
	case "uid":
		return "UID"
	case "user":
		return "USER"
	case "tty":
		return "TTY"
	case "stat":
		return "STAT"
	case "etime":
		return "ELAPSED"
	case "time":
		return "TIME"
	case "start":
		return "START"
	default:
		return strings.ToUpper(field)
	}
}

func printCustomPS(infos []procInfo, fields []string, maxCmd int, memTotal int64) {
	headers := make([]string, 0, len(fields))
	for _, field := range fields {
		headers = append(headers, psFieldHeader(field))
	}
	fmt.Println(strings.Join(headers, " "))
	for _, pi := range infos {
		values := make([]string, 0, len(fields))
		for _, field := range fields {
			values = append(values, renderPSField(pi, field, maxCmd, memTotal))
		}
		fmt.Println(strings.Join(values, " "))
	}
}

func renderPSField(pi procInfo, field string, maxCmd int, memTotal int64) string {
	switch field {
	case "pid":
		return strconv.Itoa(pi.pid)
	case "ppid":
		return strconv.Itoa(pi.ppid)
	case "args":
		cmd := pi.cmdline
		if cmd == "" {
			cmd = pi.exe
		}
		if maxCmd > 0 {
			cmd = truncateString(cmd, maxCmd)
		}
		return cmd
	case "comm":
		exe := pi.exe
		if exe == "" {
			exe = pi.cmdline
		}
		if maxCmd > 0 {
			exe = truncateString(exe, maxCmd)
		}
		return exe
	case "pcpu":
		return fmt.Sprintf("%.1f", pi.cpu)
	case "pmem":
		if memTotal <= 0 {
			return "0.0"
		}
		return fmt.Sprintf("%.1f", (float64(pi.rss)/float64(memTotal))*100.0)
	case "rss":
		return strconv.FormatInt(pi.rss/1024, 10)
	case "vsz":
		return strconv.FormatInt(pi.vsize/1024, 10)
	case "vms":
		return strconv.FormatInt(pi.vsize/1024, 10)
	case "uid":
		return strconv.Itoa(pi.uid)
	case "user":
		if pi.user != "" {
			return pi.user
		}
		return strconv.Itoa(pi.uid)
	case "tty":
		if pi.tty != "" {
			return pi.tty
		}
		return "?"
	case "stat":
		if pi.state != "" {
			return pi.state
		}
		return "?"
	case "etime":
		return formatElapsed(pi.elapsed)
	case "time":
		return formatCPUTime(pi.utime + pi.stime)
	case "start":
		return formatStartTime(pi.start)
	default:
		return "-"
	}
}

func readMemTotalBytes() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		v, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return v * 1024
	}
	return 0
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

const procClockTicks = int64(100)

func readBootTime() time.Time {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "btime ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return time.Time{}
		}
		sec, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return time.Time{}
		}
		return time.Unix(sec, 0)
	}
	return time.Time{}
}

// readProcStat reads /proc/<pid>/stat and /proc/<pid>/cmdline to populate procInfo (best-effort)
func readProcStat(pid int, pageSize int64, bootTime time.Time, now time.Time) (procInfo, error) {
	var pi procInfo
	pi.pid = pid
	pi.uid = -1
	pi.tty = "?"
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
		pi.state = rest[0]
		// ppid is overall field 4 -> rest[1]
		if n, err := strconv.Atoi(rest[1]); err == nil {
			pi.ppid = n
		}
		if tty, err := strconv.ParseInt(rest[4], 10, 64); err == nil {
			pi.tty = procTTY(pid, tty)
		}
		if ut, err := strconv.ParseInt(rest[11], 10, 64); err == nil {
			pi.utime = ut
		}
		if st, err := strconv.ParseInt(rest[12], 10, 64); err == nil {
			pi.stime = st
		}
		if start, err := strconv.ParseInt(rest[19], 10, 64); err == nil && !bootTime.IsZero() {
			pi.start = bootTime.Add(time.Duration(start/procClockTicks) * time.Second)
			if now.After(pi.start) {
				pi.elapsed = now.Sub(pi.start)
			}
		}
		if v, err := strconv.ParseInt(rest[20], 10, 64); err == nil {
			pi.vsize = v
		}
		if r, err := strconv.ParseInt(rest[21], 10, 64); err == nil {
			pi.rss = r * pageSize
		}
	}
	readProcStatus(&pi, filepath.Join("/proc", strconv.Itoa(pid), "status"))

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

func readProcStatus(pi *procInfo, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				if uid, err := strconv.Atoi(fields[1]); err == nil {
					pi.uid = uid
					pi.user = lookupUsername(uid)
				}
			}
		}
	}
}

func lookupUsername(uid int) string {
	if uid < 0 {
		return ""
	}
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil || u.Username == "" {
		return strconv.Itoa(uid)
	}
	if idx := strings.LastIndex(u.Username, "\\"); idx >= 0 {
		return u.Username[idx+1:]
	}
	return u.Username
}

func procTTY(pid int, ttyNr int64) string {
	if ttyNr == 0 {
		return "?"
	}
	target, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "fd", "0"))
	if err == nil && strings.HasPrefix(target, "/dev/") {
		return strings.TrimPrefix(target, "/dev/")
	}
	return "?"
}

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int64(d.Seconds())
	days := total / 86400
	total %= 86400
	hours := total / 3600
	total %= 3600
	mins := total / 60
	secs := total % 60
	if days > 0 {
		return fmt.Sprintf("%d-%02d:%02d:%02d", days, hours, mins, secs)
	}
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
	}
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

func formatCPUTime(jiffies int64) string {
	secs := jiffies / procClockTicks
	hours := secs / 3600
	secs %= 3600
	mins := secs / 60
	secs %= 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
	}
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

func formatStartTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	now := time.Now()
	if t.Year() != now.Year() {
		return t.Format("2006")
	}
	if now.Sub(t) > 24*time.Hour {
		return t.Format("Jan02")
	}
	return t.Format("15:04")
}
