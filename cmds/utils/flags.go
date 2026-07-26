package utils

import (
	"flag"
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
	return fs.Parse(ExpandShortClusters(args, short, long))
}

// ParseFlagSetPermute is like ParseFlagSet but first reorders args so that
// options (and any values they consume) precede positional operands, matching
// GNU getopt's default argument permutation. Use it for commands whose options
// may legitimately follow a positional (e.g. `np --scan PORTS -W 1 HOST`).
func ParseFlagSetPermute(fs *flag.FlagSet, args []string) error {
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
	permuted := PermuteArgs(args, short, long)
	return fs.Parse(ExpandShortClusters(permuted, short, long))
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
// cluster — that following argument is passed through verbatim so it is never
// mistaken for another flag cluster.
//
// long may be nil for parsers that have no long options taking a
// space-separated value.
func ExpandShortClusters(args []string, short ShortFlagClassifier, long LongFlagClassifier) []string {
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
				if defined, takesValue := long(a[2:]); defined && takesValue && i+1 < len(args) {
					out = append(out, args[i+1])
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
				if takesValue && i+1 < len(args) {
					out = append(out, args[i+1])
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
		if expectsNext && i+1 < len(args) {
			out = append(out, args[i+1])
			i++
		}
	}
	return out
}

// expandShortFlags is a thin FlagSet-based wrapper retained for tests.
func expandShortFlags(fs *flag.FlagSet, args []string) []string {
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
			result = append(result, rest)
			return result, false, true
		}
		return result, true, true
	}
	return result, false, true
}

func isBoolFlag(f *flag.Flag) bool {
	if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
		return bf.IsBoolFlag()
	}
	return false
}
