package proc

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"gobox/cmds/utils"
)

var (
	readMemInfoData = readMemInfo
	freeSleep       = time.Sleep
)

func FreeCmd(args []string) error {
	fsFlags := flag.NewFlagSet("free", flag.ContinueOnError)
	human := fsFlags.Bool("h", false, "human readable")
	bytesUnit := fsFlags.Bool("b", false, "show bytes")
	kiB := fsFlags.Bool("k", false, "show KiB (default)")
	miB := fsFlags.Bool("m", false, "show MiB")
	giB := fsFlags.Bool("g", false, "show GiB")
	interval := fsFlags.Int("s", 0, "repeat every SEC seconds")
	count := fsFlags.Int("c", 1, "repeat COUNT times")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox free [OPTION]...")
		fmt.Fprintln(os.Stderr, "Display memory and swap usage.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Units:")
		fmt.Fprintln(os.Stderr, "  -h          human readable")
		fmt.Fprintln(os.Stderr, "  -b          show bytes")
		fmt.Fprintln(os.Stderr, "  -k          show KiB (default)")
		fmt.Fprintln(os.Stderr, "  -m          show MiB")
		fmt.Fprintln(os.Stderr, "  -g          show GiB")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Sampling:")
		fmt.Fprintln(os.Stderr, "  -s SEC      repeat every SEC seconds")
		fmt.Fprintln(os.Stderr, "  -c COUNT    repeat COUNT times")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "      --help  show this help")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	_ = kiB // -k is accepted for parity with native free; KiB is already the default unit
	if *count <= 0 {
		*count = 1
	}
	for i := 0; i < *count; i++ {
		if i > 0 {
			freeSleep(time.Duration(*interval) * time.Second)
		}
		mem, err := readMemInfoData()
		if err != nil {
			return err
		}
		printFree(mem, *human, *bytesUnit, *miB, *giB)
	}
	return nil
}

func readMemInfo() (map[string]uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseMemInfo(f)
}

func parseMemInfo(r io.Reader) (map[string]uint64, error) {
	out := map[string]uint64{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			out[strings.TrimSuffix(fields[0], ":")] = v * 1024
		}
	}
	return out, scanner.Err()
}

func printFree(m map[string]uint64, human, bytesUnit, miB, giB bool) {
	total := m["MemTotal"]
	free := m["MemFree"]
	shared := m["Shmem"]
	buffCache := m["Buffers"] + m["Cached"] + m["SReclaimable"]
	available := m["MemAvailable"]
	var used uint64
	if total > free+buffCache {
		used = total - free - buffCache
	}
	swapTotal := m["SwapTotal"]
	swapFree := m["SwapFree"]
	var swapUsed uint64
	if swapTotal > swapFree {
		swapUsed = swapTotal - swapFree
	}
	fmt.Printf("%13s %12s %12s %12s %12s %12s\n", "total", "used", "free", "shared", "buff/cache", "available")
	fmt.Printf("Mem:  %12s %12s %12s %12s %12s %12s\n", formatMem(total, human, bytesUnit, miB, giB), formatMem(used, human, bytesUnit, miB, giB), formatMem(free, human, bytesUnit, miB, giB), formatMem(shared, human, bytesUnit, miB, giB), formatMem(buffCache, human, bytesUnit, miB, giB), formatMem(available, human, bytesUnit, miB, giB))
	fmt.Printf("Swap: %12s %12s %12s\n", formatMem(swapTotal, human, bytesUnit, miB, giB), formatMem(swapUsed, human, bytesUnit, miB, giB), formatMem(swapFree, human, bytesUnit, miB, giB))
}

func formatMem(v uint64, human, bytesUnit, miB, giB bool) string {
	switch {
	case human:
		return utils.HumanSize(int64(v))
	case bytesUnit:
		return fmt.Sprintf("%d", v)
	case giB:
		return fmt.Sprintf("%d", v/1024/1024/1024)
	case miB:
		return fmt.Sprintf("%d", v/1024/1024)
	default:
		return fmt.Sprintf("%d", v/1024)
	}
}
