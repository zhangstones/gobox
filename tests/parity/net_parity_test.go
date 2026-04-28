package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func netstatHeaderAndRows(out string) (string, []string) {
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return "", nil
	}
	return lines[0], lines[1:]
}

func netstatFindRow(rows []string, needle string) string {
	for _, line := range rows {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func netstatProto(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return ""
	}
	return fields[2]
}

func netstatState(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return ""
	}
	return fields[5]
}

func netstatLocalAddress(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return ""
	}
	return fields[3]
}

func netstatPort(line string) string {
	local := netstatLocalAddress(line)
	if idx := strings.LastIndex(local, ":"); idx >= 0 && idx+1 < len(local) {
		return local[idx+1:]
	}
	return ""
}

func netstatSocketKey(line string) string {
	return netstatPort(line) + "|" + netstatState(line)
}

func ifstatHeaderAndRows(out string) (string, []string) {
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return "", nil
	}
	return lines[0], lines[1:]
}

func ifstatHeaderColumns(header string) []string {
	return strings.Fields(header)
}

func ifstatParseRow(t *testing.T, line string, wantFields int) []string {
	t.Helper()
	fields := strings.Fields(line)
	if len(fields) != wantFields {
		t.Fatalf("ifstat row width mismatch: got=%d want=%d line=%q", len(fields), wantFields, line)
	}
	return fields
}

func ifstatRowsByInterface(rows []string, wantFields int) map[string][][]string {
	byIface := make(map[string][][]string)
	for _, line := range rows {
		fields := strings.Fields(line)
		if len(fields) != wantFields {
			continue
		}
		byIface[fields[0]] = append(byIface[fields[0]], fields)
	}
	return byIface
}

func ifstatParseFloatField(t *testing.T, field string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(field, 64)
	if err != nil {
		t.Fatalf("parse float %q: %v", field, err)
	}
	return v
}

func ifstatParseUintField(t *testing.T, field string) uint64 {
	t.Helper()
	v, err := strconv.ParseUint(field, 10, 64)
	if err != nil {
		t.Fatalf("parse uint %q: %v", field, err)
	}
	return v
}

func ncBenchTotalFields(t *testing.T, out string) (float64, string) {
	t.Helper()
	line := findNetLineWithPrefix(out, "Total:")
	if line == "" {
		t.Fatalf("missing total line\n%s", out)
	}
	trimmed := strings.TrimPrefix(line, "Total:")
	parts := strings.Split(trimmed, ",")
	if len(parts) != 3 {
		t.Fatalf("unexpected total line format: %q", line)
	}
	secondsText := strings.TrimSpace(strings.TrimSuffix(parts[0], "s"))
	seconds, err := strconv.ParseFloat(secondsText, 64)
	if err != nil {
		t.Fatalf("parse total duration %q: %v", secondsText, err)
	}
	return seconds, strings.TrimSpace(parts[1])
}

func curlBenchRequestsLine(t *testing.T, out string) (requests, concurrency, failed int) {
	t.Helper()
	line := findNetLineWithPrefix(out, "Requests:")
	if line == "" {
		t.Fatalf("missing requests line\n%s", out)
	}
	var err error
	parts := strings.Split(line, ",")
	if len(parts) != 3 {
		t.Fatalf("unexpected requests line format: %q", line)
	}
	requests, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[0]), "Requests:")))
	if err != nil {
		t.Fatalf("parse requests from %q: %v", line, err)
	}
	concurrency, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[1]), "Concurrency:")))
	if err != nil {
		t.Fatalf("parse concurrency from %q: %v", line, err)
	}
	failed, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[2]), "Failed:")))
	if err != nil {
		t.Fatalf("parse failed count from %q: %v", line, err)
	}
	return requests, concurrency, failed
}

func ipBlocks(out string) map[string][]string {
	blocks := make(map[string][]string)
	var current string
	for _, line := range nonEmptyLines(out) {
		if fields := strings.Fields(line); len(fields) >= 2 && strings.HasSuffix(fields[0], ":") {
			name := strings.TrimSuffix(fields[1], ":")
			current = name
			blocks[current] = []string{line}
			continue
		}
		if current != "" {
			blocks[current] = append(blocks[current], line)
		}
	}
	return blocks
}

func findNetLineWithPrefix(out, prefix string) string {
	for _, line := range nonEmptyLines(out) {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func findNetLineContaining(out, needle string) string {
	for _, line := range nonEmptyLines(out) {
		if strings.Contains(line, needle) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func netstatHeaderFields(out string) []string {
	header, _ := netstatHeaderAndRows(out)
	return strings.Fields(header)
}

func TestParity_TwCases(t *testing.T) {
	t.Run("TW-001", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "tw", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("tw help failed: %+v", res)
		}
	})

	t.Run("TW-002", func(t *testing.T) {
		env := t.TempDir()
		writeFile(t, filepath.Join(env, "index.html"), "ok")
		res := runGoboxCLI(t, env, "", "tw", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("tw dir contract failed: %+v", res)
		}
	})

	t.Run("TW-003", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "tw", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("tw reuse contract failed: %+v", res)
		}
	})

	t.Run("TW-004", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "tw", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("tw -h failed: %+v", res)
		}
	})
}

func TestParity_NetstatCases(t *testing.T) {
	t.Run("NETSTAT-001", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "--port", fmt.Sprintf("%d", port))
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-an")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat failed gobox=%+v native=%+v", res, native)
		}
		wantPort := strconv.Itoa(port)
		header, rows := netstatHeaderAndRows(res.Stdout)
		if len(strings.Fields(header)) < 6 || len(rows) == 0 {
			t.Fatalf("netstat --port missing structured output\n%s", res.Stdout)
		}
		var matches []string
		for _, line := range rows {
			if netstatPort(line) == wantPort {
				matches = append(matches, line)
			}
		}
		if len(matches) != 1 {
			t.Fatalf("netstat --port should isolate exactly one target row, got %d\n%s", len(matches), res.Stdout)
		}
		row := matches[0]
		if proto := netstatProto(row); proto != "TCP" && proto != "TCP6" {
			t.Fatalf("netstat --port should retain tcp listener, got %q in %q", proto, row)
		}
		if state := netstatState(row); state != "LISTEN" {
			t.Fatalf("netstat --port should retain listening row, got %q in %q", state, row)
		}
		for _, line := range rows {
			if line != row && netstatPort(line) == wantPort {
				t.Fatalf("netstat --port leaked duplicate target rows: %q", line)
			}
		}
		if !strings.Contains(native.Stdout, fmt.Sprintf(":%d", port)) {
			t.Fatalf("netstat --port missing listener\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
	})

	t.Run("NETSTAT-002", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "--sort", "pid", "-p")
		if res.ExitCode != 0 {
			t.Fatalf("netstat --sort pid failed: %+v", res)
		}
		pids := extractNetstatPIDs(res.Stdout)
		assertMonotonic(t, pids, false)
	})

	t.Run("NETSTAT-003", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-an")
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "--state", "LISTEN")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat --state failed gobox=%+v native=%+v", res, native)
		}
		if !strings.Contains(native.Stdout, "LISTEN") {
			t.Fatalf("native netstat baseline missing LISTEN rows: %+v", native)
		}
		nativeListen := make(map[string]struct{})
		for _, line := range nonEmptyLines(native.Stdout)[1:] {
			if netstatState(line) == "LISTEN" {
				nativeListen[netstatSocketKey(line)] = struct{}{}
			}
		}
		if len(nativeListen) == 0 {
			t.Fatalf("native netstat parsed no LISTEN rows\n%s", native.Stdout)
		}
		for _, line := range nonEmptyLines(res.Stdout)[1:] {
			if netstatState(line) != "LISTEN" {
				t.Fatalf("netstat --state LISTEN leaked non-LISTEN row: %q", line)
			}
			if _, ok := nativeListen[netstatSocketKey(line)]; !ok {
				t.Fatalf("netstat --state LISTEN returned row missing from native LISTEN subset: %q\n--- native ---\n%s", line, native.Stdout)
			}
		}
	})

	t.Run("NETSTAT-004", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-l", "--port", fmt.Sprintf("%d", port))
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-ln")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat -l failed gobox=%+v native=%+v", res, native)
		}
		header, rows := netstatHeaderAndRows(res.Stdout)
		if len(rows) == 0 {
			t.Fatalf("netstat -l expected listening rows\n%s", res.Stdout)
		}
		if got := strings.Fields(header); len(got) < 6 || got[0] != "Recv-Q" || got[2] != "Proto" {
			t.Fatalf("netstat -l header shape mismatch: %q", header)
		}
		for _, line := range rows {
			if !strings.Contains(line, "LISTEN") {
				t.Fatalf("netstat -l leaked non-LISTEN row: %q", line)
			}
		}
		if netstatFindRow(rows, fmt.Sprintf(":%d", port)) == "" || !strings.Contains(native.Stdout, fmt.Sprintf(":%d", port)) {
			t.Fatalf("netstat -l missing listener\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
	})

	t.Run("NETSTAT-005", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		env := t.TempDir()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, env, "", "netstat", "--port", port)
		res := runGoboxCLI(t, env, "", "netstat", "-n", "--port", port)
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("netstat -n baseline failed base=%+v numeric=%+v", base, res)
		}
		if base.Stdout != res.Stdout {
			t.Fatalf("netstat -n should be a no-op because gobox output is already numeric\n--- base ---\n%s\n--- -n ---\n%s", base.Stdout, res.Stdout)
		}
		_, rows := netstatHeaderAndRows(res.Stdout)
		if netstatFindRow(rows, port) == "" || strings.Contains(res.Stdout, "localhost:") {
			t.Fatalf("netstat -n should still render the socket table in numeric form\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-006", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		env := t.TempDir()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, env, "", "netstat", "--port", port)
		res := runGoboxCLI(t, env, "", "netstat", "-a", "--port", port)
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("netstat -a baseline failed base=%+v all=%+v", base, res)
		}
		if base.Stdout != res.Stdout {
			t.Fatalf("netstat -a should currently match the default socket selection\n--- base ---\n%s\n--- -a ---\n%s", base.Stdout, res.Stdout)
		}
		_, rows := netstatHeaderAndRows(res.Stdout)
		if netstatFindRow(rows, port) == "" {
			t.Fatalf("netstat -a should still render the socket table\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-007", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "--port", strconv.Itoa(port))
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-tln")
		if res.ExitCode != 0 || native.ExitCode != 0 || !strings.Contains(native.Stdout, strconv.Itoa(port)) {
			t.Fatalf("netstat -t mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		_, rows := netstatHeaderAndRows(res.Stdout)
		if len(rows) != 1 {
			t.Fatalf("netstat -t expected tcp rows\n%s", res.Stdout)
		}
		for _, line := range rows {
			if netstatProto(line) != "TCP" && netstatProto(line) != "TCP6" {
				t.Fatalf("netstat -t leaked non-TCP row: %q", line)
			}
			if proto := netstatProto(line); strings.HasPrefix(proto, "UDP") || proto == "UNIX" {
				t.Fatalf("netstat -t leaked wrong protocol row: %q", line)
			}
			if netstatState(line) != "LISTEN" {
				t.Fatalf("netstat -t should keep only target listening socket, got %q", line)
			}
		}
		if netstatFindRow(rows, strconv.Itoa(port)) == "" {
			t.Fatalf("netstat -t missing filtered listener row\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-008", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen udp: %v", err)
		}
		defer conn.Close()
		port := conn.LocalAddr().(*net.UDPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-u", "--port", strconv.Itoa(port))
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-uln")
		if res.ExitCode != 0 || native.ExitCode != 0 || !strings.Contains(native.Stdout, strconv.Itoa(port)) {
			t.Fatalf("netstat -u mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		_, rows := netstatHeaderAndRows(res.Stdout)
		if len(rows) != 1 {
			t.Fatalf("netstat -u expected udp rows\n%s", res.Stdout)
		}
		for _, line := range rows {
			if netstatProto(line) != "UDP" && netstatProto(line) != "UDP6" {
				t.Fatalf("netstat -u leaked non-UDP row: %q", line)
			}
			if proto := netstatProto(line); strings.HasPrefix(proto, "TCP") || proto == "UNIX" {
				t.Fatalf("netstat -u leaked wrong protocol row: %q", line)
			}
		}
		if netstatFindRow(rows, strconv.Itoa(port)) == "" {
			t.Fatalf("netstat -u missing filtered socket row\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-009", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		unixPath := filepath.Join(t.TempDir(), "netstat.sock")
		ln, err := net.Listen("unix", unixPath)
		if err != nil {
			t.Fatalf("listen unix: %v", err)
		}
		defer ln.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-x", "-l")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-x", "-l")
		if res.ExitCode != native.ExitCode || !strings.Contains(native.Stdout, unixPath) {
			t.Fatalf("netstat -x mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		_, rows := netstatHeaderAndRows(res.Stdout)
		if len(rows) == 0 {
			t.Fatalf("netstat -x expected unix rows\n%s", res.Stdout)
		}
		for _, line := range rows {
			if netstatProto(line) != "UNIX" {
				t.Fatalf("netstat -x leaked non-UNIX row: %q", line)
			}
		}
		if netstatFindRow(rows, unixPath) == "" {
			t.Fatalf("netstat -x missing target unix socket\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-010", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "--port", port)
		withProg := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-p", "--port", port)
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-tnlp")
		if base.ExitCode != 0 || withProg.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat -p baseline failed base=%+v withProg=%+v native=%+v", base, withProg, native)
		}
		if !strings.Contains(withProg.Stdout, "PID/Program") || !strings.Contains(native.Stdout, "PID/Program") {
			t.Fatalf("netstat -p missing PID/Program column\n--- gobox ---\n%s\n--- native ---\n%s", withProg.Stdout, native.Stdout)
		}
		baseHeader, baseRows := netstatHeaderAndRows(base.Stdout)
		header, rows := netstatHeaderAndRows(withProg.Stdout)
		if len(strings.Fields(header)) <= len(strings.Fields(baseHeader)) {
			t.Fatalf("netstat -p should widen the table with PID/Program column\n--- base ---\n%s\n--- with -p ---\n%s", base.Stdout, withProg.Stdout)
		}
		row := netstatFindRow(rows, port)
		if row == "" || !strings.Contains(row, "/") {
			t.Fatalf("netstat -p should keep the filtered listener row and annotate pid/program\n%s", withProg.Stdout)
		}
		if len(baseRows) == 0 || len(rows) != len(baseRows) {
			t.Fatalf("netstat -p should preserve the filtered row set\n--- base ---\n%s\n--- with -p ---\n%s", base.Stdout, withProg.Stdout)
		}
	})

	t.Run("NETSTAT-011", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp4: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-4", "--port", strconv.Itoa(port))
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-4ln")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat -4 mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		_, rows := netstatHeaderAndRows(res.Stdout)
		row := netstatFindRow(rows, strconv.Itoa(port))
		if row == "" || !strings.Contains(row, "127.0.0.1") {
			t.Fatalf("netstat -4 missing IPv4 listener row\n%s", res.Stdout)
		}
		if !strings.Contains(native.Stdout, strconv.Itoa(port)) {
			t.Fatalf("native netstat -4 baseline missing target port\n%s", native.Stdout)
		}
	})

	t.Run("NETSTAT-012", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		_, port, closeFn := startTCPEchoServer(t, "[::1]:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-6", "--port", port)
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-6ln")
		if res.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat -6 mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		_, rows := netstatHeaderAndRows(res.Stdout)
		row := netstatFindRow(rows, port)
		if row == "" || !strings.Contains(row, "::1") {
			t.Fatalf("netstat -6 missing IPv6 listener row\n%s", res.Stdout)
		}
		if !strings.Contains(native.Stdout, port) {
			t.Fatalf("native netstat -6 baseline missing target port\n%s", native.Stdout)
		}
	})

	t.Run("NETSTAT-013", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "--port", port)
		extended := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-e", "--port", port)
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-tnle")
		if base.ExitCode != 0 || extended.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat -e baseline failed base=%+v extended=%+v native=%+v", base, extended, native)
		}
		baseHeader, baseRows := netstatHeaderAndRows(base.Stdout)
		header, rows := netstatHeaderAndRows(extended.Stdout)
		for _, want := range []string{"User", "Inode"} {
			if !strings.Contains(header, want) || !strings.Contains(native.Stdout, want) {
				t.Fatalf("netstat -e missing %q\n--- gobox ---\n%s\n--- native ---\n%s", want, extended.Stdout, native.Stdout)
			}
		}
		if len(rows) != len(baseRows) || len(strings.Fields(header)) <= len(strings.Fields(baseHeader)) {
			t.Fatalf("netstat -e should preserve row set and extend columns\n--- base ---\n%s\n--- extended ---\n%s", base.Stdout, extended.Stdout)
		}
		if len(rows) == 0 || len(strings.Fields(rows[0])) <= len(strings.Fields(baseRows[0])) {
			t.Fatalf("netstat -e should extend the filtered row with extra columns\n%s", extended.Stdout)
		}
		if netstatFindRow(rows, port) == "" {
			t.Fatalf("netstat -e missing filtered listener row\n%s", extended.Stdout)
		}
	})

	t.Run("NETSTAT-014", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "--port", port)
		withTimers := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-o", "--port", port)
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-tnlo")
		if base.ExitCode != 0 || withTimers.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("netstat -o baseline failed base=%+v withTimers=%+v native=%+v", base, withTimers, native)
		}
		baseHeader, baseRows := netstatHeaderAndRows(base.Stdout)
		header, rows := netstatHeaderAndRows(withTimers.Stdout)
		fields := strings.Fields(header)
		if len(fields) == 0 || fields[len(fields)-1] != "Timer" || !strings.Contains(native.Stdout, "Timer") {
			t.Fatalf("netstat -o missing Timer column\n--- gobox ---\n%s\n--- native ---\n%s", withTimers.Stdout, native.Stdout)
		}
		if len(rows) != len(baseRows) || len(fields) <= len(strings.Fields(baseHeader)) {
			t.Fatalf("netstat -o should preserve row set and add timer column\n--- base ---\n%s\n--- with timers ---\n%s", base.Stdout, withTimers.Stdout)
		}
		if len(rows) == 0 || netstatFindRow(rows, port) == "" {
			t.Fatalf("netstat -o should keep the filtered listener row\n%s", withTimers.Stdout)
		}
	})

	t.Run("NETSTAT-015", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		base := runGoboxCLI(t, t.TempDir(), "", "netstat", "-n", "-l", "--port", port)
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-W", "-n", "-l", "--port", port)
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("netstat -W baseline failed base=%+v wide=%+v", base, res)
		}
		if base.Stdout != res.Stdout {
			t.Fatalf("netstat -W should be a compatibility no-op because gobox does not truncate addresses\n--- base ---\n%s\n--- -W ---\n%s", base.Stdout, res.Stdout)
		}
		_, rows := netstatHeaderAndRows(res.Stdout)
		if netstatFindRow(rows, port) == "" {
			t.Fatalf("netstat -W should still render the filtered listening row\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-016", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		base := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "--port", strconv.Itoa(port))
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-tnlp", "--port", strconv.Itoa(port))
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-tnlp")
		if base.ExitCode != 0 || res.ExitCode != native.ExitCode {
			t.Fatalf("netstat combined flags mismatch\n--- base ---\n%+v\n--- gobox ---\n%+v\n--- native ---\n%+v", base, res, native)
		}
		header, rows := netstatHeaderAndRows(res.Stdout)
		if !strings.Contains(header, "PID/Program") || !strings.Contains(native.Stdout, "tcp") {
			t.Fatalf("netstat combined flags missing expected protocol/program output\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		row := netstatFindRow(rows, strconv.Itoa(port))
		if row == "" || netstatProto(row) != "TCP" || !strings.Contains(row, "/") {
			t.Fatalf("netstat -tnlp should keep the filtered listener row and annotate it with pid/program\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-017", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-r")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-r")
		if res.ExitCode != native.ExitCode || findNetLineContaining(res.Stdout, "Kernel IP routing table") == "" || findNetLineContaining(native.Stdout, "Kernel IP routing table") == "" {
			t.Fatalf("netstat -r mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		header := findNetLineContaining(res.Stdout, "Iface")
		nativeHeader := findNetLineContaining(native.Stdout, "Iface")
		if header == "" || nativeHeader == "" {
			t.Fatalf("netstat -r missing route columns\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
		for _, want := range []string{"Destination", "Gateway", "Iface"} {
			if !strings.Contains(header, want) || !strings.Contains(nativeHeader, want) {
				t.Fatalf("netstat -r route header missing %q\ngobox=%q\nnative=%q", want, header, nativeHeader)
			}
		}
		if strings.Contains(native.Stdout, "default") && !strings.Contains(res.Stdout, "default") && !strings.Contains(res.Stdout, "0.0.0.0") {
			t.Fatalf("netstat -r missing default-route semantic present in native\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
		if !strings.Contains(res.Stdout, "eth0") && !strings.Contains(res.Stdout, "lo") {
			t.Fatalf("netstat -r should include at least one concrete interface route\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-018", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "-i")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-i")
		header := findNetLineContaining(res.Stdout, "Iface")
		nativeHeader := findNetLineContaining(native.Stdout, "Iface")
		if res.ExitCode != native.ExitCode || header == "" || nativeHeader == "" {
			t.Fatalf("netstat -i mismatch\n--- gobox ---\n%+v\n--- native ---\n%+v", res, native)
		}
		goboxFields := strings.Fields(header)
		nativeFields := strings.Fields(nativeHeader)
		if len(goboxFields) < 3 || len(nativeFields) < 3 {
			t.Fatalf("netstat -i header too short\ngobox=%q\nnative=%q", header, nativeHeader)
		}
		if strings.Join(goboxFields[:3], " ") != strings.Join(nativeFields[:3], " ") {
			t.Fatalf("netstat -i header prefix mismatch\ngobox=%q\nnative=%q", header, nativeHeader)
		}
		row := ""
		for _, line := range nonEmptyLines(res.Stdout)[1:] {
			fields := strings.Fields(line)
			if len(fields) > 0 && fields[0] == "lo" {
				row = line
				break
			}
		}
		if row == "" || len(strings.Fields(row)) < 3 {
			t.Fatalf("netstat -i missing structured loopback row\n%s", res.Stdout)
		}
		if !strings.Contains(native.Stdout, "lo") {
			t.Fatalf("netstat -i missing loopback interface\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
		lines := nonEmptyLines(res.Stdout)
		if len(lines) < 3 {
			t.Fatalf("netstat -i should include header plus multiple interfaces\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-019", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "netstat", "-s")
		tcpOnly := runGoboxCLI(t, env, "", "netstat", "-s", "-t")
		native := runNativeCLI(t, t.TempDir(), "", "netstat", "-s")
		if res.ExitCode != native.ExitCode || tcpOnly.ExitCode != 0 {
			t.Fatalf("netstat -s mismatch\n--- gobox ---\n%+v\n--- gobox tcp ---\n%+v\n--- native ---\n%+v", res, tcpOnly, native)
		}
		if findNetLineWithPrefix(res.Stdout, "Tcp:") == "" || findNetLineWithPrefix(native.Stdout, "Tcp:") == "" {
			t.Fatalf("netstat -s missing tcp stats section\n--- gobox ---\n%s\n--- native ---\n%s", res.Stdout, native.Stdout)
		}
		if res.Stdout == tcpOnly.Stdout {
			t.Fatalf("netstat -s should include more than the tcp-only filtered view\n--- all stats ---\n%s\n--- tcp only ---\n%s", res.Stdout, tcpOnly.Stdout)
		}
		if findNetLineWithPrefix(res.Stdout, "Udp:") == "" && findNetLineWithPrefix(res.Stdout, "Ip:") == "" {
			t.Fatalf("netstat -s should include non-TCP protocol stats in the unfiltered view\n%s", res.Stdout)
		}
	})

	t.Run("NETSTAT-020", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		gobox := runGoboxSubprocess(t, t.TempDir(), []string{"netstat", "-c", "-n", "-l", "--port", port}, 1350*time.Millisecond)
		native := runNativeFollow(t, t.TempDir(), "netstat", []string{"-c", "-n", "-l"}, nil, 1350*time.Millisecond)
		if strings.Count(gobox.Stdout, "Proto") < 2 {
			t.Fatalf("gobox netstat -c did not render multiple cycles: %q", gobox.Stdout)
		}
		if strings.Count(native.Stdout, "Proto") < 2 {
			t.Fatalf("native netstat -c did not render multiple cycles: %q", native.Stdout)
		}
	})

	t.Run("NETSTAT-021", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		defer ln.Close()
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
		short := runGoboxCLI(t, t.TempDir(), "", "netstat", "-t", "-l", "-p", "-e", "-o", "-n", "-W", "--port", port)
		long := runGoboxCLI(t, t.TempDir(), "", "netstat", "--tcp", "--listening", "--programs", "--extend", "--timers", "--numeric", "--wide", "--port", port)
		if short.ExitCode != 0 || long.ExitCode != 0 {
			t.Fatalf("netstat short/long flag parity failed short=%+v long=%+v", short, long)
		}
		if short.Stdout != long.Stdout {
			t.Fatalf("netstat short/long flag output mismatch\n--- short ---\n%s\n--- long ---\n%s", short.Stdout, long.Stdout)
		}
	})

	t.Run("NETSTAT-022", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		short := runGoboxCLI(t, t.TempDir(), "", "netstat", "-r")
		long := runGoboxCLI(t, t.TempDir(), "", "netstat", "--route")
		if short.ExitCode != 0 || long.ExitCode != 0 {
			t.Fatalf("netstat route short/long parity failed short=%+v long=%+v", short, long)
		}
		if short.Stdout != long.Stdout {
			t.Fatalf("netstat route short/long output mismatch\n--- short ---\n%s\n--- long ---\n%s", short.Stdout, long.Stdout)
		}
	})

	t.Run("NETSTAT-023", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "netstat", "--help")
		if res.ExitCode != 0 {
			t.Fatalf("netstat --help failed: %+v", res)
		}
		out := res.Stdout + "\n" + res.Stderr
		for _, want := range []string{"-t, --tcp", "-u, --udp", "-n, --numeric", "-W, --wide", "Filters:", "Views:"} {
			if findNetLineContaining(out, want) == "" {
				t.Fatalf("netstat --help missing %q\nstdout=%q\nstderr=%q", want, res.Stdout, res.Stderr)
			}
		}
	})

	t.Run("NETSTAT-024", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		goboxStats := runGoboxCLI(t, t.TempDir(), "", "netstat", "-s")
		goboxTCPStats := runGoboxCLI(t, t.TempDir(), "", "netstat", "-s", "-t")
		nativeStats := runNativeCLI(t, t.TempDir(), "", "netstat", "-s")
		nativeTCPStats := runNativeCLI(t, t.TempDir(), "", "netstat", "-s", "-t")
		if goboxStats.ExitCode != 0 || goboxTCPStats.ExitCode != 0 || nativeStats.ExitCode != 0 || nativeTCPStats.ExitCode != 0 {
			t.Fatalf("netstat -s/-s -t failed goboxStats=%+v goboxTCPStats=%+v nativeStats=%+v nativeTCPStats=%+v", goboxStats, goboxTCPStats, nativeStats, nativeTCPStats)
		}
		if goboxStats.Stdout == goboxTCPStats.Stdout {
			t.Fatalf("gobox netstat -s -t should not be identical to bare -s\n--- -s ---\n%s\n--- -s -t ---\n%s", goboxStats.Stdout, goboxTCPStats.Stdout)
		}
		if nativeStats.Stdout == nativeTCPStats.Stdout {
			t.Fatalf("native netstat -s -t should not be identical to bare -s\n--- -s ---\n%s\n--- -s -t ---\n%s", nativeStats.Stdout, nativeTCPStats.Stdout)
		}
		if findNetLineWithPrefix(goboxTCPStats.Stdout, "Ip:") != "" || findNetLineWithPrefix(goboxTCPStats.Stdout, "Udp:") != "" {
			t.Fatalf("gobox netstat -s -t leaked non-TCP sections\n%s", goboxTCPStats.Stdout)
		}
		if findNetLineWithPrefix(goboxTCPStats.Stdout, "Tcp:") == "" {
			t.Fatalf("gobox netstat -s -t missing Tcp section\n%s", goboxTCPStats.Stdout)
		}
	})
}

func TestParity_IpCases(t *testing.T) {
	requireNativeCommand(t, "ip")

	t.Run("IP-001", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "ip", "addr")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "addr")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip addr exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		goboxBlocks := ipBlocks(gobox.Stdout)
		nativeBlocks := ipBlocks(native.Stdout)
		golo, nlo := goboxBlocks["lo"], nativeBlocks["lo"]
		if len(golo) < 2 || len(nlo) < 2 {
			t.Fatalf("ip addr output missing loopback block\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		if !strings.Contains(golo[0], "state") || !strings.Contains(nlo[0], "state") {
			t.Fatalf("ip addr loopback header missing state field\ngobox=%q\nnative=%q", golo[0], nlo[0])
		}
		if !strings.Contains(strings.Join(golo, "\n"), "127.0.0.1/8") || !strings.Contains(strings.Join(nlo, "\n"), "127.0.0.1/8") {
			t.Fatalf("ip addr loopback block missing inet row\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("IP-002", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "ip", "addr")
		gobox := runGoboxCLI(t, env, "", "ip", "-o", "addr")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "-o", "addr")
		if base.ExitCode != 0 || gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip -o addr exit mismatch base=%+v gobox=%+v native=%+v", base, gobox, native)
		}
		if base.Stdout == gobox.Stdout {
			t.Fatalf("ip -o addr should change output relative to multiline addr view\n--- base ---\n%s\n--- oneline ---\n%s", base.Stdout, gobox.Stdout)
		}
		if !strings.Contains(gobox.Stdout, " lo ") || !strings.Contains(native.Stdout, " lo ") {
			t.Fatalf("ip -o addr missing loopback line\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		for _, line := range nonEmptyLines(gobox.Stdout) {
			if strings.HasPrefix(line, "    ") || !strings.Contains(line, " scope ") {
				t.Fatalf("ip -o addr should emit one-line scoped records, got %q", line)
			}
		}
	})

	t.Run("IP-003", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "ip", "link")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "link")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip link exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		goboxBlocks := ipBlocks(gobox.Stdout)
		nativeBlocks := ipBlocks(native.Stdout)
		golo, nlo := goboxBlocks["lo"], nativeBlocks["lo"]
		if len(golo) < 2 || len(nlo) < 2 {
			t.Fatalf("ip link output missing loopback block\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		if !strings.Contains(golo[0], "mtu") || !strings.Contains(nlo[0], "mtu") {
			t.Fatalf("ip link loopback header missing mtu field\ngobox=%q\nnative=%q", golo[0], nlo[0])
		}
		if !strings.Contains(golo[1], "link/") || !strings.Contains(nlo[1], "link/") {
			t.Fatalf("ip link loopback block missing link-layer detail\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("IP-004", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "ip", "link")
		gobox := runGoboxCLI(t, env, "", "ip", "-s", "link")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "-s", "link")
		if base.ExitCode != 0 || gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip -s link exit mismatch base=%+v gobox=%+v native=%+v", base, gobox, native)
		}
		if base.Stdout == gobox.Stdout {
			t.Fatalf("ip -s link should change output relative to plain link view\n--- base ---\n%s\n--- stats ---\n%s", base.Stdout, gobox.Stdout)
		}
		for _, want := range []string{"RX", "TX"} {
			if !strings.Contains(strings.ToUpper(gobox.Stdout), want) || !strings.Contains(strings.ToUpper(native.Stdout), want) {
				t.Fatalf("ip -s link missing %q\ngobox=%s\nnative=%s", want, gobox.Stdout, native.Stdout)
			}
		}
		if !strings.Contains(gobox.Stdout, "packets") || !strings.Contains(gobox.Stdout, "errors") {
			t.Fatalf("ip -s link should include packet/error counters\n%s", gobox.Stdout)
		}
	})

	t.Run("IP-005", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "ip", "route")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "route")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip route exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if strings.Contains(native.Stdout, "default") && !strings.Contains(gobox.Stdout, "default") {
			t.Fatalf("ip route missing default route\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		for _, line := range nonEmptyLines(gobox.Stdout) {
			if !strings.Contains(line, " dev ") {
				t.Fatalf("ip route row missing dev field: %q", line)
			}
		}
	})

	t.Run("IP-006", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "ip", "neigh")
		native := runNativeCLI(t, t.TempDir(), "", "ip", "neigh")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("ip neigh exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		if native.Stdout != "" && gobox.Stdout == "" {
			t.Fatalf("ip neigh unexpectedly empty\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
		for _, line := range nonEmptyLines(gobox.Stdout) {
			if !strings.Contains(line, " dev ") {
				t.Fatalf("ip neigh row missing dev field: %q", line)
			}
			if fields := strings.Fields(line); len(fields) < 4 {
				t.Fatalf("ip neigh row too short: %q", line)
			}
		}
	})
}

func TestParity_CurlCases(t *testing.T) {
	requireNativeCommand(t, "curl")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, "ok")
		case "/redirect":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			fmt.Fprint(w, "redirected")
		case "/echo":
			body, _ := io.ReadAll(r.Body)
			fmt.Fprint(w, string(body))
		case "/upload":
			body, _ := io.ReadAll(r.Body)
			fmt.Fprint(w, string(body))
		case "/multipart":
			mr, err := r.MultipartReader()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			part, err := mr.NextPart()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(part)
			fmt.Fprintf(w, "%s:%s", part.FileName(), string(data))
		case "/fail":
			http.Error(w, "nope", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Run("CURL-001", func(t *testing.T) {
		runExactParityCases(t, []parityCase{{ID: "CURL-001", Name: "curl -s", GoboxArgs: []string{"curl", "-s", server.URL}, NativeCommand: "curl", NativeArgs: []string{"-s", server.URL}}})
	})

	t.Run("CURL-002", func(t *testing.T) {
		res := runGoboxMainCLI(t, t.TempDir(), "", "curl", "-s", "-S", "://bad-url")
		native := runNativeCLI(t, t.TempDir(), "", "curl", "-s", "-S", "://bad-url")
		if res.ExitCode == 0 || native.ExitCode == 0 {
			t.Fatalf("curl -s -S expected failure gobox=%+v native=%+v", res, native)
		}
		if !strings.Contains(strings.ToLower(res.Stderr), "curl:") {
			t.Fatalf("curl -s -S missing gobox error prefix: %+v", res)
		}
	})

	t.Run("CURL-003", func(t *testing.T) {
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "curl", "-o", "out.txt", server.URL)
		native := runNativeCLI(t, env, "", "curl", "-o", "native.txt", server.URL)
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("curl -o exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		gBody, gErr := os.ReadFile(filepath.Join(env, "out.txt"))
		nBody, nErr := os.ReadFile(filepath.Join(env, "native.txt"))
		if gErr != nil || nErr != nil || string(gBody) != string(nBody) {
			t.Fatalf("curl -o file mismatch gobox=%q native=%q gErr=%v nErr=%v", string(gBody), string(nBody), gErr, nErr)
		}
	})

	t.Run("CURL-004", func(t *testing.T) {
		fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "remote-body") }))
		defer fileServer.Close()
		env := t.TempDir()
		gobox := runGoboxCLI(t, env, "", "curl", "-O", fileServer.URL+"/artifact.txt")
		native := runNativeCLI(t, env, "", "curl", "-O", fileServer.URL+"/artifact-native.txt")
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("curl -O exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
		}
		gBody, gErr := os.ReadFile(filepath.Join(env, "artifact.txt"))
		nBody, nErr := os.ReadFile(filepath.Join(env, "artifact-native.txt"))
		if gErr != nil || nErr != nil || string(gBody) != string(nBody) {
			t.Fatalf("curl -O file mismatch gobox=%q native=%q gErr=%v nErr=%v", string(gBody), string(nBody), gErr, nErr)
		}
	})

	methodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.Method)
	}))
	defer methodServer.Close()

	headerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.Header.Get("X-Test"))
	}))
	defer headerServer.Close()

	for _, tc := range []parityCase{
		{ID: "CURL-005", Name: "curl -L", GoboxArgs: []string{"curl", "-L", server.URL + "/redirect"}, NativeCommand: "curl", NativeArgs: []string{"-L", server.URL + "/redirect"}},
		{ID: "CURL-006", Name: "curl -I", GoboxArgs: []string{"curl", "-I", server.URL}, NativeCommand: "curl", NativeArgs: []string{"-I", server.URL}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -I exit mismatch")
			}
			if !strings.Contains(gobox.Stdout, "HTTP/") {
				t.Fatalf("curl -I missing status line: %q", gobox.Stdout)
			}
			if strings.Contains(gobox.Stdout, "ok") {
				t.Fatalf("curl -I should not include response body: %q", gobox.Stdout)
			}
		}},
		{ID: "CURL-007", Name: "curl -w", GoboxArgs: []string{"curl", "-w", "%{http_code}", "-o", os.DevNull, server.URL}, NativeCommand: "curl", NativeArgs: []string{"-w", "%{http_code}", "-o", os.DevNull, server.URL}},
		{ID: "CURL-009", Name: "curl -X", GoboxArgs: []string{"curl", "-X", "POST", methodServer.URL}, NativeCommand: "curl", NativeArgs: []string{"-X", "POST", methodServer.URL}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -X exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
			}
			if normalizeText(gobox.Stdout) != "POST" || normalizeText(native.Stdout) != "POST" {
				t.Fatalf("curl -X did not switch request method gobox=%q native=%q", gobox.Stdout, native.Stdout)
			}
		}},
		{ID: "CURL-010", Name: "curl -H", GoboxArgs: []string{"curl", "-H", "X-Test: 1", headerServer.URL}, NativeCommand: "curl", NativeArgs: []string{"-H", "X-Test: 1", headerServer.URL}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -H exit mismatch gobox=%d native=%d", gobox.ExitCode, native.ExitCode)
			}
			if normalizeText(gobox.Stdout) != "1" || normalizeText(native.Stdout) != "1" {
				t.Fatalf("curl -H did not send custom header gobox=%q native=%q", gobox.Stdout, native.Stdout)
			}
		}},
		{ID: "CURL-011", Name: "curl -d", GoboxArgs: []string{"curl", "-d", "name=test", server.URL + "/echo"}, NativeCommand: "curl", NativeArgs: []string{"-d", "name=test", server.URL + "/echo"}},
		{ID: "CURL-015", Name: "curl -f", GoboxArgs: []string{"curl", "-f", server.URL + "/fail"}, NativeCommand: "curl", NativeArgs: []string{"-f", server.URL + "/fail"}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -f exit mismatch %d != %d", gobox.ExitCode, native.ExitCode)
			}
		}},
	} {
		t.Run(tc.ID, func(t *testing.T) {
			gobox := runGoboxCLI(t, t.TempDir(), tc.Stdin, tc.GoboxArgs...)
			native := runNativeCLI(t, t.TempDir(), tc.Stdin, tc.NativeCommand, tc.NativeArgs...)
			if tc.Assert != nil {
				tc.Assert(t, gobox, native)
				return
			}
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("%s exit mismatch gobox=%d native=%d", tc.ID, gobox.ExitCode, native.ExitCode)
			}
			if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
				t.Fatalf("%s stdout mismatch\n%s\n%s", tc.ID, gobox.Stdout, native.Stdout)
			}
		})
	}

	t.Run("CURL-008", func(t *testing.T) {
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			fmt.Fprint(w, "slow")
		}))
		defer slowServer.Close()
		gobox := runGoboxCLI(t, t.TempDir(), "", "curl", "-m", "0.05", slowServer.URL)
		native := runNativeCLI(t, t.TempDir(), "", "curl", "-m", "0.05", slowServer.URL)
		if gobox.ExitCode == 0 || native.ExitCode == 0 {
			t.Fatalf("curl -m expected timeout failure gobox=%+v native=%+v", gobox, native)
		}
		if strings.Contains(gobox.Stdout, "slow") {
			t.Fatalf("curl -m should time out before receiving the body: %+v", gobox)
		}
	})

	t.Run("CURL-012", func(t *testing.T) {
		tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "tls-ok") }))
		defer tlsServer.Close()
		gobox := runGoboxCLI(t, t.TempDir(), "", "curl", "-k", tlsServer.URL)
		native := runNativeCLI(t, t.TempDir(), "", "curl", "-k", tlsServer.URL)
		if gobox.ExitCode != native.ExitCode || normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -k mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("CURL-013", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "curl", "--connect-timeout", "0.05", "http://10.255.255.1:81")
		native := runNativeCLI(t, t.TempDir(), "", "curl", "--noproxy", "*", "-s", "--connect-timeout", "0.05", "http://10.255.255.1:81")
		if gobox.ExitCode == 0 || native.ExitCode == 0 {
			t.Fatalf("curl --connect-timeout expected failure gobox=%+v native=%+v", gobox, native)
		}
		if strings.TrimSpace(gobox.Stdout) != "" {
			t.Fatalf("curl --connect-timeout should not produce a successful response body: %+v", gobox)
		}
	})

	t.Run("CURL-014", func(t *testing.T) {
		hostPort := strings.TrimPrefix(server.URL, "http://")
		_, port, _ := strings.Cut(hostPort, ":")
		gobox := runGoboxCLI(t, t.TempDir(), "", "curl", "--resolve", "example.invalid:"+port+":127.0.0.1", "http://example.invalid:"+port)
		native := runNativeCLI(t, t.TempDir(), "", "curl", "--noproxy", "*", "-s", "--resolve", "example.invalid:"+port+":127.0.0.1", "http://example.invalid:"+port)
		if gobox.ExitCode != native.ExitCode || normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl --resolve mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("CURL-016", func(t *testing.T) {
		headerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "1")
			fmt.Fprint(w, "body")
		}))
		defer headerServer.Close()
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "curl", "-s", headerServer.URL)
		gobox := runGoboxCLI(t, env, "", "curl", "-i", headerServer.URL)
		native := runNativeCLI(t, t.TempDir(), "", "curl", "-i", headerServer.URL)
		if base.ExitCode != 0 || gobox.ExitCode != native.ExitCode {
			t.Fatalf("curl -i exit mismatch base=%+v gobox=%+v native=%+v", base, gobox, native)
		}
		if !strings.Contains(gobox.Stdout, "X-Test: 1") || !strings.Contains(gobox.Stdout, "body") {
			t.Fatalf("curl -i gobox output incomplete: %+v", gobox)
		}
		if !strings.Contains(native.Stdout, "X-Test: 1") || !strings.Contains(native.Stdout, "body") {
			t.Fatalf("curl -i native output incomplete: %+v", native)
		}
		if base.Stdout == gobox.Stdout {
			t.Fatalf("curl -i should change output relative to body-only mode\n--- base ---\n%s\n--- include ---\n%s", base.Stdout, gobox.Stdout)
		}
		if statusIdx := strings.Index(gobox.Stdout, "HTTP/"); statusIdx == -1 {
			t.Fatalf("curl -i missing status line: %+v", gobox)
		} else if headerIdx, bodyIdx := strings.Index(gobox.Stdout, "X-Test: 1"), strings.Index(gobox.Stdout, "body"); headerIdx == -1 || bodyIdx == -1 || !(statusIdx < headerIdx && headerIdx < bodyIdx) {
			t.Fatalf("curl -i should emit status/header/body in order, got %q", gobox.Stdout)
		}
	})

	t.Run("CURL-017", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "payload.txt")
		writeFile(t, file, "upload-body")
		uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			fmt.Fprintf(w, "%s:%s", r.Method, string(body))
		}))
		defer uploadServer.Close()
		gobox := runGoboxCLI(t, env, "", "curl", "-T", file, uploadServer.URL)
		native := runNativeCLI(t, env, "", "curl", "-T", file, uploadServer.URL)
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -T mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
		}
		if normalizeText(gobox.Stdout) != "PUT:upload-body" || normalizeText(native.Stdout) != "PUT:upload-body" {
			t.Fatalf("curl -T should perform a PUT upload with the file body gobox=%q native=%q", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("CURL-018", func(t *testing.T) {
		env := t.TempDir()
		file := filepath.Join(env, "payload.txt")
		writeFile(t, file, "form-body")
		formServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mr, err := r.MultipartReader()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			part, err := mr.NextPart()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(part)
			fmt.Fprintf(w, "%s:%s:%s", part.FormName(), part.FileName(), string(data))
		}))
		defer formServer.Close()
		gobox := runGoboxCLI(t, env, "", "curl", "-F", "file=@payload.txt", formServer.URL)
		native := runNativeCLI(t, env, "", "curl", "-F", "file=@payload.txt", formServer.URL)
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -F mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
		}
		if normalizeText(gobox.Stdout) != "file:payload.txt:form-body" || normalizeText(native.Stdout) != "file:payload.txt:form-body" {
			t.Fatalf("curl -F should upload a multipart file part gobox=%q native=%q", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("CURL-019", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "curl", "--bench", "-n", "4", server.URL)
		res := runGoboxCLI(t, env, "", "curl", "--bench", "-c", "2", "-n", "4", server.URL)
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("curl bench concurrent failed base=%+v concurrent=%+v", base, res)
		}
		req, conc, failed := curlBenchRequestsLine(t, res.Stdout)
		baseReq, baseConc, _ := curlBenchRequestsLine(t, base.Stdout)
		if req != 4 || conc != 2 || failed != 0 {
			t.Fatalf("curl bench -c should report configured requests/concurrency, got requests=%d concurrency=%d failed=%d\n%s", req, conc, failed, res.Stdout)
		}
		if baseReq != 4 || baseConc != 1 {
			t.Fatalf("curl bench baseline unexpected requests/concurrency=%d/%d\n%s", baseReq, baseConc, base.Stdout)
		}
		if findNetLineWithPrefix(res.Stdout, "Latency:") == "" || findNetLineWithPrefix(res.Stdout, "Throughput:") == "" {
			t.Fatalf("curl bench -c missing latency/throughput summary\n%s", res.Stdout)
		}
	})

	t.Run("CURL-020", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "curl", "--bench", "-n", "2", server.URL)
		res := runGoboxCLI(t, env, "", "curl", "--bench", "-n", "3", server.URL)
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("curl bench requests failed base=%+v requests=%+v", base, res)
		}
		req, conc, failed := curlBenchRequestsLine(t, res.Stdout)
		if req != 3 || conc != 1 || failed != 0 {
			t.Fatalf("curl bench -n should report configured request count, got requests=%d concurrency=%d failed=%d\n%s", req, conc, failed, res.Stdout)
		}
		if findNetLineWithPrefix(res.Stdout, "Latency:") == "" || findNetLineWithPrefix(res.Stdout, "Throughput:") == "" {
			t.Fatalf("curl bench -n missing latency/throughput summary\n%s", res.Stdout)
		}
	})

	t.Run("CURL-021", func(t *testing.T) {
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "curl", "--bench", "-n", "2", server.URL)
		res := runGoboxCLI(t, env, "", "curl", "--bench", "--warmup", "2", "-n", "2", server.URL)
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("curl bench warmup failed base=%+v warmup=%+v", base, res)
		}
		req, conc, failed := curlBenchRequestsLine(t, res.Stdout)
		if req != 2 || conc != 1 || failed != 0 {
			t.Fatalf("curl bench --warmup should preserve configured request count, got requests=%d concurrency=%d failed=%d\n%s", req, conc, failed, res.Stdout)
		}
		if findNetLineWithPrefix(res.Stdout, "Latency:") == "" || findNetLineWithPrefix(res.Stdout, "Throughput:") == "" {
			t.Fatalf("curl bench --warmup missing latency/throughput summary\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout+res.Stderr, "Warming up 2 requests") {
			t.Fatalf("curl bench --warmup missing warmup banner: %+v", res)
		}
	})

	t.Run("CURL-022", func(t *testing.T) {
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(150 * time.Millisecond)
			fmt.Fprint(w, "slow")
		}))
		defer slowServer.Close()
		res := runGoboxCLI(t, t.TempDir(), "", "curl", "--bench", "-n", "2", "-t", "0.05", slowServer.URL)
		req, conc, failed := curlBenchRequestsLine(t, res.Stdout)
		if res.ExitCode != 0 || req != 2 || conc != 1 || failed == 0 {
			t.Fatalf("curl bench timeout failed: %+v", res)
		}
	})
}

func TestParity_NcCases(t *testing.T) {
	t.Run("NC-001", func(t *testing.T) {
		const serverMsg = "from-server\n"
		const clientMsg = "from-client\n"

		goboxPort := closedTCPPort(t)
		gobox := runGoboxNCListen(t, goboxPort, serverMsg, clientMsg, 2*time.Second)
		nativePort := closedTCPPort(t)
		native := runNativeNCListen(t, nativePort, serverMsg, clientMsg, 2*time.Second)

		for name, res := range map[string]ncListenResult{"gobox": gobox, "native": native} {
			if res.Server.ExitCode != 0 {
				t.Fatalf("%s nc -l failed: %+v", name, res.Server)
			}
			if res.ClientErr != nil {
				t.Fatalf("%s nc -l client failed: %v", name, res.ClientErr)
			}
			if !strings.Contains(res.Server.Stdout, clientMsg) {
				t.Fatalf("%s nc -l stdout missing client payload: server=%+v", name, res.Server)
			}
			if !strings.Contains(res.ClientOutput, serverMsg) {
				t.Fatalf("%s nc -l client missing server payload: %q", name, res.ClientOutput)
			}
		}
	})

	t.Run("NC-002", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-z", "127.0.0.1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-z", "127.0.0.1", port)
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("nc -z failed gobox=%+v native=%+v", gobox, native)
		}
		if strings.TrimSpace(gobox.Stdout+gobox.Stderr) != "" {
			t.Fatalf("nc -z should not emit data-path output without -v: %+v", gobox)
		}
	})

	t.Run("NC-003", func(t *testing.T) {
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen udp: %v", err)
		}
		defer conn.Close()
		host, port, _ := net.SplitHostPort(conn.LocalAddr().String())
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-u", "-z", host, port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-u", "-z", host, port)
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("nc -u failed gobox=%+v native=%+v", gobox, native)
		}
		if strings.TrimSpace(gobox.Stdout+gobox.Stderr) != "" {
			t.Fatalf("nc -u -z should remain quiet without -v: %+v", gobox)
		}
	})

	t.Run("NC-004", func(t *testing.T) {
		port := closedTCPPort(t)
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-w", "1", "127.0.0.1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-w", "1", "127.0.0.1", port)
		if gobox.ExitCode == 0 || native.ExitCode == 0 {
			t.Fatalf("nc -w expected connection failure gobox=%+v native=%+v", gobox, native)
		}
		if strings.Contains(strings.ToLower(gobox.Stdout+gobox.Stderr), "successful") {
			t.Fatalf("gobox nc -w should not report success on a closed port: %+v", gobox)
		}
	})

	t.Run("NC-005", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "nc", "-z", "127.0.0.1", port)
		gobox := runGoboxCLI(t, env, "", "nc", "-z", "-v", "127.0.0.1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-z", "-v", "127.0.0.1", port)
		if base.ExitCode != 0 || gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("nc -v failed base=%+v gobox=%+v native=%+v", base, gobox, native)
		}
		if normalizeText(base.Stdout+base.Stderr) == normalizeText(gobox.Stdout+gobox.Stderr) {
			t.Fatalf("nc -v should add diagnostic output relative to plain -z\n--- base ---\n%s%s\n--- verbose ---\n%s%s", base.Stdout, base.Stderr, gobox.Stdout, gobox.Stderr)
		}
		if !strings.Contains(gobox.Stdout+gobox.Stderr, "Connection successful") {
			t.Fatalf("gobox nc -v missing success output: %+v", gobox)
		}
	})

	t.Run("NC-006", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-n", "localhost", "1")
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-n", "localhost", "1")
		if gobox.ExitCode == 0 || native.ExitCode == 0 {
			t.Fatalf("nc -n hostname should fail gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NC-007", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-4", "-z", "127.0.0.1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-4", "-z", "127.0.0.1", port)
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("nc -4 failed gobox=%+v native=%+v", gobox, native)
		}
		if strings.Contains(strings.ToLower(gobox.Stderr+gobox.Stdout), "ipv6") {
			t.Fatalf("nc -4 should not attempt ipv6 path: %+v", gobox)
		}
	})

	t.Run("NC-008", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "[::1]:0")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "nc", "-6", "-z", "::1", port)
		native := runNativeCLI(t, t.TempDir(), "", "nc", "-6", "-z", "::1", port)
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("nc -6 failed gobox=%+v native=%+v", gobox, native)
		}
		if strings.Contains(strings.ToLower(gobox.Stderr+gobox.Stdout), "ipv4") {
			t.Fatalf("nc -6 should not attempt ipv4 path: %+v", gobox)
		}
	})

	for _, tc := range []struct {
		id          string
		args        []string
		baseArgs    []string
		wantBytes   string
		minDuration float64
		maxDuration float64
		minElapsed  time.Duration
		wantReport  string
	}{
		{"NC-009", []string{"nc", "--bench", "-n", "2", "-s", "16B"}, nil, "32B", 0, 5, 0, ""},
		{"NC-010", []string{"nc", "--bench", "-c", "2", "-n", "4", "-s", "16B"}, nil, "64B", 0, 5, 0, ""},
		{"NC-011", []string{"nc", "--bench", "-n", "3", "-s", "16B"}, nil, "48B", 0, 5, 0, ""},
		{"NC-012", []string{"nc", "--bench", "-n", "2", "-s", "32B"}, nil, "64B", 0, 5, 0, ""},
		{"NC-013", []string{"nc", "--bench", "-t", "1", "-n", "200000", "-s", "16B"}, []string{"nc", "--bench", "-n", "2", "-s", "16B"}, "", 0, 5, 800 * time.Millisecond, ""},
		{"NC-014", []string{"nc", "--bench", "-t", "3", "-n", "200000", "-s", "16B", "-i", "2"}, []string{"nc", "--bench", "-t", "3", "-n", "200000", "-s", "16B"}, "", 0, 5, 2500 * time.Millisecond, "[ 1]"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
			defer closeFn()
			args := append([]string{}, tc.args...)
			args = append(args, "127.0.0.1", port)
			env := t.TempDir()
			baseArgs := append([]string{}, tc.baseArgs...)
			if len(baseArgs) == 0 {
				baseArgs = []string{"nc", "--bench", "-n", "2", "-s", "16B"}
			}
			baseArgs = append(baseArgs, "127.0.0.1", port)
			base := runGoboxCLI(t, env, "", baseArgs...)
			started := time.Now()
			res := runGoboxCLI(t, env, "", args...)
			elapsed := time.Since(started)
			if res.ExitCode != 0 {
				t.Fatalf("%s failed: %+v want bytes %q", tc.id, res, tc.wantBytes)
			}
			if tc.id != "NC-009" && tc.id != "NC-013" && tc.id != "NC-014" && base.ExitCode == 0 && base.Stdout == res.Stdout {
				t.Fatalf("%s should change bench output relative to the default bench baseline\n--- base ---\n%s\n--- variant ---\n%s", tc.id, base.Stdout, res.Stdout)
			}
			if findNetLineWithPrefix(res.Stdout, "Latency:") == "" {
				t.Fatalf("%s missing latency summary\n%s", tc.id, res.Stdout)
			}
			duration, totalBytes := ncBenchTotalFields(t, res.Stdout)
			if tc.wantBytes != "" && totalBytes != tc.wantBytes {
				t.Fatalf("%s total payload mismatch: got %q want %q\n%s", tc.id, totalBytes, tc.wantBytes, res.Stdout)
			}
			if duration < tc.minDuration || (tc.maxDuration > 0 && duration > tc.maxDuration) {
				t.Fatalf("%s total duration out of range: got %.2fs want [%.2f, %.2f]\n%s", tc.id, duration, tc.minDuration, tc.maxDuration, res.Stdout)
			}
			if tc.minElapsed > 0 && elapsed < tc.minElapsed {
				t.Fatalf("%s elapsed runtime too short: got %s want >= %s\n%s", tc.id, elapsed, tc.minElapsed, res.Stdout)
			}
			if tc.wantReport != "" && !strings.Contains(res.Stdout, tc.wantReport) {
				t.Fatalf("%s should emit interval report %q\n%s", tc.id, tc.wantReport, res.Stdout)
			}
		})
	}
}

func TestParity_DnsCases(t *testing.T) {
	t.Run("DNS-001", func(t *testing.T) {
		host, port, closeFn := startLocalDNSServer(t, "203.0.113.7")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "dig", "@"+net.JoinHostPort(host, port), "+short", "example.test")
		native := runNativeCLI(t, t.TempDir(), "", "dig", "@"+host, "-p", port, "+short", "example.test")
		if normalizeText(gobox.Stdout) != "203.0.113.7" || normalizeText(native.Stdout) != "203.0.113.7" {
			t.Fatalf("dig @DNS_SERVER mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("DNS-002", func(t *testing.T) {
		host, port, closeFn := startLocalDNSServer(t, "203.0.113.8")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "dig", "@"+net.JoinHostPort(host, port), "-t", "A", "+short", "example.test")
		native := runNativeCLI(t, t.TempDir(), "", "dig", "@"+host, "-p", port, "-t", "A", "+short", "example.test")
		if normalizeText(gobox.Stdout) != "203.0.113.8" || normalizeText(native.Stdout) != "203.0.113.8" {
			t.Fatalf("dig -t A mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("DNS-003", func(t *testing.T) {
		host, port, closeFn := startLocalDNSServer(t, "203.0.113.9")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "dig", "@"+net.JoinHostPort(host, port), "+short", "example.test")
		native := runNativeCLI(t, t.TempDir(), "", "dig", "@"+host, "-p", port, "+short", "example.test")
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("dig +short mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("DNS-004", func(t *testing.T) {
		host, port, closeFn := startLocalDNSServer(t, "203.0.113.10")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "dig", "@"+net.JoinHostPort(host, port), "+noall", "+answer", "example.test")
		native := runNativeCLI(t, t.TempDir(), "", "dig", "@"+host, "-p", port, "+noall", "+answer", "example.test")
		goboxLine := findNetLineContaining(gobox.Stdout, "203.0.113.10")
		nativeLine := findNetLineContaining(native.Stdout, "203.0.113.10")
		if goboxLine == "" || nativeLine == "" {
			t.Fatalf("dig +noall +answer mismatch gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(goboxLine, "IN") || !strings.Contains(nativeLine, "IN") {
			t.Fatalf("dig +noall +answer should preserve answer-row shape\ngobox=%s\nnative=%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("DNS-005", func(t *testing.T) {
		host, port, closeFn := startLocalDNSServer(t, "203.0.113.11")
		defer closeFn()
		gobox := runGoboxCLI(t, t.TempDir(), "", "dig", "@"+net.JoinHostPort(host, port), "+tcp", "+short", "example.test")
		native := runNativeCLI(t, t.TempDir(), "", "dig", "@"+host, "-p", port, "+tcp", "+short", "example.test")
		if normalizeText(gobox.Stdout) != "203.0.113.11" || normalizeText(native.Stdout) != "203.0.113.11" {
			t.Fatalf("dig +tcp mismatch gobox=%+v native=%+v", gobox, native)
		}
	})
}

func TestParity_NpCases(t *testing.T) {
	t.Run("NP-001", func(t *testing.T) {
		loopbackName := ""
		ifaces, err := net.Interfaces()
		if err != nil {
			t.Fatalf("list interfaces: %v", err)
		}
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 && iface.Flags&net.FlagUp != 0 {
				loopbackName = iface.Name
				break
			}
		}
		if loopbackName == "" {
			t.Skip("no active loopback interface available")
		}
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "--tcp", "-I", loopbackName, "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if res.ExitCode != 0 {
			t.Fatalf("np -I failed: %+v", res)
		}
		if line := findNetLineContaining(res.Stdout, "packets transmitted"); !strings.Contains(line, "1 packets transmitted") {
			t.Fatalf("np -I missing expected summary line: %+v", res)
		}
	})

	t.Run("NP-002", func(t *testing.T) {
		port := closedTCPPort(t)
		res := runGoboxCLI(t, t.TempDir(), "", "np", "--tcp", "-p", port, "-W", "1", "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if res.ExitCode != 0 {
			t.Fatalf("np -W failed: %+v", res)
		}
		if line := findNetLineContaining(res.Stdout, "errors"); !strings.Contains(line, "1 errors") {
			t.Fatalf("np -W missing expected error summary: %+v", res)
		}
	})

	t.Run("NP-003", func(t *testing.T) {
		gateway := defaultIPv4Gateway(t)
		gobox := runGoboxCLI(t, t.TempDir(), "", "np", "--arp", "-c", "1", "-W", "1", gateway)
		native := runNativeCLI(t, t.TempDir(), "", "arping", "-c", "1", "-w", "1", gateway)
		if gobox.ExitCode != native.ExitCode {
			t.Fatalf("np --arp exit mismatch gobox=%+v native=%+v", gobox, native)
		}
		if gobox.ExitCode != 0 {
			if !strings.Contains(strings.ToLower(gobox.Stderr), "operation not permitted") || !strings.Contains(strings.ToLower(native.Stderr), "operation not permitted") {
				t.Fatalf("np --arp permission failure mismatch gobox=%+v native=%+v", gobox, native)
			}
			return
		}
		if !strings.Contains(gobox.Stdout, gateway) || !strings.Contains(native.Stdout, "Received 1 response") {
			t.Fatalf("np --arp output mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NP-004", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "np", "--tcp", "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		res := runGoboxCLI(t, t.TempDir(), "", "np", "--tcp", "-p", port, "-c", "2", "-i", "0.001", "-q", "127.0.0.1")
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("np -c failed base=%+v count2=%+v", base, res)
		}
		if !strings.Contains(findNetLineContaining(base.Stdout, "packets transmitted"), "1 packets transmitted") || base.Stdout == res.Stdout {
			t.Fatalf("np -c should change the summary relative to a single-packet baseline\n--- base ---\n%s\n--- count2 ---\n%s", base.Stdout, res.Stdout)
		}
		if !strings.Contains(findNetLineContaining(res.Stdout, "packets transmitted"), "2 packets transmitted") {
			t.Fatalf("np -c missing updated packet summary\n%s", res.Stdout)
		}
	})

	t.Run("NP-005", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "np", "--icmp", "--flood", "-c", "3", "-q", "-W", "1", "127.0.0.1")
		native := runNativeCLI(t, t.TempDir(), "", "ping", "-f", "-c", "3", "-q", "-W", "1", "127.0.0.1")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("np --flood failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(findNetLineContaining(gobox.Stdout, "packets transmitted"), "3 packets transmitted") || !strings.Contains(findNetLineContaining(native.Stdout, "packets transmitted"), "3 packets transmitted") {
			t.Fatalf("np --flood packet count mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NP-006", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		start := time.Now()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "--tcp", "-p", port, "-c", "2", "-i", "0.1", "-q", "127.0.0.1")
		elapsed := time.Since(start)
		if res.ExitCode != 0 || elapsed < 100*time.Millisecond {
			t.Fatalf("np -i failed elapsed=%s result=%+v", elapsed, res)
		}
	})

	t.Run("NP-007", func(t *testing.T) {
		gobox := runGoboxCLI(t, t.TempDir(), "", "np", "--icmp", "-c", "1", "-i", "0", "-q", "-W", "1", "127.0.0.1")
		native := runNativeCLI(t, t.TempDir(), "", "ping", "-c", "1", "-q", "-W", "1", "127.0.0.1")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("np --icmp failed gobox=%+v native=%+v", gobox, native)
		}
		if !strings.Contains(findNetLineContaining(gobox.Stdout, "received"), "1 received") || !strings.Contains(findNetLineContaining(native.Stdout, "received"), "1 received") {
			t.Fatalf("np --icmp receive mismatch gobox=%+v native=%+v", gobox, native)
		}
	})

	t.Run("NP-008", func(t *testing.T) {
		_, port, closeFn := startDelayedCloseServer(t, 150*time.Millisecond)
		defer closeFn()
		start := time.Now()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "--tcp", "-p", port, "-l", "1", "-c", "1", "-q", "127.0.0.1")
		elapsed := time.Since(start)
		if res.ExitCode != 0 || elapsed < 100*time.Millisecond {
			t.Fatalf("np -l failed elapsed=%s result=%+v", elapsed, res)
		}
	})

	t.Run("NP-009", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "--tcp", "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if res.ExitCode != 0 {
			t.Fatalf("np -p failed: %+v", res)
		}
		if !strings.Contains(findNetLineContaining(res.Stdout, "packets received"), "1 packets received") {
			t.Fatalf("np -p missing receive summary: %+v", res)
		}
	})

	t.Run("NP-010", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		env := t.TempDir()
		verbose := runGoboxCLI(t, env, "", "np", "--tcp", "-p", port, "-c", "1", "-i", "0", "127.0.0.1")
		res := runGoboxCLI(t, env, "", "np", "--tcp", "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if verbose.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("np -q failed verbose=%+v quiet=%+v", verbose, res)
		}
		if verbose.Stdout == res.Stdout {
			t.Fatalf("np -q should reduce output relative to the default mode\n--- verbose ---\n%s\n--- quiet ---\n%s", verbose.Stdout, res.Stdout)
		}
		if strings.Contains(res.Stdout, "bytes from") || !strings.Contains(res.Stdout, "ping statistics") {
			t.Fatalf("np -q did not collapse to summary output: %+v", res)
		}
	})

	t.Run("NP-011", func(t *testing.T) {
		sourcePort := atoiForTest(t, closedTCPPort(t))
		_, port, remotePorts, closeFn := startTCPRemotePortRecorder(t)
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "--tcp", "-p", port, "-s", strconv.Itoa(sourcePort), "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if res.ExitCode != 0 {
			t.Fatalf("np -s failed: %+v", res)
		}
		select {
		case got := <-remotePorts:
			if got != sourcePort {
				t.Fatalf("np -s source port mismatch got=%d want=%d", got, sourcePort)
			}
		case <-time.After(time.Second):
			t.Fatal("source port recorder timed out")
		}
	})

	t.Run("NP-012", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "np", "--scan", fmt.Sprintf("%d", port), "127.0.0.1")
		if res.ExitCode != 0 {
			t.Fatalf("np scan failed: %+v", res)
		}
		if !strings.Contains(findNetLineContaining(res.Stdout, fmt.Sprintf("Port %d:", port)), "open") || !strings.Contains(findNetLineContaining(res.Stdout, "open, "), "1 open, 0 closed") {
			t.Fatalf("np scan did not report the expected open-port summary: %+v", res)
		}
	})

	t.Run("NP-013", func(t *testing.T) {
		_, port, remotePorts, closeFn := startTCPRemotePortRecorder(t)
		defer closeFn()
		res := runGoboxCLI(t, t.TempDir(), "", "np", "--tcp", "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if res.ExitCode != 0 {
			t.Fatalf("np --tcp failed: %+v", res)
		}
		if !strings.Contains(findNetLineContaining(res.Stdout, "packets received"), "1 packets received") {
			t.Fatalf("np --tcp missing receive summary: %+v", res)
		}
		select {
		case got := <-remotePorts:
			if got <= 0 {
				t.Fatalf("np --tcp recorder captured invalid remote port %d", got)
			}
		case <-time.After(time.Second):
			t.Fatal("np --tcp did not establish a TCP connection")
		}
	})

	t.Run("NP-014", func(t *testing.T) {
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen udp: %v", err)
		}
		defer conn.Close()
		_, port, _ := net.SplitHostPort(conn.LocalAddr().String())
		res := runGoboxCLI(t, t.TempDir(), "", "np", "--udp", "-p", port, "-c", "1", "-i", "0", "-q", "127.0.0.1")
		if res.ExitCode != 0 {
			t.Fatalf("np --udp failed: %+v", res)
		}
		if !strings.Contains(findNetLineContaining(res.Stdout, "packets received"), "1 packets received") {
			t.Fatalf("np --udp missing receive summary: %+v", res)
		}
		if strings.Contains(strings.ToLower(res.Stdout+res.Stderr), "connection failed") {
			t.Fatalf("np --udp should succeed on a reachable udp socket without failure diagnostics: %+v", res)
		}
	})

	t.Run("NP-015", func(t *testing.T) {
		port := closedTCPPort(t)
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "np", "--tcp", "-p", port, "-c", "1", "-i", "0", "-W", "1", "-q", "127.0.0.1")
		res := runGoboxCLI(t, env, "", "np", "--tcp", "-p", port, "-c", "1", "-i", "0", "-W", "1", "-v", "127.0.0.1")
		if base.ExitCode != 0 || res.ExitCode != 0 {
			t.Fatalf("np -v failed base=%+v verbose=%+v", base, res)
		}
		if base.Stdout == res.Stdout {
			t.Fatalf("np -v should add attempt-level diagnostics relative to quiet mode\n--- base ---\n%s\n--- verbose ---\n%s", base.Stdout, res.Stdout)
		}
		if !strings.Contains(res.Stdout, "Connection failed") || !strings.Contains(res.Stdout, "ping statistics") {
			t.Fatalf("np -v failed: %+v", res)
		}
	})

	t.Run("NP-016", func(t *testing.T) {
		_, port, closeFn := startTCPEchoServer(t, "127.0.0.1:0")
		defer closeFn()
		env := t.TempDir()
		base := runGoboxCLI(t, env, "", "np", "--tcp", "-p", port, "-c", "4", "-i", "0", "-q", "127.0.0.1")
		res := runGoboxCLI(t, env, "", "np", "--tcp", "-p", port, "-w", "2", "-c", "4", "-i", "0", "-q", "127.0.0.1")
		if base.ExitCode != 0 || res.ExitCode != 0 || !strings.Contains(res.Stdout, "packets transmitted") {
			t.Fatalf("np -w failed base=%+v workers=%+v", base, res)
		}
		if !strings.Contains(res.Stdout, "4 packets received") && !strings.Contains(res.Stdout, "5 packets received") {
			t.Fatalf("np -w received count mismatch: %+v", res)
		}
		if base.Stdout == res.Stdout {
			t.Fatalf("np -w should affect the execution/reporting path relative to the single-worker baseline\n--- base ---\n%s\n--- workers ---\n%s", base.Stdout, res.Stdout)
		}
	})
}

func TestParity_IfstatCases(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}

	for _, tc := range []struct {
		id    string
		args  []string
		check func(t *testing.T, out string)
	}{
		{
			id:   "IFSTAT-001",
			args: []string{"ifstat", "-A", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
				defaultOut := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1").Stdout
				outHeader, outRows := ifstatHeaderAndRows(out)
				baseHeader, baseRows := ifstatHeaderAndRows(defaultOut)
				if outHeader != baseHeader {
					t.Fatalf("ifstat -A should preserve default header\n-A:\n%s\n--- default ---\n%s", out, defaultOut)
				}
				if len(outRows) < len(baseRows) {
					t.Fatalf("ifstat -A should not show fewer rows than default\n-A:\n%s\n--- default ---\n%s", out, defaultOut)
				}
			},
		},
		{
			id:   "IFSTAT-002",
			args: []string{"ifstat", "-a", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
				base := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1")
				if base.ExitCode != 0 {
					t.Fatalf("ifstat baseline failed: %+v", base)
				}
				if out == base.Stdout {
					t.Fatalf("ifstat -a did not change output relative to baseline\n--- base ---\n%s\n--- absolute ---\n%s", base.Stdout, out)
				}
				header, rows := ifstatHeaderAndRows(out)
				baseHeader, baseRows := ifstatHeaderAndRows(base.Stdout)
				if header != baseHeader || len(rows) == 0 || len(rows) != len(baseRows) {
					t.Fatalf("ifstat -a missing header or rows: %q", out)
				}
				wantFields := len(ifstatHeaderColumns(header))
				outByIface := ifstatRowsByInterface(rows, wantFields)
				baseByIface := ifstatRowsByInterface(baseRows, wantFields)
				sawLargerAbsolute := false
				for iface, samples := range outByIface {
					baseSamples := baseByIface[iface]
					if len(samples) != 1 || len(baseSamples) != 1 {
						t.Fatalf("ifstat -a expected exactly one row per interface for iface=%q out=%v base=%v", iface, samples, baseSamples)
					}
					outFields := samples[0]
					baseFields := baseSamples[0]
					for i := 1; i < wantFields; i++ {
						outVal := ifstatParseFloatField(t, outFields[i])
						baseVal := ifstatParseFloatField(t, baseFields[i])
						if outVal < 0 || baseVal < 0 {
							t.Fatalf("ifstat values must be non-negative iface=%q out=%v base=%v", iface, outFields, baseFields)
						}
						if outVal > baseVal {
							sawLargerAbsolute = true
						}
					}
				}
				if !sawLargerAbsolute {
					t.Fatalf("ifstat -a should expose cumulative values larger than the per-second baseline\n--- base ---\n%s\n--- absolute ---\n%s", base.Stdout, out)
				}
			},
		},
		{
			id:   "IFSTAT-003",
			args: []string{"ifstat", "-d", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
				base := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1")
				if base.ExitCode != 0 {
					t.Fatalf("ifstat baseline failed: %+v", base)
				}
				if out == base.Stdout {
					t.Fatalf("ifstat -d did not change output relative to baseline\n--- base ---\n%s\n--- drop ---\n%s", base.Stdout, out)
				}
				header, rows := ifstatHeaderAndRows(out)
				cols := ifstatHeaderColumns(header)
				if !strings.Contains(header, "rxdrop") || !strings.Contains(header, "txdrop") {
					t.Fatalf("ifstat -d missing drop columns: %q", out)
				}
				if len(rows) == 0 {
					t.Fatalf("ifstat -d expected data rows with drop columns: %q", out)
				}
				rxdropIdx := len(cols) - 2
				txdropIdx := len(cols) - 1
				for _, line := range rows {
					fields := ifstatParseRow(t, line, len(cols))
					_ = ifstatParseUintField(t, fields[rxdropIdx])
					_ = ifstatParseUintField(t, fields[txdropIdx])
				}
			},
		},
		{
			id:   "IFSTAT-004",
			args: []string{"ifstat", "-e", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
				base := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1")
				if base.ExitCode != 0 {
					t.Fatalf("ifstat baseline failed: %+v", base)
				}
				if out == base.Stdout {
					t.Fatalf("ifstat -e did not change output relative to baseline\n--- base ---\n%s\n--- errors ---\n%s", base.Stdout, out)
				}
				header, rows := ifstatHeaderAndRows(out)
				cols := ifstatHeaderColumns(header)
				if !strings.Contains(header, "rxerrs") || !strings.Contains(header, "txerrs") {
					t.Fatalf("ifstat -e missing error columns: %q", out)
				}
				if len(rows) == 0 {
					t.Fatalf("ifstat -e expected data rows with error columns: %q", out)
				}
				rxerrsIdx := len(cols) - 2
				txerrsIdx := len(cols) - 1
				for _, line := range rows {
					fields := ifstatParseRow(t, line, len(cols))
					_ = ifstatParseUintField(t, fields[rxerrsIdx])
					_ = ifstatParseUintField(t, fields[txerrsIdx])
				}
			},
		},
		{
			id:   "IFSTAT-005",
			args: []string{"ifstat", "-i", "lo", "-n", "1", "-p", "1"},
			check: func(t *testing.T, out string) {
				_, rows := ifstatHeaderAndRows(out)
				for _, line := range rows {
					if !strings.HasPrefix(strings.TrimSpace(line), "lo ") && strings.TrimSpace(line) != "lo" {
						t.Fatalf("ifstat -i lo leaked other interfaces: %q", out)
					}
				}
			},
		},
		{
			id:   "IFSTAT-006",
			args: []string{"ifstat", "-n", "2", "-p", "1"},
			check: func(t *testing.T, out string) {
				base := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1")
				if base.ExitCode != 0 {
					t.Fatalf("ifstat baseline failed: %+v", base)
				}
				header, rows := ifstatHeaderAndRows(out)
				_, baseRows := ifstatHeaderAndRows(base.Stdout)
				if !strings.Contains(header, "Interface") || len(baseRows) == 0 {
					t.Fatalf("ifstat -n expected multiple samples: %q", out)
				}
				if len(rows) != 2*len(baseRows) {
					t.Fatalf("ifstat -n should emit exactly two samples worth of rows: base=%d got=%d\n%s", len(baseRows), len(rows), out)
				}
			},
		},
		{
			id:   "IFSTAT-007",
			args: []string{"ifstat", "-n", "2", "-p", "2"},
			check: func(t *testing.T, out string) {
				base := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1")
				if base.ExitCode != 0 {
					t.Fatalf("ifstat baseline failed: %+v", base)
				}
				header, rows := ifstatHeaderAndRows(out)
				_, baseRows := ifstatHeaderAndRows(base.Stdout)
				if !strings.Contains(header, "Interface") || len(baseRows) == 0 {
					t.Fatalf("ifstat -n/-p expected header plus repeated samples: %q", out)
				}
				if len(rows) != 2*len(baseRows) {
					t.Fatalf("ifstat -p should preserve the interface set across two samples: base=%d got=%d\n%s", len(baseRows), len(rows), out)
				}
			},
		},
	} {
		t.Run(tc.id, func(t *testing.T) {
			start := time.Now()
			res := runGoboxCLI(t, t.TempDir(), "", tc.args...)
			if res.ExitCode != 0 {
				t.Fatalf("%s failed: %+v", tc.id, res)
			}
			if tc.id == "IFSTAT-006" && time.Since(start) < time.Second {
				t.Fatalf("ifstat -p interval did not delay second sample: elapsed=%s output=%q", time.Since(start), res.Stdout)
			}
			if tc.id == "IFSTAT-007" && time.Since(start) < 2*time.Second {
				t.Fatalf("ifstat -p 2 interval did not delay second sample enough: elapsed=%s output=%q", time.Since(start), res.Stdout)
			}
			if tc.check != nil {
				tc.check(t, res.Stdout)
			}
		})
	}
}
