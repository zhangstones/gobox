package proc

import (
	"testing"
)

func TestTruncateString(t *testing.T) {
	if got := truncateString("hello", 0); got != "hello" {
		t.Fatalf("expected no truncation, got %q", got)
	}
	if got := truncateString("hello", 3); got != "hel" {
		t.Fatalf("expected hard truncation to 3, got %q", got)
	}
	if got := truncateString("hello", 4); got != "h..." {
		t.Fatalf("expected ellipsis truncation, got %q", got)
	}
	if got := truncateString("hi", 5); got != "hi" {
		t.Fatalf("expected no truncation, got %q", got)
	}
}
