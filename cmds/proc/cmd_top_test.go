package proc

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTopCmdHelpPrefersCanonicalSortFlag(t *testing.T) {
	out, err := captureProcOutput(t, func() error {
		return TopCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("TopCmd help failed: %v", err)
	}
	for _, want := range []string{"Usage: gobox top [OPTION]...", "--sort FIELD", "-o FIELD", "full thread view on Linux", "pid|cpu|rss|vms|pmem|cmd|comm|user|ppid|start|etime|time"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
	if strings.Contains(out, " -sort") || strings.Contains(out, "\n-sort") {
		t.Fatalf("expected help to hide non-canonical -sort form, got %q", out)
	}
}

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

func TestFilterTopInfosThreadModeMatchesOwningProcess(t *testing.T) {
	infos := []procInfo{
		{pid: 101, tgid: 100, user: "root"},
		{pid: 102, tgid: 100, user: "root"},
		{pid: 200, tgid: 200, user: "root"},
	}
	filtered := filterTopInfos(infos, map[int]bool{100: true}, nil, nil, false, true)
	if len(filtered) != 2 || filtered[0].pid != 101 || filtered[1].pid != 102 {
		t.Fatalf("thread-mode pid filtering should match thread group ids, got %+v", filtered)
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

// TestBuildTopSummaryTasksWidthMatchesTotalDigits is a regression test for
// the Tasks: line previously having no column alignment at all (bare %d);
// native top right-justifies all 5 counts to a shared width (the digit
// width of the total, minimum 3).
func TestBuildTopSummaryTasksWidthMatchesTotalDigits(t *testing.T) {
	infos := make([]procInfo, 150)
	for i := range infos {
		infos[i] = procInfo{state: "S"}
	}
	lines := buildTopSummary(procSnapshot{}, procSnapshot{}, infos)
	tasksLine := lines[1]
	if !strings.Contains(tasksLine, "150 total") {
		t.Fatalf("expected 3-digit total unpadded, got %q", tasksLine)
	}
	if !strings.Contains(tasksLine, "  0 running") {
		t.Fatalf("expected zero counts right-justified to width 3, got %q", tasksLine)
	}
}

// TestFormatTopCPUTimeUsesMinutesSecondsCentiseconds is a regression test:
// top's TIME+ column previously reused ps's HH:MM:SS formatter
// (formatCPUTime); native top has no hour field and shows centiseconds
// instead, e.g. 4628 seconds is "77:08.00", not "01:17:08".
func TestFormatTopCPUTimeUsesMinutesSecondsCentiseconds(t *testing.T) {
	jiffies := int64(4628) * procClockTicks // 4628 seconds = 77m 8s
	if got, want := formatTopCPUTime(jiffies), "77:08.00"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	// Centisecond precision: 1.5 seconds worth of extra ticks.
	jiffies = int64(4628)*procClockTicks + procClockTicks/2
	if got, want := formatTopCPUTime(jiffies), "77:08.50"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// TestRenderTopScreenUsesSingleLetterStateHeader is a regression test: the
// process table previously labeled the state column "STATE" (a full word);
// native top uses the single letter "S".
func TestRenderTopScreenUsesSingleLetterStateHeader(t *testing.T) {
	infos := []procInfo{{pid: 1, state: "S", user: "root"}}
	out, err := captureProcOutput(t, func() error {
		renderTopScreen(procSnapshot{}, procSnapshot{}, infos, false, true, 0, "pid", 0, false, false)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	headerLine := strings.Split(out, "\n")[len(strings.Split(out, "\n"))-2]
	fields := strings.Fields(headerLine)
	found := false
	for _, f := range fields {
		if f == "S" {
			found = true
		}
		if f == "STATE" {
			t.Fatalf("expected single-letter \"S\" header, got \"STATE\" in %q", headerLine)
		}
	}
	if !found {
		t.Fatalf("expected \"S\" header column, got %q", headerLine)
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
	// reverse direction should be preserved when cycling columns
	nextField, nextReverse := advanceTopSort("cpu", false, 1)
	if nextField != "pmem" || nextReverse {
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
