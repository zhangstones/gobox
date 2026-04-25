package proc

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"gobox/cmds/utils"
)

var topSortColumns = []string{"pid", "user", "vms", "rss", "pcpu", "pmem", "time", "cmd"}

func TopCmd(args []string) error {
	fsFlags := flag.NewFlagSet("top", flag.ContinueOnError)
	interval := fsFlags.String("d", "2", "delay in seconds between updates")
	count := fsFlags.Int("n", 0, "number of iterations (0 = infinite)")
	batch := fsFlags.Bool("b", false, "batch mode")
	pids := fsFlags.String("p", "", "show only comma-separated process IDs")
	users := fsFlags.String("u", "", "show only processes for comma-separated users or UIDs")
	threads := fsFlags.Bool("H", false, "thread display mode (accepted; process-level output)")
	hideIdle := fsFlags.Bool("i", false, "hide processes with zero sampled CPU")
	fullCmd := fsFlags.Bool("c", false, "show full command lines")
	orderBy := fsFlags.String("o", "", "sort by field")
	sortBy := fsFlags.String("sort", "cpu", "sort by: pid|cpu|rss|vms|cmd")
	rev := fsFlags.Bool("r", true, "reverse sort order")

	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox top [OPTIONS]")
		fmt.Fprintln(os.Stderr, "Display dynamic real-time view of running processes (very small subset).")
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
	_ = threads
	delay, err := parseTopDelay(*interval)
	if err != nil {
		return err
	}
	if *orderBy != "" {
		*sortBy = normalizeTopOrderBy(*orderBy)
	}
	sortField, sortReverse := normalizePSSortField(*sortBy)
	if sortReverse {
		*rev = !*rev
	}
	if sortField == "" {
		sortField = "cpu"
	}

	iterations := *count
	if iterations < 0 {
		iterations = 0
	}
	interactiveTTY := !*batch && utils.IsTerminal(os.Stdout)

	if runtime.GOOS != "linux" {
		return runTopViaPS(*batch, *pids, *users, *hideIdle, *fullCmd, sortField, *rev, delay, iterations)
	}

	var pidFilter map[int]bool
	if *pids != "" {
		pidFilter, err = parsePIDList(*pids)
		if err != nil {
			return err
		}
	}
	var userNames map[string]bool
	var userIDs map[int]bool
	if *users != "" {
		userNames, userIDs, err = parseUserList(*users)
		if err != nil {
			return err
		}
	}

	prev, err := captureLinuxProcSnapshot()
	if err != nil {
		return err
	}
	if interactiveTTY {
		hideTopCursor()
		defer restoreTopScreen()
	}

	memTotal := readMemTotalBytes()
	var sortInput <-chan topInputEvent
	var stopInput func()
	var sigCh chan os.Signal
	if interactiveTTY {
		var err error
		sortInput, stopInput, err = startTopInput()
		if err != nil {
			return err
		}
		defer stopInput()
		sigCh = make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		defer signal.Stop(sigCh)
	}

	i := 0
	currentSort := sortField
	currentReverse := *rev
	sortIndex := topSortColumnIndex(currentSort)
	firstDraw := true
	for {
		wait := delay
		if firstDraw {
			wait = initialTopSampleDelay(delay)
		}
		if interactiveTTY {
			nextSort, nextReverse, redraw, quit, err := waitTopEvent(wait, sortInput, sigCh, currentSort, currentReverse)
			if err != nil {
				return err
			}
			if quit {
				break
			}
			currentSort = nextSort
			currentReverse = nextReverse
			sortIndex = topSortColumnIndex(currentSort)
			if !redraw {
				continue
			}
		} else if wait > 0 {
			time.Sleep(wait)
		}
		curr, err := captureLinuxProcSnapshot()
		if err != nil {
			return err
		}
		infos := diffProcSnapshots(prev, curr)
		prev = curr
		infos = filterTopInfos(infos, pidFilter, userNames, userIDs, *hideIdle)
		renderTopScreen(prev, curr, infos, *fullCmd, *batch, memTotal, currentSort, sortIndex, interactiveTTY, currentReverse)
		i++
		firstDraw = false
		if iterations != 0 && i >= iterations {
			break
		}
	}
	return nil
}

func parseTopDelay(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("invalid delay %q", value)
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("invalid delay %q", value)
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func runTopViaPS(batch bool, pids, users string, hideIdle, fullCmd bool, sortField string, rev bool, delay time.Duration, iterations int) error {
	interactiveTTY := !batch && utils.IsTerminal(os.Stdout)
	if interactiveTTY {
		hideTopCursor()
		defer restoreTopScreen()
	}
	i := 0
	for {
		curr, err := captureLinuxProcSnapshot()
		if err != nil {
			return err
		}
		infos := diffProcSnapshots(curr, curr)
		if pids != "" {
			pidFilter, err := parsePIDList(pids)
			if err != nil {
				return err
			}
			infos = filterProcInfos(infos, func(pi procInfo) bool { return pidFilter[pi.pid] })
		}
		if users != "" {
			userNames, userIDs, err := parseUserList(users)
			if err != nil {
				return err
			}
			infos = filterTopInfos(infos, nil, userNames, userIDs, hideIdle)
		} else if hideIdle {
			infos = filterTopInfos(infos, nil, nil, nil, hideIdle)
		}
		renderTopScreen(curr, curr, infos, fullCmd, batch, readMemTotalBytes(), sortField, topSortColumnIndex(sortField), interactiveTTY, rev)
		i++
		if iterations != 0 && i >= iterations {
			return nil
		}
		if delay > 0 {
			time.Sleep(delay)
		}
	}
}

func hideTopCursor() {
	fmt.Print("\033[?25l")
}

func showTopCursor() {
	fmt.Print("\033[?25h")
}

func restoreTopScreen() {
	showTopCursor()
	fmt.Print("\n")
}

func clearTopScreen() {
	// Clear only from the home position to screen end. This avoids the more
	// disruptive full-screen reset and reduces visible flicker/scrolling.
	fmt.Print("\033[H\033[J")
}

func initialTopSampleDelay(delay time.Duration) time.Duration {
	const defaultSample = 200 * time.Millisecond
	if delay > 0 && delay < defaultSample {
		return delay
	}
	return defaultSample
}

func normalizeTopOrderBy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "%cpu", "cpu":
		return "cpu"
	case "%mem", "mem", "res", "rss":
		return "rss"
	case "virt", "vsz", "vms":
		return "vms"
	case "command", "cmd", "args":
		return "cmd"
	default:
		return value
	}
}

func filterTopInfos(infos []procInfo, pidFilter map[int]bool, userNames map[string]bool, userIDs map[int]bool, hideIdle bool) []procInfo {
	if len(pidFilter) > 0 {
		infos = filterProcInfos(infos, func(pi procInfo) bool { return pidFilter[pi.pid] })
	}
	if len(userNames) > 0 || len(userIDs) > 0 {
		infos = filterProcInfos(infos, func(pi procInfo) bool {
			return userNames[pi.user] || userIDs[pi.uid]
		})
	}
	if hideIdle {
		infos = filterProcInfos(infos, func(pi procInfo) bool { return pi.cpu != 0 })
	}
	return infos
}

func renderTopScreen(prev, curr procSnapshot, infos []procInfo, fullCmd, batch bool, memTotal int64, sortField string, sortIndex int, interactive bool, reverse bool) {
	ttyWidth := 0
	ttyHeight := 0
	if utils.IsTerminal(os.Stdout) {
		if width, height, ok := utils.StdoutSize(); ok {
			ttyWidth = width
			ttyHeight = height
		}
	}
	var out bytes.Buffer
	if interactive {
		out.WriteString("\033[H\033[J")
	}
	summary := buildTopSummary(prev, curr, infos)
	for _, line := range summary {
		out.WriteString(line)
		out.WriteByte('\n')
	}
	out.WriteByte('\n')
	sortTopInfos(infos, sortField, reverse, memTotal)
	rowLimit := topVisibleRowLimit(batch, ttyHeight, len(summary))
	if rowLimit > 0 && len(infos) > rowLimit {
		infos = infos[:rowLimit]
	}
	rows := make([][]string, 0, len(infos))
	for _, pi := range infos {
		rows = append(rows, []string{
			strconv.Itoa(pi.pid),
			topRenderUser(pi),
			utils.HumanSize(pi.vsize),
			utils.HumanSize(pi.rss),
			topRenderState(pi),
			fmt.Sprintf("%.1f", pi.cpu),
			renderTopPMem(pi, memTotal),
			formatCPUTime(pi.utime + pi.stime),
			renderTopCommand(pi, fullCmd),
		})
	}
	headers := []string{"PID", "USER", "VIRT", "RES", "STATE", "%CPU", "%MEM", "TIME+", "COMMAND"}
	headers = highlightTopSortHeader(headers, sortField, sortIndex)
	out.WriteString(renderTopTable(headers, rows, ttyWidth, interactive))
	frame := out.String()
	if interactive {
		frame = strings.TrimRight(frame, "\n")
	}
	fmt.Print(frame)
}

func renderTopCommand(pi procInfo, fullCmd bool) string {
	if fullCmd {
		return renderPSCommand(pi.cmdline, pi.exe, 0)
	}
	return renderPSCommand(pi.exe, pi.cmdline, 0)
}

func sortTopInfos(infos []procInfo, sortField string, reverse bool, memTotal int64) {
	lessInt64 := func(left, right int64) bool {
		if reverse {
			return left > right
		}
		return left < right
	}
	lessFloat64 := func(left, right float64) bool {
		if reverse {
			return left > right
		}
		return left < right
	}
	lessString := func(left, right string) bool {
		if reverse {
			return left > right
		}
		return left < right
	}
	sort.SliceStable(infos, func(i, j int) bool {
		left := infos[i]
		right := infos[j]
		switch sortField {
		case "cpu", "pcpu":
			if left.cpuTicks != right.cpuTicks {
				return lessInt64(left.cpuTicks, right.cpuTicks)
			}
			if left.cpu != right.cpu {
				return lessFloat64(left.cpu, right.cpu)
			}
		case "pmem":
			if left.rss != right.rss {
				return lessInt64(left.rss, right.rss)
			}
			leftMem := topPMemValue(left, memTotal)
			rightMem := topPMemValue(right, memTotal)
			if leftMem != rightMem {
				return lessFloat64(leftMem, rightMem)
			}
		case "rss":
			if left.rss != right.rss {
				return lessInt64(left.rss, right.rss)
			}
		case "vms", "vsize", "vsz":
			if left.vsize != right.vsize {
				return lessInt64(left.vsize, right.vsize)
			}
		case "cmd", "args", "command":
			leftCmd := psFullCommand(left)
			rightCmd := psFullCommand(right)
			if leftCmd != rightCmd {
				return lessString(leftCmd, rightCmd)
			}
		case "comm":
			if left.exe != right.exe {
				return lessString(left.exe, right.exe)
			}
		case "user", "uid":
			leftUser := topRenderUser(left)
			rightUser := topRenderUser(right)
			if leftUser != rightUser {
				return lessString(leftUser, rightUser)
			}
		case "ppid":
			if left.ppid != right.ppid {
				return lessInt64(int64(left.ppid), int64(right.ppid))
			}
		case "start":
			if !left.start.Equal(right.start) {
				if reverse {
					return left.start.After(right.start)
				}
				return left.start.Before(right.start)
			}
		case "etime":
			if left.elapsed != right.elapsed {
				return lessInt64(int64(left.elapsed), int64(right.elapsed))
			}
		case "time":
			leftCPU := left.utime + left.stime
			rightCPU := right.utime + right.stime
			if leftCPU != rightCPU {
				return lessInt64(leftCPU, rightCPU)
			}
		default:
			if left.pid != right.pid {
				return lessInt64(int64(left.pid), int64(right.pid))
			}
		}
		return left.pid < right.pid
	})
}

func topVisibleRowLimit(batch bool, ttyHeight, summaryLines int) int {
	if batch || ttyHeight <= 0 {
		return 0
	}
	// summary + blank line + header + one spare row.
	available := ttyHeight - summaryLines - 3
	if available < 1 {
		return 1
	}
	return available
}

func renderTopTable(headers []string, rows [][]string, ttyWidth int, interactive bool) string {
	widths := []int{7, 12, 8, 8, 5, 6, 6, 8, 0}
	if len(headers) != len(widths) {
		return renderPSAlignedTableWithHeaders(headers, rows, ttyWidth)
	}
	availableForCommand := 0
	if ttyWidth > 0 {
		fixed := 0
		for i := 0; i < len(widths)-1; i++ {
			fixed += widths[i]
		}
		fixed += len(widths) - 1
		availableForCommand = ttyWidth - fixed
		if availableForCommand < len("COMMAND") {
			availableForCommand = len("COMMAND")
		}
	}
	var b strings.Builder
	b.WriteString(renderTopLine(headers, widths, availableForCommand, false))
	for idx, row := range rows {
		last := interactive && idx == len(rows)-1
		b.WriteString(renderTopLine(row, widths, availableForCommand, last))
	}
	return b.String()
}

func renderTopLine(values []string, widths []int, commandWidth int, lastLine bool) string {
	var b strings.Builder
	for i, value := range values {
		if i > 0 {
			b.WriteByte(' ')
		}
		width := widths[i]
		if i == len(values)-1 {
			if commandWidth > 0 {
				value = truncateString(value, commandWidth)
			}
			b.WriteString(value)
			continue
		}
		value = truncateString(value, width)
		fmt.Fprintf(&b, "%-*s", width, value)
	}
	if !lastLine {
		b.WriteByte('\n')
	}
	return b.String()
}

func topRenderUser(pi procInfo) string {
	if pi.user != "" {
		return pi.user
	}
	if pi.uid >= 0 {
		return strconv.Itoa(pi.uid)
	}
	return "-"
}

func topRenderState(pi procInfo) string {
	if pi.state != "" {
		return pi.state
	}
	return "?"
}

func topPMemValue(pi procInfo, memTotal int64) float64 {
	if memTotal <= 0 {
		return 0
	}
	return (float64(pi.rss) / float64(memTotal)) * 100.0
}

func renderTopPMem(pi procInfo, memTotal int64) string {
	return fmt.Sprintf("%.1f", topPMemValue(pi, memTotal))
}

func buildTopSummary(prev, curr procSnapshot, infos []procInfo) []string {
	now := time.Now().Format("15:04:05")
	load1, load5, load15 := readTopLoadAvg()
	uptime := formatTopUptime(readTopUptime())
	mem, _ := readMemInfoData()
	memTotal := mem["MemTotal"]
	memFree := mem["MemFree"]
	memAvail := mem["MemAvailable"]
	buffCache := mem["Buffers"] + mem["Cached"] + mem["SReclaimable"]
	memUsed := uint64(0)
	if memTotal > memFree+buffCache {
		memUsed = memTotal - memFree - buffCache
	}
	swapTotal := mem["SwapTotal"]
	swapFree := mem["SwapFree"]
	swapUsed := uint64(0)
	if swapTotal > swapFree {
		swapUsed = swapTotal - swapFree
	}

	total, running, sleeping, stopped, zombie := summarizeTopTasks(infos)
	cpu := summarizeTopCPU(prev.cpuTimes, curr.cpuTimes)

	return []string{
		fmt.Sprintf("top - %s up %s, load average: %.2f, %.2f, %.2f", now, uptime, load1, load5, load15),
		fmt.Sprintf("Tasks: %d total, %d running, %d sleeping, %d stopped, %d zombie", total, running, sleeping, stopped, zombie),
		fmt.Sprintf("%%Cpu(s): %4.1f us, %4.1f sy, %4.1f ni, %4.1f id, %4.1f wa, %4.1f hi, %4.1f si, %4.1f st", cpu.user, cpu.system, cpu.nice, cpu.idle, cpu.iowait, cpu.irq, cpu.softirq, cpu.steal),
		fmt.Sprintf("MiB Mem : %7.1f total, %7.1f free, %7.1f used, %7.1f buff/cache", bytesToMiB(memTotal), bytesToMiB(memFree), bytesToMiB(memUsed), bytesToMiB(buffCache)),
		fmt.Sprintf("MiB Swap: %7.1f total, %7.1f free, %7.1f used. %7.1f avail Mem", bytesToMiB(swapTotal), bytesToMiB(swapFree), bytesToMiB(swapUsed), bytesToMiB(memAvail)),
	}
}

type topCPUSummary struct {
	user    float64
	system  float64
	nice    float64
	idle    float64
	iowait  float64
	irq     float64
	softirq float64
	steal   float64
}

type topInputEvent struct {
	sortDelta int
	toggleDir bool
	quit      bool
}

func summarizeTopCPU(prev, curr cpuTimes) topCPUSummary {
	diff := func(a, b uint64) uint64 {
		if b >= a {
			return b - a
		}
		return 0
	}
	total := diff(prev.user, curr.user) + diff(prev.nice, curr.nice) + diff(prev.system, curr.system) +
		diff(prev.idle, curr.idle) + diff(prev.iowait, curr.iowait) + diff(prev.irq, curr.irq) +
		diff(prev.softirq, curr.softirq) + diff(prev.steal, curr.steal)
	if total == 0 {
		total = 1
	}
	pct := func(delta uint64) float64 {
		return (float64(delta) / float64(total)) * 100.0
	}
	return topCPUSummary{
		user:    pct(diff(prev.user, curr.user)),
		system:  pct(diff(prev.system, curr.system)),
		nice:    pct(diff(prev.nice, curr.nice)),
		idle:    pct(diff(prev.idle, curr.idle)),
		iowait:  pct(diff(prev.iowait, curr.iowait)),
		irq:     pct(diff(prev.irq, curr.irq)),
		softirq: pct(diff(prev.softirq, curr.softirq)),
		steal:   pct(diff(prev.steal, curr.steal)),
	}
}

func summarizeTopTasks(infos []procInfo) (total, running, sleeping, stopped, zombie int) {
	total = len(infos)
	for _, pi := range infos {
		switch pi.state {
		case "R":
			running++
		case "T", "t":
			stopped++
		case "Z":
			zombie++
		default:
			sleeping++
		}
	}
	return total, running, sleeping, stopped, zombie
}

func readTopLoadAvg() (float64, float64, float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	parse := func(s string) float64 {
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}
	return parse(fields[0]), parse(fields[1]), parse(fields[2])
}

func readTopUptime() time.Duration {
	f, err := os.Open("/proc/uptime")
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) == 0 {
		return 0
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func formatTopUptime(d time.Duration) string {
	if d <= 0 {
		return "0 min"
	}
	totalMinutes := int(d / time.Minute)
	days := totalMinutes / (24 * 60)
	hours := (totalMinutes / 60) % 24
	minutes := totalMinutes % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%d day%s, %2d:%02d", days, pluralSuffix(days), hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%2d:%02d", hours, minutes)
	default:
		return fmt.Sprintf("%d min", minutes)
	}
}

func pluralSuffix(v int) string {
	if v == 1 {
		return ""
	}
	return "s"
}

func bytesToMiB(v uint64) float64 {
	return float64(v) / 1024.0 / 1024.0
}

func topSortColumnIndex(sortField string) int {
	sortField = normalizeTopSortField(sortField)
	for i, field := range topSortColumns {
		if field == sortField {
			return i
		}
	}
	return 0
}

func highlightTopSortHeader(headers []string, sortField string, sortIndex int) []string {
	out := append([]string(nil), headers...)
	sortField = normalizeTopSortField(sortField)
	labelByField := map[string]string{
		"pid":  "PID",
		"user": "USER",
		"vms":  "VIRT",
		"rss":  "RES",
		"pcpu": "%CPU",
		"pmem": "%MEM",
		"time": "TIME+",
		"cmd":  "COMMAND",
	}
	if sortIndex >= 0 && sortIndex < len(topSortColumns) {
		sortField = topSortColumns[sortIndex]
	}
	want := labelByField[sortField]
	for i, header := range out {
		if header == want {
			out[i] = "[" + header + "]"
			return out
		}
	}
	return out
}

func normalizeTopSortField(sortField string) string {
	switch sortField {
	case "cpu":
		return "pcpu"
	case "vsz", "vsize":
		return "vms"
	case "command", "args":
		return "cmd"
	default:
		return sortField
	}
}

func startTopInput() (<-chan topInputEvent, func(), error) {
	fd := int(os.Stdin.Fd())
	oldState, err := makeTopRaw(fd)
	if err != nil {
		return nil, nil, err
	}
	events := make(chan topInputEvent, 4)
	stop := make(chan struct{})
	go readTopInput(fd, events, stop)
	return events, func() {
		close(stop)
		_ = restoreTopTerminal(fd, oldState)
	}, nil
}

func waitTopEvent(delay time.Duration, input <-chan topInputEvent, sigCh <-chan os.Signal, sortField string, reverse bool) (string, bool, bool, bool, error) {
	timer := time.NewTimer(delay)
	if delay <= 0 {
		timer.Stop()
	}
	if delay <= 0 {
		return sortField, reverse, true, false, nil
	}
	defer timer.Stop()
	for {
		select {
		case event, ok := <-input:
			if !ok {
				return sortField, reverse, false, true, nil
			}
			if event.quit {
				return sortField, reverse, false, true, nil
			}
			nextField, nextReverse := sortField, reverse
			if event.toggleDir {
				nextReverse = !nextReverse
			}
			if event.sortDelta != 0 {
				nextField, nextReverse = advanceTopSort(nextField, nextReverse, event.sortDelta)
			}
			return nextField, nextReverse, true, false, nil
		case <-sigCh:
			return sortField, reverse, false, true, nil
		case <-timer.C:
			return sortField, reverse, true, false, nil
		}
	}
}

func advanceTopSort(sortField string, reverse bool, delta int) (string, bool) {
	idx := topSortColumnIndex(sortField)
	n := len(topSortColumns)
	if n == 0 {
		return sortField, reverse
	}
	idx = ((idx+delta)%n + n) % n
	return topSortColumns[idx], true
}

func readTopInput(fd int, events chan<- topInputEvent, stop <-chan struct{}) {
	defer close(events)
	buf := make([]byte, 1)
	for {
		select {
		case <-stop:
			return
		default:
		}
		n, err := syscall.Read(fd, buf)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				time.Sleep(25 * time.Millisecond)
				continue
			}
			return
		}
		if n == 0 {
			time.Sleep(25 * time.Millisecond)
			continue
		}
		switch buf[0] {
		case 'q', 'Q':
			events <- topInputEvent{quit: true}
			return
		case 27:
			seq := make([]byte, 2)
			read := 0
			for read < len(seq) {
				n, err := syscall.Read(fd, seq[read:])
				if err != nil {
					if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
						time.Sleep(10 * time.Millisecond)
						continue
					}
					return
				}
				if n == 0 {
					break
				}
				read += n
			}
			if read == 2 && seq[0] == '[' {
				switch seq[1] {
				case 'C':
					events <- topInputEvent{sortDelta: 1}
				case 'D':
					events <- topInputEvent{sortDelta: -1}
				case 'A', 'B':
					events <- topInputEvent{toggleDir: true}
				}
			}
		}
	}
}

func makeTopRaw(fd int) (*syscall.Termios, error) {
	state, err := topTermios(fd)
	if err != nil {
		return nil, err
	}
	raw := *state
	raw.Iflag &^= syscall.ICRNL | syscall.INLCR | syscall.IXON
	raw.Lflag &^= syscall.ECHO | syscall.ICANON
	raw.Cc[syscall.VMIN] = 0
	raw.Cc[syscall.VTIME] = 1
	if err := setTopTermios(fd, &raw); err != nil {
		return nil, err
	}
	_ = syscall.SetNonblock(fd, true)
	return state, nil
}

func restoreTopTerminal(fd int, state *syscall.Termios) error {
	_ = syscall.SetNonblock(fd, false)
	if state == nil {
		return nil
	}
	return setTopTermios(fd, state)
}

func topTermios(fd int) (*syscall.Termios, error) {
	state := &syscall.Termios{}
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(state)), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}
	return state, nil
}

func setTopTermios(fd int, state *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(state)), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
