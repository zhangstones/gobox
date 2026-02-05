package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPathDepth(t *testing.T) {
	cases := map[string]int{
		"":      0,
		".":     0,
		string(filepath.Separator): 0,
		"a":     1,
		"a/b":   2,
		"a/b/":  2,
		"a/b/c": 3,
	}
	for input, want := range cases {
		if got := pathDepth(input); got != want {
			t.Fatalf("pathDepth(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParseSize(t *testing.T) {
	if _, _, err := parseSize(""); err == nil {
		t.Fatalf("expected error for empty size spec")
	}
	size, op, err := parseSize("+10K")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 10*1024 || op != 1 {
		t.Fatalf("unexpected parseSize result: size=%d op=%d", size, op)
	}
}

func TestMatchSize(t *testing.T) {
	if matchSize(5, "bogus") {
		t.Fatalf("expected bogus size spec to return false")
	}
	if !matchSize(12, "+10") {
		t.Fatalf("expected size to match +10")
	}
	if matchSize(9, "+10") {
		t.Fatalf("expected size 9 to not match +10")
	}
	if !matchSize(9, "-10") {
		t.Fatalf("expected size 9 to match -10")
	}
	if !matchSize(10, "10") {
		t.Fatalf("expected size 10 to match 10")
	}
}

func TestParseTime(t *testing.T) {
	if _, _, err := parseTime(""); err == nil {
		t.Fatalf("expected error for empty time spec")
	}
	dur, op, err := parseTime("-2h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op != -1 || dur != 2*time.Hour {
		t.Fatalf("unexpected parseTime result: op=%d dur=%v", op, dur)
	}
}

func TestMatchTimeMTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if !matchTime(info, "1d", "mtime") {
		t.Fatalf("expected mtime to match within 1 day")
	}
	if matchTime(info, "bogus", "mtime") {
		t.Fatalf("expected bogus time spec to return false")
	}
	if matchTime(info, "1d", "unsupported") {
		t.Fatalf("expected unsupported time type to return false")
	}
}

func TestMatchTimeATimeOlder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if !matchTime(info, "-1h", "atime") {
		t.Fatalf("expected atime older than 1h to match")
	}
}
