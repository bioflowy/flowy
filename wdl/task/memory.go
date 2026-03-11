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

// splitNumberUnit splits a concatenated number+unit string like "1GB" into ("1", "GB").
// Returns empty strings if the input doesn't match the expected pattern.
func splitNumberUnit(s string) (string, string) {
	// Find the first byte that is a letter (start of unit suffix).
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return s[:i], s[i:]
		}
	}
	return "", ""
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
		// Try bare integer (bytes) first.
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err == nil {
			return n, nil
		}
		// Try concatenated number+unit (e.g. "1GB", "1.5GiB").
		numStr, unit := splitNumberUnit(fields[0])
		if numStr != "" && unit != "" {
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("parseMemoryString: cannot parse numeric part %q: %v", numStr, err)
			}
			mult, ok := memUnits[unit]
			if !ok {
				return 0, fmt.Errorf("parseMemoryString: unknown memory unit %q", unit)
			}
			return int64(num * mult), nil
		}
		return 0, fmt.Errorf("parseMemoryString: cannot parse %q as integer bytes: %v", s, err)
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
