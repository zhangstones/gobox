package proc

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
		fmt.Fprintln(os.Stderr, "List open files for visible processes.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Filters:")
		fmt.Fprintln(os.Stderr, "  -p PID        filter by PID")
		fmt.Fprintln(os.Stderr, "  -c PREFIX     filter by command prefix")
		fmt.Fprintln(os.Stderr, "  -i            show network files")
		fmt.Fprintln(os.Stderr, "  -iTCP         show TCP network files only")
		fmt.Fprintln(os.Stderr, "  -iUDP         show UDP network files only")
		fmt.Fprintln(os.Stderr, "  -i :PORT      show network files bound to PORT")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Output:")
		fmt.Fprintln(os.Stderr, "  -n            do not resolve host names")
		fmt.Fprintln(os.Stderr, "  -P            do not resolve port names")
		fmt.Fprintln(os.Stderr, "  -t            print only PIDs")
		fmt.Fprintln(os.Stderr, "  -h, --help    show this help")
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
		printLsofTable(rows, protoFilter, portFilter)
		return nil
	}
	for _, r := range rows {
		if protoFilter != "" && strings.ToUpper(r.node) != protoFilter {
			continue
		}
		if portFilter != "" && !strings.Contains(r.name, ":"+portFilter) {
			continue
		}
		if !printed[r.pid] {
			fmt.Println(r.pid)
			printed[r.pid] = true
		}
	}
	return nil
}

type lsofRow struct {
	command string
	pid     int
	user    string
	fd      string
	typ     string
	device  string
	sizeOff string
	node    string
	name    string
}

func printLsofTable(rows []lsofRow, protoFilter, portFilter string) {
	filtered := make([]lsofRow, 0, len(rows))
	commandWidth := len("COMMAND")
	pidWidth := len("PID")
	userWidth := len("USER")
	fdWidth := len("FD")
	typeWidth := len("TYPE")
	deviceWidth := len("DEVICE")
	sizeOffWidth := len("SIZE/OFF")
	nodeWidth := len("NODE")
	for _, r := range rows {
		if protoFilter != "" && strings.ToUpper(r.node) != protoFilter {
			continue
		}
		if portFilter != "" && !strings.Contains(r.name, ":"+portFilter) {
			continue
		}
		filtered = append(filtered, r)
		if len(r.command) > commandWidth {
			commandWidth = len(r.command)
		}
		if l := len(strconv.Itoa(r.pid)); l > pidWidth {
			pidWidth = l
		}
		if len(r.user) > userWidth {
			userWidth = len(r.user)
		}
		if len(r.fd) > fdWidth {
			fdWidth = len(r.fd)
		}
		if len(r.typ) > typeWidth {
			typeWidth = len(r.typ)
		}
		if len(r.device) > deviceWidth {
			deviceWidth = len(r.device)
		}
		if len(r.sizeOff) > sizeOffWidth {
			sizeOffWidth = len(r.sizeOff)
		}
		if len(r.node) > nodeWidth {
			nodeWidth = len(r.node)
		}
	}
	fmt.Printf("%-*s %*s %-*s %-*s %-*s %*s %*s %*s %s\n",
		commandWidth, "COMMAND", pidWidth, "PID", userWidth, "USER", fdWidth, "FD",
		typeWidth, "TYPE", deviceWidth, "DEVICE", sizeOffWidth, "SIZE/OFF", nodeWidth, "NODE", "NAME")
	for _, r := range filtered {
		fmt.Printf("%-*s %*d %-*s %-*s %-*s %*s %*s %*s %s\n",
			commandWidth, r.command, pidWidth, r.pid, userWidth, r.user, fdWidth, r.fd,
			typeWidth, r.typ, deviceWidth, r.device, sizeOffWidth, r.sizeOff, nodeWidth, r.node, r.name)
	}
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
		user := ""
		if info, err := os.Stat(filepath.Join(lsofProcRoot, e.Name())); err == nil {
			if st, ok := info.Sys().(*syscall.Stat_t); ok {
				user = lookupUsername(int(st.Uid))
			}
		}
		fdDir := filepath.Join(lsofProcRoot, e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			fdPath := filepath.Join(fdDir, fd.Name())
			target, err := os.Readlink(fdPath)
			if err != nil {
				continue
			}
			inode := socketInode(target)
			// -i means "internet sockets" specifically: native lsof -i
			// never lists unix domain sockets, only resolved TCP/UDP ones.
			if netOnly && (inode == "" || !isInternetSocketDetail(sockets[inode])) {
				continue
			}
			row := lsofRow{command: comm, pid: pid, user: user, fd: fd.Name()}
			if inode != "" {
				fillLsofSocketColumns(&row, inode, sockets[inode])
			} else {
				fillLsofFileColumns(&row, fdPath, target)
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
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// fillLsofSocketColumns populates TYPE/DEVICE/SIZE-OFF/NODE/NAME for a
// socket fd. detail is the "PROTO local->remote" string collectProcNetSockets
// produces for TCP/UDP sockets, or "UNIX path" for unix domain sockets
// (empty for socket types gobox still can't resolve at all).
func fillLsofSocketColumns(row *lsofRow, inode, detail string) {
	row.sizeOff = "0t0"
	if detail == "" {
		row.typ = "sock"
		row.node = inode
		row.name = "socket:[" + inode + "]"
		return
	}
	parts := strings.SplitN(detail, " ", 2)
	proto := parts[0]
	rest := ""
	if len(parts) > 1 {
		rest = parts[1]
	}
	if proto == "UNIX" {
		row.typ = "unix"
		row.node = inode
		if rest == "" {
			// Matches native lsof's placeholder for unbound/anonymous unix
			// sockets, which have no filesystem path in /proc/net/unix.
			rest = "socket"
		}
		row.name = rest
		return
	}
	row.typ = "IPv4"
	if strings.Contains(rest, "[") {
		row.typ = "IPv6"
	}
	row.device = inode
	row.node = proto
	row.name = rest
}

// isInternetSocketDetail reports whether a collectProcNetSockets detail
// string describes a resolved TCP/UDP socket, as opposed to a unix domain
// socket ("UNIX ...") or an unresolved one (""). Used to keep -i scoped to
// internet sockets only, matching native lsof -i.
func isInternetSocketDetail(detail string) bool {
	return strings.HasPrefix(detail, "TCP") || strings.HasPrefix(detail, "UDP")
}

// fillLsofFileColumns populates TYPE/DEVICE/SIZE-OFF/NODE for a regular
// file/directory fd by stat-ing the fd symlink (which follows it to
// whatever is currently open), matching native lsof's REG/DIR + "MAJ,MIN"
// device + byte size + inode columns.
func fillLsofFileColumns(row *lsofRow, fdPath, target string) {
	row.name = target
	info, err := os.Stat(fdPath)
	if err != nil {
		return
	}
	isDevice := false
	switch {
	case info.IsDir():
		row.typ = "DIR"
	case info.Mode().IsRegular():
		row.typ = "REG"
	case info.Mode()&os.ModeCharDevice != 0:
		row.typ = "CHR"
		isDevice = true
	case info.Mode()&os.ModeNamedPipe != 0:
		row.typ = "FIFO"
	default:
		row.typ = "unk"
	}
	row.sizeOff = strconv.FormatInt(info.Size(), 10)
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		// For device special files, lsof's DEVICE column identifies the
		// device itself (st_rdev, e.g. "1,3" for /dev/null), not the
		// filesystem the directory entry lives on (st_dev).
		dev := st.Dev
		if isDevice {
			dev = st.Rdev
		}
		row.device = fmt.Sprintf("%d,%d", devMajor(dev), devMinor(dev))
		row.node = strconv.FormatUint(st.Ino, 10)
	}
}

// devMajor/devMinor decode a Linux dev_t the way glibc's gnu_dev_major/minor
// macros do, matching lsof's "MAJ,MIN" DEVICE column.
func devMajor(dev uint64) uint64 {
	return ((dev >> 8) & 0xfff) | ((dev >> 32) &^ 0xfff)
}

func devMinor(dev uint64) uint64 {
	return (dev & 0xff) | ((dev >> 12) &^ 0xff)
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
	readProcNetUnix(out, "/proc/net/unix")
	return out
}

// readProcNetUnix adds unix domain socket inode->detail entries ("UNIX
// path") from /proc/net/unix, so lsof can resolve NAME for unix socket fds
// instead of falling back to a bare "socket:[inode]" placeholder. Columns
// per `man 5 proc`: Num RefCount Protocol Flags Type St Inode Path.
func readProcNetUnix(out map[string]string, path string) {
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
		if len(fields) < 7 {
			continue
		}
		inode := fields[6]
		name := ""
		if len(fields) >= 8 {
			name = strings.Join(fields[7:], " ")
		}
		out[inode] = strings.TrimSpace("UNIX " + name)
	}
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
	ih := parts[0]
	if len(ih) == 8 {
		var b [4]byte
		for i := 0; i < 4; i++ {
			v, err := strconv.ParseUint(ih[i*2:i*2+2], 16, 8)
			if err != nil {
				return value
			}
			b[3-i] = byte(v)
		}
		return fmt.Sprintf("%d.%d.%d.%d:%d", b[0], b[1], b[2], b[3], port)
	}
	if len(ih) == 32 {
		raw, err := hex.DecodeString(ih)
		if err != nil || len(raw) != 16 {
			return value
		}
		for i := 0; i < 16; i += 4 {
			raw[i], raw[i+1], raw[i+2], raw[i+3] = raw[i+3], raw[i+2], raw[i+1], raw[i]
		}
		return fmt.Sprintf("[%s]:%d", net.IP(raw).String(), port)
	}
	return fmt.Sprintf("%s:%d", ih, port)
}
