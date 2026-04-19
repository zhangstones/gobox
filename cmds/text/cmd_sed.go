package text

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// sedCmd implements a subset of sed functionality
func SedCmd(args []string) error {
	var (
		quiet       bool
		inPlace     string
		expressions []string
		scriptFile  string
		showHelp    bool
	)

	// Manual flag parsing to handle -i.bak style
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-n":
			quiet = true
		case arg == "-h" || arg == "--help":
			showHelp = true
		case arg == "-e":
			if i+1 >= len(args) {
				return fmt.Errorf("-e requires an argument")
			}
			i++
			expressions = append(expressions, args[i])
		case arg == "-f":
			if i+1 >= len(args) {
				return fmt.Errorf("-f requires an argument")
			}
			i++
			scriptFile = args[i]
		case arg == "-i":
			// -i without backup suffix
			inPlace = ""
			// Mark that we saw -i, but don't consume next arg
			// Next arg should be the script or file
		case strings.HasPrefix(arg, "-i") && len(arg) > 2:
			// Handle -i.bak style
			inPlace = arg[2:]
		case strings.HasPrefix(arg, "-i") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-"):
			// Handle -i backup (separate arg)
			i++
			inPlace = args[i]
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown option: %s", arg)
			}
			// Not a flag, stop parsing
			goto doneFlags
		}
		i++
	}
doneFlags:

	if showHelp {
		printUsage(os.Stdout)
		return nil
	}

	// Collect scripts from -e
	scripts := append([]string{}, expressions...)

	// Collect scripts from -f
	if scriptFile != "" {
		content, err := os.ReadFile(scriptFile)
		if err != nil {
			return fmt.Errorf("cannot read script file %s: %w", scriptFile, err)
		}
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				scripts = append(scripts, line)
			}
		}
	}

	// Remaining args: first is script (if no -e/-f), rest are files
	remaining := args[i:]
	if len(scripts) == 0 && len(remaining) > 0 {
		scripts = append(scripts, remaining[0])
		remaining = remaining[1:]
	}

	if len(scripts) == 0 {
		fmt.Fprintln(os.Stderr, "sed: no script provided")
		printUsage(os.Stderr)
		return fmt.Errorf("script required")
	}

	files := remaining

	// Parse scripts into commands
	commands, err := parseScripts(scripts)
	if err != nil {
		return err
	}

	// If no files, read from stdin
	if len(files) == 0 {
		if err := sedReader(os.Stdin, os.Stdout, commands, quiet); err != nil {
			return err
		}
		return nil
	}

	// Process files
	for _, file := range files {
		if inPlace != "" || (i > 0 && args[i-1] == "-i") {
			if err := sedFileInPlace(file, commands, quiet, inPlace); err != nil {
				return err
			}
		} else {
			if err := sedFile(file, os.Stdout, commands, quiet); err != nil {
				return err
			}
		}
	}

	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox sed [OPTIONS] [SCRIPT] [FILE...]")
	fmt.Fprintln(w, "Stream editor for filtering and transforming text.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -n           Suppress automatic printing of pattern space")
	fmt.Fprintln(w, "  -i[SUFFIX]   Edit files in place (makes backup if SUFFIX supplied)")
	fmt.Fprintln(w, "  -e SCRIPT    Add the script to the commands to be executed")
	fmt.Fprintln(w, "  -f FILE      Add the contents of FILE to the commands to be executed")
	fmt.Fprintln(w, "  -h, --help   Show this help message")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  s/pattern/replacement/flags  Substitute pattern with replacement")
	fmt.Fprintln(w, "  d                            Delete pattern space")
	fmt.Fprintln(w, "  p                            Print pattern space")
	fmt.Fprintln(w, "  =                            Print current line number")
	fmt.Fprintln(w, "  i\\text                      Insert text before addressed line")
	fmt.Fprintln(w, "  a\\text                      Append text after addressed line")
	fmt.Fprintln(w, "  c\\text                      Change addressed line to text")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Substitute flags:")
	fmt.Fprintln(w, "  g  Global replacement (all occurrences)")
	fmt.Fprintln(w, "  i  Case-insensitive matching")
	fmt.Fprintln(w, "  p  Print the line if substitution was made")
	fmt.Fprintln(w, "  N  Replace Nth occurrence (1-9)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox sed 's/foo/bar/' file.txt")
	fmt.Fprintln(w, "  gobox sed 's/foo/bar/g' file.txt")
	fmt.Fprintln(w, "  gobox sed -n 's/foo/bar/p' file.txt")
	fmt.Fprintln(w, "  gobox sed -i.bak 's/old/new/g' file.txt")
	fmt.Fprintln(w, "  gobox sed -e 's/foo/bar/' -e 's/baz/qux/' file.txt")
	fmt.Fprintln(w, "  gobox sed '/pattern/i\\NEW LINE' file.txt")
	fmt.Fprintln(w, "  gobox sed '3a\\AFTER LINE 3' file.txt")
	fmt.Fprintln(w, "  cat file.txt | gobox sed 's/old/new/g'")
}

// Command types
type cmdType int

const (
	cmdSubstitute cmdType = iota
	cmdDelete
	cmdPrint
	cmdPrintLineNum
	cmdInsert
	cmdAppend
	cmdChange
)

type sedCommand struct {
	typ             cmdType
	address         string
	addressNum      int // For numeric addresses like "3i", -1 for $ (last line)
	addressNumEnd   int // For range addresses like "2,4d" (0 = no range)
	pattern         *regexp.Regexp
	replacement     string
	text            string // For i/a/c commands
	flags           string
	global          bool
	caseInsensitive bool
	replaceNth      int
	printOnMatch    bool // For s///p flag
}

func parseScripts(scripts []string) ([]sedCommand, error) {
	var commands []sedCommand

	for _, script := range scripts {
		cmd, err := parseCommand(script)
		if err != nil {
			return nil, fmt.Errorf("invalid script '%s': %w", script, err)
		}
		commands = append(commands, cmd)
	}

	return commands, nil
}

func parseCommand(script string) (sedCommand, error) {
	script = strings.TrimSpace(script)
	cmd := sedCommand{}

	// Handle substitute command: s/pattern/replacement/flags
	if strings.HasPrefix(script, "s") {
		cmd.typ = cmdSubstitute
		return parseSubstitute(script[1:])
	}

	// Handle $ address (last line): $d, $p, etc.
	if strings.HasPrefix(script, "$") {
		cmd.address = "$"
		cmd.addressNum = -1 // Special marker for last line
		rest := script[1:]
		if len(rest) == 0 {
			return cmd, fmt.Errorf("missing command after $ address")
		}
		switch rest[0] {
		case 'd':
			cmd.typ = cmdDelete
		case 'p':
			cmd.typ = cmdPrint
		case '=':
			cmd.typ = cmdPrintLineNum
		default:
			return cmd, fmt.Errorf("unsupported command after $: %s", rest)
		}
		return cmd, nil
	}

	// Handle range address: 2,4d or 1,3p
	if len(script) >= 4 {
		// Check for pattern like "num,numcmd"
		commaIdx := strings.Index(script, ",")
		if commaIdx > 0 && commaIdx < len(script)-1 {
			num1Str := script[:commaIdx]
			rest := script[commaIdx+1:]

			// Find where num2 ends
			num2End := 0
			for num2End < len(rest) && rest[num2End] >= '0' && rest[num2End] <= '9' {
				num2End++
			}
			if num2End > 0 {
				num1, err1 := strconv.Atoi(num1Str)
				num2, err2 := strconv.Atoi(rest[:num2End])
				if err1 == nil && err2 == nil && num2End < len(rest) {
					cmd.addressNum = num1    // Start of range
					cmd.addressNumEnd = num2 // End of range
					cmdCmd := rest[num2End:]
					switch cmdCmd[0] {
					case 'd':
						cmd.typ = cmdDelete
						return cmd, nil
					case 'p':
						cmd.typ = cmdPrint
						return cmd, nil
					}
				}
			}
		}
	}

	// Handle numeric address + command: 1d, 2d, 3i, 5a, 2c, etc.
	if len(script) >= 2 {
		// Check for numeric address
		if script[0] >= '0' && script[0] <= '9' {
			idx := 0
			for idx < len(script) && script[idx] >= '0' && script[idx] <= '9' {
				idx++
			}
			if idx < len(script) {
				numStr := script[:idx]
				num, err := strconv.Atoi(numStr)
				if err == nil {
					cmd.addressNum = num
					cmdCmd := script[idx:]
					switch cmdCmd[0] {
					case 'd':
						cmd.typ = cmdDelete
						return cmd, nil
					case 'p':
						cmd.typ = cmdPrint
						return cmd, nil
					case 'i':
						cmd.typ = cmdInsert
						cmd.text = parseInsertText(cmdCmd[1:])
						return cmd, nil
					case 'a':
						cmd.typ = cmdAppend
						cmd.text = parseInsertText(cmdCmd[1:])
						return cmd, nil
					case 'c':
						cmd.typ = cmdChange
						cmd.text = parseInsertText(cmdCmd[1:])
						return cmd, nil
					case '=':
						cmd.typ = cmdPrintLineNum
						return cmd, nil
					}
				}
			}
		}
	}

	// Handle address + command: /pattern/d, /pattern/p, /pattern/i\text, /pattern/=, etc.
	if strings.HasPrefix(script, "/") {
		idx := strings.Index(script[1:], "/")
		if idx == -1 {
			return cmd, fmt.Errorf("invalid address pattern")
		}
		pattern := script[1 : idx+1]
		cmd.address = pattern
		var err error
		cmd.pattern, err = regexp.Compile(pattern)
		if err != nil {
			return cmd, fmt.Errorf("invalid regex: %w", err)
		}
		rest := script[idx+2:]

		if len(rest) == 0 {
			return cmd, fmt.Errorf("missing command after address")
		}

		switch rest[0] {
		case 'd':
			cmd.typ = cmdDelete
		case 'p':
			cmd.typ = cmdPrint
		case 'i':
			cmd.typ = cmdInsert
			cmd.text = parseInsertText(rest[1:])
		case 'a':
			cmd.typ = cmdAppend
			cmd.text = parseInsertText(rest[1:])
		case 'c':
			cmd.typ = cmdChange
			cmd.text = parseInsertText(rest[1:])
		case '=':
			cmd.typ = cmdPrintLineNum
		default:
			return cmd, fmt.Errorf("unsupported command: %s", rest)
		}
		return cmd, nil
	}

	// Simple commands
	switch script {
	case "d":
		cmd.typ = cmdDelete
	case "p":
		cmd.typ = cmdPrint
	case "=":
		cmd.typ = cmdPrintLineNum
	default:
		if strings.HasPrefix(script, "i\\") {
			cmd.typ = cmdInsert
			cmd.text = parseInsertText(script[1:])
			return cmd, nil
		}
		if strings.HasPrefix(script, "a\\") {
			cmd.typ = cmdAppend
			cmd.text = parseInsertText(script[1:])
			return cmd, nil
		}
		if strings.HasPrefix(script, "c\\") {
			cmd.typ = cmdChange
			cmd.text = parseInsertText(script[1:])
			return cmd, nil
		}
		return cmd, fmt.Errorf("unsupported command: %s", script)
	}

	return cmd, nil
}

func parseInsertText(rest string) string {
	// Handle i\text, a\text, c\text
	if len(rest) == 0 {
		return ""
	}

	// Skip backslash or space
	start := 0
	if rest[0] == '\\' {
		start = 1
	} else if rest[0] == ' ' {
		start = 1
	}

	return rest[start:]
}

func parseSubstitute(script string) (sedCommand, error) {
	cmd := sedCommand{typ: cmdSubstitute}

	if len(script) < 1 {
		return cmd, fmt.Errorf("empty substitute pattern")
	}

	delimiter := script[0]
	script = script[1:]

	// Split by delimiter, handling escapes
	// Only \delimiter is an escape sequence, \1, \2 etc are kept as-is
	var parts []string
	var current strings.Builder
	escaped := false
	for i := 0; i < len(script); i++ {
		c := script[i]
		if escaped {
			current.WriteByte(c)
			escaped = false
		} else if c == '\\' {
			// Check if next char is the delimiter
			if i+1 < len(script) && script[i+1] == delimiter {
				escaped = true
				// Don't write the backslash, next iteration will write the delimiter
			} else {
				// Keep the backslash (for \1, \2, etc.)
				current.WriteByte(c)
			}
		} else if c == delimiter {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteByte(c)
		}
	}
	parts = append(parts, current.String())

	if len(parts) < 2 {
		return cmd, fmt.Errorf("invalid substitute syntax: need at least pattern and replacement")
	}

	pattern := parts[0]
	replacement := parts[1]
	flags := ""
	if len(parts) >= 3 {
		flags = parts[2]
	}

	// Parse flags
	for _, f := range flags {
		switch f {
		case 'g':
			cmd.global = true
		case 'i':
			cmd.caseInsensitive = true
		case 'p':
			cmd.printOnMatch = true
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			n, _ := strconv.Atoi(string(f))
			cmd.replaceNth = n
		}
	}

	// Convert sed-style regex to Go regex
	// \( -> (, \) -> )
	pattern = strings.ReplaceAll(pattern, "\\(", "(")
	pattern = strings.ReplaceAll(pattern, "\\)", ")")

	// Compile regex
	var err error
	if cmd.caseInsensitive {
		cmd.pattern, err = regexp.Compile("(?i)" + pattern)
	} else {
		cmd.pattern, err = regexp.Compile(pattern)
	}
	if err != nil {
		return cmd, fmt.Errorf("invalid regex '%s': %w", pattern, err)
	}

	// Process replacement for backreferences
	cmd.replacement = processReplacement(replacement)
	cmd.flags = flags

	return cmd, nil
}

func processReplacement(replacement string) string {
	result := replacement
	for i := 9; i >= 1; i-- {
		old := "\\" + strconv.Itoa(i)
		new := "${" + strconv.Itoa(i) + "}"
		result = strings.ReplaceAll(result, old, new)
	}
	return result
}

func sedFile(filename string, out io.Writer, commands []sedCommand, quiet bool) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", filename, err)
	}
	defer file.Close()

	return sedReader(file, out, commands, quiet)
}

func sedFileInPlace(filename string, commands []sedCommand, quiet bool, backup string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", filename, err)
	}

	if backup != "" {
		backupName := filename + backup
		if err := os.WriteFile(backupName, content, 0644); err != nil {
			return fmt.Errorf("cannot create backup %s: %w", backupName, err)
		}
	}

	var output strings.Builder
	reader := strings.NewReader(string(content))
	if err := sedReader(reader, &output, commands, quiet); err != nil {
		return err
	}

	if err := os.WriteFile(filename, []byte(output.String()), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", filename, err)
	}

	return nil
}

func sedReader(r io.Reader, out io.Writer, commands []sedCommand, quiet bool) error {
	scanner := bufio.NewScanner(r)
	lineNum := 0
	var lines []string

	// First pass: read all lines
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// Second pass: process lines
	totalLines := len(lines)
	for lineNum < totalLines {
		line := lines[lineNum]
		printLine := !quiet
		deleteLine := false
		var insertTexts []string
		var appendTexts []string
		changeLine := ""
		changeMatch := false

		for _, cmd := range commands {
			matched := false

			// Check numeric address (including $ for last line)
			if cmd.addressNum != 0 {
				if cmd.addressNum == -1 {
					// $ address - last line
					if lineNum == totalLines-1 {
						matched = true
					}
				} else if cmd.addressNumEnd > 0 {
					// Range address: 2,4d
					if lineNum+1 >= cmd.addressNum && lineNum+1 <= cmd.addressNumEnd {
						matched = true
					}
				} else {
					// Single numeric address: 1d, 2d, etc.
					if lineNum+1 == cmd.addressNum {
						matched = true
					}
				}
			}

			// Check pattern address
			if cmd.address != "" && cmd.address != "$" && cmd.pattern != nil {
				if cmd.pattern.MatchString(line) {
					matched = true
				}
			}

			// If no address specified, command applies to all lines
			if cmd.addressNum == 0 && cmd.address == "" {
				matched = true
			}

			if !matched {
				continue
			}

			switch cmd.typ {
			case cmdSubstitute:
				if cmd.pattern == nil {
					continue
				}
				newLine, changed := applySubstitute(line, cmd)
				if changed {
					line = newLine
					if cmd.printOnMatch {
						printLine = true
					}
				}
			case cmdDelete:
				deleteLine = true
			case cmdPrint:
				printLine = true
			case cmdInsert:
				insertTexts = append(insertTexts, cmd.text)
			case cmdAppend:
				appendTexts = append(appendTexts, cmd.text)
			case cmdChange:
				changeLine = cmd.text
				changeMatch = true
			case cmdPrintLineNum:
				fmt.Fprintf(out, "%d\n", lineNum+1)
			}
		}

		// Output insert texts before current line
		for _, text := range insertTexts {
			fmt.Fprintln(out, text)
		}

		// Handle change command (replaces the entire line)
		if changeMatch {
			if !quiet {
				fmt.Fprintln(out, changeLine)
			}
			lineNum++
			continue
		}

		// Output current line if not deleted
		if !deleteLine && printLine {
			fmt.Fprintln(out, line)
		}

		// Output append texts after current line
		for _, text := range appendTexts {
			fmt.Fprintln(out, text)
		}

		lineNum++
	}

	return nil
}

func applySubstitute(line string, cmd sedCommand) (string, bool) {
	if !cmd.pattern.MatchString(line) {
		return line, false
	}

	if cmd.replaceNth > 0 {
		// Replace Nth occurrence
		count := 0
		result := cmd.pattern.ReplaceAllStringFunc(line, func(match string) string {
			count++
			if count == cmd.replaceNth {
				return cmd.pattern.ReplaceAllString(match, cmd.replacement)
			}
			return match
		})
		return result, result != line
	} else if cmd.global {
		result := cmd.pattern.ReplaceAllString(line, cmd.replacement)
		return result, result != line
	} else {
		// Replace first occurrence only
		idx := cmd.pattern.FindStringIndex(line)
		if idx == nil {
			return line, false
		}
		before := line[:idx[0]]
		matched := line[idx[0]:idx[1]]
		after := line[idx[1]:]
		replaced := cmd.pattern.ReplaceAllString(matched, cmd.replacement)
		result := before + replaced + after
		return result, result != line
	}
}
