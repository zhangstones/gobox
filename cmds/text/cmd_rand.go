package text

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type randConfig struct {
	numBytes int
	hex      bool
	base64   bool
	output   string
}

func RandCmd(args []string) error {
	cfg := randConfig{
		numBytes: 32,
		hex:      true,
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-n":
			if i+1 >= len(args) {
				return fmt.Errorf("-n requires an argument")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of bytes: %s", args[i])
			}
			cfg.numBytes = n
		case strings.HasPrefix(arg, "-n"):
			n, err := strconv.Atoi(arg[2:])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of bytes: %s", arg[2:])
			}
			cfg.numBytes = n
		case arg == "-hex":
			cfg.hex = true
			cfg.base64 = false
		case arg == "-base64":
			cfg.base64 = true
			cfg.hex = false
		case arg == "-out":
			if i+1 >= len(args) {
				return fmt.Errorf("-out requires an argument")
			}
			i++
			cfg.output = args[i]
		case arg == "-h", arg == "--help":
			printRandUsage(os.Stdout)
			return nil
		case strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--"):
			// Handle combined short flags
			for j := 1; j < len(arg); j++ {
				switch arg[j] {
				case 'n':
					if j+1 < len(arg) {
						n, err := strconv.Atoi(arg[j+1:])
						if err != nil || n < 0 {
							return fmt.Errorf("invalid number of bytes: %s", arg[j+1:])
						}
						cfg.numBytes = n
						goto doneFlags
					} else if i+1 < len(args) {
						n, err := strconv.Atoi(args[i+1])
						if err != nil || n < 0 {
							return fmt.Errorf("invalid number of bytes: %s", args[i+1])
						}
						cfg.numBytes = n
						goto doneFlags
					}
					return fmt.Errorf("-n requires an argument")
				case 'h':
					cfg.hex = true
					cfg.base64 = false
				case 'b':
					cfg.base64 = true
					cfg.hex = false
				default:
					return fmt.Errorf("unknown option: -%c", arg[j])
				}
			}
			goto doneFlags
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown option: %s", arg)
			}
			// Positional argument: number of bytes
			n, err := strconv.Atoi(arg)
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of bytes: %s", arg)
			}
			cfg.numBytes = n
		}
		i++
	}

doneFlags:

	// Generate random bytes
	data := make([]byte, cfg.numBytes)
	_, err := rand.Read(data)
	if err != nil {
		return fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Format output
	var outData string
	if cfg.base64 {
		outData = base64.StdEncoding.EncodeToString(data)
	} else {
		outData = hex.EncodeToString(data)
	}

	// Write output
	var out io.Writer = os.Stdout
	if cfg.output != "" {
		f, err := os.Create(cfg.output)
		if err != nil {
			return fmt.Errorf("cannot create output file: %w", err)
		}
		out = f
		defer f.Close()
	}

	fmt.Fprintln(out, outData)
	return nil
}

func printRandUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox rand [OPTIONS] [NUM]")
	fmt.Fprintln(w, "Generate random bytes.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -n NUM, -NUM    Number of bytes to generate (default: 32)")
	fmt.Fprintln(w, "  -hex            Hex output (default)")
	fmt.Fprintln(w, "  -base64         Base64 output")
	fmt.Fprintln(w, "  -out FILE       Write to FILE")
	fmt.Fprintln(w, "  -h, --help      Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox rand 32                      Generate 32 random bytes (hex)")
	fmt.Fprintln(w, "  gobox rand -n 32 -hex              Generate 32 random bytes (hex)")
	fmt.Fprintln(w, "  gobox rand -n 24 -base64           Generate 24 random bytes (base64)")
	fmt.Fprintln(w, "  gobox rand -n 16 -out /tmp/key     Generate 16 bytes to file")
}
