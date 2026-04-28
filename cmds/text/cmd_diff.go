package text

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type diffExitError int

func (e diffExitError) Error() string          { return fmt.Sprintf("exit code %d", int(e)) }
func (e diffExitError) ExitCode() int          { return int(e) }
func (e diffExitError) SuppressCLIError() bool { return true }

type diffOptions struct {
	unified         bool
	brief           bool
	recursive       bool
	newFile         bool
	stripTrailingCR bool
}

type diffFile struct {
	name   string
	data   []byte
	exists bool
}

type diffLine struct {
	text string
}

type diffOp struct {
	kind byte
	old  diffLine
	new  diffLine
}

func DiffCmd(args []string) error {
	opts, files, err := parseDiffArgs(args)
	if err != nil {
		return err
	}
	if files == nil {
		return nil
	}
	different, err := diffPaths(files[0], files[1], opts)
	if err != nil {
		return err
	}
	if different {
		return diffExitError(1)
	}
	return nil
}

func parseDiffArgs(args []string) (diffOptions, []string, error) {
	var opts diffOptions
	fsFlags := flag.NewFlagSet("diff", flag.ContinueOnError)
	fsFlags.BoolVar(&opts.unified, "u", false, "output unified diff")
	fsFlags.BoolVar(&opts.unified, "unified", false, "output unified diff")
	fsFlags.BoolVar(&opts.brief, "q", false, "report only whether files differ")
	fsFlags.BoolVar(&opts.brief, "brief", false, "report only whether files differ")
	fsFlags.BoolVar(&opts.recursive, "r", false, "recursively compare directories")
	fsFlags.BoolVar(&opts.recursive, "recursive", false, "recursively compare directories")
	fsFlags.BoolVar(&opts.newFile, "N", false, "treat missing files as empty")
	fsFlags.BoolVar(&opts.newFile, "new-file", false, "treat missing files as empty")
	fsFlags.BoolVar(&opts.stripTrailingCR, "strip-trailing-cr", false, "strip trailing carriage returns")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox diff [OPTION]... FILE1 FILE2")
		fmt.Fprintln(os.Stderr, "Compare files line by line.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -u, --unified            output unified diff")
		fmt.Fprintln(os.Stderr, "  -q, --brief              report only whether files differ")
		fmt.Fprintln(os.Stderr, "  -r, --recursive          recursively compare directories")
		fmt.Fprintln(os.Stderr, "  -N, --new-file           treat missing files as empty")
		fmt.Fprintln(os.Stderr, "  --strip-trailing-cr      strip trailing carriage returns")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return opts, nil, nil
		}
		return opts, nil, err
	}
	files := fsFlags.Args()
	if len(files) != 2 {
		return opts, nil, fmt.Errorf("diff requires two operands")
	}
	if files[0] == "-" && files[1] == "-" {
		return opts, nil, fmt.Errorf("diff: both operands cannot be standard input")
	}
	return opts, files, nil
}

func diffPaths(a, b string, opts diffOptions) (bool, error) {
	if a == "-" || b == "-" {
		left, err := readDiffFile(a, opts.newFile)
		if err != nil {
			return false, err
		}
		right, err := readDiffFile(b, opts.newFile)
		if err != nil {
			return false, err
		}
		return diffFilePair(left, right, opts)
	}

	infoA, errA := os.Lstat(a)
	infoB, errB := os.Lstat(b)
	if err := missingDiffPathError(a, errA, opts.newFile); err != nil {
		return false, err
	}
	if err := missingDiffPathError(b, errB, opts.newFile); err != nil {
		return false, err
	}
	if errA != nil || errB != nil {
		left, err := readDiffFile(a, opts.newFile)
		if err != nil {
			return false, err
		}
		right, err := readDiffFile(b, opts.newFile)
		if err != nil {
			return false, err
		}
		return diffFilePair(left, right, opts)
	}
	if infoA.IsDir() || infoB.IsDir() {
		if !infoA.IsDir() || !infoB.IsDir() {
			return false, fmt.Errorf("diff: file/directory comparisons require matching operand types")
		}
		if !opts.recursive {
			return false, fmt.Errorf("diff: %s and %s are directories; use -r", a, b)
		}
		return diffDirectories(a, b, opts)
	}
	left, err := readDiffFile(a, false)
	if err != nil {
		return false, err
	}
	right, err := readDiffFile(b, false)
	if err != nil {
		return false, err
	}
	return diffFilePair(left, right, opts)
}

func missingDiffPathError(path string, err error, newFile bool) error {
	if err == nil || newFile && os.IsNotExist(err) {
		return nil
	}
	return err
}

func readDiffFile(path string, newFile bool) (diffFile, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return diffFile{}, err
		}
		return diffFile{name: path, data: data, exists: true}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if newFile && os.IsNotExist(err) {
			return diffFile{name: path, exists: false}, nil
		}
		return diffFile{}, err
	}
	return diffFile{name: path, data: data, exists: true}, nil
}

func diffDirectories(a, b string, opts diffOptions) (bool, error) {
	entriesA, err := collectDiffFiles(a)
	if err != nil {
		return false, err
	}
	entriesB, err := collectDiffFiles(b)
	if err != nil {
		return false, err
	}
	names := make(map[string]struct{}, len(entriesA)+len(entriesB))
	for name := range entriesA {
		names[name] = struct{}{}
	}
	for name := range entriesB {
		names[name] = struct{}{}
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	different := false
	for _, rel := range sorted {
		left, leftOK := entriesA[rel]
		right, rightOK := entriesB[rel]
		leftName := filepath.Join(a, rel)
		rightName := filepath.Join(b, rel)
		if !leftOK || !rightOK {
			different = true
			if !opts.newFile {
				base := a
				if !leftOK {
					base = b
				}
				fmt.Printf("Only in %s: %s\n", filepath.Join(base, filepath.Dir(rel)), filepath.Base(rel))
				continue
			}
		}
		if leftOK && rightOK && (left.IsDir() || right.IsDir()) {
			if left.IsDir() != right.IsDir() {
				return false, fmt.Errorf("diff: %s and %s are different file types", leftName, rightName)
			}
			continue
		}
		if opts.newFile && ((!leftOK && right.IsDir()) || (!rightOK && left.IsDir())) {
			continue
		}
		dfA := diffFile{name: leftName, exists: leftOK}
		dfB := diffFile{name: rightName, exists: rightOK}
		if leftOK {
			data, err := os.ReadFile(leftName)
			if err != nil {
				return false, err
			}
			dfA.data = data
		}
		if rightOK {
			data, err := os.ReadFile(rightName)
			if err != nil {
				return false, err
			}
			dfB.data = data
		}
		if !opts.brief && !diffFilesEqual(dfA.data, dfB.data, opts) {
			fmt.Printf("diff -r %s %s\n", leftName, rightName)
		}
		fileDifferent, err := diffFilePair(dfA, dfB, opts)
		if err != nil {
			return false, err
		}
		if fileDifferent {
			different = true
		}
	}
	return different, nil
}

func collectDiffFiles(root string) (map[string]os.FileInfo, error) {
	files := make(map[string]os.FileInfo)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[rel] = info
		return nil
	})
	return files, err
}

func diffFilePair(a, b diffFile, opts diffOptions) (bool, error) {
	if diffFilesEqual(a.data, b.data, opts) {
		return false, nil
	}
	if opts.brief {
		fmt.Printf("Files %s and %s differ\n", a.name, b.name)
		return true, nil
	}
	if isBinaryDiffData(a.data) || isBinaryDiffData(b.data) {
		fmt.Printf("Binary files %s and %s differ\n", a.name, b.name)
		return true, nil
	}
	oldLines := splitDiffLines(a.data, opts.stripTrailingCR)
	newLines := splitDiffLines(b.data, opts.stripTrailingCR)
	ops := buildDiffOps(oldLines, newLines)
	if opts.unified {
		printUnifiedDiff(a.name, b.name, ops, len(oldLines), len(newLines))
	} else {
		printNormalDiff(ops)
	}
	return true, nil
}

func diffFilesEqual(a, b []byte, opts diffOptions) bool {
	if bytes.Equal(a, b) {
		return true
	}
	return opts.stripTrailingCR && bytes.Equal(stripTrailingCRBytes(a), stripTrailingCRBytes(b))
}

func stripTrailingCRBytes(data []byte) []byte {
	lines := splitDiffLines(data, true)
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line.text)
	}
	if len(data) > 0 && data[len(data)-1] == '\n' {
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func isBinaryDiffData(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0
}

func splitDiffLines(data []byte, stripTrailingCR bool) []diffLine {
	if len(data) == 0 {
		return nil
	}
	parts := strings.Split(string(data), "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	lines := make([]diffLine, len(parts))
	for i, part := range parts {
		if stripTrailingCR {
			part = strings.TrimSuffix(part, "\r")
		}
		lines[i] = diffLine{text: part}
	}
	return lines
}

func buildDiffOps(oldLines, newLines []diffLine) []diffOp {
	m, n := len(oldLines), len(newLines)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if oldLines[i].text == newLines[j].text {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var ops []diffOp
	for i, j := 0, 0; i < m || j < n; {
		switch {
		case i < m && j < n && oldLines[i].text == newLines[j].text:
			ops = append(ops, diffOp{kind: ' ', old: oldLines[i], new: newLines[j]})
			i++
			j++
		case j < n && (i == m || dp[i][j+1] > dp[i+1][j]):
			ops = append(ops, diffOp{kind: '+', new: newLines[j]})
			j++
		default:
			ops = append(ops, diffOp{kind: '-', old: oldLines[i]})
			i++
		}
	}
	return ops
}

func printNormalDiff(ops []diffOp) {
	oldLine, newLine := 1, 1
	for i := 0; i < len(ops); {
		if ops[i].kind == ' ' {
			oldLine++
			newLine++
			i++
			continue
		}
		oldStart, newStart := oldLine, newLine
		var dels, adds []string
		for i < len(ops) && ops[i].kind != ' ' {
			switch ops[i].kind {
			case '-':
				dels = append(dels, ops[i].old.text)
				oldLine++
			case '+':
				adds = append(adds, ops[i].new.text)
				newLine++
			}
			i++
		}
		switch {
		case len(dels) > 0 && len(adds) > 0:
			fmt.Printf("%sc%s\n", normalRange(oldStart, len(dels)), normalRange(newStart, len(adds)))
			for _, line := range dels {
				fmt.Printf("< %s\n", line)
			}
			fmt.Println("---")
			for _, line := range adds {
				fmt.Printf("> %s\n", line)
			}
		case len(dels) > 0:
			fmt.Printf("%sd%s\n", normalRange(oldStart, len(dels)), normalRange(newStart-1, 1))
			for _, line := range dels {
				fmt.Printf("< %s\n", line)
			}
		case len(adds) > 0:
			fmt.Printf("%sa%s\n", normalRange(oldStart-1, 1), normalRange(newStart, len(adds)))
			for _, line := range adds {
				fmt.Printf("> %s\n", line)
			}
		}
	}
}

func normalRange(start, count int) string {
	if count <= 1 {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, start+count-1)
}

func printUnifiedDiff(oldName, newName string, ops []diffOp, oldCount, newCount int) {
	fmt.Printf("--- %s\n", oldName)
	fmt.Printf("+++ %s\n", newName)
	fmt.Printf("@@ -%s +%s @@\n", unifiedRange(1, oldCount), unifiedRange(1, newCount))
	for _, op := range ops {
		switch op.kind {
		case ' ':
			fmt.Printf(" %s\n", op.old.text)
		case '-':
			fmt.Printf("-%s\n", op.old.text)
		case '+':
			fmt.Printf("+%s\n", op.new.text)
		}
	}
}

func unifiedRange(start, count int) string {
	if count == 0 {
		return "0,0"
	}
	return fmt.Sprintf("%d,%d", start, count)
}
