package proc

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

type signalSpec struct {
	name string
	sig  syscall.Signal
}

// killSignal sends a signal to a pid; overridable in tests so batch-kill
// error handling (e.g. skipping EPERM/ESRCH) can be exercised without
// needing an actual unprivileged target process.
var killSignal = syscall.Kill

// supportedSignals covers the full set of Linux signals recognized by GNU
// kill -l: standard signals 1-31 (32/33 are unused/reserved and omitted,
// matching native kill -l), plus real-time signals 34-64 named relative to
// SIGRTMIN/SIGRTMAX using GNU's symmetric convention.
var supportedSignals = buildSupportedSignals()

func buildSupportedSignals() []signalSpec {
	specs := []signalSpec{
		{name: "HUP", sig: syscall.SIGHUP},
		{name: "INT", sig: syscall.SIGINT},
		{name: "QUIT", sig: syscall.SIGQUIT},
		{name: "ILL", sig: syscall.SIGILL},
		{name: "TRAP", sig: syscall.SIGTRAP},
		{name: "ABRT", sig: syscall.SIGABRT},
		{name: "BUS", sig: syscall.SIGBUS},
		{name: "FPE", sig: syscall.SIGFPE},
		{name: "KILL", sig: syscall.SIGKILL},
		{name: "USR1", sig: syscall.SIGUSR1},
		{name: "SEGV", sig: syscall.SIGSEGV},
		{name: "USR2", sig: syscall.SIGUSR2},
		{name: "PIPE", sig: syscall.SIGPIPE},
		{name: "ALRM", sig: syscall.SIGALRM},
		{name: "TERM", sig: syscall.SIGTERM},
		{name: "STKFLT", sig: syscall.SIGSTKFLT},
		{name: "CHLD", sig: syscall.SIGCHLD},
		{name: "CONT", sig: syscall.SIGCONT},
		{name: "STOP", sig: syscall.SIGSTOP},
		{name: "TSTP", sig: syscall.SIGTSTP},
		{name: "TTIN", sig: syscall.SIGTTIN},
		{name: "TTOU", sig: syscall.SIGTTOU},
		{name: "URG", sig: syscall.SIGURG},
		{name: "XCPU", sig: syscall.SIGXCPU},
		{name: "XFSZ", sig: syscall.SIGXFSZ},
		{name: "VTALRM", sig: syscall.SIGVTALRM},
		{name: "PROF", sig: syscall.SIGPROF},
		{name: "WINCH", sig: syscall.SIGWINCH},
		{name: "IO", sig: syscall.SIGIO},
		{name: "PWR", sig: syscall.SIGPWR},
		{name: "SYS", sig: syscall.SIGSYS},
	}
	// Real-time signals 34-64. GNU kill -l names the lower half relative to
	// RTMIN (34=RTMIN, 35=RTMIN+1, ... 49=RTMIN+15) and the upper half
	// relative to RTMAX (50=RTMAX-14, ... 63=RTMAX-1, 64=RTMAX).
	const rtMin, rtMax = 34, 64
	for n := rtMin; n <= rtMax; n++ {
		var name string
		switch {
		case n == rtMax:
			name = "RTMAX"
		case n-rtMin <= rtMax-n:
			if n == rtMin {
				name = "RTMIN"
			} else {
				name = fmt.Sprintf("RTMIN+%d", n-rtMin)
			}
		default:
			name = fmt.Sprintf("RTMAX-%d", rtMax-n)
		}
		specs = append(specs, signalSpec{name: name, sig: syscall.Signal(n)})
	}
	return specs
}

func KillCmd(args []string) error {
	signal := syscall.SIGTERM
	if len(args) > 0 && strings.HasPrefix(args[0], "-") && len(args[0]) > 1 && !strings.HasPrefix(args[0], "--") {
		name := args[0][1:]
		if name != "l" && name != "s" && name != "f" && name != "x" && name != "P" && name != "n" && name != "o" {
			sig, err := parseSignal(name)
			if err == nil {
				signal = sig.(syscall.Signal)
				args = args[1:]
			} else if name[0] >= '0' && name[0] <= '9' {
				return err
			}
		}
	}
	fsFlags := flag.NewFlagSet("kill", flag.ContinueOnError)
	list := fsFlags.Bool("l", false, "list signals")
	fsFlags.BoolVar(list, "list", false, "list signals")
	sigName := fsFlags.String("s", "", "signal")
	full := fsFlags.String("f", "", "match full command line")
	exact := fsFlags.String("x", "", "match exact command name")
	ppid := fsFlags.Int("P", -1, "match parent pid")
	newest := fsFlags.Bool("n", false, "newest match")
	oldest := fsFlags.Bool("o", false, "oldest match")
	dryRun := fsFlags.Bool("dry-run", false, "print matches only")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox kill [OPTION]... PID... | PATTERN")
		fmt.Fprintln(os.Stderr, "Send signals to processes by PID or by match criteria.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Signals:")
		fmt.Fprintln(os.Stderr, "  -l, --list        list signals")
		fmt.Fprintln(os.Stderr, "  -s SIGNAL         signal name or number")
		fmt.Fprintln(os.Stderr, "  -SIGNAL           short form signal selector")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Matching:")
		fmt.Fprintln(os.Stderr, "  -f PATTERN        match full command line")
		fmt.Fprintln(os.Stderr, "  -x PATTERN        match exact command name")
		fmt.Fprintln(os.Stderr, "  -P PPID           match parent PID")
		fmt.Fprintln(os.Stderr, "  -n                choose newest match")
		fmt.Fprintln(os.Stderr, "  -o                choose oldest match")
		fmt.Fprintln(os.Stderr, "  --dry-run         print matches only")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *list {
		if rest := fsFlags.Args(); len(rest) > 0 {
			if num, err := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(rest[0]), "SIG")); err == nil {
				name, ok := signalName(syscall.Signal(num))
				if !ok {
					return fmt.Errorf("unsupported signal %s", rest[0])
				}
				fmt.Println(name)
				return nil
			}
			sig, err := parseSignal(rest[0])
			if err != nil {
				return err
			}
			fmt.Println(int(sig.(syscall.Signal)))
			return nil
		}
		printSignalGrid(supportedSignals)
		return nil
	}
	if *sigName != "" {
		sig, err := parseSignal(*sigName)
		if err != nil {
			return err
		}
		signal = sig.(syscall.Signal)
	}
	rest := fsFlags.Args()
	if *full == "" && *exact == "" && *ppid < 0 {
		if len(rest) == 0 {
			return fmt.Errorf("missing pid")
		}
		for _, p := range rest {
			pid, err := strconv.Atoi(p)
			if err != nil {
				return err
			}
			if *dryRun {
				fmt.Println(pid)
				continue
			}
			if err := syscall.Kill(pid, signal); err != nil {
				return err
			}
		}
		return nil
	}
	pattern := *full
	mode := "full"
	if pattern == "" {
		pattern = *exact
		mode = "exact"
	}
	if pattern == "" && len(rest) > 0 {
		pattern = rest[0]
	}
	matches, err := findProcesses(pattern, mode, *ppid)
	if err != nil {
		return err
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].start < matches[j].start })
	if *newest && len(matches) > 1 {
		matches = matches[len(matches)-1:]
	}
	if *oldest && len(matches) > 1 {
		matches = matches[:1]
	}
	return signalMatches(matches, signal, *dryRun)
}

// signalMatches sends signal to each matched process for pattern-based kill
// (-f/-x/-P/-n/-o). It mirrors pkill's per-process fault tolerance: a
// process that exited between the /proc scan and the kill (ESRCH) or one we
// lack permission to signal (EPERM, e.g. a different UID) is skipped so the
// rest of the batch still gets signaled, instead of aborting the whole
// match set on the first failure.
func signalMatches(matches []procMatch, signal syscall.Signal, dryRun bool) error {
	for _, p := range matches {
		if dryRun {
			fmt.Printf("%d %s\n", p.pid, p.cmd)
			continue
		}
		if err := killSignal(p.pid, signal); err != nil {
			if err == syscall.ESRCH || err == syscall.EPERM {
				continue
			}
			return err
		}
	}
	return nil
}

type procMatch struct {
	pid   int
	ppid  int
	start uint64
	comm  string
	cmd   string
}

func findProcesses(pattern, mode string, ppid int) ([]procMatch, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var re *regexp.Regexp
	if pattern != "" && mode == "full" {
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
	}
	var out []procMatch
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		pm, err := readProcMatch(pid)
		if err != nil {
			continue
		}
		if ppid >= 0 && pm.ppid != ppid {
			continue
		}
		ok := pattern == ""
		if pattern != "" {
			if mode == "exact" {
				ok = pm.comm == pattern
			} else if re != nil {
				ok = re.MatchString(pm.cmd)
			} else {
				ok = strings.Contains(pm.cmd, pattern)
			}
		}
		if ok {
			out = append(out, pm)
		}
	}
	return out, nil
}

func readProcMatch(pid int) (procMatch, error) {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return procMatch{}, err
	}
	// /proc/PID/stat format: "pid (comm) state ppid ..."
	// comm may contain spaces; find the last ')' to split correctly.
	statStr := string(stat)
	closeIdx := strings.LastIndex(statStr, ")")
	ppid, start := 0, uint64(0)
	if closeIdx >= 0 {
		afterComm := strings.Fields(statStr[closeIdx+1:])
		// afterComm: [state, ppid, pgrp, session, tty, ...]
		// starttime is at offset 19 from state (afterComm[19])
		if len(afterComm) > 1 {
			ppid, _ = strconv.Atoi(afterComm[1])
		}
		if len(afterComm) > 19 {
			start, _ = strconv.ParseUint(afterComm[19], 10, 64)
		}
	}
	commBytes, _ := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	cmdBytes, _ := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	cmd := strings.ReplaceAll(strings.TrimRight(string(cmdBytes), "\x00"), "\x00", " ")
	if cmd == "" {
		cmd = strings.TrimSpace(string(commBytes))
	}
	return procMatch{pid: pid, ppid: ppid, start: start, comm: strings.TrimSpace(string(commBytes)), cmd: cmd}, nil
}

func signalName(sig syscall.Signal) (string, bool) {
	for _, spec := range supportedSignals {
		if spec.sig == sig {
			return spec.name, true
		}
	}
	return "", false
}

// printSignalGrid prints the signal table the way GNU kill -l does: a
// 5-column, tab-separated grid of " N) SIGNAME" entries, number right
// justified to 2 characters, with a possibly-short final row.
func printSignalGrid(specs []signalSpec) {
	const cols = 5
	for i, spec := range specs {
		fmt.Printf("%2d) SIG%s", int(spec.sig), spec.name)
		if (i+1)%cols == 0 {
			fmt.Println()
		} else {
			fmt.Print("\t")
		}
	}
	if len(specs)%cols != 0 {
		fmt.Println()
	}
}
