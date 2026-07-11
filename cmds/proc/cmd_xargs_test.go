package proc

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"
)

// runXargsWithStdin feeds stdinInput to XargsCmd and captures combined
// stdout+stderr, the same pattern TestXargsCmdShortIDoesNotConsumeCommandToken
// already uses inline, factored out for reuse by the flag-coverage tests
// below.
func runXargsWithStdin(t *testing.T, args []string, stdinInput string) string {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	if _, err := w.WriteString(stdinInput); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	out, err := captureProcOutput(t, func() error {
		return XargsCmd(args)
	})
	if err != nil {
		t.Fatalf("XargsCmd(%v) failed: %v (output: %q)", args, err, out)
	}
	return out
}

func TestXargsCmdHelpUsesStructuredOptions(t *testing.T) {
	out, err := captureProcOutput(t, func() error {
		return XargsCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("xargs --help failed: %v", err)
	}
	for _, want := range []string{"Usage: gobox xargs [OPTION]... [COMMAND [ARG]...]", "Options:", "-i REPL", "-P N", "-v"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
}

// TestXargsCmdShortIDoesNotConsumeCommandToken is a regression test for a bug
// where `-i` (BSD-style, no argument, implicitly {}) incorrectly consumed the
// following token as the replace-string, treating the actual command (e.g.
// "echo") as the REPL value instead of leaving it as the command to run.
func TestXargsCmdShortIDoesNotConsumeCommandToken(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	if _, err := w.WriteString("item\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	out, err := captureProcOutput(t, func() error {
		return XargsCmd([]string{"-i", "echo", "item: {}"})
	})
	if err != nil {
		t.Fatalf("xargs -i echo failed: %v (output: %q)", err, out)
	}
	if !strings.Contains(out, "item: item") {
		t.Fatalf("expected {} to be replaced with input line, got %q", out)
	}
}

func TestParseXargsInputsDefaultDelimiterTrimsWhitespace(t *testing.T) {
	input := "  alpha  \n\nbeta\n"
	got, err := parseXargsInputs(strings.NewReader(input), "\n")
	if err != nil {
		t.Fatalf("parseXargsInputs returned error: %v", err)
	}

	want := []string{"alpha", "beta"}
	if len(got) != len(want) {
		t.Fatalf("expected %d tokens, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestParseXargsInputsCustomDelimiterPreservesWhitespace(t *testing.T) {
	input := " alpha , beta ,gamma "
	got, err := parseXargsInputs(strings.NewReader(input), ",")
	if err != nil {
		t.Fatalf("parseXargsInputs returned error: %v", err)
	}

	want := []string{" alpha ", " beta ", "gamma "}
	if len(got) != len(want) {
		t.Fatalf("expected %d tokens, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestMakeDelimiterSplitFunc(t *testing.T) {
	split := makeDelimiterSplitFunc("::")
	scanner := bufio.NewScanner(strings.NewReader("a::b::c"))
	scanner.Split(split)

	var got []string
	for scanner.Scan() {
		got = append(got, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %d tokens, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

// The following tests exercise -n/-I/-t/-v/-r/-d/-P through XargsCmd
// end-to-end. Previously only -i (and the pure-function helpers
// parseXargsInputs/makeDelimiterSplitFunc) had any coverage at all through
// the real command, so a broken batching/placeholder/tracing/no-run/
// delimiter/parallelism implementation could ship undetected.

func TestXargsCmdBatchSizeSplitsIntoMultipleInvocations(t *testing.T) {
	out := runXargsWithStdin(t, []string{"-n", "2", "echo"}, "a\nb\nc\nd\n")
	want := "a b\nc d\n"
	if out != want {
		t.Fatalf("expected -n 2 to batch inputs two at a time producing %q, got %q", want, out)
	}
}

func TestXargsCmdCustomPlaceholder(t *testing.T) {
	out := runXargsWithStdin(t, []string{"-I", "XXX", "echo", "before-XXX-after"}, "x\n")
	want := "before-x-after\n"
	if out != want {
		t.Fatalf("expected -I XXX to substitute the custom placeholder producing %q, got %q", want, out)
	}
}

func TestXargsCmdTracePrintsCommandBeforeRunning(t *testing.T) {
	out := runXargsWithStdin(t, []string{"-t", "echo"}, "a\n")
	if !strings.Contains(out, "echo a") {
		t.Fatalf("expected -t to print the command line %q, got %q", "echo a", out)
	}
	if !strings.Contains(out, "a\n") {
		t.Fatalf("expected the command's own output to still be present, got %q", out)
	}
}

func TestXargsCmdVerboseAliasPrintsCommandBeforeRunning(t *testing.T) {
	out := runXargsWithStdin(t, []string{"-v", "echo"}, "a\n")
	if !strings.Contains(out, "echo a") {
		t.Fatalf("expected -v (legacy alias for -t) to print the command line %q, got %q", "echo a", out)
	}
}

func TestXargsCmdNoRunSuppressesCommandOnEmptyInput(t *testing.T) {
	out := runXargsWithStdin(t, []string{"-r", "echo", "hello"}, "")
	if out != "" {
		t.Fatalf("expected -r with empty stdin to suppress running the command entirely, got %q", out)
	}
}

func TestXargsCmdWithoutNoRunStillRunsOnceOnEmptyInput(t *testing.T) {
	// Establishes the contrast for the -r test above: without -r, xargs
	// runs the command once even with no input, matching GNU xargs default
	// behavior (only -r opts out of that).
	out := runXargsWithStdin(t, []string{"echo", "hello"}, "")
	want := "hello\n"
	if out != want {
		t.Fatalf("expected xargs without -r to still run the command once on empty input producing %q, got %q", want, out)
	}
}

func TestXargsCmdCustomDelimiterEndToEnd(t *testing.T) {
	out := runXargsWithStdin(t, []string{"-d", ",", "echo"}, "a,b,c")
	want := "a b c\n"
	if out != want {
		t.Fatalf("expected -d , to split on commas producing %q, got %q", want, out)
	}
}

func TestXargsCmdParallelProcessesRunConcurrently(t *testing.T) {
	// Each batch invokes a fresh "sleep 0.2" process (via -n 1, so one input
	// per invocation). With -P 1 (default) the 4 invocations run one after
	// another (~0.8s total); with -P 4 they should run concurrently
	// (~0.2s total). A no-op -P that always ran sequentially would fail
	// the elapsed-time bound below.
	stdin := "0.2\n0.2\n0.2\n0.2\n"

	start := time.Now()
	runXargsWithStdin(t, []string{"-n", "1", "-P", "1", "sleep"}, stdin)
	sequential := time.Since(start)

	start = time.Now()
	runXargsWithStdin(t, []string{"-n", "1", "-P", "4", "sleep"}, stdin)
	parallel := time.Since(start)

	if parallel >= sequential {
		t.Fatalf("expected -P 4 to run faster than -P 1 (sequential=%s, parallel=%s)", sequential, parallel)
	}
	// Generous bound: 4 concurrent 0.2s sleeps should finish well under the
	// ~0.8s a fully-sequential run would take.
	if parallel > 600*time.Millisecond {
		t.Fatalf("expected -P 4 to complete near 0.2s, took %s (sequential took %s)", parallel, sequential)
	}
}
