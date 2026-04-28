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

var supportedSignals = []struct {
	name string
	sig  syscall.Signal
}{
	{name: "HUP", sig: syscall.SIGHUP},
	{name: "INT", sig: syscall.SIGINT},
	{name: "QUIT", sig: syscall.SIGQUIT},
	{name: "KILL", sig: syscall.SIGKILL},
	{name: "TERM", sig: syscall.SIGTERM},
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
		names := make([]string, 0, len(supportedSignals))
		for _, spec := range supportedSignals {
			names = append(names, spec.name)
		}
		fmt.Println(strings.Join(names, " "))
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
	for _, p := range matches {
		if *dryRun {
			fmt.Printf("%d %s\n", p.pid, p.cmd)
			continue
		}
		if err := syscall.Kill(p.pid, signal); err != nil {
			if err == syscall.ESRCH {
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
		re, _ = regexp.Compile(pattern)
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
	fields := strings.Fields(string(stat))
	ppid, start := 0, uint64(0)
	if len(fields) > 21 {
		ppid, _ = strconv.Atoi(fields[3])
		start, _ = strconv.ParseUint(fields[21], 10, 64)
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
