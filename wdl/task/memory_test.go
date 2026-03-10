package task_test

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/task"
)

func TestParseMemoryString(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		// Binary units (IEC)
		{"2 GiB", 2 * 1024 * 1024 * 1024, false},
		{"1 GiB", 1 * 1024 * 1024 * 1024, false},
		{"512 MiB", 512 * 1024 * 1024, false},
		{"1 TiB", 1024 * 1024 * 1024 * 1024, false},
		{"1 KiB", 1024, false},
		// Decimal units (SI)
		{"512 MB", 512 * 1000 * 1000, false},
		{"1 GB", 1 * 1000 * 1000 * 1000, false},
		{"1 TB", 1 * 1000 * 1000 * 1000 * 1000, false},
		{"1 kB", 1 * 1000, false},
		// Byte units
		{"1024 B", 1024, false},
		{"100 bytes", 100, false},
		// Bare integer (bytes)
		{"1024", 1024, false},
		{"65536", 65536, false},
		// Float values
		{"1.5 GiB", int64(1.5 * 1024 * 1024 * 1024), false},
		// Invalid
		{"invalid", 0, true},
		{"", 0, true},
		{"GiB", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := task.ParseMemoryString(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseMemoryString(%q) expected error, got %d", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseMemoryString(%q) unexpected error: %v", tc.input, err)
				return
			}
			if got != tc.want {
				t.Errorf("ParseMemoryString(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}
