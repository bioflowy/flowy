package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripLeadingWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		result   string
	}{
		{
			name:     "no leading whitespace",
			input:    "line1\nline2\nline3",
			expected: 0,
			result:   "line1\nline2\nline3",
		},
		{
			name:     "consistent leading whitespace",
			input:    "    line1\n    line2\n    line3",
			expected: 4,
			result:   "line1\nline2\nline3",
		},
		{
			name:     "mixed leading whitespace",
			input:    "  line1\n    line2\n  line3",
			expected: 2,
			result:   "line1\n  line2\nline3",
		},
		{
			name:     "empty lines",
			input:    "  line1\n\n  line3",
			expected: 2,
			result:   "line1\n\nline3",
		},
		{
			name:     "single line",
			input:    "    single line",
			expected: 4,
			result:   "single line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stripped, result := StripLeadingWhitespace(tt.input)
			if stripped != tt.expected {
				t.Errorf("Expected stripped count %d, got %d", tt.expected, stripped)
			}
			if result != tt.result {
				t.Errorf("Expected result %q, got %q", tt.result, result)
			}
		})
	}
}

func TestAdjacencyMatrix(t *testing.T) {
	adj := NewAdjM[string]()

	// Add nodes
	adj.AddNode("A")
	adj.AddNode("B")
	adj.AddNode("C")

	// Initially all nodes should be unconstrained
	unconstrained := adj.GetUnconstrained()
	if len(unconstrained) != 3 {
		t.Errorf("Expected 3 unconstrained nodes, got %d", len(unconstrained))
	}

	// Add edges A -> B, B -> C
	adj.AddEdge("A", "B")
	adj.AddEdge("B", "C")

	// Now only A should be unconstrained
	unconstrained = adj.GetUnconstrained()
	if len(unconstrained) != 1 || unconstrained[0] != "A" {
		t.Errorf("Expected only A to be unconstrained, got %v", unconstrained)
	}

	// Test sinks
	sinks := adj.GetSinks("A")
	if len(sinks) != 1 || sinks[0] != "B" {
		t.Errorf("Expected A to have sink B, got %v", sinks)
	}

	// Test sources
	sources := adj.GetSources("B")
	if len(sources) != 1 || sources[0] != "A" {
		t.Errorf("Expected B to have source A, got %v", sources)
	}

	// Test removing edge
	adj.RemoveEdge("A", "B")
	unconstrained = adj.GetUnconstrained()
	expectedUnconstrained := map[string]bool{"A": true, "B": true}
	if len(unconstrained) != 2 {
		t.Errorf("Expected 2 unconstrained nodes after removing edge, got %d", len(unconstrained))
	}
	for _, node := range unconstrained {
		if !expectedUnconstrained[node] {
			t.Errorf("Unexpected unconstrained node: %s", node)
		}
	}
}

func TestTopologicalSort(t *testing.T) {
	adj := NewAdjM[string]()

	// Create a simple DAG: A -> B -> C, D (independent)
	adj.AddNode("A")
	adj.AddNode("B")
	adj.AddNode("C")
	adj.AddNode("D")
	adj.AddEdge("A", "B")
	adj.AddEdge("B", "C")

	sorted := TopSort(adj)

	if len(sorted) != 4 {
		t.Errorf("Expected 4 nodes in sorted order, got %d", len(sorted))
	}

	// Verify topological order
	posA, posB, posC := -1, -1, -1
	for i, node := range sorted {
		switch node {
		case "A":
			posA = i
		case "B":
			posB = i
		case "C":
			posC = i
		}
	}

	if posA >= posB || posB >= posC {
		t.Errorf("Expected A < B < C in topological order, got positions A:%d B:%d C:%d", posA, posB, posC)
	}
}

func TestTopologicalSortWithCycle(t *testing.T) {
	adj := NewAdjM[string]()

	// Create a cycle: A -> B -> C -> A
	adj.AddNode("A")
	adj.AddNode("B")
	adj.AddNode("C")
	adj.AddEdge("A", "B")
	adj.AddEdge("B", "C")
	adj.AddEdge("C", "A")

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected TopSort to panic on cycle, but it didn't")
		}
	}()

	TopSort(adj)
}

func TestWriteAtomic(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "test.txt")
	content := "test content"

	err := WriteAtomic(content, filename)
	if err != nil {
		t.Fatalf("WriteAtomic failed: %v", err)
	}

	// Check if file exists and has correct content
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	expected := content + "\n"
	if string(data) != expected {
		t.Errorf("Expected content %q, got %q", expected, string(data))
	}
}

func TestWriteAtomicCustomEnd(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "test.txt")
	content := "test content"
	customEnd := ""

	err := WriteAtomicWithEnd(content, filename, customEnd)
	if err != nil {
		t.Fatalf("WriteAtomicWithEnd failed: %v", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content %q, got %q", content, string(data))
	}
}

func TestSymlinkForce(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "source.txt")
	dstLink := filepath.Join(tempDir, "link.txt")

	// Create source file
	err := os.WriteFile(srcFile, []byte("source content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create symlink
	err = SymlinkForce(srcFile, dstLink, false)
	if err != nil {
		t.Fatalf("SymlinkForce failed: %v", err)
	}

	// Check if symlink exists and points to correct file
	target, err := os.Readlink(dstLink)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	if target != srcFile {
		t.Errorf("Expected symlink to point to %q, got %q", srcFile, target)
	}

	// Test replacing existing symlink
	newSrc := filepath.Join(tempDir, "newsource.txt")
	err = os.WriteFile(newSrc, []byte("new content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create new source file: %v", err)
	}

	err = SymlinkForce(newSrc, dstLink, false)
	if err != nil {
		t.Fatalf("SymlinkForce replacement failed: %v", err)
	}

	target, err = os.Readlink(dstLink)
	if err != nil {
		t.Fatalf("Failed to read replaced symlink: %v", err)
	}

	if target != newSrc {
		t.Errorf("Expected replaced symlink to point to %q, got %q", newSrc, target)
	}
}

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"100", 100, false},
		{"1024", 1024, false},
		{"1K", 1024, false},
		{"1KB", 1024, false},
		{"1M", 1024 * 1024, false},
		{"1MB", 1024 * 1024, false},
		{"1G", 1024 * 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"1.5K", 1536, false},
		{"invalid", 0, true},
		{"-1", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseByteSize(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input %q, but got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d for input %q, got %d", tt.expected, tt.input, result)
				}
			}
		})
	}
}

func TestSplitAll(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"/usr/local/bin", []string{"/", "usr", "local", "bin"}},
		{"usr/local/bin", []string{"usr", "local", "bin"}},
		{"/", []string{"/"}},
		{"", []string{}},
		{"single", []string{"single"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SplitAll(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d parts, got %d", len(tt.expected), len(result))
				return
			}
			for i, part := range result {
				if part != tt.expected[i] {
					t.Errorf("Expected part %d to be %q, got %q", i, tt.expected[i], part)
				}
			}
		})
	}
}

func TestPathReallyWithin(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	testFile := filepath.Join(subDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test file within directory
	if !PathReallyWithin(testFile, tempDir) {
		t.Error("Expected test file to be within temp directory")
	}

	// Test file not within directory
	otherDir := filepath.Join(os.TempDir(), "other")
	if PathReallyWithin(testFile, otherDir) {
		t.Error("Expected test file not to be within other directory")
	}

	// Test with same path
	if !PathReallyWithin(tempDir, tempDir) {
		t.Error("Expected directory to be within itself")
	}
}