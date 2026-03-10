package task

import (
	"fmt"
	"strconv"
	"strings"
)

// memUnits maps WDL memory unit strings to their multipliers in bytes.
// Covers both IEC binary units (KiB, MiB, GiB, TiB) and SI decimal units
// (kB, KB, MB, GB, TB) as well as the "B" and "bytes" forms.
var memUnits = map[string]float64{
	"B":     1,
	"bytes": 1,
	"kB":    1_000,
	"KB":    1_000,
	"MB":    1_000_000,
	"GB":    1_000_000_000,
	"TB":    1_000_000_000_000,
	"KiB":   1024,
	"MiB":   1024 * 1024,
	"GiB":   1024 * 1024 * 1024,
	"TiB":   1024 * 1024 * 1024 * 1024,
}

// ParseMemoryString converts a WDL memory string to an int64 byte count.
// Supported formats:
//   - "2 GiB"   -> 2147483648
//   - "512 MB"  -> 512000000
//   - "1.5 GiB" -> 1610612736
//   - "1024"    -> 1024 (bare integer is treated as bytes)
//
// Returns an error for unrecognised unit strings or malformed input.
func ParseMemoryString(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("parseMemoryString: empty string")
	}

	fields := strings.Fields(s)
	switch len(fields) {
	case 1:
		// Bare integer (bytes) or a bare unit without a number — treat as integer
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parseMemoryString: cannot parse %q as integer bytes: %v", s, err)
		}
		return n, nil
	case 2:
		num, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return 0, fmt.Errorf("parseMemoryString: cannot parse numeric part %q: %v", fields[0], err)
		}
		mult, ok := memUnits[fields[1]]
		if !ok {
			return 0, fmt.Errorf("parseMemoryString: unknown memory unit %q", fields[1])
		}
		return int64(num * mult), nil
	default:
		return 0, fmt.Errorf("parseMemoryString: unexpected format %q (expected '<number> <unit>' or '<integer>')", s)
	}
}
