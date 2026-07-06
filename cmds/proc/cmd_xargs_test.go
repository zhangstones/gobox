package proc

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

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
