package proc

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormalizeTopOrderBy(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"%CPU", "cpu"},
		{"RES", "rss"},
		{"VIRT", "vms"},
		{"COMMAND", "cmd"},
		{"PID", "PID"},
	} {
		if got := normalizeTopOrderBy(tc.in); got != tc.want {
			t.Fatalf("normalizeTopOrderBy(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestRenderTopCommandRespectsCommandMode(t *testing.T) {
	pi := procInfo{
		exe:     "sleep",
		cmdline: "sleep 10 --marker top-full-cmd",
	}
	if got := renderTopCommand(pi, false); got != "sleep" {
		t.Fatalf("expected default top command to prefer executable name, got %q", got)
	}
	if got := renderTopCommand(pi, true); got != "sleep 10 --marker top-full-cmd" {
		t.Fatalf("expected top -c mode to show full command line, got %q", got)
	}
}

func TestSortTopInfosKeepsPidTieBreakersStable(t *testing.T) {
	infos := []procInfo{
		{pid: 42, cpu: 0, exe: "z"},
		{pid: 7, cpu: 0, exe: "a"},
		{pid: 19, cpu: 0, exe: "m"},
	}
	sortTopInfos(infos, "cpu", true, 0)
	var got []int
	for _, pi := range infos {
		got = append(got, pi.pid)
	}
	want := []int{7, 19, 42}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortTopInfos should keep PID tie-breakers deterministic, got %v want %v", got, want)
	}
}

func TestTopVisibleRowLimitReservesSummarySpace(t *testing.T) {
	if got := topVisibleRowLimit(false, 10, 5); got != 2 {
		t.Fatalf("topVisibleRowLimit()=%d want 2", got)
	}
	if got := topVisibleRowLimit(true, 10, 5); got != 0 {
		t.Fatalf("batch mode should not clamp rows, got %d", got)
	}
}

func TestFormatTopUptime(t *testing.T) {
	if got := formatTopUptime(3*time.Minute + 10*time.Second); got != "3 min" {
		t.Fatalf("unexpected short uptime format %q", got)
	}
	if got := formatTopUptime(26*time.Hour + 4*time.Minute); got != "1 day,  2:04" {
		t.Fatalf("unexpected long uptime format %q", got)
	}
}

func TestBuildTopSummaryIncludesNativeLikeSections(t *testing.T) {
	prev := procSnapshot{
		cpuTimes: cpuTimes{user: 10, system: 5, idle: 80},
	}
	curr := procSnapshot{
		cpuTimes: cpuTimes{user: 20, system: 15, idle: 160},
	}
	lines := buildTopSummary(prev, curr, []procInfo{{state: "R"}, {state: "S"}})
	if len(lines) != 5 {
		t.Fatalf("expected 5 summary lines, got %d", len(lines))
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"top - ", "Tasks:", "%Cpu(s):", "MiB Mem :", "MiB Swap:"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("summary missing %q in %q", want, joined)
		}
	}
}

func TestTopCursorVisibilitySequences(t *testing.T) {
	out, err := captureProcOutput(t, func() error {
		hideTopCursor()
		showTopCursor()
		return nil
	})
	if err != nil {
		t.Fatalf("cursor helpers failed: %v", err)
	}
	for _, want := range []string{"\x1b[?25l", "\x1b[?25h"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got %q", want, out)
		}
	}
}

func TestAdvanceTopSortCyclesColumns(t *testing.T) {
	nextField, nextReverse := advanceTopSort("cpu", false, 1)
	if nextField != "pmem" || !nextReverse {
		t.Fatalf("unexpected next sort %q reverse=%v", nextField, nextReverse)
	}
	prevField, prevReverse := advanceTopSort("pid", true, -1)
	if prevField != "cmd" || !prevReverse {
		t.Fatalf("unexpected previous sort %q reverse=%v", prevField, prevReverse)
	}
}

func TestWaitTopEventToggleDirection(t *testing.T) {
	input := make(chan topInputEvent, 1)
	input <- topInputEvent{toggleDir: true}
	sortField, reverse, redraw, quit, err := waitTopEvent(time.Second, input, nil, "pcpu", true)
	if err != nil {
		t.Fatalf("waitTopEvent failed: %v", err)
	}
	if sortField != "pcpu" || reverse || !redraw || quit {
		t.Fatalf("unexpected toggle result field=%q reverse=%v redraw=%v quit=%v", sortField, reverse, redraw, quit)
	}
}

func TestRenderTopTableTruncatesUserColumn(t *testing.T) {
	headers := []string{"PID", "USER", "VIRT", "RES", "STATE", "%CPU", "%MEM", "TIME+", "COMMAND"}
	rows := [][]string{{"123", "verylongusername", "1.0GB", "10MB", "S", "1.0", "0.1", "00:01", "sleep 10"}}
	out := renderTopTable(headers, rows, 80, false)
	if !strings.Contains(out, "verylongu...") {
		t.Fatalf("expected truncated username, got %q", out)
	}
}

func TestRenderTopTableInteractiveHasNoTrailingNewline(t *testing.T) {
	headers := []string{"PID", "USER", "VIRT", "RES", "STATE", "%CPU", "%MEM", "TIME+", "COMMAND"}
	rows := [][]string{{"1", "root", "1.0GB", "10MB", "S", "0.0", "0.1", "00:01", "bash"}}
	out := renderTopTable(headers, rows, 80, true)
	if strings.HasSuffix(out, "\n") {
		t.Fatalf("interactive top frame should not end with newline, got %q", out)
	}
}

func TestRenderTopTableKeepsFullPIDVisible(t *testing.T) {
	headers := []string{"PID", "USER", "VIRT", "RES", "STATE", "%CPU", "%MEM", "TIME+", "COMMAND"}
	rows := [][]string{{"1234567", "root", "1.0GB", "10MB", "S", "0.0", "0.1", "00:01", "bash"}}
	out := renderTopTable(headers, rows, 80, false)
	if !strings.Contains(out, "1234567") {
		t.Fatalf("expected full PID to remain visible, got %q", out)
	}
}
