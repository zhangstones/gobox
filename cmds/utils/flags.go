package utils

import (
	"flag"
	"fmt"
	"strings"
)

// ShortFlagClassifier reports, for a single-letter short flag, whether it is
// defined and whether it consumes a value.
type ShortFlagClassifier func(name byte) (defined, takesValue bool)

// LongFlagClassifier reports, for a long flag name (without leading dashes),
// whether it is defined and whether it consumes a value. It covers both
// double-dash options (--output) and single-dash long names (find's -name).
type LongFlagClassifier func(name string) (defined, takesValue bool)

// ParseFlagSet parses args into fs after normalizing GNU-style short-option
// clusters that the standard library flag package does not understand on its
// own, namely bundled boolean flags (-zv == -z -v) and attached values
// (-n1 == -n 1). Commands should call this instead of fs.Parse(args).
func ParseFlagSet(fs *flag.FlagSet, args []string) error {
	short, long := classifiersFor(fs)
	expanded, err := ExpandShortClusters(args, short, long)
	if err != nil {
		return err
	}
	return fs.Parse(expanded)
}

// ParseFlagSetPermute is like ParseFlagSet but first reorders args so that
// options (and any values they consume) precede positional operands, matching
// GNU getopt's default argument permutation. Use it for commands whose options
// may legitimately follow a positional (e.g. `np --scan PORTS -W 1 HOST`).
func ParseFlagSetPermute(fs *flag.FlagSet, args []string) error {
	short, long := classifiersFor(fs)
	permuted := PermuteArgs(args, short, long)
	expanded, err := ExpandShortClusters(permuted, short, long)
	if err != nil {
		return err
	}
	return fs.Parse(expanded)
}

func classifiersFor(fs *flag.FlagSet) (ShortFlagClassifier, LongFlagClassifier) {
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
	return short, long
}

// PermuteArgs reorders args so recognized options and the values they consume
// come before positional operands, leaving the relative order within each group
// intact. Everything after a literal "--" is treated as positional. Tokens that
// look like options but name no defined flag are treated as positionals so they
// surface to the parser unchanged.
func PermuteArgs(args []string, short ShortFlagClassifier, long LongFlagClassifier) []string {
	opts := make([]string, 0, len(args))
	pos := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' {
			pos = append(pos, a)
			continue
		}
		if strings.HasPrefix(a, "--") {
			opts = append(opts, a)
			if long != nil && !strings.Contains(a, "=") {
				if defined, takesValue := long(a[2:]); defined && takesValue && i+1 < len(args) {
					opts = append(opts, args[i+1])
					i++
				}
			}
			continue
		}
		body := a[1:]
		if long != nil {
			if defined, takesValue := long(body); defined {
				opts = append(opts, a)
				if takesValue && i+1 < len(args) {
					opts = append(opts, args[i+1])
					i++
				}
				continue
			}
		}
		_, expectsNext, ok := expandShortCluster(a, short)
		if !ok {
			pos = append(pos, a)
			continue
		}
		opts = append(opts, a)
		if expectsNext && i+1 < len(args) {
			opts = append(opts, args[i+1])
			i++
		}
	}
	return append(opts, pos...)
}

// ExpandShortClusters rewrites GNU-style short-option clusters into the spaced
// forms understood by simple token-matching parsers. It mirrors the flag
// package's parsing boundaries: option processing stops at "--", at a bare "-",
// or at the first non-option argument, after which remaining args pass through
// verbatim.
//
// Tokens that already name a defined flag (a single-dash long name like -name,
// or a canonical -A) are left untouched; only bundled/attached short clusters
// are expanded. Whenever a flag takes its value from the following argument —
// whether a long option, a canonical short flag, or the trailing flag of a
// cluster — ExpandShortClusters requires that following argument to NOT itself
// be a recognized flag. If it is (e.g. `kill -f --dry-run PATTERN`, where -f
// takes a value and --dry-run is a real boolean flag), consuming it as -f's
// value would silently disable --dry-run and use the literal string
// "--dry-run" as the kill pattern instead — so this returns an error ("flag
// needs an argument: -f") rather than swallowing it.
//
// long may be nil for parsers that have no long options taking a
// space-separated value.
func ExpandShortClusters(args []string, short ShortFlagClassifier, long LongFlagClassifier) ([]string, error) {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" || len(a) < 2 || a[0] != '-' {
			out = append(out, args[i:]...)
			break
		}
		// Double-dash long option.
		if strings.HasPrefix(a, "--") {
			out = append(out, a)
			if long != nil && !strings.Contains(a, "=") {
				if defined, takesValue := long(a[2:]); defined && takesValue {
					val, err := takeNextValue(args, i, short, long)
					if err != nil {
						return nil, err
					}
					out = append(out, val)
					i++
				}
			}
			continue
		}
		body := a[1:]
		// A token that already names a defined flag is canonical (-A, -name,
		// -type, -size): leave it, and pass its value through if it takes one.
		if long != nil {
			if defined, takesValue := long(body); defined {
				out = append(out, a)
				if takesValue {
					val, err := takeNextValue(args, i, short, long)
					if err != nil {
						return nil, err
					}
					out = append(out, val)
					i++
				}
				continue
			}
		}
		expanded, expectsNext, ok := expandShortCluster(a, short)
		if !ok {
			out = append(out, a)
			continue
		}
		out = append(out, expanded...)
		if expectsNext {
			val, err := takeNextValue(args, i, short, long)
			if err != nil {
				return nil, err
			}
			out = append(out, val)
			i++
		}
	}
	return out, nil
}

// takeNextValue returns args[i+1] to bind as the value for the value-taking
// option token args[i], refusing to hand back a token that is itself a
// recognized flag. Without this guard, a value-taking option placed right
// before another real flag (most commonly a safety switch like --dry-run)
// would silently absorb that flag's name as its own value, leaving the
// intended flag at its default and the option matching on unexpected text.
func takeNextValue(args []string, i int, short ShortFlagClassifier, long LongFlagClassifier) (string, error) {
	if i+1 >= len(args) || isExactRegisteredFlag(args[i+1], short, long) {
		return "", fmt.Errorf("flag needs an argument: %s", args[i])
	}
	return args[i+1], nil
}

// isExactRegisteredFlag reports whether tok is unambiguously another flag
// rather than a value, by actually consulting this FlagSet's own short/long
// classifiers: either an exact (optionally "="-valued) match of a registered
// long option (--dry-run, --dry-run=x), or a single registered short letter
// with nothing else attached (-x). It deliberately does NOT flag longer
// single-dash tokens whose first letter merely happens to match a defined
// short flag (e.g. "-pid", "-sI"): those collide with real, intentional GNU
// conventions elsewhere in this codebase — ps's `--sort -pid` (leading '-'
// means descending) and curl's `-d -sI` (arbitrary data that happens to look
// like a flag cluster) both rely on such tokens being taken literally as
// values, not rejected as flags. Restricting the check to exact matches
// still catches the reported danger (`kill -f --dry-run PATTERN` silently
// disabling the dry-run switch) without misfiring on legitimate short,
// dash-prefixed values.
//
// This is deliberately a different, more precise check than the exported
// LooksLikeFlag below: that one has no flag registry to consult (it's used
// by hand-rolled parsers with no FlagSet) and so can only go by shape, not by
// what's actually registered. Do not use them interchangeably.
func isExactRegisteredFlag(tok string, short ShortFlagClassifier, long LongFlagClassifier) bool {
	if len(tok) < 2 || tok[0] != '-' {
		return false
	}
	if strings.HasPrefix(tok, "--") {
		name := tok[2:]
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			name = name[:idx]
		}
		if name == "" || long == nil {
			return false
		}
		defined, _ := long(name)
		return defined
	}
	body := tok[1:]
	// Single-dash long names (find's -name, -maxdepth, ...): an exact match
	// of the whole remaining string against a registered long flag is
	// unambiguous, unlike matching just its first letter against a short
	// flag, so it carries none of the -pid/-sI false-positive risk above.
	if long != nil {
		if defined, _ := long(body); defined {
			return true
		}
	}
	if len(tok) == 2 && short != nil {
		if defined, _ := short(tok[1]); defined {
			return true
		}
	}
	return false
}

// expandShortFlags is a thin FlagSet-based wrapper retained for tests.
func expandShortFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	short, long := classifiersFor(fs)
	return ExpandShortClusters(args, short, long)
}

// expandShortCluster expands a single "-xxx" short cluster. expectsNext is true
// when the cluster ends in a value flag with no attached value, meaning the
// following argument is that flag's value. ok is false when any leading rune is
// not a defined single-letter flag, so the token should be left unchanged.
func expandShortCluster(token string, short ShortFlagClassifier) (expanded []string, expectsNext, ok bool) {
	body := token[1:]
	result := make([]string, 0, len(body))
	for j := 0; j < len(body); j++ {
		name := body[j]
		defined, takesValue := short(name)
		if !defined {
			return nil, false, false
		}
		if !takesValue {
			result = append(result, "-"+string(name))
			continue
		}
		result = append(result, "-"+string(name))
		if rest := body[j+1:]; rest != "" {
			rest = strings.TrimPrefix(rest, "=")
			result = append(result, rest)
			return result, false, true
		}
		return result, true, true
	}
	return result, false, true
}

// LooksLikeFlag is a coarse heuristic for the hand-rolled per-command parsers
// (curl, nc, tw, sort, seq, sed, rand, dig/nslookup, ...) that parse args with
// their own token loop instead of going through ParseFlagSet/ExpandShortClusters
// and so have no flag registry to consult (contrast with the classifier-aware,
// exact-match isExactRegisteredFlag above — the two are not interchangeable).
// It reports whether tok is unambiguously shaped like an option rather than a
// value: a "--long" style token (virtually never a legitimate literal value),
// or a bare "-x" single short flag. It deliberately does NOT flag longer
// single-dash clusters or attached-value tokens (e.g. "-sI", "-pid"): those
// collide with real, intentional GNU conventions elsewhere in this codebase,
// such as curl's `-d -sI` (arbitrary POST data that happens to look like a
// flag cluster) — real curl takes such values literally too, so treating them
// as flags here would be a gobox-specific regression, not a fix.
//
// A bare "-x" is only treated as a value instead of a flag when x is a digit
// (e.g. "-5"), a legitimate single-digit negative number; anything else
// two-character and dash-prefixed (like "-w") is a flag.
func LooksLikeFlag(tok string) bool {
	if len(tok) < 2 || tok[0] != '-' {
		return false
	}
	if strings.HasPrefix(tok, "--") {
		return len(tok) > 2
	}
	if len(tok) != 2 {
		return false
	}
	if tok[1] >= '0' && tok[1] <= '9' {
		return false
	}
	return true
}

func isBoolFlag(f *flag.Flag) bool {
	if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
		return bf.IsBoolFlag()
	}
	return false
}
