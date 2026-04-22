package proc

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	lsofProcRoot       = "/proc"
	collectSocketsLsof = collectProcNetSockets
)

func LsofCmd(args []string) error {
	fsFlags := flag.NewFlagSet("lsof", flag.ContinueOnError)
	pidFilter := fsFlags.Int("p", 0, "pid")
	cmdFilter := fsFlags.String("c", "", "command prefix")
	netOnly := fsFlags.Bool("i", false, "network files")
	noHost := fsFlags.Bool("n", false, "no host lookup")
	noPort := fsFlags.Bool("P", false, "no port lookup")
	pidsOnly := fsFlags.Bool("t", false, "pids only")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox lsof [OPTION]... [FILE]...")
		fsFlags.PrintDefaults()
	}
	filterArgs := normalizeLsofArgs(args)
	protoFilter, portFilter := parseLsofIFilters(args)
	if err := fsFlags.Parse(filterArgs); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	_ = noHost
	_ = noPort
	files := fsFlags.Args()
	rows, err := collectLsofRows(*pidFilter, *cmdFilter, *netOnly || protoFilter != "" || portFilter != "", files)
	if err != nil {
		return err
	}
	printed := map[int]bool{}
	if !*pidsOnly {
		fmt.Printf("%-10s %6s %-4s %s\n", "COMMAND", "PID", "FD", "NAME")
	}
	for _, r := range rows {
		if protoFilter != "" && !strings.Contains(strings.ToUpper(r.name), protoFilter) {
			continue
		}
		if portFilter != "" && !strings.Contains(r.name, ":"+portFilter) {
			continue
		}
		if *pidsOnly {
			if !printed[r.pid] {
				fmt.Println(r.pid)
				printed[r.pid] = true
			}
			continue
		}
		fmt.Printf("%-10s %6d %-4s %s\n", r.command, r.pid, r.fd, r.name)
	}
	return nil
}

type lsofRow struct {
	command string
	pid     int
	fd      string
	name    string
}

func normalizeLsofArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-i") && arg != "-i" {
			continue
		}
		if arg == "-i" && i+1 < len(args) && strings.HasPrefix(args[i+1], ":") {
			out = append(out, arg)
			i++
			continue
		}
		out = append(out, arg)
	}
	return out
}

func parseLsofIFilters(args []string) (string, string) {
	var proto, port string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-iTCP") {
			proto = "TCP"
		} else if strings.HasPrefix(arg, "-iUDP") {
			proto = "UDP"
		} else if strings.HasPrefix(arg, "-i") && strings.Contains(arg, ":") {
			port = strings.TrimPrefix(strings.SplitN(arg, ":", 2)[1], ":")
		} else if arg == "-i" && i+1 < len(args) && strings.HasPrefix(args[i+1], ":") {
			port = strings.TrimPrefix(args[i+1], ":")
			i++
		}
	}
	return proto, port
}

func collectLsofRows(pidFilter int, cmdFilter string, netOnly bool, files []string) ([]lsofRow, error) {
	entries, err := os.ReadDir(lsofProcRoot)
	if err != nil {
		return nil, err
	}
	sockets := collectSocketsLsof()
	fileTargets := map[string]bool{}
	for _, f := range files {
		if abs, err := filepath.Abs(f); err == nil {
			fileTargets[abs] = true
		}
	}
	var rows []lsofRow
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if pidFilter != 0 && pid != pidFilter {
			continue
		}
		commBytes, _ := os.ReadFile(filepath.Join(lsofProcRoot, e.Name(), "comm"))
		comm := strings.TrimSpace(string(commBytes))
		if cmdFilter != "" && !strings.HasPrefix(comm, cmdFilter) {
			continue
		}
		fdDir := filepath.Join(lsofProcRoot, e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if netOnly && !strings.HasPrefix(target, "socket:") {
				continue
			}
			if detail, ok := sockets[socketInode(target)]; ok {
				target = detail
			}
			if len(fileTargets) > 0 {
				abs := target
				if !filepath.IsAbs(abs) {
					abs, _ = filepath.Abs(abs)
				}
				if !fileTargets[abs] {
					continue
				}
			}
			rows = append(rows, lsofRow{command: comm, pid: pid, fd: fd.Name(), name: target})
		}
	}
	return rows, nil
}

func socketInode(target string) string {
	if !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
}

func collectProcNetSockets() map[string]string {
	out := map[string]string{}
	readProcNet(out, "/proc/net/tcp", "TCP")
	readProcNet(out, "/proc/net/tcp6", "TCP")
	readProcNet(out, "/proc/net/udp", "UDP")
	readProcNet(out, "/proc/net/udp6", "UDP")
	return out
}

func readProcNet(out map[string]string, path, proto string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		inode := fields[9]
		out[inode] = fmt.Sprintf("%s %s->%s", proto, procNetEndpoint(fields[1]), procNetEndpoint(fields[2]))
	}
}

func procNetEndpoint(value string) string {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return value
	}
	port, err := strconv.ParseInt(parts[1], 16, 32)
	if err != nil {
		return value
	}
	return fmt.Sprintf("%s:%d", parts[0], port)
}
