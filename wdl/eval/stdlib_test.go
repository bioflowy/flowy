package eval

import "testing"

// TestStdlibFileIO tests file I/O standard library functions (STDLIB-01).
// Covers: read_string, read_int, read_float, read_boolean, read_lines, read_tsv,
// read_map, read_json, write_lines, write_tsv, write_map, write_json.
func TestStdlibFileIO(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestStdlibFilesystem tests filesystem standard library functions (STDLIB-02).
// Covers: glob, size, basename, dirname, sub.
func TestStdlibFilesystem(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestStdlibArray tests array standard library functions (STDLIB-03).
// Covers: length, range, flatten, transpose, zip, cross, prefix,
// suffix, quote, squote, as_pairs, as_map, keys, collect_by_key,
// select_first, select_all, defined.
func TestStdlibArray(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestStdlibString tests string standard library functions (STDLIB-04).
// Covers: sub, sep, find, matches, quote, squote.
func TestStdlibString(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestStdlibUtility tests utility standard library functions (STDLIB-05).
// Covers: defined, select_first, select_all, basename, flatten.
func TestStdlibUtility(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestStdlibNumeric tests numeric standard library functions (STDLIB-06).
// Covers: floor, ceil, round, min, max.
func TestStdlibNumeric(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestRoundHalfUp tests the round() function's half-up semantics (STDLIB-06).
// Spec requirement: round(-2.5) must return -2 (round half away from zero), not -3.
func TestRoundHalfUp(t *testing.T) {
	t.Skip("not yet implemented")
}
