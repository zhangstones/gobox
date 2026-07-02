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
	pid      int
	tgid     int
	ppid     int
	exe      string
	cmdline  string
	vsize    int64 // bytes
	rss      int64 // bytes
	utime    int64
	stime    int64
	cpu      float64 // percent
	uid      int
	user     string
	state    string
	tty      string
	start    time.Time
	elapsed  time.Duration
	cpuTicks int64
}

type procSnapshot struct {
	totalJiffies int64
	cpuTimes     cpuTimes
	infos        map[int]procInfo
}

type cpuTimes struct {
	user    uint64
	nice    uint64
	system  uint64
	idle    uint64
	iowait  uint64
	irq     uint64
	softirq uint64
	steal   uint64
}

type psBSDMode struct {
	allUsers     bool
	includeNoTTY bool
	userFormat   bool
}

func PsCmd(args []string) error {
	args, bsdMode := normalizePSArgs(args)
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
		printPSUsage()
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
	maxCmdExplicit := false
	fsFlags.Visit(func(f *flag.Flag) {
		if f.Name == "maxcmd" || f.Name == "ww" {
			maxCmdExplicit = true
		}
	})
	if *wideWide {
		*maxCmd = 0
		maxCmdExplicit = true
	}
	if *fullFilter != "" && *commFilter != "" {
		return fmt.Errorf("--full and --comm cannot be used together")
	}

	var customFields []string
	if *outputFormat != "" {
		var unknownFields []string
		customFields, unknownFields = parsePSOutputFields(*outputFormat)
		if len(unknownFields) > 0 {
			return fmt.Errorf("unsupported output fields: %s", strings.Join(unknownFields, ", "))
		}
		if len(customFields) == 0 {
			return fmt.Errorf("no valid output fields in %q", *outputFormat)
		}
	}
	if bsdMode.userFormat {
		*maxCmd = 0
		if len(customFields) == 0 {
			customFields = []string{"user", "pid", "pcpu", "pmem", "vsz", "rss", "tty", "stat", "start", "time", "args"}
		}
	} else if (bsdMode.allUsers || bsdMode.includeNoTTY) && len(customFields) == 0 && !*full && !*extendedFull && !*longFormat {
		customFields = []string{"pid", "tty", "stat", "time", "bsdargs"}
	}
	showFullCommand := *fullFilter != "" || *full || *extendedFull || *longFormat || bsdMode.userFormat
	ttyWidth := 0
	if !maxCmdExplicit && !utils.IsTerminal(os.Stdout) {
		*maxCmd = 0
	}
	if !maxCmdExplicit && utils.IsTerminal(os.Stdout) {
		if width, ok := utils.StdoutWidth(); ok {
			ttyWidth = width
			*maxCmd = 0
		}
	}
	sortField, sortReverse := normalizePSSortField(*sortBy)
	if !isSupportedPSSortField(sortField) {
		return fmt.Errorf("unsupported sort field: %s", strings.TrimSpace(*sortBy))
	}
	if sortReverse {
		*rev = !*rev
	}

	if runtime.GOOS == "linux" {
		infos, err := gatherLinuxProcInfos(time.Duration(*sampleMs) * time.Millisecond)
		if err != nil {
			// fallback to go-ps listing if gathering detailed info fails
			return psFallback(fsFlags, all, full)
		}
		hasSelection := hasPSSelection(*all, bsdMode, *pidFilter, *userFilter, *commandFilter)
		if !hasSelection {
			infos = applyPSDefaultSelection(infos)
		} else {
			infos, err = applyPSExplicitSelections(infos, *all, bsdMode, *pidFilter, *userFilter, *commandFilter)
			if err != nil {
				return err
			}
		}
		if *hideIdle {
			infos = filterProcInfos(infos, func(pi procInfo) bool { return pi.cpu != 0 })
		}
		if *fullFilter != "" {
			fullRe, err := regexp.Compile(*fullFilter)
			if err != nil {
				return fmt.Errorf("invalid --full pattern: %w", err)
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
				return fmt.Errorf("invalid --comm pattern: %w", err)
			}
			filtered := infos[:0]
			for _, pi := range infos {
				if commRe.MatchString(pi.exe) {
					filtered = append(filtered, pi)
				}
			}
			infos = filtered
		}

		sortProcInfos(infos, sortField)
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
			printCustomPS(infos, customFields, *maxCmd, memTotal, ttyWidth)
			return nil
		}
		if *extendedFull {
			printCustomPS(infos, []string{"uid", "pid", "ppid", "pcpu", "pmem", "vsz", "rss", "tty", "stat", "start", "time", "args"}, *maxCmd, memTotal, ttyWidth)
			return nil
		}
		if *longFormat {
			printCustomPS(infos, []string{"pid", "ppid", "stat", "tty", "time", "args"}, *maxCmd, memTotal, ttyWidth)
			return nil
		}
		if *full {
			printPSFullFormat(infos, *maxCmd, ttyWidth)
			return nil
		}

		rows := make([][]string, 0, len(infos))
		for _, pi := range infos {
			rss := utils.HumanSize(pi.rss)
			vms := utils.HumanSize(pi.vsize)
			cmd := renderPSCommand(pi.exe, pi.cmdline, *maxCmd)
			if showFullCommand {
				cmd = renderPSCommand(pi.cmdline, pi.exe, *maxCmd)
			}
			rows = append(rows, []string{
				strconv.Itoa(pi.pid),
				fmt.Sprintf("%.1f", pi.cpu),
				rss,
				vms,
				cmd,
			})
		}
		printPSAlignedTableWithHeaders([]string{"PID", "%CPU", "RSS", "VMS", "CMD"}, rows, ttyWidth)
		return nil
	}

	// Non-Linux fallback using go-ps (limited info)
	if len(customFields) > 0 {
		return psFallbackCustom(customFields, *maxCmd)
	}
	return psFallback(fsFlags, all, full)
}

func printPSUsage() {
	fmt.Fprintln(os.Stderr, "Usage: gobox ps [OPTION]...")
	fmt.Fprintln(os.Stderr, "List processes. On Linux this shows CPU% and memory (RSS/VMS) by sampling /proc.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  -e, -A            show all processes")
	fmt.Fprintln(os.Stderr, "  -f                full format (UID/PPID/STIME/TTY/TIME/CMD)")
	fmt.Fprintln(os.Stderr, "  -F                extra full format")
	fmt.Fprintln(os.Stderr, "  -u USERS          show only comma-separated users or UIDs")
	fmt.Fprintln(os.Stderr, "  -p PIDS           show only comma-separated process IDs")
	fmt.Fprintln(os.Stderr, "  -C NAMES          show only comma-separated command names")
	fmt.Fprintln(os.Stderr, "  --comm PATTERN    exact process-name filter (pgrep -x style)")
	fmt.Fprintln(os.Stderr, "  --full REGEXP     full command-line regexp filter (pgrep -f style)")
	fmt.Fprintln(os.Stderr, "  -o FIELDS         custom fields: pid,ppid,uid,user,comm,cmd,args,pcpu,pmem,rss,vsz,vms,tty,stat,start,etime,time")
	fmt.Fprintln(os.Stderr, "  --sort FIELD      sort by: pid|ppid|cpu|pcpu|pmem|rss|vsz|vms|comm|cmd|user|start|etime|time")
	fmt.Fprintln(os.Stderr, "  -r                reverse sort order")
	fmt.Fprintln(os.Stderr, "  -n N              show only N entries (0 = all)")
	fmt.Fprintln(os.Stderr, "  -i MS             CPU sample interval in milliseconds")
	fmt.Fprintln(os.Stderr, "  -ww               do not truncate command width")
	fmt.Fprintln(os.Stderr, "  --maxcmd N        max command length (0 = unlimited)")
	fmt.Fprintln(os.Stderr, "  --hide-idle       hide processes with zero sampled CPU")
	fmt.Fprintln(os.Stderr, "  --long            long format")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Compatibility:")
	fmt.Fprintln(os.Stderr, "  ps aux            BSD-style process table with user-oriented columns")
}

func normalizePSArgs(args []string) ([]string, psBSDMode) {
	out := make([]string, 0, len(args))
	mode := psBSDMode{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") && isBSDPSMode(arg) {
			if strings.ContainsRune(arg, 'a') {
				mode.allUsers = true
			}
			if strings.ContainsRune(arg, 'x') {
				mode.includeNoTTY = true
			}
			if strings.ContainsRune(arg, 'u') {
				mode.userFormat = true
			}
			continue
		}
		out = append(out, arg)
	}
	return out, mode
}

func isBSDPSMode(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch r {
		case 'a', 'u', 'x':
		default:
			return false
		}
	}
	return true
}

func applyPSBSDSelection(infos []procInfo, mode psBSDMode) []procInfo {
	currentUID := os.Geteuid()
	return filterProcInfos(infos, func(pi procInfo) bool {
		sameUser := pi.uid == currentUID
		hasTTY := pi.tty != "" && pi.tty != "?"
		if !mode.allUsers && !sameUser {
			return false
		}
		if !mode.includeNoTTY && !hasTTY {
			return false
		}
		return true
	})
}

func hasPSSelection(all bool, mode psBSDMode, pidFilter, userFilter, commandFilter string) bool {
	if all || mode.allUsers || mode.includeNoTTY || mode.userFormat {
		return true
	}
	return pidFilter != "" || userFilter != "" || commandFilter != ""
}

func applyPSDefaultSelection(infos []procInfo) []procInfo {
	currentUID := os.Geteuid()
	currentTTY := currentProcessTTY()
	return filterProcInfos(infos, func(pi procInfo) bool {
		if pi.tgid != 0 && pi.pid != pi.tgid {
			return false
		}
		if pi.uid != currentUID {
			return false
		}
		if currentTTY == "" {
			return true
		}
		return pi.tty == currentTTY
	})
}

func applyPSExplicitSelections(infos []procInfo, all bool, mode psBSDMode, pidFilter, userFilter, commandFilter string) ([]procInfo, error) {
	selected := make(map[int]bool, len(infos))
	if all {
		for _, pi := range infos {
			selected[pi.pid] = true
		}
	}
	if mode.allUsers || mode.includeNoTTY || mode.userFormat {
		for _, pi := range applyPSBSDSelection(infos, mode) {
			selected[pi.pid] = true
		}
	}
	if pidFilter != "" {
		pids, err := parsePIDList(pidFilter)
		if err != nil {
			return nil, err
		}
		for _, pi := range infos {
			if pids[pi.pid] || (pi.tgid != 0 && pids[pi.tgid]) {
				selected[pi.pid] = true
			}
		}
	}
	if userFilter != "" {
		users, uids, err := parseUserList(userFilter)
		if err != nil {
			return nil, err
		}
		for _, pi := range infos {
			if uids[pi.uid] || users[pi.user] || users[strconv.Itoa(pi.uid)] {
				selected[pi.pid] = true
			}
		}
	}
	if commandFilter != "" {
		names := parseStringSet(commandFilter)
		for _, pi := range infos {
			if names[pi.exe] {
				selected[pi.pid] = true
			}
		}
	}
	return filterProcInfos(infos, func(pi procInfo) bool { return selected[pi.pid] }), nil
}

func currentProcessTTY() string {
	target, err := os.Readlink("/proc/self/fd/0")
	if err != nil || target == "" {
		return ""
	}
	if strings.HasPrefix(target, "/dev/") {
		tty := strings.TrimPrefix(target, "/dev/")
		if strings.HasPrefix(tty, "pts/") || strings.HasPrefix(tty, "tty") {
			return tty
		}
		return ""
	}
	return target
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
		fmt.Printf("UID\tPID\tPPID\tC\tSTIME\tTTY\tTIME\tCMD\n")
		for _, p := range procs {
			fmt.Printf("-\t%d\t%d\t0\t-\t?\t0:00\t%s\n", p.Pid(), p.PPid(), p.Executable())
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

	rows := make([][]string, 0, len(procs))
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
		rows = append(rows, values)
	}
	printPSAlignedTable(fields, rows, 0)
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

func captureLinuxProcSnapshot() (procSnapshot, error) {
	pids, err := listPIDsProc()
	if err != nil {
		return procSnapshot{}, err
	}
	pageSize := int64(os.Getpagesize())
	total, cpu, bootTime := readProcStatOnce()
	now := time.Now()

	snapshot := procSnapshot{
		totalJiffies: total,
		cpuTimes:     cpu,
		infos:        make(map[int]procInfo, len(pids)),
	}
	for _, pid := range pids {
		pi, err := readProcStat(pid, pageSize, bootTime, now)
		if err != nil {
			continue
		}
		snapshot.infos[pi.pid] = pi
	}
	return snapshot, nil
}

func captureLinuxThreadSnapshot() (procSnapshot, error) {
	pids, err := listPIDsProc()
	if err != nil {
		return procSnapshot{}, err
	}
	pageSize := int64(os.Getpagesize())
	total, cpu, bootTime := readProcStatOnce()
	now := time.Now()

	snapshot := procSnapshot{
		totalJiffies: total,
		cpuTimes:     cpu,
		infos:        make(map[int]procInfo),
	}
	for _, pid := range pids {
		tids, err := listTaskIDsProc(pid)
		if err != nil {
			continue
		}
		for _, tid := range tids {
			pi, err := readProcTaskStat(pid, tid, pageSize, bootTime, now)
			if err != nil {
				continue
			}
			snapshot.infos[pi.pid] = pi
		}
	}
	return snapshot, nil
}

func diffProcSnapshots(prev, curr procSnapshot) []procInfo {
	infos := make([]procInfo, 0, len(curr.infos))
	deltaTotal := curr.totalJiffies - prev.totalJiffies
	numCPU := float64(runtime.NumCPU())
	for pid, pi := range curr.infos {
		if prevPi, ok := prev.infos[pid]; ok && deltaTotal > 0 {
			deltaProc := (pi.utime + pi.stime) - (prevPi.utime + prevPi.stime)
			pi.cpuTicks = deltaProc
			pi.cpu = (float64(deltaProc) / float64(deltaTotal)) * 100.0 * numCPU
		}
		infos = append(infos, pi)
	}
	return infos
}

// gatherLinuxProcInfos samples process and system jiffies to compute CPU% and reads memory info.
// interval is the sampling duration (e.g. 500ms). CPU% is normalized by CPU count to better match top.
func gatherLinuxProcInfos(interval time.Duration) ([]procInfo, error) {
	prev, err := captureLinuxProcSnapshot()
	if err != nil {
		return nil, err
	}
	if interval > 0 {
		time.Sleep(interval)
	}
	curr, err := captureLinuxProcSnapshot()
	if err != nil {
		return nil, err
	}
	return diffProcSnapshots(prev, curr), nil
}

func sortProcInfos(infos []procInfo, sortField string) {
	switch sortField {
	case "cpu", "pcpu":
		sort.Slice(infos, func(i, j int) bool { return infos[i].cpu < infos[j].cpu })
	case "pmem", "rss":
		sort.Slice(infos, func(i, j int) bool { return infos[i].rss < infos[j].rss })
	case "vms", "vsize", "vsz":
		sort.Slice(infos, func(i, j int) bool { return infos[i].vsize < infos[j].vsize })
	case "cmd", "args", "command":
		sort.Slice(infos, func(i, j int) bool { return psFullCommand(infos[i]) < psFullCommand(infos[j]) })
	case "comm":
		sort.Slice(infos, func(i, j int) bool { return infos[i].exe < infos[j].exe })
	case "user", "uid":
		sort.Slice(infos, func(i, j int) bool {
			if infos[i].uid == infos[j].uid {
				return infos[i].pid < infos[j].pid
			}
			return infos[i].uid < infos[j].uid
		})
	case "ppid":
		sort.Slice(infos, func(i, j int) bool {
			if infos[i].ppid == infos[j].ppid {
				return infos[i].pid < infos[j].pid
			}
			return infos[i].ppid < infos[j].ppid
		})
	case "start":
		sort.Slice(infos, func(i, j int) bool { return infos[i].start.Before(infos[j].start) })
	case "etime":
		sort.Slice(infos, func(i, j int) bool { return infos[i].elapsed < infos[j].elapsed })
	case "time":
		sort.Slice(infos, func(i, j int) bool { return infos[i].utime+infos[i].stime < infos[j].utime+infos[j].stime })
	default:
		sort.Slice(infos, func(i, j int) bool { return infos[i].pid < infos[j].pid })
	}
}

func listTaskIDsProc(pid int) ([]int, error) {
	entries, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(pid), "task"))
	if err != nil {
		return nil, err
	}
	var tids []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if tid, err := strconv.Atoi(e.Name()); err == nil {
			tids = append(tids, tid)
		}
	}
	return tids, nil
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

func parsePSOutputFields(spec string) ([]string, []string) {
	parts := strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	fields := make([]string, 0, len(parts))
	unknown := make([]string, 0)
	for _, part := range parts {
		if field := normalizePSField(part); field != "" {
			fields = append(fields, field)
			continue
		}
		unknown = append(unknown, strings.TrimSpace(part))
	}
	return fields, unknown
}

func isSupportedPSSortField(field string) bool {
	switch field {
	case "pid", "pcpu", "pmem", "rss", "vms", "vsz", "args", "comm", "user", "uid", "ppid", "start", "etime", "time":
		return true
	default:
		return false
	}
}

func psFieldHeader(field string) string {
	switch field {
	case "pid":
		return "PID"
	case "ppid":
		return "PPID"
	case "args":
		return "CMD"
	case "bsdargs":
		return "COMMAND"
	case "command":
		return "COMMAND"
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

func printCustomPS(infos []procInfo, fields []string, maxCmd int, memTotal int64, ttyWidth int) {
	rows := make([][]string, 0, len(infos))
	for _, pi := range infos {
		values := make([]string, 0, len(fields))
		for _, field := range fields {
			values = append(values, renderPSField(pi, field, maxCmd, memTotal))
		}
		rows = append(rows, values)
	}
	printPSAlignedTable(fields, rows, ttyWidth)
}

func printPSAlignedTable(fields []string, rows [][]string, ttyWidth int) {
	headers := make([]string, len(fields))
	for i, field := range fields {
		headers[i] = psFieldHeader(field)
	}
	printPSAlignedTableWithHeaders(headers, rows, ttyWidth)
}

func printPSAlignedLine(values []string, widths []int) {
	fmt.Print(renderPSAlignedLine(values, widths))
}

func renderPSAlignedLine(values []string, widths []int) string {
	var b strings.Builder
	for i, value := range values {
		if i > 0 {
			b.WriteByte(' ')
		}
		if i == len(values)-1 {
			b.WriteString(value)
			continue
		}
		fmt.Fprintf(&b, "%-*s", widths[i], value)
	}
	b.WriteByte('\n')
	return b.String()
}

func printPSFullFormat(infos []procInfo, maxCmd int, ttyWidth int) {
	rows := make([][]string, 0, len(infos))
	for _, pi := range infos {
		cmd := renderPSCommand(pi.cmdline, pi.exe, maxCmd)
		userName := pi.user
		if userName == "" {
			userName = strconv.Itoa(pi.uid)
		}
		rows = append(rows, []string{
			userName,
			strconv.Itoa(pi.pid),
			strconv.Itoa(pi.ppid),
			strconv.Itoa(int(pi.cpu)),
			formatStartTime(pi.start),
			renderPSField(pi, "tty", maxCmd, 0),
			formatCPUTime(pi.utime + pi.stime),
			cmd,
		})
	}
	printPSAlignedTableWithHeaders([]string{"UID", "PID", "PPID", "C", "STIME", "TTY", "TIME", "CMD"}, rows, ttyWidth)
}

func printPSAlignedTableWithHeaders(headers []string, rows [][]string, ttyWidth int) {
	fmt.Print(renderPSAlignedTableWithHeaders(headers, rows, ttyWidth))
}

func renderPSAlignedTableWithHeaders(headers []string, rows [][]string, ttyWidth int) string {
	rows = fitPSRowsToWidth(headers, rows, ttyWidth)
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i, value := range row {
			if len(value) > widths[i] {
				widths[i] = len(value)
			}
		}
	}
	var b strings.Builder
	b.WriteString(renderPSAlignedLine(headers, widths))
	for _, row := range rows {
		b.WriteString(renderPSAlignedLine(row, widths))
	}
	return b.String()
}

func fitPSRowsToWidth(headers []string, rows [][]string, ttyWidth int) [][]string {
	if ttyWidth <= 0 || len(headers) == 0 {
		return rows
	}
	last := len(headers) - 1
	reserved := 0
	for i := 0; i < last; i++ {
		width := len(headers[i])
		for _, row := range rows {
			if i < len(row) && len(row[i]) > width {
				width = len(row[i])
			}
		}
		reserved += width
	}
	reserved += last
	maxLast := ttyWidth - reserved
	if maxLast < len(headers[last]) {
		maxLast = len(headers[last])
	}
	if maxLast <= 0 {
		return rows
	}
	fitted := make([][]string, 0, len(rows))
	for _, row := range rows {
		cloned := append([]string(nil), row...)
		if last < len(cloned) {
			cloned[last] = truncateString(cloned[last], maxLast)
		}
		fitted = append(fitted, cloned)
	}
	return fitted
}

func renderPSCommand(cmdline, fallback string, maxCmd int) string {
	cmd := sanitizePSDisplay(cmdline)
	if cmd == "" {
		cmd = sanitizePSDisplay(fallback)
	}
	if maxCmd > 0 {
		cmd = truncateString(cmd, maxCmd)
	}
	return cmd
}

func sanitizePSDisplay(s string) string {
	if s == "" {
		return ""
	}
	replacer := strings.NewReplacer("\r", " ", "\n", " ", "\t", " ")
	return strings.Join(strings.Fields(replacer.Replace(s)), " ")
}

func renderPSField(pi procInfo, field string, maxCmd int, memTotal int64) string {
	switch field {
	case "pid":
		return strconv.Itoa(pi.pid)
	case "ppid":
		return strconv.Itoa(pi.ppid)
	case "args", "bsdargs", "command", "cmd":
		return renderPSCommand(pi.cmdline, pi.exe, maxCmd)
	case "comm":
		return renderPSCommand(pi.exe, pi.cmdline, maxCmd)
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

// readProcStatOnce reads /proc/stat once and returns total jiffies, per-cpu times,
// and boot time — avoiding three separate opens of the same file.
func readProcStatOnce() (total int64, cpu cpuTimes, bootTime time.Time) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "cpu "):
			fields := strings.Fields(line)
			parseU := func(idx int) uint64 {
				if idx >= len(fields) {
					return 0
				}
				v, _ := strconv.ParseUint(fields[idx], 10, 64)
				return v
			}
			for _, v := range fields[1:] {
				n, _ := strconv.ParseInt(v, 10, 64)
				total += n
			}
			cpu.user = parseU(1)
			cpu.nice = parseU(2)
			cpu.system = parseU(3)
			cpu.idle = parseU(4)
			cpu.iowait = parseU(5)
			cpu.irq = parseU(6)
			cpu.softirq = parseU(7)
			cpu.steal = parseU(8)
		case strings.HasPrefix(line, "btime "):
			fields := strings.Fields(line)
			if len(fields) == 2 {
				sec, _ := strconv.ParseInt(fields[1], 10, 64)
				bootTime = time.Unix(sec, 0)
			}
		}
	}
	return
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

func readCPUTimes() (cpuTimes, error) {
	var times cpuTimes
	f, err := os.Open("/proc/stat")
	if err != nil {
		return times, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return times, scanner.Err()
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 8 || fields[0] != "cpu" {
		return times, fmt.Errorf("unexpected /proc/stat cpu line")
	}
	parse := func(idx int) uint64 {
		if idx >= len(fields) {
			return 0
		}
		v, _ := strconv.ParseUint(fields[idx], 10, 64)
		return v
	}
	times.user = parse(1)
	times.nice = parse(2)
	times.system = parse(3)
	times.idle = parse(4)
	times.iowait = parse(5)
	times.irq = parse(6)
	times.softirq = parse(7)
	times.steal = parse(8)
	return times, nil
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

func readProcStatFromPaths(pid, tgid int, statPath, statusPath, cmdPath, commPath, exePath string, pageSize int64, bootTime time.Time, now time.Time) (procInfo, error) {
	var pi procInfo
	pi.pid = pid
	pi.tgid = tgid
	pi.uid = -1
	pi.tty = "?"
	data, err := os.ReadFile(statPath)
	if err != nil {
		return pi, err
	}
	s := string(data)
	li := strings.Index(s, "(")
	ri := strings.LastIndex(s, ")")
	if li < 0 || ri < 0 || ri <= li {
		return pi, fmt.Errorf("unexpected stat format")
	}
	rest := strings.Fields(s[ri+1:])
	if len(rest) > 21 {
		pi.state = rest[0]
		if n, err := strconv.Atoi(rest[1]); err == nil {
			pi.ppid = n
		}
		if tty, err := strconv.ParseInt(rest[4], 10, 64); err == nil {
			pi.tty = procTTY(tgid, tty)
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
	readProcStatus(&pi, statusPath)

	if data, err := os.ReadFile(cmdPath); err == nil {
		cmdline := strings.ReplaceAll(string(data), "\x00", " ")
		cmdline = strings.TrimSpace(cmdline)
		if cmdline == "" {
			if p, err := os.Readlink(exePath); err == nil {
				pi.cmdline = p
			}
		} else {
			pi.cmdline = cmdline
		}
	}

	if data, err := os.ReadFile(commPath); err == nil {
		pi.exe = strings.TrimSpace(string(data))
	} else if p, err := os.Readlink(exePath); err == nil {
		pi.exe = filepath.Base(p)
	}

	return pi, nil
}

func readProcTaskStat(tgid, tid int, pageSize int64, bootTime time.Time, now time.Time) (procInfo, error) {
	base := filepath.Join("/proc", strconv.Itoa(tgid), "task", strconv.Itoa(tid))
	return readProcStatFromPaths(tid, tgid,
		filepath.Join(base, "stat"),
		filepath.Join(base, "status"),
		filepath.Join("/proc", strconv.Itoa(tgid), "cmdline"),
		filepath.Join(base, "comm"),
		filepath.Join("/proc", strconv.Itoa(tgid), "exe"),
		pageSize, bootTime, now,
	)
}

// readProcStat reads /proc/<pid>/stat and /proc/<pid>/cmdline to populate procInfo (best-effort)
func readProcStat(pid int, pageSize int64, bootTime time.Time, now time.Time) (procInfo, error) {
	var pi procInfo
	pi.pid = pid
	pi.tgid = pid
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
