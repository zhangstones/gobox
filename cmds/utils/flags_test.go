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
		{"value flag no attached value untouched", []string{"-n"}, []string{"-n"}},
		{"empty", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := expandShortFlags(newTestFlagSet(), tc.in)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expandShortFlags(%q) = %q, want %q", tc.in, got, tc.want)
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
