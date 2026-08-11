package utils

import (
	"flag"
	"reflect"
	"testing"
)

// newTestFlagSet builds a FlagSet resembling a GNU-style command with a mix of
// boolean short flags, value-taking short flags, and single-dash long names.
func newTestFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("z", false, "bool z")
	fs.Bool("v", false, "bool v")
	fs.Bool("s", false, "bool s")
	fs.Bool("I", false, "bool I (head-style)")
	fs.Int("n", 0, "value n")
	fs.Int("A", 0, "value A")
	fs.String("o", "", "value o")
	// single-dash long name (find-style)
	fs.String("name", "", "long name")
	fs.Int("maxdepth", 0, "long maxdepth")
	return fs
}

func TestExpandShortFlags(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"bundle two bools", []string{"-zv"}, []string{"-z", "-v"}},
		{"bundle three bools", []string{"-zvs"}, []string{"-z", "-v", "-s"}},
		{"attached value int", []string{"-n1"}, []string{"-n", "1"}},
		{"attached value A", []string{"-A3"}, []string{"-A", "3"}},
		{"bundle then value", []string{"-zn5"}, []string{"-z", "-n", "5"}},
		{"attached string value", []string{"-o/tmp/x"}, []string{"-o", "/tmp/x"}},
		{"canonical spaced value untouched", []string{"-n", "1"}, []string{"-n", "1"}},
		{"canonical single bool untouched", []string{"-z"}, []string{"-z"}},
		{"single-dash long name untouched", []string{"-name", "*.go"}, []string{"-name", "*.go"}},
		{"single-dash long maxdepth untouched", []string{"-maxdepth", "2"}, []string{"-maxdepth", "2"}},
		{"double-dash long untouched", []string{"--name=x"}, []string{"--name=x"}},
		{"unknown short letter passthrough", []string{"-xyz"}, []string{"-xyz"}},
		{"double-dash terminator stops expansion", []string{"-zv", "--", "-n5"}, []string{"-z", "-v", "--", "-n5"}},
		{"positional stops expansion", []string{"-z", "file", "-n5"}, []string{"-z", "file", "-n5"}},
		{"bare dash is positional", []string{"-z", "-"}, []string{"-z", "-"}},
		{"attached value with explicit equals", []string{"-o=/tmp/x"}, []string{"-o", "/tmp/x"}},
		{"empty", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := expandShortFlags(newTestFlagSet(), tc.in)
			if err != nil {
				t.Fatalf("expandShortFlags(%q) returned error: %v", tc.in, err)
			}
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expandShortFlags(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestExpandShortClustersRejectsSwallowingDefinedFlags is a regression test
// for the bug where a value-taking option placed immediately before another
// recognized flag (most dangerously kill's `-f --dry-run PATTERN`) silently
// consumed that flag's name as its own value instead of erroring. See
// cmds/proc/cmd_kill_test.go for the end-to-end kill regression.
func TestExpandShortClustersRejectsSwallowingDefinedFlags(t *testing.T) {
	cases := []struct {
		name string
		in   []string
	}{
		{"long value flag followed by defined bool", []string{"-n", "-z"}},
		{"long option followed by defined long option", []string{"-name", "-maxdepth"}},
		{"double-dash long value flag followed by defined flag", []string{"--maxdepth", "-z"}},
		{"trailing cluster value flag followed by defined flag", []string{"-zn", "-v"}},
		{"value flag at end of args with nothing to consume", []string{"-n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := expandShortFlags(newTestFlagSet(), tc.in); err == nil {
				t.Fatalf("expandShortFlags(%q) = nil error, want an error instead of swallowing the next flag", tc.in)
			}
		})
	}
}

// TestExpandShortClustersAllowsDashLikeValues ensures the new guard only
// rejects tokens that are actually registered flags, not any token that
// merely starts with '-' (e.g. a legitimate negative-looking value).
func TestExpandShortClustersAllowsDashLikeValues(t *testing.T) {
	got, err := expandShortFlags(newTestFlagSet(), []string{"-o", "-unregistered-thing"})
	if err != nil {
		t.Fatalf("expandShortFlags returned unexpected error: %v", err)
	}
	want := []string{"-o", "-unregistered-thing"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandShortFlags = %q, want %q", got, want)
	}
}

// TestExpandShortClustersAllowsMultiCharDashValueSharingAFlagLetter is a
// regression test for a false positive the first version of the "don't
// swallow a defined flag" guard introduced: `ps --sort -pid` is a real GNU
// convention (a leading '-' on a --sort value means descending order) whose
// value happens to start with the same letter as ps's own -p flag. The guard
// must only reject a token that IS a flag (exactly "-n", or a registered
// long name), not one that merely starts with a registered short letter
// ("-n" is defined as a value flag in the test set; "-nice" must pass
// through untouched, just like "-pid" must for ps's real -p flag).
func TestExpandShortClustersAllowsMultiCharDashValueSharingAFlagLetter(t *testing.T) {
	got, err := expandShortFlags(newTestFlagSet(), []string{"-o", "-nice"})
	if err != nil {
		t.Fatalf("expandShortFlags returned unexpected error: %v", err)
	}
	want := []string{"-o", "-nice"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandShortFlags = %q, want %q", got, want)
	}
}

// TestLooksLikeFlag exercises the exported heuristic used by the hand-rolled
// per-command parsers (curl, nc, sort, seq, rand, sed, tw, dig/nslookup) that
// have no flag registry to consult. It must catch exact "--long" and "-x"
// shapes (the swallowed tokens in the reported bugs: base64/hex/sort/curl's
// "-o -d"/"-o -v"/"-o -u"/"-o -s", seq's "-f -w") while still allowing
// multi-character single-dash tokens and negative numbers through, since
// those are legitimate values in real GNU/curl usage (ps's `--sort -pid`,
// curl's `-d -sI`, `-n -5`).
func TestLooksLikeFlag(t *testing.T) {
	cases := []struct {
		tok  string
		want bool
	}{
		{"-d", true},
		{"-v", true},
		{"-s", true},
		{"-u", true},
		{"-w", true},
		{"--dry-run", true},
		{"--output", true},
		{"-pid", false},
		{"-sI", false},
		{"-nice", false},
		{"-5", false},   // exactly two chars, second is a digit: a legitimate negative number
		{"-3.2", false}, // longer than two chars: caught by the length gate, not the digit check
		{"-", false},
		{"", false},
		{"file.txt", false},
		{"--", false}, // the "--" end-of-options terminator itself, not a named flag
	}
	for _, tc := range cases {
		t.Run(tc.tok, func(t *testing.T) {
			if got := LooksLikeFlag(tc.tok); got != tc.want {
				t.Fatalf("LooksLikeFlag(%q) = %v, want %v", tc.tok, got, tc.want)
			}
		})
	}
}

func TestParseFlagSetBundledAndAttached(t *testing.T) {
	fs := newTestFlagSet()
	z := fs.Lookup("z").Value
	v := fs.Lookup("v").Value
	n := fs.Lookup("n").Value
	if err := ParseFlagSet(fs, []string{"-zv", "-n5", "file.txt"}); err != nil {
		t.Fatalf("ParseFlagSet returned error: %v", err)
	}
	if z.String() != "true" || v.String() != "true" {
		t.Fatalf("expected -z and -v set, got z=%s v=%s", z, v)
	}
	if n.String() != "5" {
		t.Fatalf("expected -n=5, got %s", n)
	}
	if got := fs.Args(); !reflect.DeepEqual(got, []string{"file.txt"}) {
		t.Fatalf("expected positional [file.txt], got %q", got)
	}
}

func TestParseFlagSetLongNamesStillWork(t *testing.T) {
	fs := newTestFlagSet()
	if err := ParseFlagSet(fs, []string{"-name", "*.go", "-maxdepth", "2"}); err != nil {
		t.Fatalf("ParseFlagSet returned error: %v", err)
	}
	if fs.Lookup("name").Value.String() != "*.go" {
		t.Fatalf("expected -name=*.go, got %s", fs.Lookup("name").Value)
	}
	if fs.Lookup("maxdepth").Value.String() != "2" {
		t.Fatalf("expected -maxdepth=2, got %s", fs.Lookup("maxdepth").Value)
	}
}

// TestParseFlagSetPermuteOptionsAfterPositional is a regression test for np's
// arg-ordering bug: options that follow a positional (e.g. `-n 1` between the
// scan ports and the host) were ignored and the next option token was mistaken
// for a positional. Permutation must recover both the option value and the
// operands.
func TestParseFlagSetPermuteOptionsAfterPositional(t *testing.T) {
	fs := newTestFlagSet()
	// mirrors `np --scan 22,39999 -n 1 host`: positional, then value flag, then positional.
	if err := ParseFlagSetPermute(fs, []string{"22,39999", "-n", "1", "host"}); err != nil {
		t.Fatalf("ParseFlagSetPermute returned error: %v", err)
	}
	if fs.Lookup("n").Value.String() != "1" {
		t.Fatalf("expected -n=1, got %s", fs.Lookup("n").Value)
	}
	if got := fs.Args(); !reflect.DeepEqual(got, []string{"22,39999", "host"}) {
		t.Fatalf("expected operands [22,39999 host], got %q", got)
	}
}

// TestParseFlagSetRejectsSwallowingDefinedFlagEndToEnd exercises the full
// ParseFlagSet entry point (not just the internal expandShortFlags helper)
// to confirm the error returned by ExpandShortClusters actually propagates
// out to callers like kill/ps/np instead of being lost or having fs.Parse
// paper over it. This mirrors the exact reported shape of the bug: a
// value-taking option (kill's -f) immediately followed by a real boolean
// flag (--dry-run).
func TestParseFlagSetRejectsSwallowingDefinedFlagEndToEnd(t *testing.T) {
	fs := newTestFlagSet()
	err := ParseFlagSet(fs, []string{"-n", "-z"})
	if err == nil {
		t.Fatal("ParseFlagSet(-n -z) = nil error, want an error instead of binding -n to \"-z\"")
	}
	if got := fs.Lookup("n").Value.String(); got != "0" {
		t.Fatalf("expected -n to keep its default when the parse fails, got %s", got)
	}
	if got := fs.Lookup("z").Value.String(); got != "false" {
		t.Fatalf("expected -z to keep its default when the parse fails, got %s", got)
	}
}

// TestParseFlagSetPermuteRejectsSwallowingDefinedFlagEndToEnd is the permuted
// counterpart: np and other commands using ParseFlagSetPermute must also
// surface the error rather than silently reordering their way around it.
func TestParseFlagSetPermuteRejectsSwallowingDefinedFlagEndToEnd(t *testing.T) {
	fs := newTestFlagSet()
	err := ParseFlagSetPermute(fs, []string{"host", "-n", "-z"})
	if err == nil {
		t.Fatal("ParseFlagSetPermute(host -n -z) = nil error, want an error instead of binding -n to \"-z\"")
	}
	if got := fs.Lookup("n").Value.String(); got != "0" {
		t.Fatalf("expected -n to keep its default when the parse fails, got %s", got)
	}
	if got := fs.Lookup("z").Value.String(); got != "false" {
		t.Fatalf("expected -z to keep its default when the parse fails, got %s", got)
	}
}

func TestPermuteArgs(t *testing.T) {
	fs := newTestFlagSet()
	short := func(c byte) (bool, bool) {
		f := fs.Lookup(string(c))
		if f == nil {
			return false, false
		}
		return true, !isBoolFlag(f)
	}
	long := func(name string) (bool, bool) {
		f := fs.Lookup(name)
		if f == nil {
			return false, false
		}
		return true, !isBoolFlag(f)
	}
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"value flag after positional", []string{"ports", "-n", "1", "host"}, []string{"-n", "1", "ports", "host"}},
		{"bool after positional", []string{"host", "-z"}, []string{"-z", "host"}},
		{"attached value after positional", []string{"host", "-n5"}, []string{"-n5", "host"}},
		{"already ordered untouched", []string{"-z", "-n", "1", "host"}, []string{"-z", "-n", "1", "host"}},
		{"double-dash keeps rest positional", []string{"-z", "--", "-n", "host"}, []string{"-z", "--", "-n", "host"}},
		{"unknown option treated as positional", []string{"-q", "host"}, []string{"-q", "host"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PermuteArgs(tc.in, short, long)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("PermuteArgs(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
