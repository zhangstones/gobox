package main

import (
	"strings"
	"testing"
)

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
